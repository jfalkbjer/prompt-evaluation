// Package kev calls a local Kev server's System One endpoint.
package kev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"prompt-evaluation/internal/rubric"
)

const (
	questionID   = "pii"
	defaultModel = "kev-latest"
)

// Client scores prompts with a noul question.
type Client struct {
	BaseURL    string
	Model      string
	Rubric     rubric.Noul
	HTTPClient *http.Client
	runName    string
}

// New returns a client for a local server. The timeout covers a cold MLX forward pass.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		Model:   defaultModel,
		Rubric:  rubric.Original,
		HTTPClient: &http.Client{
			Timeout: 3 * time.Minute,
		},
	}
}

// Name is the checkpoint the server reported, such as kev-0.8b.
func (c *Client) Name() string {
	if c.runName == "" {
		return "kev"
	}
	return c.runName
}

// Ping checks that the server is accepting requests and records which checkpoint is loaded.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/models", nil)
	if err != nil {
		return err
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("kev server at %s is not reachable: %w", c.BaseURL, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("kev GET /v1/models: %s: %s", res.Status, bytes.TrimSpace(body))
	}
	var parsed modelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("decode kev models: %w", err)
	}
	if len(parsed.Models) == 0 || parsed.Models[0].Run == "" {
		return fmt.Errorf("kev GET /v1/models: response did not name a checkpoint")
	}
	c.runName = shortRun(parsed.Models[0].Run)
	return nil
}

func shortRun(run string) string {
	run = strings.TrimSpace(run)
	if i := strings.LastIndex(run, "/"); i >= 0 {
		run = run[i+1:]
	}
	if i := strings.Index(run, "@"); i >= 0 {
		run = run[:i]
	}
	return run
}

// Decide returns whether the text contains sensitive PII and the noul probability of yes.
func (c *Client) Decide(ctx context.Context, text string) (bool, *float64, error) {
	payload := request{
		State: text,
		Model: c.model(),
		Questions: map[string]question{
			questionID: {
				Type:         "noul",
				Instructions: c.Rubric.Instructions,
				Criteria: map[string]string{
					"true":  c.Rubric.CriteriaTrue,
					"false": c.Rubric.CriteriaFalse,
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return false, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/systemone", bytes.NewReader(raw))
	if err != nil {
		return false, nil, err
	}
	req.Header.Set("content-type", "application/json")
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return false, nil, err
	}
	if res.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("kev POST /v1/systemone: %s: %s", res.Status, bytes.TrimSpace(body))
	}
	var parsed response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false, nil, fmt.Errorf("decode kev response: %w", err)
	}
	answer, ok := parsed.Answers[questionID]
	if !ok {
		return false, nil, fmt.Errorf("kev response missing %q", questionID)
	}
	p := answer.Noul
	return p >= 0.5, &p, nil
}

// Choice is Kev's pick among the PII kinds, plus that option's probability and the choice confidence.
type Choice struct {
	Name          string
	Probability   float64
	Confidence    float64
	Probabilities map[string]float64
}

// Choose asks which kind of PII the text contains.
func (c *Client) Choose(ctx context.Context, text string) (Choice, error) {
	raw, err := c.choiceRequest(text)
	if err != nil {
		return Choice{}, err
	}
	body, err := c.post(ctx, raw)
	if err != nil {
		return Choice{}, err
	}
	var parsed choiceResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Choice{}, fmt.Errorf("decode kev response: %w", err)
	}
	answer, ok := parsed.Answers[questionID]
	if !ok || answer.Choice == "" {
		return Choice{}, fmt.Errorf("kev response missing choice %q", questionID)
	}
	return Choice{
		Name:          answer.Choice,
		Probability:   answer.Probabilities[answer.Choice],
		Confidence:    answer.Confidence,
		Probabilities: answer.Probabilities,
	}, nil
}

func (c *Client) model() string {
	if c.Model == "" {
		return defaultModel
	}
	return c.Model
}

func (c *Client) choiceRequest(text string) ([]byte, error) {
	criteria := make([]byte, 0, 1024)
	criteria = append(criteria, '{')
	for i, opt := range rubric.TypeOptions {
		if i > 0 {
			criteria = append(criteria, ',')
		}
		name, err := json.Marshal(opt.Name)
		if err != nil {
			return nil, err
		}
		desc, err := json.Marshal(opt.Description)
		if err != nil {
			return nil, err
		}
		criteria = append(criteria, name...)
		criteria = append(criteria, ':')
		criteria = append(criteria, desc...)
	}
	criteria = append(criteria, '}')
	instructions, err := json.Marshal(rubric.TypeInstructions)
	if err != nil {
		return nil, err
	}
	state, err := json.Marshal(text)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(`{"state":`)
	buf.Write(state)
	buf.WriteString(`,"model":`)
	model, err := json.Marshal(c.model())
	if err != nil {
		return nil, err
	}
	buf.Write(model)
	buf.WriteString(`,"questions":{"` + questionID + `":{"type":"choice","instructions":`)
	buf.Write(instructions)
	buf.WriteString(`,"criteria":`)
	buf.Write(criteria)
	buf.WriteString(`}}}`)
	return buf.Bytes(), nil
}

func (c *Client) post(ctx context.Context, raw []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/systemone", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("POST /v1/systemone: %s: %s", res.Status, bytes.TrimSpace(body))
	}
	return body, nil
}

type request struct {
	State     string              `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]question `json:"questions"`
}

type question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type modelsResponse struct {
	Models []struct {
		Run string `json:"run"`
	} `json:"models"`
}

type response struct {
	Answers map[string]struct {
		Noul float64 `json:"noul"`
	} `json:"answers"`
}

type choiceResponse struct {
	Answers map[string]struct {
		Choice        string             `json:"choice"`
		Confidence    float64            `json:"confidence"`
		Probabilities map[string]float64 `json:"probabilities"`
	} `json:"answers"`
}
