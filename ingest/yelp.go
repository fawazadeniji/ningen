package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"ningen/domain"
)

// YelpJsonl streams the SetFit/yelp_review_full JSONL format.
// Each line: {"label": <int>, "text": "<review>"}
type YelpJsonl struct {
	URL string
}

func NewYelpJsonl(url string) *YelpJsonl {
	return &YelpJsonl{URL: url}
}

func (s *YelpJsonl) Stream(ctx context.Context, out chan<- domain.Item) error {
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

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var record struct {
			Label json.RawMessage `json:"label"`
			Text  string          `json:"text"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if record.Text == "" {
			continue
		}

		meta, _ := json.Marshal(map[string]json.RawMessage{"label": record.Label})
		select {
		case out <- domain.Item{
			ID:         deterministicID("yelp", record.Text),
			Domain:     "yelp",
			Metadata:   string(meta),
			SearchText: record.Text,
		}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return scanner.Err()
}
