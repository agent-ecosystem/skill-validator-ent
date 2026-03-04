package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/cobra"

	"github.com/agent-ecosystem/skill-validator/evaluate"
	"github.com/agent-ecosystem/skill-validator/judge"
	"github.com/agent-ecosystem/skill-validator/report"
	"github.com/agent-ecosystem/skill-validator/types"

	"github.com/agent-ecosystem/skill-validator-ent/internal/bedrock"
)

var (
	evalProvider      string
	evalModel         string
	evalRegion        string
	evalProfile       string
	evalRescore       bool
	evalSkillOnly     bool
	evalRefsOnly      bool
	evalDisplay       string
	evalFullContent   bool
	evalMaxRespTokens int32
)

var scoreEvaluateCmd = &cobra.Command{
	Use:   "evaluate <path>",
	Short: "Score a skill using an LLM judge (via AWS Bedrock)",
	Long: `Score a skill's quality using an LLM-as-judge approach via AWS Bedrock.

The path can be:
  - A skill directory (containing SKILL.md) — scores SKILL.md and references
  - A multi-skill parent directory — scores each skill
  - A specific .md file — scores just that reference file

Requires AWS credentials configured via environment, shared config, or instance profile.
Use --region and --profile to override the default AWS configuration.`,
	Args: cobra.ExactArgs(1),
	RunE: runScoreEvaluate,
}

func init() {
	scoreEvaluateCmd.Flags().StringVar(&evalProvider, "provider", "bedrock", "LLM provider (only \"bedrock\" is supported)")
	scoreEvaluateCmd.Flags().StringVar(&evalModel, "model", "", "Bedrock model ID (required, e.g. us.anthropic.claude-sonnet-4-5-20250929-v1:0)")
	scoreEvaluateCmd.Flags().StringVar(&evalRegion, "region", "", "AWS region (overrides default config)")
	scoreEvaluateCmd.Flags().StringVar(&evalProfile, "profile", "", "AWS shared config profile (overrides default)")
	scoreEvaluateCmd.Flags().BoolVar(&evalRescore, "rescore", false, "re-score and overwrite cached results")
	scoreEvaluateCmd.Flags().BoolVar(&evalSkillOnly, "skill-only", false, "score only SKILL.md, skip reference files")
	scoreEvaluateCmd.Flags().BoolVar(&evalRefsOnly, "refs-only", false, "score only reference files, skip SKILL.md")
	scoreEvaluateCmd.Flags().StringVar(&evalDisplay, "display", "aggregate", "reference score display: aggregate or files")
	scoreEvaluateCmd.Flags().BoolVar(&evalFullContent, "full-content", false, "send full file content to LLM (default: truncate to 8,000 chars)")
	scoreEvaluateCmd.Flags().Int32Var(&evalMaxRespTokens, "max-response-tokens", 500, "maximum tokens in the LLM response")
	_ = scoreEvaluateCmd.MarkFlagRequired("model")
	scoreCmd.AddCommand(scoreEvaluateCmd)
}

func runScoreEvaluate(cmd *cobra.Command, args []string) error {
	if evalSkillOnly && evalRefsOnly {
		return fmt.Errorf("--skill-only and --refs-only are mutually exclusive")
	}

	if evalDisplay != "aggregate" && evalDisplay != "files" {
		return fmt.Errorf("--display must be \"aggregate\" or \"files\"")
	}

	// Provider gating: only Bedrock is supported in the enterprise tool.
	switch strings.ToLower(evalProvider) {
	case "bedrock":
		// supported
	case "anthropic", "openai":
		return fmt.Errorf("provider %q is not supported in skill-validator-ent — use the base skill-validator tool for direct API access", evalProvider)
	default:
		return fmt.Errorf("unsupported provider %q", evalProvider)
	}

	// Build Bedrock client
	client, err := buildBedrockClient(cmd.Context())
	if err != nil {
		return err
	}

	opts := evaluate.Options{
		Rescore:   evalRescore,
		SkillOnly: evalSkillOnly,
		RefsOnly:  evalRefsOnly,
		MaxLen:    evalMaxLen(),
		Progress: func(event, detail string) {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", event, detail)
		},
	}

	ctx := context.Background()
	path := args[0]

	// Check if path is a file (single reference scoring)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path not found: %s", path)
	}

	if !info.IsDir() {
		result, err := evaluate.EvaluateSingleFile(ctx, absPath, client, opts)
		if err != nil {
			return err
		}
		return report.FormatEvalResults(os.Stdout, []*evaluate.Result{result}, outputFormat, evalDisplay)
	}

	// Directory mode — detect skills
	_, mode, dirs, err := detectAndResolve(args)
	if err != nil {
		return err
	}

	switch mode {
	case types.SingleSkill:
		result, err := evaluate.EvaluateSkill(ctx, dirs[0], client, opts)
		if err != nil {
			return err
		}
		return report.FormatEvalResults(os.Stdout, []*evaluate.Result{result}, outputFormat, evalDisplay)

	case types.MultiSkill:
		var results []*evaluate.Result
		for _, dir := range dirs {
			result, err := evaluate.EvaluateSkill(ctx, dir, client, opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scoring %s: %v\n", filepath.Base(dir), err)
				continue
			}
			results = append(results, result)
		}
		return report.FormatMultiEvalResults(os.Stdout, results, outputFormat, evalDisplay)
	}

	return nil
}

// buildBedrockClient loads AWS config and creates a Bedrock LLM client.
func buildBedrockClient(ctx context.Context) (judge.LLMClient, error) {
	var optFns []func(*awsconfig.LoadOptions) error
	if evalRegion != "" {
		optFns = append(optFns, awsconfig.WithRegion(evalRegion))
	}
	if evalProfile != "" {
		optFns = append(optFns, awsconfig.WithSharedConfigProfile(evalProfile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	return bedrock.NewClient(cfg, evalModel, evalMaxRespTokens), nil
}

func evalMaxLen() int {
	if evalFullContent {
		return 0
	}
	return judge.DefaultMaxContentLen
}
