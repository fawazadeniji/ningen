package rag

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Result is a single item retrieved from the vector store.
type Result struct {
	ItemID     string
	Domain     string
	SearchText string
	Score      float64
}

// VectorStore wraps the Postgres connection pool and exposes
// pgvector-powered retrieval operations.
type VectorStore struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *VectorStore {
	return &VectorStore{pool: pool}
}

// Search performs an HNSW cosine similarity search against the items table.
// It returns up to limit items ordered by ascending distance (most similar first).
// If domains is non-empty, results are filtered to those domains only.
func (vs *VectorStore) Search(ctx context.Context, embedding []float32, limit int, domains []string) ([]Result, error) {
	vecLiteral := formatVector(embedding)

	var (
		query string
		args  []any
	)

	if len(domains) > 0 {
		placeholders := make([]string, len(domains))
		args = []any{vecLiteral, limit}
		for i, d := range domains {
			args = append(args, d)
			placeholders[i] = fmt.Sprintf("$%d", i+3)
		}
		query = fmt.Sprintf(`
			SELECT item_id, domain, search_text,
			       (embedding <=> $1::vector) AS distance
			FROM   items
			WHERE  domain IN (%s)
			ORDER  BY distance
			LIMIT  $2`,
			strings.Join(placeholders, ", "),
		)
	} else {
		args = []any{vecLiteral, limit}
		query = `
			SELECT item_id, domain, search_text,
			       (embedding <=> $1::vector) AS distance
			FROM   items
			ORDER  BY distance
			LIMIT  $2`
	}

	rows, err := vs.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("vector search query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.ItemID, &r.Domain, &r.SearchText, &r.Score); err != nil {
			return nil, fmt.Errorf("vector search scan: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vector search rows: %w", err)
	}

	return results, nil
}

// SearchByText performs a cold-start fallback: full-text ILIKE search
// when no meaningful embedding is available (e.g. new user, no history).
func (vs *VectorStore) SearchByText(ctx context.Context, query string, limit int) ([]Result, error) {
	rows, err := vs.pool.Query(ctx, `
		SELECT item_id, domain, search_text, 0.0 AS distance
		FROM   items
		WHERE  search_text ILIKE $1
		LIMIT  $2`,
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("text search query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.ItemID, &r.Domain, &r.SearchText, &r.Score); err != nil {
			return nil, fmt.Errorf("text search scan: %w", err)
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// SearchByVectors runs one HNSW query per embedding vector and unions the results.
// Results are deduplicated by item_id, keeping the best (lowest) cosine distance per item.
// The returned slice is sorted ascending by score and capped at limit.
func (vs *VectorStore) SearchByVectors(ctx context.Context, vecs [][]float32, limit int) ([]Result, error) {
	seen := make(map[string]Result)
	var lastErr error
	for _, vec := range vecs {
		results, err := vs.Search(ctx, vec, limit, nil)
		if err != nil {
			lastErr = err
			continue
		}
		for _, r := range results {
			if existing, ok := seen[r.ItemID]; !ok || r.Score < existing.Score {
				seen[r.ItemID] = r
			}
		}
	}
	if len(seen) == 0 && lastErr != nil {
		return nil, lastErr
	}

	merged := make([]Result, 0, len(seen))
	for _, r := range seen {
		merged = append(merged, r)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score < merged[j].Score
	})
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// formatVector converts a float32 slice to the Postgres vector literal '[a,b,c,...]'.
func formatVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
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
