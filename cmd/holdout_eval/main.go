// holdout_eval streams items from the END of each source dataset (never ingested),
// embeds them via the sidecar, finds ground-truth DB neighbors by cosine distance,
// queries the recommendation API, and reports NDCG@10, Hit@10, and MRR.
//
// Skip counts are derived automatically from live DB row counts per domain.
//
// Usage:
//
//	go run ./cmd/holdout_eval
//
// Environment variables (all optional):
//
//	DB_URL           postgres://postgres:postgres@localhost:5434/postgres?sslmode=disable
//	EMBEDDER_URL     http://localhost:8001
//	API_URL          http://localhost:8080
//	PROVIDER         gemini
//	SEEDS_PER_DOMAIN 20
//	GT_THRESHOLD     0.45   (cosine distance — lower = stricter ground truth)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"ningen/domain"
	"ningen/embed"
	"ningen/ingest"

	pgvectorpgx "github.com/pgvector/pgvector-go/pgx"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	yelpTestURL  = "https://huggingface.co/datasets/SetFit/yelp_review_full/resolve/main/test.jsonl"
	amazonURL    = "https://snap.stanford.edu/data/amazon/productGraph/categoryFiles/reviews_Electronics.json.gz"
	goodreadsURL = "https://huggingface.co/datasets/Pauleera/Goodreads-Book-Reviews/resolve/main/reviews_reduced.csv"
	topK         = 10
)

