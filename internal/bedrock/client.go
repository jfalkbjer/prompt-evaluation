// Package bedrock classifies prompts with Claude Haiku 4.5 on Amazon Bedrock.
package bedrock

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"prompt-evaluation/internal/rubric"
)

const (
	// ModelID is the EU geographic inference profile for Claude Haiku 4.5.
	ModelID   = "eu.anthropic.claude-haiku-4-5-20251001-v1:0"
	profile   = "ai-dev"
	region    = "eu-north-1"
	maxTokens = 32
)

// Client calls the Bedrock Converse API.
type Client struct {
	Rubric  rubric.Noul
	runtime *bedrockruntime.Client
	modelID string
}

// New loads the ai-dev profile in eu-north-1.
func New(ctx context.Context) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &Client{
		Rubric:  rubric.Original,
		runtime: bedrockruntime.NewFromConfig(cfg),
		modelID: ModelID,
	}, nil
}

// Name is the report label.
func (c *Client) Name() string { return "bedrock-haiku-4.5" }

// Decide classifies text. The bool is false when the model does not reply
// with a single true or false; the error is returned alongside so the caller
// can record a parse failure.
func (c *Client) Decide(ctx context.Context, text string) (bool, error) {
	out, err := c.runtime.Converse(ctx, &bedrockruntime.ConverseInput{
		ModelId: aws.String(c.modelID),
		System: []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: c.Rubric.SystemPrompt()},
		},
		Messages: []types.Message{{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: "Text to classify:\n" + text},
			},
		}},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens:   aws.Int32(maxTokens),
			Temperature: aws.Float32(0),
		},
	})
	if err != nil {
		return false, err
	}
	reply, err := messageText(out)
	if err != nil {
		return false, err
	}
	return ParseLabel(reply)
}

// Classify asks Haiku which PII kind the text contains.
func (c *Client) Classify(ctx context.Context, text string) (string, error) {
	out, err := c.runtime.Converse(ctx, &bedrockruntime.ConverseInput{
		ModelId: aws.String(c.modelID),
		System: []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: rubric.TypePrompt()},
		},
		Messages: []types.Message{{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: "Text to classify:\n" + text},
			},
		}},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens:   aws.Int32(maxTokens),
			Temperature: aws.Float32(0),
		},
	})
	if err != nil {
		return "", err
	}
	reply, err := messageText(out)
	if err != nil {
		return "", err
	}
	return ParseChoice(reply, typeNames())
}

func typeNames() []string {
	names := make([]string, len(rubric.TypeOptions))
	for i, opt := range rubric.TypeOptions {
		names[i] = opt.Name
	}
	return names
}

func messageText(out *bedrockruntime.ConverseOutput) (string, error) {
	msg, ok := out.Output.(*types.ConverseOutputMemberMessage)
	if !ok || msg == nil {
		return "", fmt.Errorf("bedrock response had no message")
	}
	var b strings.Builder
	for _, block := range msg.Value.Content {
		text, ok := block.(*types.ContentBlockMemberText)
		if ok {
			b.WriteString(text.Value)
		}
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("bedrock response had no text")
	}
	return b.String(), nil
}
