// Package decider calls a local Mapika decider server's System One endpoint.
package decider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"prompt-evaluation/internal/kev"
)

const (
	modelName   = "decider"
	defaultName = "decider-4b"
)

// Client scores prompts with a Mapika decider checkpoint.
type Client struct {
	*kev.Client
	name string
}

// New returns a client for a local decider.serve process.
func New(baseURL string) *Client {
	inner := kev.New(baseURL)
	inner.Model = modelName
	return &Client{Client: inner, name: defaultName}
}

// Name is the report label. Ping replaces it with the checkpoint the server loaded.
func (c *Client) Name() string { return c.name }

// Ping checks GET /health, which reports ok only after the model has loaded and warmed up.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("decider server at %s is not reachable: %w", c.BaseURL, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("decider GET /health: %s: %s", res.Status, body)
	}
	var health struct {
		OK    bool   `json:"ok"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("decode decider health: %w", err)
	}
	if !health.OK {
		return fmt.Errorf("decider server at %s is not ready", c.BaseURL)
	}
	if i := strings.LastIndex(health.Model, "/"); i >= 0 {
		c.name = health.Model[i+1:]
	} else if health.Model != "" {
		c.name = health.Model
	}
	return nil
}
