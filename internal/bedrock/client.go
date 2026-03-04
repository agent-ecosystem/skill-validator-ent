// Package bedrock implements [judge.LLMClient] using the AWS Bedrock
// Converse API. It translates the system-prompt / user-message pair into
// a Converse request and extracts the text response.
package bedrock

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// ConverseAPI is the subset of bedrockruntime.Client used by Client.
// It exists so unit tests can substitute a mock.
type ConverseAPI interface {
	Converse(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error)
}

// Client implements judge.LLMClient using the Bedrock Converse API.
type Client struct {
	api       ConverseAPI
	model     string
	maxTokens int32
}

// NewClient creates a Bedrock-backed LLMClient.
//
//   - cfg is a loaded aws.Config (with region/profile already resolved).
//   - model is a Bedrock model ID such as "us.anthropic.claude-sonnet-4-5-20250929-v1:0".
//   - maxTokens caps the response length (0 defaults to 500).
func NewClient(cfg aws.Config, model string, maxTokens int32) *Client {
	if maxTokens <= 0 {
		maxTokens = 500
	}
	return &Client{
		api:       bedrockruntime.NewFromConfig(cfg),
		model:     model,
		maxTokens: maxTokens,
	}
}

// newClientWithAPI is used by tests to inject a mock ConverseAPI.
func newClientWithAPI(api ConverseAPI, model string, maxTokens int32) *Client {
	if maxTokens <= 0 {
		maxTokens = 500
	}
	return &Client{api: api, model: model, maxTokens: maxTokens}
}

// Complete sends a system prompt and user content to Bedrock and returns
// the text response. It satisfies the judge.LLMClient interface.
func (c *Client) Complete(ctx context.Context, systemPrompt, userContent string) (string, error) {
	input := &bedrockruntime.ConverseInput{
		ModelId: aws.String(c.model),
		System: []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: systemPrompt},
		},
		Messages: []types.Message{
			{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{Value: userContent},
				},
			},
		},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens: aws.Int32(c.maxTokens),
		},
	}

	output, err := c.api.Converse(ctx, input)
	if err != nil {
		return "", classifyError(err)
	}

	msg, ok := output.Output.(*types.ConverseOutputMemberMessage)
	if !ok || len(msg.Value.Content) == 0 {
		return "", fmt.Errorf("empty response from Bedrock model %s", c.model)
	}

	textBlock, ok := msg.Value.Content[0].(*types.ContentBlockMemberText)
	if !ok {
		return "", fmt.Errorf("unexpected content block type from Bedrock model %s", c.model)
	}

	return textBlock.Value, nil
}

// Provider returns "bedrock".
func (c *Client) Provider() string { return "bedrock" }

// ModelName returns the Bedrock model ID.
func (c *Client) ModelName() string { return c.model }

// classifyError inspects the error message and wraps it with a
// user-friendly hint when the cause is a common AWS/Bedrock issue.
func classifyError(err error) error {
	msg := err.Error()

	switch {
	case strings.Contains(msg, "AccessDeniedException"):
		return fmt.Errorf("access denied: ensure your IAM role has bedrock:InvokeModel permission for this model: %w", err)
	case strings.Contains(msg, "ResourceNotFoundException"):
		return fmt.Errorf("model not found: check the model ID and ensure it is enabled in your Bedrock region: %w", err)
	case strings.Contains(msg, "ThrottlingException"):
		return fmt.Errorf("request throttled: you have exceeded the Bedrock rate limit, try again shortly: %w", err)
	case strings.Contains(msg, "ValidationException"):
		return fmt.Errorf("validation error: the request was rejected by Bedrock (check model ID and parameters): %w", err)
	default:
		return fmt.Errorf("bedrock API error: %w", err)
	}
}
