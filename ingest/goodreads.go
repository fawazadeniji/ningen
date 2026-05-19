package ingest

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ningen/domain"
)

type GoodreadsCSV struct {
	URL string
}

func NewGoodreadsCSV(url string) *GoodreadsCSV {
	return &GoodreadsCSV{URL: url}
}

func (s *GoodreadsCSV) Stream(ctx context.Context, out chan<- domain.Item) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return fmt.Errorf("goodreads create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("goodreads do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("goodreads unexpected status: %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)
	// Some fields might have multiple lines
	reader.FieldsPerRecord = -1

	// Skip header
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("goodreads read header: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("goodreads read csv: %w", err)
		}

		if len(record) < 8 {
			continue
		}

		rating := record[6]
		text := record[7]

		if text == "" {
			continue
		}

		meta, _ := json.Marshal(map[string]string{"rating": rating})
		select {
		case out <- domain.Item{
			ID:         deterministicID("goodreads", text),
			Domain:     "goodreads",
			Metadata:   string(meta),
			SearchText: text,
		}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
