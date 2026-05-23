// backfill_amazon_names resolves Amazon product titles from the SNAP metadata
// file and writes them into the `metadata` JSONB column of existing items.
//
// It re-streams the same reviews file used during ETL to recompute item_ids
// deterministically (same namespace + same reviewText = same UUID), then joins
// against the metadata file on ASIN to get the product title.
//
// No re-embedding. Pure data backfill — safe to run alongside the ETL worker.
//
// Usage (inside the ETL container):
//
//	go run ./cmd/backfill_amazon_names
//
// Env vars (all optional):
//
//	DB_URL        postgres://...   defaults to localhost:5432
//	META_URL      override metadata file URL
//	REVIEWS_URL   override reviews file URL
//	DRY_RUN       set to "true" to print matches without writing
package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMetaURL    = "https://snap.stanford.edu/data/amazon/productGraph/categoryFiles/meta_Electronics.json.gz"
	defaultReviewsURL = "https://snap.stanford.edu/data/amazon/productGraph/categoryFiles/reviews_Electronics.json.gz"
	flushEvery        = 500
	logEvery          = 10_000
)

// itemNS must match ingest/id.go exactly — same namespace = same IDs.
var itemNS = uuid.MustParse("3f7a9c2e-4b81-4d6f-9e32-a1b5c8d70f4e")

// SNAP metadata uses Python-literal dicts (single quotes) rather than valid JSON.
// These regexes extract asin and title from either format.
var (
	// ASIN is always alphanumeric — safe to match greedily up to closing quote.
	metaAsinRe = regexp.MustCompile(`['"]asin['"]\s*:\s*u?'([A-Z0-9]+)'`)
	// Title may use single or double quotes as outer delimiter.
	metaTitleRe = regexp.MustCompile(`['"]title['"]\s*:\s*u?(?:'([^']+)'|"([^"]+)")`)
)

func deterministicID(domain, text string) string {
	return uuid.NewSHA1(itemNS, []byte(domain+"\x00"+text)).String()
}

func main() {
	ctx := context.Background()

	dbURL := envOr("DB_URL", "postgres://postgres:postgres@db:5432/postgres?sslmode=disable")
	metaURL := envOr("META_URL", defaultMetaURL)
	reviewsURL := envOr("REVIEWS_URL", defaultReviewsURL)
	dryRun := os.Getenv("DRY_RUN") == "true"

	if dryRun {
		log.Println("DRY RUN — no writes will be made")
	}

	// 1. Load metadata: asin → title
	log.Printf("Downloading product metadata from %s ...", metaURL)
	asinTitle, err := loadMetadata(metaURL)
	if err != nil {
		log.Fatalf("load metadata: %v", err)
	}
	log.Printf("Loaded %d product titles", len(asinTitle))
	if len(asinTitle) == 0 {
		log.Fatalf("metadata file returned 0 titles — aborting to prevent empty backfill")
	}

	// 2. Connect to DB
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	// 3. Verify sample IDs match before writing anything.
	// Streams the first ~500 reviews, computes their item_ids, and checks that
	// at least 10 exist in the DB. This confirms our ID formula matches the ETL.
	// Note: we do NOT recompute IDs from search_text in the DB because the ETL
	// truncates search_text to 1000 chars after computing the ID, so they differ.
	log.Println("Verifying ID computation against DB sample...")
	if err := verifyIDs(ctx, pool, reviewsURL); err != nil {
		log.Fatalf("verification failed — aborting: %v", err)
	}
	log.Println("Verification passed — IDs match. Proceeding with backfill.")

	// 4. Stream reviews and batch-update names
	log.Printf("Streaming reviews from %s ...", reviewsURL)
	updated, skipped, err := backfill(ctx, pool, reviewsURL, asinTitle, dryRun)
	if err != nil {
		log.Fatalf("backfill: %v", err)
	}
	log.Printf("Done. DB rows updated: %d | Skipped (no title in metadata): %d", updated, skipped)
}

