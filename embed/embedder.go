package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Embedder generates a fixed-length vector for a given piece of text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// SidecarEmbedder calls the lightweight Python sidecar service.
type SidecarEmbedder struct {
	url    string
	client *http.Client
}

func NewSidecarEmbedder(url string) *SidecarEmbedder {
	return &SidecarEmbedder{
		url: url,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type embedRequest struct {
	Text string `json:"text"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (e *SidecarEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody, err := json.Marshal(embedRequest{Text: text})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.url+"/embed", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sidecar request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sidecar returned status %d", resp.StatusCode)
	}

	var res embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode sidecar response: %w", err)
	}

	return res.Embedding, nil
}