func main() {
	ctx := context.Background()

	dbURL      := envOr("DB_URL",           "postgres://postgres:postgres@localhost:5434/postgres?sslmode=disable")
	embedURL   := envOr("EMBEDDER_URL",     "http://localhost:8001")
	apiURL     := envOr("API_URL",          "http://localhost:8080")
	provider   := envOr("PROVIDER",         "gemini")
	nSeeds     := envInt("SEEDS_PER_DOMAIN", 20)
	gtThresh   := envFloat("GT_THRESHOLD",  0.45)

	log.Printf("holdout_eval  provider=%s  seeds/domain=%d  gt_threshold=%.2f", provider, nSeeds, gtThresh)

	// Verify API health
	if err := checkHealth(apiURL); err != nil {
		log.Fatalf("API not healthy: %v", err)
	}

	// Connect to DB
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("parse db config: %v", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgvectorpgx.RegisterTypes(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	// Domain counts → skip counts
	counts, err := domainCounts(ctx, pool)
	if err != nil {
		log.Fatalf("domain counts: %v", err)
	}
	log.Printf("DB domain counts: yelp=%d  amazon=%d  goodreads=%d",
		counts["yelp"], counts["amazon"], counts["goodreads"])

	embedder := embed.NewSidecarEmbedder(embedURL)

	var allResults []seedResult

	type domainJob struct {
		name   string
		src    ingest.Source
		skip   int
		query  string
		persona string
	}

	jobs := []domainJob{
		{
			name:    "yelp",
			src:     ingest.NewYelpJsonl(yelpTestURL), // test split — different file from ETL
			skip:    0,                                // no skip: test.jsonl is a fresh file
			query:   "Recommend a restaurant or food experience I'd love.",
			persona: "A food enthusiast and regular restaurant-goer",
		},
		{
			name:    "amazon",
			src:     ingest.NewAmazonGzJsonl(amazonURL),
			skip:    counts["amazon"],
			query:   "Recommend a product I would likely enjoy buying.",
			persona: "A frequent online shopper who buys tech and electronics",
		},
		{
			name:    "goodreads",
			src:     ingest.NewGoodreadsCSV(goodreadsURL),
			skip:    counts["goodreads"],
			query:   "Recommend a book I would love to read.",
			persona: "An avid reader who reads across many genres",
		},
	}

	for _, job := range jobs {
		fmt.Printf("\n%s\n", strings.Repeat("═", 60))
		fmt.Printf("DOMAIN: %s  (skipping %d ingested rows)\n", strings.ToUpper(job.name), job.skip)

		texts := collectHoldout(ctx, job.src, job.skip, nSeeds)
		if len(texts) == 0 {
			fmt.Printf("  WARNING: no holdout items retrieved for %s\n", job.name)
			continue
		}
		fmt.Printf("  Collected %d holdout items\n", len(texts))

		results := evaluateDomain(ctx, pool, embedder, apiURL, provider, texts, job.query, job.persona, gtThresh)
		printDomainReport(job.name, results)

		allResults = append(allResults, results...)
	}

	fmt.Printf("\n%s\n", strings.Repeat("═", 60))
	printOverallReport(allResults)
}

// collectHoldout streams from src, discards `skip` items, then returns the next `n` texts.
func collectHoldout(ctx context.Context, src ingest.Source, skip, n int) []string {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan domain.Item, 200)
	go func() {
		if err := src.Stream(streamCtx, ch); err != nil && streamCtx.Err() == nil {
			log.Printf("  source error: %v", err)
		}
		close(ch)
	}()

	var texts []string
	skipped := 0

	for item := range ch {
		if skipped < skip {
			skipped++
			if skipped%10_000 == 0 {
				log.Printf("  skipped %d/%d...", skipped, skip)
			}
			continue
		}
		texts = append(texts, item.SearchText)
		if len(texts) >= n {
			cancel() // stop streaming
			break
		}
	}
	// drain remaining items after cancel
	for range ch {
	}
	return texts
}

// evaluateDomain runs the full eval loop for one domain's holdout texts.
func evaluateDomain(
	ctx context.Context,
	pool *pgxpool.Pool,
	embedder embed.Embedder,
	apiURL, provider string,
	texts []string,
	query, personaBase string,
	gtThresh float64,
) []seedResult {
	var results []seedResult

	for i, text := range texts {
		excerpt := text
		if len(excerpt) > 80 {
			excerpt = excerpt[:80] + "..."
		}
		fmt.Printf("\n  [%d/%d] %s\n", i+1, len(texts), excerpt)

		// 1. Embed the holdout item
		vec, err := embedder.Embed(ctx, text)
		if err != nil || len(vec) == 0 {
			fmt.Println("    → skip: embed failed")
			continue
		}

		// 2. Find ground-truth neighbors in DB
		gt, err := findGT(ctx, pool, vec, gtThresh)
		if err != nil || len(gt) == 0 {
			fmt.Printf("    → skip: no DB neighbors within dist=%.2f\n", gtThresh)
			continue
		}
		fmt.Printf("    gt_size=%-4d ", len(gt))

		// 3. Build persona + call API (with requires_input follow-through)
		persona := fmt.Sprintf("%s. Sample of what I enjoy: %q", personaBase, truncate(text, 300))
		ranked, err := callAPIWithFollowThrough(apiURL, provider, persona, query, text)
		if err != nil || len(ranked) == 0 {
			fmt.Println("→ API returned nothing")
			results = append(results, seedResult{gtSize: len(gt)})
			continue
		}

		n  := ndcgAtK(ranked, gt, topK)
		h  := hitAtK(ranked, gt, topK)
		rr := mrrScore(ranked, gt)
		fmt.Printf("NDCG@%d=%.3f  Hit@%d=%d  MRR=%.3f\n", topK, n, topK, int(h), rr)

		results = append(results, seedResult{ndcg: n, hit: h, mrr: rr, gtSize: len(gt)})
	}

	return results
}

// findGT queries DB for items within cosine distance threshold of the embedding.
func findGT(ctx context.Context, pool *pgxpool.Pool, vec []float32, threshold float64) (map[string]float64, error) {
	lit := formatVec(vec)
	rows, err := pool.Query(ctx, `
		SELECT item_id, (embedding <=> $1::vector) AS dist
		FROM   items
		WHERE  (embedding <=> $1::vector) < $2
		ORDER  BY dist
		LIMIT  100
	`, lit, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gt := make(map[string]float64)
	for rows.Next() {
		var id string
		var dist float64
		if err := rows.Scan(&id, &dist); err != nil {
			continue
		}
		gt[id] = dist
	}
	return gt, rows.Err()
}

// callAPIWithFollowThrough calls POST /recommend and handles requires_input gracefully.
// When the API asks a clarifying question, it synthesizes an answer from the seed text
// and fires a second request — simulating a real user responding to the question.
func callAPIWithFollowThrough(apiURL, provider, persona, query, seedText string) ([]string, error) {
	history := []map[string]string{{"role": "user", "content": query}}

	ids, requiresInput, question, err := callRecommend(apiURL, provider, persona, history)
	if err != nil {
		return nil, err
	}
	if !requiresInput {
		return ids, nil
	}

	// Synthesize an answer from the seed text and retry once.
	fmt.Printf("\n      → requires_input (Q: %.80s...)\n      → sending follow-up  ", question)
	answer := fmt.Sprintf("Based on what I enjoy: %s", truncate(seedText, 200))
	history = append(history,
		map[string]string{"role": "assistant", "content": question},
		map[string]string{"role": "user", "content": answer},
	)

	ids, _, _, err = callRecommend(apiURL, provider, persona, history)
	return ids, err
}

// callRecommend sends one POST /recommend. Returns (item_ids, requiresInput, question, err).
var httpClient = &http.Client{Timeout: 120 * time.Second}

func callRecommend(apiURL, provider, persona string, history []map[string]string) ([]string, bool, string, error) {
	body, _ := json.Marshal(map[string]any{
		"user_persona": persona,
		"history":      history,
		"limit":        topK,
		"provider":     provider,
	})
	resp, err := httpClient.Post(apiURL+"/recommend", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, false, "", err
	}
	defer resp.Body.Close()

	var data struct {
		Recommendations []struct {
			ItemID string `json:"item_id"`
		} `json:"recommendations"`
		RequiresInput bool   `json:"requires_input"`
		Question      string `json:"question"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, false, "", err
	}
	if data.RequiresInput {
		return nil, true, data.Question, nil
	}
	ids := make([]string, len(data.Recommendations))
	for i, r := range data.Recommendations {
		ids[i] = r.ItemID
	}
	return ids, false, "", nil
}

// ── metrics ──────────────────────────────────────────────────────────────────

func ndcgAtK(ranked []string, gt map[string]float64, k int) float64 {
	dcg := 0.0
	for i, id := range ranked {
		if i >= k {
			break
		}
		if _, ok := gt[id]; ok {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}
	ideal := min(len(gt), k)
	idcg := 0.0
	for i := range ideal {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func hitAtK(ranked []string, gt map[string]float64, k int) float64 {
	for i, id := range ranked {
		if i >= k {
			break
		}
		if _, ok := gt[id]; ok {
			return 1.0
		}
	}
	return 0.0
}

func mrrScore(ranked []string, gt map[string]float64) float64 {
	for i, id := range ranked {
		if _, ok := gt[id]; ok {
			return 1.0 / float64(i+1)
		}
	}
	return 0.0
}

// ── reporting ─────────────────────────────────────────────────────────────────

type seedResult struct {
	ndcg   float64
	hit    float64
	mrr    float64
	gtSize int
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := mean(vals)
	sum := 0.0
	for _, v := range vals {
		sum += (v - m) * (v - m)
	}
	return math.Sqrt(sum / float64(len(vals)))
}

func printDomainReport(domain string, results []seedResult) {
	sep := strings.Repeat("─", 60)
	if len(results) == 0 {
		fmt.Printf("\n%s\n%s: no valid seeds\n", sep, strings.ToUpper(domain))
		return
	}
	ndcgs := make([]float64, len(results))
	hits  := make([]float64, len(results))
	mrrs  := make([]float64, len(results))
	gts   := make([]float64, len(results))
	for i, r := range results {
		ndcgs[i] = r.ndcg
		hits[i]  = r.hit
		mrrs[i]  = r.mrr
		gts[i]   = float64(r.gtSize)
	}
	fmt.Printf("\n%s\n%s report  (%d seeds evaluated)\n", sep, strings.ToUpper(domain), len(results))
	fmt.Printf("  NDCG@%-2d : %.4f  (std %.4f)\n", topK, mean(ndcgs), stddev(ndcgs))
	fmt.Printf("  Hit@%-2d  : %.4f\n", topK, mean(hits))
	fmt.Printf("  MRR     : %.4f\n", mean(mrrs))
	fmt.Printf("  Avg GT  : %.1f neighbors/seed\n", mean(gts))
}

func printOverallReport(results []seedResult) {
	sep := strings.Repeat("═", 60)
	if len(results) == 0 {
		fmt.Println("No results to report.")
		return
	}
	ndcgs := make([]float64, len(results))
	hits  := make([]float64, len(results))
	mrrs  := make([]float64, len(results))
	for i, r := range results {
		ndcgs[i] = r.ndcg
		hits[i]  = r.hit
		mrrs[i]  = r.mrr
	}
	fmt.Printf("%s\nOVERALL  (%d seeds across all domains)\n", sep, len(results))
	fmt.Printf("  NDCG@%-2d : %.4f\n", topK, mean(ndcgs))
	fmt.Printf("  Hit@%-2d  : %.4f\n", topK, mean(hits))
	fmt.Printf("  MRR     : %.4f\n", mean(mrrs))
	fmt.Printf("%s\n", sep)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func domainCounts(ctx context.Context, pool *pgxpool.Pool) (map[string]int, error) {
	rows, err := pool.Query(ctx, `SELECT domain, COUNT(*) FROM items GROUP BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]int)
	for rows.Next() {
		var d string
		var c int
		if err := rows.Scan(&d, &c); err != nil {
			continue
		}
		m[d] = c
	}
	return m, rows.Err()
}

func formatVec(v []float32) string {
	sb := strings.Builder{}
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", f)
	}
	sb.WriteByte(']')
	return sb.String()
}

func checkHealth(apiURL string) error {
	resp, err := http.Get(apiURL + "/health")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscan(v, &n); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		var f float64
		if _, err := fmt.Sscan(v, &f); err == nil && f > 0 {
			return f
		}
	}
	return fallback
}