// loadMetadata streams the gzip'd metadata file and builds asin → title map.
// Handles both valid JSON lines and Python-literal dicts (SNAP legacy format).
func loadMetadata(url string) (map[string]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata download returned HTTP %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gzip metadata: %w", err)
	}
	defer gz.Close()

	m := make(map[string]string)
	scanner := bufio.NewScanner(gz)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024) // metadata lines can be large

	for scanner.Scan() {
		line := scanner.Bytes()

		// Try valid JSON first (newer dataset releases).
		var rec struct {
			ASIN  string `json:"asin"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal(line, &rec); err == nil {
			if rec.ASIN != "" && rec.Title != "" {
				m[rec.ASIN] = rec.Title
			}
			continue
		}

		// Fall back to regex for Python-literal dicts (SNAP legacy format).
		asinMatch := metaAsinRe.FindSubmatch(line)
		titleMatch := metaTitleRe.FindSubmatch(line)
		if asinMatch == nil || titleMatch == nil {
			continue
		}
		asin := string(asinMatch[1])
		// titleMatch[1] = single-quoted value, titleMatch[2] = double-quoted value
		title := string(titleMatch[1])
		if title == "" {
			title = string(titleMatch[2])
		}
		if asin != "" && title != "" {
			m[asin] = title
		}
	}
	return m, scanner.Err()
}

// verifyIDs streams the first ~500 reviews, computes their item_ids using the
// same deterministic formula as the ETL, and confirms that at least 10 of them
// exist in the DB. Aborts if fewer than 10 match (wrong namespace or formula).
func verifyIDs(ctx context.Context, pool *pgxpool.Pool, reviewsURL string) error {
	resp, err := http.Get(reviewsURL)
	if err != nil {
		return fmt.Errorf("download reviews for verification: %w", err)
	}
	defer resp.Body.Close()

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip reviews: %w", err)
	}
	defer gz.Close()

	scanner := bufio.NewScanner(gz)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var sampleIDs []string
	for scanner.Scan() && len(sampleIDs) < 500 {
		var rec struct {
			ReviewText string `json:"reviewText"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil || rec.ReviewText == "" {
			continue
		}
		sampleIDs = append(sampleIDs, deterministicID("amazon", rec.ReviewText))
	}

	if len(sampleIDs) < 20 {
		return fmt.Errorf("too few reviews parsed for verification (%d)", len(sampleIDs))
	}

	// Check a sample of 20 IDs against the DB.
	found := 0
	for _, id := range sampleIDs[:20] {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM items WHERE item_id = $1 AND domain = 'amazon')`, id,
		).Scan(&exists); err != nil {
			return fmt.Errorf("db check: %w", err)
		}
		if exists {
			found++
		}
	}

	log.Printf("Verification: %d/20 sampled review IDs found in DB", found)
	if found < 10 {
		return fmt.Errorf("only %d/20 sampled IDs exist in DB — ID formula mismatch or wrong DB", found)
	}
	return nil
}

type pair struct {
	itemID string
	name   string
}

func backfill(ctx context.Context, pool *pgxpool.Pool, reviewsURL string, asinTitle map[string]string, dryRun bool) (int, int, error) {
	resp, err := http.Get(reviewsURL)
	if err != nil {
		return 0, 0, fmt.Errorf("download reviews: %w", err)
	}
	defer resp.Body.Close()

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("gzip reviews: %w", err)
	}
	defer gz.Close()

	scanner := bufio.NewScanner(gz)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var batch []pair
	updated, skipped := 0, 0

	flush := func() error {
		if dryRun {
			for _, p := range batch {
				fmt.Printf("DRY_RUN: item_id=%s name=%.80s\n", p.itemID, p.name)
			}
			updated += len(batch)
			batch = batch[:0]
			return nil
		}
		n, err := flushBatch(ctx, pool, batch)
		if err != nil {
			return err
		}
		updated += n
		batch = batch[:0]
		return nil
	}

	for scanner.Scan() {
		var rec struct {
			ASIN       string `json:"asin"`
			ReviewText string `json:"reviewText"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		if rec.ReviewText == "" {
			continue
		}
		title, ok := asinTitle[rec.ASIN]
		if !ok || title == "" {
			skipped++
			continue
		}

		batch = append(batch, pair{
			itemID: deterministicID("amazon", rec.ReviewText),
			name:   title,
		})

		if len(batch) >= flushEvery {
			if err := flush(); err != nil {
				return updated, skipped, err
			}
			if updated%logEvery == 0 && updated > 0 {
				log.Printf("Updated %d rows so far...", updated)
			}
		}
	}

	if len(batch) > 0 {
		if err := flush(); err != nil {
			return updated, skipped, err
		}
	}

	return updated, skipped, scanner.Err()
}

func flushBatch(ctx context.Context, pool *pgxpool.Pool, batch []pair) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	b := &pgx.Batch{}
	for _, p := range batch {
		b.Queue(
			`UPDATE items
			    SET metadata = metadata || jsonb_build_object('name', $1::text)
			  WHERE item_id = $2
			    AND metadata->>'name' IS NULL`,
			p.name, p.itemID,
		)
	}

	br := tx.SendBatch(ctx, b)
	count := 0
	for range batch {
		cmd, err := br.Exec()
		if err != nil {
			br.Close()
			return 0, err
		}
		count += int(cmd.RowsAffected())
	}
	br.Close()

	return count, tx.Commit(ctx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
