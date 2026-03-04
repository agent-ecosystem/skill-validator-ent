package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	// check flags
	checkOnly = ""
	checkSkip = ""
	perFileCheck = false
	checkSkipOrphans = false
	strictCheck = false

	// validate structure flags
	skipOrphans = false
	strictStructure = false

	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
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
	_, _, err := executeCommand("score", "evaluate", "testdata/valid-skill")
	if err == nil {
		t.Fatal("expected error when --model is missing")
	}
	if !strings.Contains(err.Error(), "required flag") {
		t.Errorf("expected 'required flag' error, got: %v", err)
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
