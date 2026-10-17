// Package gliner calls a local GLiNER2.5 Decide server's System One endpoint.
package gliner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"prompt-evaluation/internal/kev"
)

const (
	modelName   = "gliner2.5-decide"
	defaultName = "GLiNER2.5-Decide"
)

// Client scores prompts with GLiNER2.5 Decide.
type Client struct {
	*kev.Client
	name string
}

// New returns a client for scripts/gliner_server.py.
func New(baseURL string) *Client {
	inner := kev.New(baseURL)
	inner.Model = modelName
	return &Client{Client: inner, name: defaultName}
}

// Name is the report label. Ping replaces it with the checkpoint the server loaded.
func (c *Client) Name() string { return c.name }

// Ping checks GET /health. The GLiNER server does not serve Kev's /v1/models card.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("gliner server at %s is not reachable: %w", c.BaseURL, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("gliner GET /health: %s: %s", res.Status, body)
	}
	var health struct {
		Status string `json:"status"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("decode gliner health: %w", err)
	}
	if health.Status != "ok" {
		return fmt.Errorf("gliner health status %q", health.Status)
	}
	if health.Model != "" {
		c.name = health.Model
	}
	return nil
}
