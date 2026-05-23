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

	// 2. Connect to DB
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	// 3. Verify sample IDs match before writing anything
	log.Println("Verifying ID computation against DB sample...")
	if err := verifyIDs(ctx, pool, reviewsURL, asinTitle); err != nil {
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
func loadMetadata(url string) (map[string]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download metadata: %w", err)
	}
	defer resp.Body.Close()

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
		var rec struct {
			ASIN  string `json:"asin"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		if rec.ASIN != "" && rec.Title != "" {
			m[rec.ASIN] = rec.Title
		}
	}
	return m, scanner.Err()
}

// verifyIDs takes the first 10 reviews that have a DB match and confirms
// the computed item_id equals what's stored. Aborts if any mismatch.
func verifyIDs(ctx context.Context, pool *pgxpool.Pool, reviewsURL string, asinTitle map[string]string) error {
	// Pull 20 real item_ids from DB to check against
	rows, err := pool.Query(ctx, `SELECT item_id, search_text FROM items WHERE domain='amazon' LIMIT 20`)
	if err != nil {
		return err
	}
	defer rows.Close()

	dbItems := make(map[string]string) // search_text → item_id
	for rows.Next() {
		var id, text string
		if err := rows.Scan(&id, &text); err != nil {
			continue
		}
		dbItems[text] = id
	}

	mismatches := 0
	for text, dbID := range dbItems {
		computed := deterministicID("amazon", text)
		if computed != dbID {
			log.Printf("MISMATCH: db=%s computed=%s text=%.60s...", dbID, computed, text)
			mismatches++
		}
	}
	if mismatches > 0 {
		return fmt.Errorf("%d ID mismatches detected", mismatches)
	}
	log.Printf("All %d sampled IDs match", len(dbItems))
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
