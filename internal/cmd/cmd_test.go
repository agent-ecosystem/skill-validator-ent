package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/agent-ecosystem/skill-validator/orchestrate"
	"github.com/agent-ecosystem/skill-validator/skillcheck"
	"github.com/agent-ecosystem/skill-validator/structure"
	"github.com/agent-ecosystem/skill-validator/types"
)

// executeCommand runs rootCmd with the given args and returns stdout,
// stderr, and any error. This is the idiomatic Cobra testing pattern:
// set args, capture output, execute, check results.
func executeCommand(args ...string) (stdout, stderr string, err error) {
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)

	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs(args)

	// Reset flag defaults that may have been changed by prior tests —
	// Cobra reuses the command tree across calls.
	outputFormat = "text"
	emitAnnotations = false
	verbose = false

	// score evaluate flags
	evalProvider = "bedrock"
	evalModel = ""
	evalRegion = ""
	evalProfile = ""
	evalRescore = false
	evalSkillOnly = false
	evalRefsOnly = false
	evalDisplay = "aggregate"
	evalFullContent = false
	evalMaxRespTokens = 500

	// score report flags
	reportList = false
	reportCompare = false
	reportModel = ""

	// check flags
	checkOnly = ""
	checkSkip = ""
	perFileCheck = false
	checkSkipOrphans = false
	strictCheck = false

	// validate structure flags
	skipOrphans = false
	strictStructure = false

	// Reset cobra's Changed state on all flags so that config values
	// can be applied in tests that don't pass CLI flags.
	resetFlagChanged(rootCmd)

	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// resetFlagChanged resets the Changed state on all flags in the command tree
// so that cobra doesn't think flags were explicitly set by a prior test.
func resetFlagChanged(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
	for _, sub := range cmd.Commands() {
		resetFlagChanged(sub)
	}
}

// fixtureDir returns the absolute path to a testdata fixture.
func fixtureDir(t *testing.T, name string) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("fixture %q not found: %v", name, err)
	}
	return dir
}

func TestValidateCommand_ValidSkill(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")

	r := structure.Validate(dir, structure.Options{})
	if r.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", r.Errors)
		for _, res := range r.Results {
			if res.Level == types.Error {
				t.Logf("  error: %s: %s", res.Category, res.Message)
			}
		}
	}

	hasStructure := false
	hasFrontmatter := false
	hasMarkdown := false
	for _, res := range r.Results {
		if res.Category == "Structure" {
			hasStructure = true
		}
		if res.Category == "Frontmatter" {
			hasFrontmatter = true
		}
		if res.Category == "Markdown" {
			hasMarkdown = true
		}
	}
	if !hasStructure {
		t.Error("expected Structure results from validate")
	}
	if !hasFrontmatter {
		t.Error("expected Frontmatter results from validate")
	}
	if !hasMarkdown {
		t.Error("expected Markdown results from validate (code fence checks)")
	}
}

func TestValidateCommand_InvalidSkill(t *testing.T) {
	dir := fixtureDir(t, "invalid-skill")

	r := structure.Validate(dir, structure.Options{})
	if r.Errors == 0 {
		t.Error("expected errors for invalid skill")
	}
}

func TestValidateCommand_MultiSkill(t *testing.T) {
	dir := fixtureDir(t, "multi-skill")

	mode, dirs := skillcheck.DetectSkills(dir)
	if mode != types.MultiSkill {
		t.Fatalf("expected MultiSkill, got %d", mode)
	}

	mr := structure.ValidateMulti(dirs, structure.Options{})
	if len(mr.Skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(mr.Skills))
	}
}

func TestResolveCheckGroups(t *testing.T) {
	t.Run("default all enabled", func(t *testing.T) {
		enabled, err := resolveCheckGroups("", "")
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range []orchestrate.CheckGroup{
			orchestrate.GroupStructure, orchestrate.GroupLinks,
			orchestrate.GroupContent, orchestrate.GroupContamination,
		} {
			if !enabled[g] {
				t.Errorf("expected %s enabled by default", g)
			}
		}
	})

	t.Run("only structure,links", func(t *testing.T) {
		enabled, err := resolveCheckGroups("structure,links", "")
		if err != nil {
			t.Fatal(err)
		}
		if !enabled[orchestrate.GroupStructure] || !enabled[orchestrate.GroupLinks] {
			t.Error("expected structure and links enabled")
		}
		if enabled[orchestrate.GroupContent] || enabled[orchestrate.GroupContamination] {
			t.Error("expected content and contamination disabled")
		}
	})

	t.Run("skip contamination", func(t *testing.T) {
		enabled, err := resolveCheckGroups("", "contamination")
		if err != nil {
			t.Fatal(err)
		}
		if !enabled[orchestrate.GroupStructure] || !enabled[orchestrate.GroupLinks] || !enabled[orchestrate.GroupContent] {
			t.Error("expected structure, links, content enabled")
		}
		if enabled[orchestrate.GroupContamination] {
			t.Error("expected contamination disabled")
		}
	})

	t.Run("invalid group", func(t *testing.T) {
		_, err := resolveCheckGroups("structure,bogus", "")
		if err == nil {
			t.Error("expected error for invalid group")
		}
	})
}

