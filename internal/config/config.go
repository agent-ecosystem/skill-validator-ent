package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config uses pointer fields so we can distinguish "not set" from zero values.
type Config struct {
	Model             *string `yaml:"model"`
	Region            *string `yaml:"region"`
	Profile           *string `yaml:"profile"`
	Provider          *string `yaml:"provider"`
	MaxResponseTokens *int32  `yaml:"max_response_tokens"`
	Display           *string `yaml:"display"`
	FullContent       *bool   `yaml:"full_content"`
	Output            *string `yaml:"output"`
	EmitAnnotations   *bool   `yaml:"emit_annotations"`
	Strict            *bool   `yaml:"strict"`
}

// ParseFile reads and unmarshals a single YAML config file.
func ParseFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Merge returns a new Config where override's non-nil fields win over base.
func Merge(base, override *Config) *Config {
	out := &Config{}
	if base != nil {
		*out = *base
	}
	if override == nil {
		return out
	}
	if override.Model != nil {
		out.Model = override.Model
	}
	if override.Region != nil {
		out.Region = override.Region
	}
	if override.Profile != nil {
		out.Profile = override.Profile
	}
	if override.Provider != nil {
		out.Provider = override.Provider
	}
	if override.MaxResponseTokens != nil {
		out.MaxResponseTokens = override.MaxResponseTokens
	}
	if override.Display != nil {
		out.Display = override.Display
	}
	if override.FullContent != nil {
		out.FullContent = override.FullContent
	}
	if override.Output != nil {
		out.Output = override.Output
	}
	if override.EmitAnnotations != nil {
		out.EmitAnnotations = override.EmitAnnotations
	}
	if override.Strict != nil {
		out.Strict = override.Strict
	}
	return out
}

// FindProjectConfig walks from startDir up to the git root looking for
// .skill-validator-ent.yaml. Returns the path if found, or empty string.
func FindProjectConfig(startDir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, ".skill-validator-ent.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		// Stop at git root.
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // reached filesystem root
		}
		dir = parent
	}
}

// FindUserConfig resolves the user-level config file path using:
// 1. SKILL_VALIDATOR_CONFIG_DIR env var
// 2. XDG_CONFIG_HOME/skill-validator-ent/config.yaml
// 3. ~/.config/skill-validator-ent/config.yaml
// Returns the path if the file exists, or empty string.
func FindUserConfig() string {
	if envDir := os.Getenv("SKILL_VALIDATOR_CONFIG_DIR"); envDir != "" {
		candidate := filepath.Join(envDir, "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		return ""
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidate := filepath.Join(xdg, "skill-validator-ent", "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		return ""
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(home, ".config", "skill-validator-ent", "config.yaml")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// Load discovers, parses, and merges config files. Returns the merged config
// and a list of loaded file paths (for verbose logging). Parse errors are
// logged via slog but not fatal (graceful degradation).
func Load(startDir string) (*Config, []string) {
	merged := &Config{}
	var loaded []string

	// User-level config (base).
	if userPath := FindUserConfig(); userPath != "" {
		cfg, err := ParseFile(userPath)
		if err != nil {
			slog.Warn("failed to parse user config", "path", userPath, "error", err)
		} else {
			merged = Merge(merged, cfg)
			loaded = append(loaded, userPath)
		}
	}

	// Project-level config (override).
	if projectPath := FindProjectConfig(startDir); projectPath != "" {
		cfg, err := ParseFile(projectPath)
		if err != nil {
			slog.Warn("failed to parse project config", "path", projectPath, "error", err)
		} else {
			merged = Merge(merged, cfg)
			loaded = append(loaded, projectPath)
		}
	}

	return merged, loaded
}
