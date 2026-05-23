package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"time"
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
	// If YELP_FILE points to a local file, use it instead of HTTP.
	// This is opt-in: the default path is always HTTP streaming.
	if path := os.Getenv("YELP_FILE"); path != "" {
		return s.streamFile(ctx, path, out)
	}
	return s.streamHTTP(ctx, out)
}

func (s *YelpJsonl) streamFile(ctx context.Context, path string, out chan<- domain.Item) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("yelp open local file: %w", err)
	}
	defer f.Close()
	log.Printf("Yelp: reading from local file %s", path)
	return s.scanLines(ctx, f, out)
}

func (s *YelpJsonl) streamHTTP(ctx context.Context, out chan<- domain.Item) error {
	// No client-level Timeout: the JSONL file is hundreds of MB and streams for
	// many minutes. A total-request timeout would kill the connection mid-stream.
	// Transport-level timeouts guard against hung connections during setup only.
	client := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 2 * time.Minute,
			MaxConnsPerHost:       1,
			MaxIdleConnsPerHost:   1,
			IdleConnTimeout:       90 * time.Second,
		},
	}

	// Retry logic with exponential backoff (max 3 attempts)
	var resp *http.Response
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			log.Printf("Yelp download attempt %d: retrying in %v (previous error: %v)", attempt+1, backoff, err)
			time.Sleep(backoff)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
		if err != nil {
			return fmt.Errorf("yelp create request: %w", err)
		}

		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break // Success
		}

		if err != nil {
			if attempt < 2 {
				continue // Retry on network errors
			}
			return fmt.Errorf("yelp do request (after 3 attempts): %w", err)
		}

		if resp != nil && resp.StatusCode != http.StatusOK {
			if attempt < 2 {
				resp.Body.Close()
				continue // Retry on HTTP errors (except last attempt)
			}
			return fmt.Errorf("yelp unexpected status: %d", resp.StatusCode)
		}
	}

	if resp == nil {
		return fmt.Errorf("yelp: failed to establish connection after 3 attempts")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("yelp unexpected status: %d", resp.StatusCode)
	}

	return s.scanLines(ctx, resp.Body, out)
}

func (s *YelpJsonl) scanLines(ctx context.Context, r io.Reader, out chan<- domain.Item) error {
	scanner := bufio.NewScanner(r)
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
