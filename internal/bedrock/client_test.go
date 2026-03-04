package bedrock

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// mockConverseAPI is a test double for the Converse call.
type mockConverseAPI struct {
	output *bedrockruntime.ConverseOutput
	err    error
	// captured stores the last ConverseInput for assertions.
	captured *bedrockruntime.ConverseInput
}

func (m *mockConverseAPI) Converse(ctx context.Context, params *bedrockruntime.ConverseInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
	m.captured = params
	return m.output, m.err
}

func TestComplete_Success(t *testing.T) {
	mock := &mockConverseAPI{
		output: &bedrockruntime.ConverseOutput{
			Output: &types.ConverseOutputMemberMessage{
				Value: types.Message{
					Content: []types.ContentBlock{
						&types.ContentBlockMemberText{Value: `{"clarity": 4}`},
					},
				},
			},
		},
	}

	c := newClientWithAPI(mock, "us.anthropic.claude-sonnet-4-5-20250929-v1:0", 500)

	got, err := c.Complete(context.Background(), "system prompt", "user content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"clarity": 4}` {
		t.Errorf("got %q, want %q", got, `{"clarity": 4}`)
	}

	// Verify the input was constructed correctly.
	if mock.captured == nil {
		t.Fatal("expected captured input")
	}
	if *mock.captured.ModelId != "us.anthropic.claude-sonnet-4-5-20250929-v1:0" {
		t.Errorf("model = %q, want %q", *mock.captured.ModelId, "us.anthropic.claude-sonnet-4-5-20250929-v1:0")
	}
	if len(mock.captured.System) != 1 {
		t.Fatalf("expected 1 system block, got %d", len(mock.captured.System))
	}
	sysBlock, ok := mock.captured.System[0].(*types.SystemContentBlockMemberText)
	if !ok {
		t.Fatal("expected SystemContentBlockMemberText")
	}
	if sysBlock.Value != "system prompt" {
		t.Errorf("system = %q, want %q", sysBlock.Value, "system prompt")
	}
	if *mock.captured.InferenceConfig.MaxTokens != 500 {
		t.Errorf("maxTokens = %d, want 500", *mock.captured.InferenceConfig.MaxTokens)
	}
}

func TestComplete_EmptyResponse(t *testing.T) {
	mock := &mockConverseAPI{
		output: &bedrockruntime.ConverseOutput{
			Output: &types.ConverseOutputMemberMessage{
				Value: types.Message{
					Content: []types.ContentBlock{},
				},
			},
		},
	}

	c := newClientWithAPI(mock, "test-model", 500)
	_, err := c.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("error = %q, want 'empty response'", err)
	}
}

func TestComplete_AccessDenied(t *testing.T) {
	mock := &mockConverseAPI{
		err: errors.New("operation error: AccessDeniedException: User is not authorized"),
	}

	c := newClientWithAPI(mock, "test-model", 500)
	_, err := c.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("error = %q, want 'access denied'", err)
	}
}

func TestComplete_ResourceNotFound(t *testing.T) {
	mock := &mockConverseAPI{
		err: errors.New("ResourceNotFoundException: model xyz not found"),
	}

	c := newClientWithAPI(mock, "test-model", 500)
	_, err := c.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("error = %q, want 'model not found'", err)
	}
}

func TestComplete_Throttled(t *testing.T) {
	mock := &mockConverseAPI{
		err: errors.New("ThrottlingException: rate exceeded"),
	}

	c := newClientWithAPI(mock, "test-model", 500)
	_, err := c.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "throttled") {
		t.Errorf("error = %q, want 'throttled'", err)
	}
}

func TestComplete_ValidationError(t *testing.T) {
	mock := &mockConverseAPI{
		err: errors.New("ValidationException: invalid model id"),
	}

	c := newClientWithAPI(mock, "test-model", 500)
	_, err := c.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "validation error") {
		t.Errorf("error = %q, want 'validation error'", err)
	}
}

func TestComplete_GenericError(t *testing.T) {
	mock := &mockConverseAPI{
		err: errors.New("some unknown error"),
	}

	c := newClientWithAPI(mock, "test-model", 500)
	_, err := c.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bedrock API error") {
		t.Errorf("error = %q, want 'bedrock API error'", err)
	}
}

func TestProvider(t *testing.T) {
	c := newClientWithAPI(nil, "test-model", 500)
	if got := c.Provider(); got != "bedrock" {
		t.Errorf("Provider() = %q, want %q", got, "bedrock")
	}
}

func TestModelName(t *testing.T) {
	c := newClientWithAPI(nil, "us.anthropic.claude-sonnet-4-5-20250929-v1:0", 500)
	if got := c.ModelName(); got != "us.anthropic.claude-sonnet-4-5-20250929-v1:0" {
		t.Errorf("ModelName() = %q, want %q", got, "us.anthropic.claude-sonnet-4-5-20250929-v1:0")
	}
}

func TestDefaultMaxTokens(t *testing.T) {
	c := newClientWithAPI(nil, "test-model", 0)
	if c.maxTokens != 500 {
		t.Errorf("maxTokens = %d, want 500 (default)", c.maxTokens)
	}
}
