package store

import (
	"context"
	"fmt"
	"ningen/domain"

	pgvector "github.com/pgvector/pgvector-go"
	pgvectorpgx "github.com/pgvector/pgvector-go/pgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage interface {
	Init(ctx context.Context) error
	Count(ctx context.Context) (int, error)
	BulkInsert(ctx context.Context, items []domain.Item) error
	CreateIndex(ctx context.Context) error
	Close()
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, connString string) (*PostgresStore, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgvectorpgx.RegisterTypes(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to db: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Init(ctx context.Context) error {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS vector;`,
		`CREATE TABLE IF NOT EXISTS items (
			item_id UUID PRIMARY KEY,
			domain VARCHAR(50) NOT NULL,
			metadata JSONB,
			search_text TEXT,
			embedding VECTOR(384),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
	}

	for _, q := range queries {
		if _, err := s.pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("execute init query: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) Count(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM items;`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}
	return count, nil
}

func (s *PostgresStore) BulkInsert(ctx context.Context, items []domain.Item) error {
	if len(items) == 0 {
		return nil
	}

	rows := make([][]interface{}, 0, len(items))
	for _, item := range items {
		rows = append(rows, []interface{}{
			item.ID,
			item.Domain,
			item.Metadata,
			item.SearchText,
			pgvector.NewVector(item.Embedding),
		})
	}

	_, err := s.pool.CopyFrom(
		ctx,
		pgx.Identifier{"items"},
		[]string{"item_id", "domain", "metadata", "search_text", "embedding"},
		pgx.CopyFromRows(rows),
	)

	if err != nil {
		return fmt.Errorf("bulk insert copy from: %w", err)
	}

	return nil
}

func (s *PostgresStore) CreateIndex(ctx context.Context) error {
	query := `CREATE INDEX IF NOT EXISTS items_embedding_idx ON items USING hnsw (embedding vector_cosine_ops);`
	if _, err := s.pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("create hnsw index: %w", err)
	}
	return nil
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}
