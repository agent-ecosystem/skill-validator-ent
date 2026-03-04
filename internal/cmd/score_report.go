package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/agent-ecosystem/skill-validator/judge"
	"github.com/agent-ecosystem/skill-validator/report"
)

var (
	reportList    bool
	reportCompare bool
	reportModel   string
)

var scoreReportCmd = &cobra.Command{
	Use:   "report <path>",
	Short: "View cached LLM scores",
	Long: `View and compare cached LLM quality scores without making API calls.

By default, shows the most recent scores for each file. Use flags to
list all cached entries, compare across models, or filter by model.`,
	Args: cobra.ExactArgs(1),
	RunE: runScoreReport,
}

func init() {
	scoreReportCmd.Flags().BoolVar(&reportList, "list", false, "list all cached score entries with metadata")
	scoreReportCmd.Flags().BoolVar(&reportCompare, "compare", false, "compare scores across models side-by-side")
	scoreReportCmd.Flags().StringVar(&reportModel, "model", "", "filter to scores from a specific model")
	scoreCmd.AddCommand(scoreReportCmd)
}

func runScoreReport(cmd *cobra.Command, args []string) error {
	absDir, err := resolvePath(args)
	if err != nil {
		return err
	}

	cacheDir := judge.CacheDir(absDir)
	results, err := judge.ListCached(cacheDir)
	if err != nil {
		return fmt.Errorf("reading cache: %w", err)
	}

	if len(results) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No cached scores found. Run 'score evaluate' first.")
		return nil
	}

	if reportModel != "" {
		results = judge.FilterByModel(results, reportModel)
		if len(results) == 0 {
			_, _ = fmt.Fprintf(os.Stdout, "No cached scores found for model %q.\n", reportModel)
			return nil
		}
	}

	// Shorten Bedrock model IDs for display. The base library's text
	// formatter truncates model names to 14 chars, which makes all
	// Bedrock IDs like "us.anthropic.claude-..." look identical.
	displayResults := shortenModelNames(results)

	switch {
	case reportList:
		return report.List(os.Stdout, displayResults, absDir, outputFormat)
	case reportCompare:
		return report.Compare(os.Stdout, displayResults, absDir, outputFormat)
	default:
		return report.Default(os.Stdout, displayResults, absDir, outputFormat)
	}
}

// bedrockModelPrefix matches Bedrock cross-region model ID prefixes
// like "us.anthropic.", "eu.anthropic.", "ap.meta.", etc.
var bedrockModelPrefix = regexp.MustCompile(`^[a-z]{2}\.[\w-]+\.`)

// shortenModelNames returns shallow copies of cached results with Bedrock
// regional prefixes stripped from model names for more readable display.
func shortenModelNames(results []*judge.CachedResult) []*judge.CachedResult {
	out := make([]*judge.CachedResult, len(results))
	for i, r := range results {
		cp := *r
		cp.Model = bedrockModelPrefix.ReplaceAllString(r.Model, "")
		out[i] = &cp
	}
	return out
}
