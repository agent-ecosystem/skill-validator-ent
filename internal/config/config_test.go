package config

import (
	"os"
	"path/filepath"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	mustWriteFile(t, path, []byte("model: my-model\nregion: us-west-2\n"))

	cfg, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model == nil || *cfg.Model != "my-model" {
		t.Errorf("model = %v, want my-model", cfg.Model)
	}
	if cfg.Region == nil || *cfg.Region != "us-west-2" {
		t.Errorf("region = %v, want us-west-2", cfg.Region)
	}
	if cfg.Profile != nil {
		t.Errorf("profile should be nil, got %v", *cfg.Profile)
	}
}

func TestParseFile_AllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
model: test-model
region: eu-west-1
profile: myprofile
provider: bedrock
max_response_tokens: 1000
display: files
full_content: true
output: json
emit_annotations: true
strict: true
`
	mustWriteFile(t, path, []byte(content))

	cfg, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if *cfg.Model != "test-model" {
		t.Errorf("model = %s", *cfg.Model)
	}
	if *cfg.MaxResponseTokens != 1000 {
		t.Errorf("max_response_tokens = %d", *cfg.MaxResponseTokens)
	}
	if *cfg.FullContent != true {
		t.Error("full_content should be true")
	}
	if *cfg.EmitAnnotations != true {
		t.Error("emit_annotations should be true")
	}
	if *cfg.Strict != true {
		t.Error("strict should be true")
	}
}

func TestParseFile_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	mustWriteFile(t, path, []byte(":::bad yaml[[["))

	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestParseFile_NotFound(t *testing.T) {
	_, err := ParseFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseFile_UnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	mustWriteFile(t, path, []byte("model: m\nunknown_key: whatever\n"))

	cfg, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unknown keys should not cause error: %v", err)
	}
	if *cfg.Model != "m" {
		t.Errorf("model = %s", *cfg.Model)
	}
}

func TestMerge(t *testing.T) {
	base := &Config{
		Model:  ptr("base-model"),
		Region: ptr("us-east-1"),
	}
	override := &Config{
		Model:   ptr("override-model"),
		Profile: ptr("override-profile"),
	}

	merged := Merge(base, override)

	if *merged.Model != "override-model" {
		t.Errorf("model = %s, want override-model", *merged.Model)
	}
	if *merged.Region != "us-east-1" {
		t.Errorf("region = %s, want us-east-1 (from base)", *merged.Region)
	}
	if *merged.Profile != "override-profile" {
		t.Errorf("profile = %s, want override-profile", *merged.Profile)
	}
}

func TestMerge_AllFields(t *testing.T) {
	// Base has every field set.
	base := &Config{
		Model:             ptr("base-model"),
		Region:            ptr("base-region"),
		Profile:           ptr("base-profile"),
		Provider:          ptr("base-provider"),
		MaxResponseTokens: ptr(int32(100)),
		Display:           ptr("base-display"),
		FullContent:       ptr(false),
		Output:            ptr("text"),
		EmitAnnotations:   ptr(false),
		Strict:            ptr(false),
	}

	// Override sets every field to a different value.
	override := &Config{
		Model:             ptr("over-model"),
		Region:            ptr("over-region"),
		Profile:           ptr("over-profile"),
		Provider:          ptr("over-provider"),
		MaxResponseTokens: ptr(int32(999)),
		Display:           ptr("over-display"),
		FullContent:       ptr(true),
		Output:            ptr("json"),
		EmitAnnotations:   ptr(true),
		Strict:            ptr(true),
	}

	merged := Merge(base, override)

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Model", *merged.Model, "over-model"},
		{"Region", *merged.Region, "over-region"},
		{"Profile", *merged.Profile, "over-profile"},
		{"Provider", *merged.Provider, "over-provider"},
		{"MaxResponseTokens", *merged.MaxResponseTokens, int32(999)},
		{"Display", *merged.Display, "over-display"},
		{"FullContent", *merged.FullContent, true},
		{"Output", *merged.Output, "json"},
		{"EmitAnnotations", *merged.EmitAnnotations, true},
		{"Strict", *merged.Strict, true},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestMerge_BasePreservedWhenOverrideNil(t *testing.T) {
	// Base has every field; override is empty.
	base := &Config{
		Model:             ptr("kept-model"),
		Region:            ptr("kept-region"),
		Profile:           ptr("kept-profile"),
		Provider:          ptr("kept-provider"),
		MaxResponseTokens: ptr(int32(42)),
		Display:           ptr("kept-display"),
		FullContent:       ptr(true),
		Output:            ptr("markdown"),
		EmitAnnotations:   ptr(true),
		Strict:            ptr(true),
	}
	override := &Config{} // all nil

	merged := Merge(base, override)

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Model", *merged.Model, "kept-model"},
		{"Region", *merged.Region, "kept-region"},
		{"Profile", *merged.Profile, "kept-profile"},
		{"Provider", *merged.Provider, "kept-provider"},
		{"MaxResponseTokens", *merged.MaxResponseTokens, int32(42)},
		{"Display", *merged.Display, "kept-display"},
		{"FullContent", *merged.FullContent, true},
		{"Output", *merged.Output, "markdown"},
		{"EmitAnnotations", *merged.EmitAnnotations, true},
		{"Strict", *merged.Strict, true},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestMerge_NilInputs(t *testing.T) {
	t.Run("nil base", func(t *testing.T) {
		override := &Config{Model: ptr("m")}
		merged := Merge(nil, override)
		if *merged.Model != "m" {
			t.Errorf("model = %s", *merged.Model)
		}
	})

	t.Run("nil override", func(t *testing.T) {
		base := &Config{Model: ptr("m")}
		merged := Merge(base, nil)
		if *merged.Model != "m" {
			t.Errorf("model = %s", *merged.Model)
		}
	})

	t.Run("both nil", func(t *testing.T) {
		merged := Merge(nil, nil)
		if merged.Model != nil {
			t.Error("expected nil model")
		}
	})
}

func TestFindProjectConfig(t *testing.T) {
	// Create a fake project tree:
	// root/.git/
	// root/.skill-validator-ent.yaml
	// root/sub/deep/
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	configPath := filepath.Join(root, ".skill-validator-ent.yaml")
	mustWriteFile(t, configPath, []byte("model: proj-model\n"))

	deep := filepath.Join(root, "sub", "deep")
	mustMkdirAll(t, deep)

	found := FindProjectConfig(deep)
	if found != configPath {
		t.Errorf("found = %s, want %s", found, configPath)
	}
}

func TestFindProjectConfig_NoConfig(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))

	found := FindProjectConfig(root)
	if found != "" {
		t.Errorf("expected empty, got %s", found)
	}
}

func TestFindProjectConfig_StopsAtGitRoot(t *testing.T) {
	// Config is above the git root — should NOT be found.
	outer := t.TempDir()
	mustWriteFile(t, filepath.Join(outer, ".skill-validator-ent.yaml"), []byte("model: x\n"))

	inner := filepath.Join(outer, "repo")
	mustMkdirAll(t, filepath.Join(inner, ".git"))

	found := FindProjectConfig(inner)
	if found != "" {
		t.Errorf("expected empty (config above git root), got %s", found)
	}
}

func TestFindUserConfig(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "skill-validator-ent")
	mustMkdirAll(t, configDir)
	mustWriteFile(t, filepath.Join(configDir, "config.yaml"), []byte("model: user-model\n"))

	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", configDir)
	t.Setenv("XDG_CONFIG_HOME", "") // clear to test env var priority

	found := FindUserConfig()
	if found != filepath.Join(configDir, "config.yaml") {
		t.Errorf("found = %s", found)
	}
}

func TestFindUserConfig_XDG(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "skill-validator-ent")
	mustMkdirAll(t, configDir)
	mustWriteFile(t, filepath.Join(configDir, "config.yaml"), []byte("model: xdg-model\n"))

	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", dir)

	found := FindUserConfig()
	if found != filepath.Join(configDir, "config.yaml") {
		t.Errorf("found = %s", found)
	}
}

func TestFindUserConfig_DefaultHome(t *testing.T) {
	// Simulate the ~/.config/ fallback by pointing HOME to a temp dir.
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "skill-validator-ent")
	mustMkdirAll(t, configDir)
	configPath := filepath.Join(configDir, "config.yaml")
	mustWriteFile(t, configPath, []byte("model: home-model\n"))

	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)

	found := FindUserConfig()
	if found != configPath {
		t.Errorf("found = %s, want %s", found, configPath)
	}
}

func TestFindUserConfig_EnvDirMissingFile(t *testing.T) {
	// SKILL_VALIDATOR_CONFIG_DIR is set but contains no config.yaml.
	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	found := FindUserConfig()
	if found != "" {
		t.Errorf("expected empty when config dir has no file, got %s", found)
	}
}

func TestFindUserConfig_NoConfig(t *testing.T) {
	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // empty dir, no config

	found := FindUserConfig()
	if found != "" {
		t.Errorf("expected empty, got %s", found)
	}
}

func TestLoad(t *testing.T) {
	// Set up user config.
	userDir := t.TempDir()
	configDir := filepath.Join(userDir, "skill-validator-ent")
	mustMkdirAll(t, configDir)
	mustWriteFile(t, filepath.Join(configDir, "config.yaml"), []byte("model: user-model\nregion: us-east-1\n"))
	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", configDir)

	// Set up project config.
	projRoot := t.TempDir()
	mustMkdirAll(t, filepath.Join(projRoot, ".git"))
	mustWriteFile(t, filepath.Join(projRoot, ".skill-validator-ent.yaml"), []byte("model: proj-model\n"))

	cfg, loaded := Load(projRoot)
	if len(loaded) != 2 {
		t.Fatalf("loaded %d files, want 2", len(loaded))
	}
	// Project overrides user.
	if *cfg.Model != "proj-model" {
		t.Errorf("model = %s, want proj-model", *cfg.Model)
	}
	// User region preserved.
	if *cfg.Region != "us-east-1" {
		t.Errorf("region = %s, want us-east-1", *cfg.Region)
	}
}

func TestLoad_MalformedYAMLGraceful(t *testing.T) {
	// Malformed project config should be skipped gracefully.
	projRoot := t.TempDir()
	mustMkdirAll(t, filepath.Join(projRoot, ".git"))
	mustWriteFile(t, filepath.Join(projRoot, ".skill-validator-ent.yaml"), []byte(":::bad[[["))

	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, loaded := Load(projRoot)
	if len(loaded) != 0 {
		t.Errorf("loaded %d files, want 0 (malformed should be skipped)", len(loaded))
	}
	if cfg.Model != nil {
		t.Error("expected nil model after malformed config")
	}
}

func TestLoad_NoConfigs(t *testing.T) {
	t.Setenv("SKILL_VALIDATOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	mustMkdirAll(t, filepath.Join(dir, ".git"))

	cfg, loaded := Load(dir)
	if len(loaded) != 0 {
		t.Errorf("loaded %d files, want 0", len(loaded))
	}
	if cfg.Model != nil {
		t.Error("expected nil model with no config files")
	}
}
