// Package laya calls a local Laya server's System One endpoint.
package laya

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"prompt-evaluation/internal/kev"
)

const modelName = "multilingual"

// Client scores prompts with the multilingual Laya checkpoint.
type Client struct {
	*kev.Client
}

// New returns a client for a local laya-serve process.
func New(baseURL string) *Client {
	inner := kev.New(baseURL)
	inner.Model = modelName
	return &Client{Client: inner}
}

// Name is the report label. The request pins the multilingual checkpoint.
func (c *Client) Name() string { return "laya-multilingual" }

// Ping checks GET /health. Laya does not serve Kev's /v1/models card.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("laya server at %s is not reachable: %w", c.BaseURL, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("laya GET /health: %s: %s", res.Status, body)
	}
	return nil
}
