package ingest

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ningen/domain"

	"github.com/google/uuid"
)

type YelpCSV struct {
	URL string
}

func NewYelpCSV(url string) *YelpCSV {
	return &YelpCSV{URL: url}
}

func (s *YelpCSV) Stream(ctx context.Context, out chan<- domain.Item) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return fmt.Errorf("yelp create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("yelp do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("yelp unexpected status: %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)
	// Yelp CSV format: label, text
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
			return fmt.Errorf("yelp read csv: %w", err)
		}

		if len(record) < 2 {
			continue
		}

		label := record[0]
		text := record[1]

		meta, _ := json.Marshal(map[string]string{"label": label})
		out <- domain.Item{
			ID:         uuid.NewString(),
			Domain:     "yelp",
			Metadata:   string(meta),
			SearchText: text,
		}
	}
}
