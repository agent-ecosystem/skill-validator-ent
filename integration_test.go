// Package integration_test runs full-binary integration tests.
//
// These tests build the actual binary and run it as a subprocess, verifying
// exit codes, stdout, and stderr. This catches wiring issues that unit tests
// miss: broken imports, init() registration, output formatting end-to-end.
//
// Gated behind: go test -tags integration ./...
//
//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all tests.
	tmp, err := os.MkdirTemp("", "skill-validator-ent-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	binaryPath = filepath.Join(tmp, "skill-validator-ent")
	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/skill-validator-ent")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

// run executes the binary with the given args and returns stdout, stderr,
// and the exit code. A non-zero exit code is not treated as a test failure
// — the caller decides what to assert.
func run(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("failed to run binary: %v", err)
	}

	return outBuf.String(), errBuf.String(), exitCode
}

func TestBinary_Version(t *testing.T) {
	stdout, _, code := run(t, "--version")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "skill-validator-ent version") {
		t.Errorf("expected version string, got: %s", stdout)
	}
}

func TestBinary_Help(t *testing.T) {
	stdout, _, code := run(t, "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, sub := range []string{"validate", "analyze", "score", "check"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("expected %q in help output", sub)
		}
	}
}

func TestBinary_ValidateStructure_ValidSkill(t *testing.T) {
	stdout, _, code := run(t, "validate", "structure", "testdata/valid-skill")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "SKILL.md found") {
		t.Error("expected 'SKILL.md found' in output")
	}
	if !strings.Contains(stdout, "passed") {
		t.Error("expected 'passed' in output")
	}
}

func TestBinary_ValidateStructure_InvalidSkill(t *testing.T) {
	_, _, code := run(t, "validate", "structure", "testdata/invalid-skill")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (validation errors)", code)
	}
}

func TestBinary_ValidateStructure_Strict(t *testing.T) {
	_, _, code := run(t, "validate", "structure", "testdata/warnings-only-skill")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (warnings)", code)
	}

	_, _, code = run(t, "validate", "structure", "--strict", "testdata/warnings-only-skill")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (strict promotes warnings)", code)
	}
}

func TestBinary_ValidateStructure_JSONOutput(t *testing.T) {
	stdout, _, code := run(t, "validate", "structure", "-o", "json", "testdata/valid-skill")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	trimmed := strings.TrimSpace(stdout)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		t.Errorf("expected JSON object, got:\n%.200s", stdout)
	}
}

func TestBinary_ValidateStructure_MarkdownOutput(t *testing.T) {
	stdout, _, code := run(t, "validate", "structure", "-o", "markdown", "testdata/valid-skill")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "#") {
		t.Error("expected markdown heading in output")
	}
}

func TestBinary_ValidateLinks(t *testing.T) {
	_, _, code := run(t, "validate", "links", "testdata/valid-skill")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestBinary_AnalyzeContent(t *testing.T) {
	stdout, _, code := run(t, "analyze", "content", "testdata/valid-skill")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Content") {
		t.Error("expected content analysis in output")
	}
}

func TestBinary_AnalyzeContamination(t *testing.T) {
	stdout, _, code := run(t, "analyze", "contamination", "testdata/valid-skill")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Contamination") {
		t.Error("expected contamination analysis in output")
	}
}

func TestBinary_Check_ValidSkill(t *testing.T) {
	_, _, code := run(t, "check", "testdata/valid-skill")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestBinary_Check_OnlyStructure(t *testing.T) {
	_, _, code := run(t, "check", "--only", "structure", "testdata/valid-skill")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestBinary_Check_MultiSkill(t *testing.T) {
	_, _, code := run(t, "check", "testdata/multi-skill")
	// multi-skill fixture may have errors or warnings; accept 0, 1, or 2
	if code == 3 {
		t.Fatalf("exit code = 3 (CLI error), expected a validation exit code (0, 1, or 2)")
	}
}

func TestBinary_ScoreEvaluate_MissingModel(t *testing.T) {
	_, stderr, code := run(t, "score", "evaluate", "testdata/valid-skill")
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (CLI error)", code)
	}
	if !strings.Contains(stderr, "model") {
		t.Errorf("expected 'model' in error output, got: %s", stderr)
	}
}

func TestBinary_ScoreEvaluate_ProviderRejected(t *testing.T) {
	_, stderr, code := run(t, "score", "evaluate",
		"--model", "test", "--provider", "anthropic",
		"testdata/valid-skill")
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (CLI error)", code)
	}
	if !strings.Contains(stderr, "not supported") {
		t.Errorf("expected 'not supported' in error, got: %s", stderr)
	}
}

func TestBinary_ScoreEvaluate_MutuallyExclusive(t *testing.T) {
	_, stderr, code := run(t, "score", "evaluate",
		"--model", "test", "--skill-only", "--refs-only",
		"testdata/valid-skill")
	if code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %s", stderr)
	}
}

func TestBinary_NoArgs(t *testing.T) {
	stdout, _, code := run(t, "validate", "structure")
	// Missing path arg — cobra prints usage and exits non-zero
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing path arg, got 0\n%s", stdout)
	}
}

func TestBinary_NonexistentPath(t *testing.T) {
	_, stderr, code := run(t, "validate", "structure", "/nonexistent/path")
	if code == 0 {
		t.Fatal("expected non-zero exit for nonexistent path")
	}
	if !strings.Contains(stderr, "not a valid directory") {
		t.Errorf("expected 'not a valid directory' in error, got: %s", stderr)
	}
}

func TestBinary_EmitAnnotations(t *testing.T) {
	// valid-skill passes, so no annotations emitted — just verify the flag is accepted
	_, _, code := run(t, "validate", "structure", "--emit-annotations", "testdata/valid-skill")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}
