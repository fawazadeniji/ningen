package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"ningen/domain"
	"ningen/embed"
	"ningen/ingest"
	"ningen/store"
)

const (
	batchSize   = 5_000
	workerCount = 10
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("Received termination signal, shutting down...")
		cancel()
	}()

	if err := run(ctx); err != nil {
		log.Fatalf("ETL Pipeline failed: %v", err)
	}
}

func run(ctx context.Context) error {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}

	targetItemCount := envInt("TARGET_ITEM_COUNT", 100_000)

	log.Println("Initializing Postgres storage...")
	db, err := store.NewPostgresStore(ctx, dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Init(ctx); err != nil {
		return err
	}

	// Idempotency Check
	count, err := db.Count(ctx)
	if err != nil {
		return err
	}

	if count >= targetItemCount {
		log.Printf("Data already ingested (count: %d). Skipping ETL pipeline to save resources.", count)
		os.Exit(0)
	}
	log.Printf("Current item count is %d. Starting ETL process...", count)

	embedderURL := os.Getenv("EMBEDDER_URL")
	if embedderURL == "" {
		embedderURL = "http://embedder:8000"
	}

	log.Printf("Connecting to embedder sidecar at %s...", embedderURL)
	embedder := embed.NewSidecarEmbedder(embedderURL)

	type limitedSource struct {
		src   ingest.Source
		limit int // max items to take from this source; 0 = unlimited
	}

	// Per-source limits ensure cross-domain diversity regardless of target size.
	// Ratios: 50% yelp (food/restaurants), 25% amazon electronics (products), 25% amazon books.
	sources := []limitedSource{
		{ingest.NewYelpJsonl("https://huggingface.co/datasets/SetFit/yelp_review_full/resolve/main/train.jsonl"), targetItemCount * 50 / 100},
		{ingest.NewAmazonGzJsonl("https://snap.stanford.edu/data/amazon/productGraph/categoryFiles/reviews_Electronics.json.gz"), targetItemCount * 25 / 100},
		{ingest.NewAmazonGzJsonl("https://snap.stanford.edu/data/amazon/productGraph/categoryFiles/reviews_Books.json.gz"), targetItemCount * 25 / 100},
	}

	rawItems := make(chan domain.Item, 1000)
	embeddedItems := make(chan domain.Item, 1000)

	var wg sync.WaitGroup

	// 1. Writer Goroutine
	errChan := make(chan error, 1)
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		batch := make([]domain.Item, 0, batchSize)
		totalIngested := 0

		for item := range embeddedItems {
			batch = append(batch, item)
			if len(batch) >= batchSize {
				log.Printf("Bulk inserting batch of %d items...", len(batch))
				if err := db.BulkInsert(ctx, batch); err != nil {
					errChan <- fmt.Errorf("bulk insert: %w", err)
					return
				}
				totalIngested += len(batch)
				log.Printf("Total items ingested so far: %d", totalIngested)
				batch = batch[:0]
			}
		}

		// Insert remaining
		if len(batch) > 0 {
			log.Printf("Bulk inserting final batch of %d items...", len(batch))
			if err := db.BulkInsert(ctx, batch); err != nil {
				errChan <- fmt.Errorf("final bulk insert: %w", err)
				return
			}
			totalIngested += len(batch)
		}
		log.Printf("Writer finished. Total ingested: %d", totalIngested)
	}()

	// 2. Worker Pool for Embedding
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for item := range rawItems {
				item.SearchText = truncateText(item.SearchText, 1000)
				vec, err := embedder.Embed(ctx, item.SearchText)
				if err != nil || len(vec) == 0 {
					log.Printf("Worker %d: skipping item %s: embed returned empty (err=%v)", workerID, item.ID, err)
					continue
				}
				item.Embedding = vec
				
				select {
				case embeddedItems <- item:
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}

	// 3. Reader Routine (Sequential across sources)
	// Skip logic removed: deterministic IDs + ON CONFLICT DO NOTHING handle dedup.
	// Limits are pre-checked before sending so the channel buffer never causes overshoot.
	go func() {
		defer close(rawItems)
		itemsCollected := 0

		for _, ls := range sources {
			log.Printf("Streaming from source: %T (limit: %d)", ls.src, ls.limit)
			sourceCtx, sourceCancel := context.WithCancel(ctx)
			sourceChan := make(chan domain.Item, 100)
			sourceErrChan := make(chan error, 1)
			go func() {
				sourceErrChan <- ls.src.Stream(sourceCtx, sourceChan)
				close(sourceChan)
			}()

			sourceCollected := 0
		streamLoop:
			for {
				select {
				case <-ctx.Done():
					sourceCancel()
					return
				case err := <-sourceErrChan:
					if err != nil {
						log.Printf("Source %T returned error: %v", ls.src, err)
					}
					break streamLoop
				case item, ok := <-sourceChan:
					if !ok {
						break streamLoop
					}

					// Pre-check limits before sending — avoids channel-buffer overshoot.
					if ls.limit > 0 && sourceCollected >= ls.limit {
						log.Printf("Source %T hit per-source limit (%d). Moving to next source.", ls.src, ls.limit)
						sourceCancel()
						break streamLoop
					}

					select {
					case rawItems <- item:
						itemsCollected++
						sourceCollected++
						if itemsCollected%1000 == 0 {
							log.Printf("Read %d new items so far", itemsCollected)
						}
					case <-ctx.Done():
						sourceCancel()
						return
					}
				}
			}
			sourceCancel()
		}
	}()

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(embeddedItems)
	}()

	// Wait for writer to finish or context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	case <-writerDone:
	}

	// 4. Index Creation
	log.Println("Creating HNSW Index...")
	if err := db.CreateIndex(ctx); err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	log.Println("HNSW Index created successfully.")

	log.Println("ETL Pipeline completed successfully.")
	return nil
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

// truncateText cuts s to at most maxBytes bytes on a valid UTF-8 boundary.
func truncateText(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	// Walk back from maxBytes to find a valid rune boundary.
	for maxBytes > 0 && (s[maxBytes]&0xC0) == 0x80 {
		maxBytes--
	}
	return s[:maxBytes]
}
