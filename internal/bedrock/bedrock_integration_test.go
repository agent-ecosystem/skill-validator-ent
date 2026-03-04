// Bedrock integration tests — smoke-test the real Converse API.
//
// Gated behind: BEDROCK_INTEGRATION_TEST=1
//
// Required environment:
//   - AWS credentials (via profile, env vars, or instance role)
//   - BEDROCK_MODEL: model ID (e.g. us.anthropic.claude-sonnet-4-5-20250929-v1:0)
//   - BEDROCK_REGION: AWS region (e.g. us-east-1)
//   - BEDROCK_PROFILE: (optional) AWS shared config profile
//
// Example:
//
//	BEDROCK_INTEGRATION_TEST=1 \
//	BEDROCK_MODEL=us.anthropic.claude-sonnet-4-5-20250929-v1:0 \
//	BEDROCK_REGION=us-east-1 \
//	BEDROCK_PROFILE=ai-prod-llm \
//	go test -v ./internal/bedrock/ -run Integration -count=1
package bedrock

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

func skipUnlessBedrock(t *testing.T) (model, region, profile string) {
	t.Helper()
	if os.Getenv("BEDROCK_INTEGRATION_TEST") == "" {
		t.Skip("set BEDROCK_INTEGRATION_TEST=1 to run Bedrock integration tests")
	}
	model = os.Getenv("BEDROCK_MODEL")
	if model == "" {
		t.Fatal("BEDROCK_MODEL is required")
	}
	region = os.Getenv("BEDROCK_REGION")
	if region == "" {
		t.Fatal("BEDROCK_REGION is required")
	}
	profile = os.Getenv("BEDROCK_PROFILE")
	return
}

func loadTestClient(t *testing.T) *Client {
	t.Helper()
	model, region, profile := skipUnlessBedrock(t)

	var optFns []func(*awsconfig.LoadOptions) error
	optFns = append(optFns, awsconfig.WithRegion(region))
	if profile != "" {
		optFns = append(optFns, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {
		t.Fatalf("loading AWS config: %v", err)
	}

	return NewClient(cfg, model, 100)
}

func TestIntegration_Complete(t *testing.T) {
	client := loadTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Complete(ctx, "You are a helpful assistant.", "Respond with exactly: PONG")
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	if !strings.Contains(strings.ToUpper(resp), "PONG") {
		t.Errorf("expected PONG in response, got: %s", resp)
	}
}

func TestIntegration_ProviderAndModel(t *testing.T) {
	client := loadTestClient(t)

	if got := client.Provider(); got != "bedrock" {
		t.Errorf("Provider() = %q, want %q", got, "bedrock")
	}

	model := os.Getenv("BEDROCK_MODEL")
	if got := client.ModelName(); got != model {
		t.Errorf("ModelName() = %q, want %q", got, model)
	}
}

func TestIntegration_JSONResponse(t *testing.T) {
	client := loadTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Complete(ctx,
		"You are a scoring judge. Respond with ONLY a JSON object.",
		`Score this text: "Hello world". Respond with: {"clarity": 5, "brief_assessment": "test"}`)
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	if !strings.Contains(resp, "clarity") {
		t.Errorf("expected JSON with 'clarity' key, got: %s", resp)
	}
}

func TestIntegration_InvalidModel(t *testing.T) {
	if os.Getenv("BEDROCK_INTEGRATION_TEST") == "" {
		t.Skip("set BEDROCK_INTEGRATION_TEST=1 to run Bedrock integration tests")
	}

	region := os.Getenv("BEDROCK_REGION")
	if region == "" {
		t.Fatal("BEDROCK_REGION is required")
	}
	profile := os.Getenv("BEDROCK_PROFILE")

	var optFns []func(*awsconfig.LoadOptions) error
	optFns = append(optFns, awsconfig.WithRegion(region))
	if profile != "" {
		optFns = append(optFns, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {
		t.Fatalf("loading AWS config: %v", err)
	}

	client := NewClient(cfg, "nonexistent.model-v99", 100)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = client.Complete(ctx, "system", "user")
	if err == nil {
		t.Fatal("expected error for invalid model")
	}

	// Should get a friendly error, not a raw AWS exception
	errMsg := err.Error()
	if !strings.Contains(errMsg, "model not found") && !strings.Contains(errMsg, "validation error") {
		t.Errorf("expected friendly error message, got: %s", errMsg)
	}
}