func TestResolvePath_ValidDir(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	resolved, err := resolvePath([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != dir {
		t.Errorf("expected %s, got %s", dir, resolved)
	}
}

func TestResolvePath_NoArgs(t *testing.T) {
	_, err := resolvePath([]string{})
	if err == nil {
		t.Error("expected error for empty args")
	}
}

func TestResolvePath_NotADirectory(t *testing.T) {
	path := filepath.Join(fixtureDir(t, "valid-skill"), "SKILL.md")
	_, err := resolvePath([]string{path})
	if err == nil {
		t.Error("expected error for file path")
	}
	if !strings.Contains(err.Error(), "not a valid directory") {
		t.Errorf("expected 'not a valid directory' error, got: %v", err)
	}
}

func TestDetectAndResolve_SingleSkill(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	_, mode, dirs, err := detectAndResolve([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != types.SingleSkill {
		t.Errorf("expected SingleSkill, got %d", mode)
	}
	if len(dirs) != 1 {
		t.Errorf("expected 1 dir, got %d", len(dirs))
	}
}

func TestDetectAndResolve_MultiSkill(t *testing.T) {
	dir := fixtureDir(t, "multi-skill")
	_, mode, dirs, err := detectAndResolve([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != types.MultiSkill {
		t.Errorf("expected MultiSkill, got %d", mode)
	}
	if len(dirs) < 2 {
		t.Errorf("expected multiple dirs, got %d", len(dirs))
	}
}

func TestDetectAndResolve_NoSkill(t *testing.T) {
	dir := t.TempDir()
	_, _, _, err := detectAndResolve([]string{dir})
	if err == nil {
		t.Error("expected error for directory with no skills")
	}
	if !strings.Contains(err.Error(), "no skills found") {
		t.Errorf("expected 'no skills found' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Command execution tests — idiomatic Cobra pattern: set args, execute,
// check output/error. These test the CLI glue (flag parsing, validation,
// routing) without re-testing the underlying library.
// ---------------------------------------------------------------------------

func TestScoreEvaluate_RequiresModel(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	// Isolate from any real project config that might supply a model.
	noConfigDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(noConfigDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(noConfigDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_, _, err = executeCommand("score", "evaluate", dir)
	if err == nil {
		t.Fatal("expected error when --model is missing")
	}
	if !strings.Contains(err.Error(), "--model is required") {
		t.Errorf("expected '--model is required' error, got: %v", err)
	}
}

func TestScoreEvaluate_ProviderGating(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")

	tests := []struct {
		name     string
		provider string
		wantErr  string
	}{
		{"anthropic rejected", "anthropic", "not supported in skill-validator-ent"},
		{"openai rejected", "openai", "not supported in skill-validator-ent"},
		{"unknown rejected", "cohere", "unsupported provider"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := executeCommand("score", "evaluate",
				"--model", "some-model",
				"--provider", tt.provider,
				dir)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestScoreEvaluate_MutuallyExclusiveFlags(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	_, _, err := executeCommand("score", "evaluate",
		"--model", "some-model",
		"--skill-only", "--refs-only",
		dir)
	if err == nil {
		t.Fatal("expected error for --skill-only + --refs-only")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want 'mutually exclusive'", err)
	}
}

func TestScoreEvaluate_InvalidDisplay(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	_, _, err := executeCommand("score", "evaluate",
		"--model", "some-model",
		"--display", "bogus",
		dir)
	if err == nil {
		t.Fatal("expected error for invalid --display")
	}
	if !strings.Contains(err.Error(), `--display must be`) {
		t.Errorf("error = %q, want '--display must be'", err)
	}
}

func TestScoreEvaluate_PathNotFound(t *testing.T) {
	_, _, err := executeCommand("score", "evaluate",
		"--model", "some-model",
		"/nonexistent/path/xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	if !strings.Contains(err.Error(), "path not found") {
		t.Errorf("error = %q, want 'path not found'", err)
	}
}

func TestCheck_OnlySkipMutuallyExclusive(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	_, _, err := executeCommand("check",
		"--only", "structure",
		"--skip", "links",
		dir)
	if err == nil {
		t.Fatal("expected error for --only + --skip")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want 'mutually exclusive'", err)
	}
}

func TestCheck_InvalidGroup(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	_, _, err := executeCommand("check", "--only", "bogus", dir)
	if err == nil {
		t.Fatal("expected error for unknown check group")
	}
	if !strings.Contains(err.Error(), "unknown check group") {
		t.Errorf("error = %q, want 'unknown check group'", err)
	}
}

func TestValidateStructure_ViaCommand(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	// Output goes to os.Stdout (not cobra buffer) since report
	// functions write there directly. We verify the command succeeds.
	_, _, err := executeCommand("validate", "structure", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateStructure_InvalidSkillExitCode(t *testing.T) {
	dir := fixtureDir(t, "invalid-skill")
	_, _, err := executeCommand("validate", "structure", dir)

	ec, ok := err.(exitCodeError)
	if !ok {
		t.Fatalf("expected exitCodeError, got %T: %v", err, err)
	}
	if ec.code != ExitError {
		t.Errorf("exit code = %d, want %d", ec.code, ExitError)
	}
}

func TestValidateStructure_StrictMode(t *testing.T) {
	dir := fixtureDir(t, "warnings-only-skill")
	// Without --strict, warnings produce exit code 2.
	_, _, err := executeCommand("validate", "structure", dir)
	ec, ok := err.(exitCodeError)
	if !ok || ec.code != ExitWarning {
		t.Fatalf("expected exit code %d, got %v", ExitWarning, err)
	}

	// With --strict, warnings produce exit code 1.
	_, _, err = executeCommand("validate", "structure", "--strict", dir)
	ec, ok = err.(exitCodeError)
	if !ok || ec.code != ExitError {
		t.Fatalf("expected exit code %d with --strict, got %v", ExitError, err)
	}
}

func TestCheck_ValidSkill(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	_, _, err := executeCommand("check", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheck_OnlyStructure(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	_, _, err := executeCommand("check", "--only", "structure", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvalMaxLen(t *testing.T) {
	evalFullContent = false
	if got := evalMaxLen(); got != 8000 {
		t.Errorf("evalMaxLen() = %d, want 8000", got)
	}

	evalFullContent = true
	if got := evalMaxLen(); got != 0 {
		t.Errorf("evalMaxLen() with full-content = %d, want 0", got)
	}
	evalFullContent = false // reset
}

func TestVersion(t *testing.T) {
	stdout, _, err := executeCommand("--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, version) {
		t.Errorf("expected version %q in output, got: %s", version, stdout)
	}
}

func TestCommandTree(t *testing.T) {
	// Verify all expected subcommands are registered.
	subs := map[string]bool{
		"validate": false,
		"analyze":  false,
		"score":    false,
		"check":    false,
	}
	for _, cmd := range rootCmd.Commands() {
		if _, ok := subs[cmd.Name()]; ok {
			subs[cmd.Name()] = true
		}
	}
	for name, found := range subs {
		if !found {
			t.Errorf("expected subcommand %q not found on root", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Config integration tests
// ---------------------------------------------------------------------------

// setupProjectConfig creates a temp project root with .git and a config file,
// clears user config env vars, and chdirs into the project root. It registers
// cleanup to restore the original working directory.
func setupProjectConfig(t *testing.T, yaml string) {
	t.Helper()
	projRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projRoot, ".skill-validator-ent.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(projRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
}

func TestConfig_ModelFromConfigFile(t *testing.T) {
	setupProjectConfig(t, "model: config-model\n")

	// score evaluate with a nonexistent path — we expect a path error,
	// NOT a "model is required" error, because the config should supply it.
	_, _, err := executeCommand("score", "evaluate", "/nonexistent/path/xyz")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "--model is required") {
		t.Errorf("config should have supplied model, got: %v", err)
	}
}

func TestConfig_CLIFlagOverridesConfig(t *testing.T) {
	// Resolve fixture dir BEFORE changing cwd.
	dir := fixtureDir(t, "valid-skill")
	setupProjectConfig(t, "model: config-model\nprovider: anthropic\n")

	// --provider flag should override config's "anthropic" with "bedrock".
	// --model flag should override config's "config-model".
	// This should get past model and provider checks, then fail at AWS.
	_, _, err := executeCommand("score", "evaluate",
		"--provider", "bedrock",
		"--model", "cli-model",
		dir)
	// Should fail at AWS client build, not at flag validation.
	if err != nil && strings.Contains(err.Error(), "--model is required") {
		t.Errorf("CLI flag should have overridden config, got: %v", err)
	}
}

func TestConfig_VerboseFlag(t *testing.T) {
	_, _, err := executeCommand("--verbose", "--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfig_StrictFromConfig(t *testing.T) {
	dir := fixtureDir(t, "warnings-only-skill")
	setupProjectConfig(t, "strict: true\n")

	// Without config strict, warnings-only-skill exits with ExitWarning (2).
	// With strict: true from config, warnings should be promoted to ExitError (1).
	_, _, err := executeCommand("validate", "structure", dir)
	ec, ok := err.(exitCodeError)
	if !ok {
		t.Fatalf("expected exitCodeError, got %T: %v", err, err)
	}
	if ec.code != ExitError {
		t.Errorf("exit code = %d, want %d (strict from config should promote warnings)", ec.code, ExitError)
	}
}

func TestConfig_StrictFromConfigOverriddenByCLI(t *testing.T) {
	dir := fixtureDir(t, "warnings-only-skill")
	setupProjectConfig(t, "strict: true\n")

	// Explicit --strict=false on CLI should override config's strict: true.
	_, _, err := executeCommand("validate", "structure", "--strict=false", dir)
	ec, ok := err.(exitCodeError)
	if !ok {
		t.Fatalf("expected exitCodeError, got %T: %v", err, err)
	}
	if ec.code != ExitWarning {
		t.Errorf("exit code = %d, want %d (CLI --strict=false should override config)", ec.code, ExitWarning)
	}
}

func TestConfig_OutputFormatFromConfig(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	setupProjectConfig(t, "output: json\n")

	// Run a command so PersistentPreRunE applies config.
	_, _, _ = executeCommand("validate", "structure", dir)
	// The report writes to os.Stdout directly (not cobra buffer), so we
	// verify the config was applied by checking the variable.
	if outputFormat != "json" {
		t.Errorf("outputFormat = %s, want json (from config)", outputFormat)
	}
}

func TestConfig_EmitAnnotationsFromConfig(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	setupProjectConfig(t, "emit_annotations: true\n")

	// Run a command so PersistentPreRunE applies config.
	_, _, _ = executeCommand("validate", "structure", dir)
	if !emitAnnotations {
		t.Error("emitAnnotations = false, want true (from config)")
	}
}

func TestConfig_DisplayAndMaxTokensFromConfig(t *testing.T) {
	// This test verifies that display and max_response_tokens config values
	// are applied to score evaluate flags. We can't run a full score without
	// AWS credentials, but we can verify the config doesn't cause a model error
	// and instead fails at the AWS stage.
	setupProjectConfig(t, "model: cfg-model\ndisplay: files\nmax_response_tokens: 200\nfull_content: true\n")

	_, _, err := executeCommand("score", "evaluate", "/nonexistent/path/xyz")
	if err == nil {
		t.Fatal("expected error")
	}
	// Should NOT fail on model-required or display validation.
	errStr := err.Error()
	if strings.Contains(errStr, "--model is required") {
		t.Errorf("config should have supplied model, got: %v", err)
	}
	if strings.Contains(errStr, "--display must be") {
		t.Errorf("config display: files should be valid, got: %v", err)
	}
}

func TestConfig_ModelNotAppliedToScoreReport(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	setupProjectConfig(t, "model: us.anthropic.claude-sonnet-4-5-20250929-v1:0\n")

	// score report should NOT have its --model filter populated by config.
	// The config "model" is for score evaluate (which model to call), not
	// score report (which model to filter cached results by).
	_, _, _ = executeCommand("score", "report", dir)
	if reportModel != "" {
		t.Errorf("config model should not apply to score report --model filter, got %q", reportModel)
	}
}

func TestConfig_ModelNotAppliedToScoreReportCompare(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	setupProjectConfig(t, "model: us.anthropic.claude-sonnet-4-5-20250929-v1:0\n")

	// --compare should show ALL models, not be filtered to the config model.
	_, _, _ = executeCommand("score", "report", "--compare", dir)
	if reportModel != "" {
		t.Errorf("config model should not filter --compare results, got %q", reportModel)
	}
}

func TestConfig_ScoreReportExplicitModelStillFilters(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	setupProjectConfig(t, "model: config-model\n")

	// Explicit --model on CLI should still work as a filter for score report.
	_, _, _ = executeCommand("score", "report", "--model", "explicit-filter", dir)
	if reportModel != "explicit-filter" {
		t.Errorf("explicit --model should be used as filter, got %q", reportModel)
	}
}

func TestConfig_VerboseEnvVar(t *testing.T) {
	t.Setenv("SKILL_VALIDATOR_DEBUG", "1")
	_, _, err := executeCommand("--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfig_NoConfigBehavesLikeToday(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")

	// Chdir to a temp dir with .git so no project config is found.
	noConfigDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(noConfigDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(noConfigDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// With no config files and no --model flag, should get model-required error.
	_, _, err = executeCommand("score", "evaluate", dir)
	if err == nil {
		t.Fatal("expected error when --model is missing")
	}
	if !strings.Contains(err.Error(), "--model is required") {
		t.Errorf("expected '--model is required' error, got: %v", err)
	}
}
