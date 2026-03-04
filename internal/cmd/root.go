package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/agent-ecosystem/skill-validator/skillcheck"
	"github.com/agent-ecosystem/skill-validator/types"

	"github.com/agent-ecosystem/skill-validator-ent/internal/config"
)

const version = "v0.1.0"

var (
	outputFormat    string
	emitAnnotations bool
	verbose         bool
)

var rootCmd = &cobra.Command{
	Use:   "skill-validator-ent",
	Short: "Validate and analyze agent skills (enterprise)",
	Long:  "An enterprise CLI for validating skill directory structure, analyzing content quality, detecting cross-language contamination, and scoring skills via AWS Bedrock.",
	// Once a command starts running (args parsed successfully), don't print
	// usage on error — the error is operational, not a CLI mistake.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		initLogger()

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		cfg, loaded := config.Load(cwd)
		for _, p := range loaded {
			slog.Debug("loaded config", "path", p)
		}

		applyConfig(cmd, cfg)
		return nil
	},
}

func init() {
	rootCmd.Version = version
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "output format: text, json, or markdown")
	rootCmd.PersistentFlags().BoolVar(&emitAnnotations, "emit-annotations", false, "emit GitHub Actions workflow command annotations (::error/::warning) alongside normal output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose/debug logging")
}

// Execute runs the root command.
func Execute() {
	// We handle error printing ourselves so that exitCodeError (validation
	// failures) doesn't produce cobra's default "Error: exit code N" noise.
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		if ec, ok := err.(exitCodeError); ok {
			// Validation failure — report was already printed.
			os.Exit(ec.code)
		}
		// CLI/usage error — print and exit.
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(ExitCobra)
	}
}

// initLogger sets up slog with Debug level if --verbose or SKILL_VALIDATOR_DEBUG=1.
func initLogger() {
	level := slog.LevelInfo
	if verbose || isEnvTrue("SKILL_VALIDATOR_DEBUG") {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

func isEnvTrue(key string) bool {
	v, _ := strconv.ParseBool(os.Getenv(key))
	return v
}

// applyConfig sets flag values from config for any flags not explicitly set on the CLI.
func applyConfig(cmd *cobra.Command, cfg *config.Config) {
	if cfg == nil {
		return
	}

	// Global persistent flags.
	root := cmd.Root()
	applyStringFlag(root.PersistentFlags(), "output", cfg.Output)
	applyBoolFlag(root.PersistentFlags(), "emit-annotations", cfg.EmitAnnotations)

	// Score evaluate flags — only apply to the evaluate command.
	// The "model" flag on "score report" is a filter (default empty), not a
	// default model, so it must NOT be populated from config.
	if cmd == scoreEvaluateCmd {
		applyStringFlag(cmd.Flags(), "model", cfg.Model)
		applyStringFlag(cmd.Flags(), "region", cfg.Region)
		applyStringFlag(cmd.Flags(), "profile", cfg.Profile)
		applyStringFlag(cmd.Flags(), "provider", cfg.Provider)
		applyInt32Flag(cmd.Flags(), "max-response-tokens", cfg.MaxResponseTokens)
		applyStringFlag(cmd.Flags(), "display", cfg.Display)
		applyBoolFlag(cmd.Flags(), "full-content", cfg.FullContent)
	}

	// Flags shared across multiple commands (check, validate structure).
	applyBoolFlag(cmd.Flags(), "strict", cfg.Strict)
}

func applyStringFlag(fs *pflag.FlagSet, name string, val *string) {
	if val == nil {
		return
	}
	f := fs.Lookup(name)
	if f == nil || f.Changed {
		return
	}
	_ = fs.Set(name, *val)
}

func applyBoolFlag(fs *pflag.FlagSet, name string, val *bool) {
	if val == nil {
		return
	}
	f := fs.Lookup(name)
	if f == nil || f.Changed {
		return
	}
	_ = fs.Set(name, strconv.FormatBool(*val))
}

func applyInt32Flag(fs *pflag.FlagSet, name string, val *int32) {
	if val == nil {
		return
	}
	f := fs.Lookup(name)
	if f == nil || f.Changed {
		return
	}
	_ = fs.Set(name, strconv.FormatInt(int64(*val), 10))
}

// resolvePath resolves a path argument to an absolute directory path.
func resolvePath(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("path argument required")
	}

	dir := args[0]
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s is not a valid directory", dir)
	}

	return absDir, nil
}

// detectAndResolve resolves the path and detects skills.
func detectAndResolve(args []string) (string, types.SkillMode, []string, error) {
	absDir, err := resolvePath(args)
	if err != nil {
		return "", 0, nil, err
	}

	mode, dirs := skillcheck.DetectSkills(absDir)
	if mode == types.NoSkill {
		return "", 0, nil, fmt.Errorf("no skills found in %s (expected SKILL.md or subdirectories containing SKILL.md)", args[0])
	}

	return absDir, mode, dirs, nil
}
