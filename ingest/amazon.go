package ingest

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"ningen/domain"
)

type AmazonGzJsonl struct {
	URL string
}

func NewAmazonGzJsonl(url string) *AmazonGzJsonl {
	return &AmazonGzJsonl{URL: url}
}

func (s *AmazonGzJsonl) Stream(ctx context.Context, out chan<- domain.Item) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return fmt.Errorf("amazon create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("amazon do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("amazon unexpected status: %d", resp.StatusCode)
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("amazon gzip reader: %w", err)
	}
	defer gzReader.Close()

	scanner := bufio.NewScanner(gzReader)
	// Some JSON lines can be large
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		var record map[string]interface{}
		if err := json.Unmarshal(line, &record); err != nil {
			continue // skip invalid JSON
		}

		text, _ := record["reviewText"].(string)
		if text == "" {
			continue
		}

		rating := 0.0
		if r, ok := record["overall"].(float64); ok {
			rating = r
		}

		meta, _ := json.Marshal(map[string]float64{"rating": rating})
		select {
		case out <- domain.Item{
			ID:         deterministicID("amazon", text),
			Domain:     "amazon",
			Metadata:   string(meta),
			SearchText: text,
		}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("amazon scanner error: %w", err)
	}

	return nil
}
