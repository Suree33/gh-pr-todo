// Package config provides severity configuration loading from YAML files.
// It supports global, local repo, and remote config sources with defined
// precedence: narrower scope (more specific) overrides broader scope.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Suree33/gh-pr-todo/internal/todotype"
	"gopkg.in/yaml.v3"
)

// Config holds parsed severity overrides from configuration files.
type Config struct {
	Severities map[string]todotype.Severity
}

// File represents the YAML configuration file schema.
type File struct {
	Severity map[string]string `yaml:"severity"`
}

// Parse parses YAML config data and validates severity values.
// source is used in error messages to identify the file origin.
func Parse(data []byte, source string) (Config, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return Config{}, fmt.Errorf("%s: invalid YAML: %w", source, err)
	}

	if len(f.Severity) == 0 {
		return Config{Severities: make(map[string]todotype.Severity)}, nil
	}

	severities := make(map[string]todotype.Severity, len(f.Severity))
	for todoType, levelStr := range f.Severity {
		normalizedType := strings.TrimSpace(todoType)
		if normalizedType == "" {
			return Config{}, fmt.Errorf("%s: type name is empty", source)
		}
		normalizedType = strings.ToUpper(normalizedType)

		var sev todotype.Severity
		switch strings.ToLower(strings.TrimSpace(levelStr)) {
		case "notice":
			sev = todotype.SeverityNotice
		case "warning":
			sev = todotype.SeverityWarning
		case "error":
			sev = todotype.SeverityError
		default:
			return Config{}, fmt.Errorf("%s: invalid severity %q for type %q: allowed values are notice, warning, error",
				source, levelStr, todoType)
		}
		severities[normalizedType] = sev
	}

	return Config{Severities: severities}, nil
}

// discoverRepoRoot walks up from cwd looking for a .git directory or file.
func discoverRepoRoot(cwd string) (string, bool) {
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}
	return "", false
}

// LoadGlobal loads the user-level config file.
func LoadGlobal(userConfigDir string) (Config, error) {
	merged := Config{Severities: make(map[string]todotype.Severity)}
	if userConfigDir == "" {
		return merged, nil
	}
	globalPath := filepath.Join(userConfigDir, "gh-pr-todo", "config.yml")
	if err := mergeFile(globalPath, &merged); err != nil {
		return merged, err
	}
	return merged, nil
}

// LoadLocal loads and merges config files from global and local paths.
// cwd is the current working directory used to discover the repo root.
// userConfigDir is typically os.UserConfigDir().
//
// Precedence (later wins): global < repo root .gh-pr-todo.yml < repo .github/gh-pr-todo.yml.
// Missing files are silently ignored; parse errors are returned.
func LoadLocal(cwd, userConfigDir string) (Config, error) {
	merged, err := LoadGlobal(userConfigDir)
	if err != nil {
		return merged, err
	}

	// Repository root discovery
	repoRoot, found := discoverRepoRoot(cwd)
	if !found {
		return merged, nil
	}

	// Repo root config: <repo>/.gh-pr-todo.yml
	rootPath := filepath.Join(repoRoot, ".gh-pr-todo.yml")
	if err := mergeFile(rootPath, &merged); err != nil {
		return merged, err
	}

	// Narrower scope config: <repo>/.github/gh-pr-todo.yml
	narrowPath := filepath.Join(repoRoot, ".github", "gh-pr-todo.yml")
	if err := mergeFile(narrowPath, &merged); err != nil {
		return merged, err
	}

	return merged, nil
}

// mergeFile reads a YAML config file at path and merges its severity
// overrides into the target Config. Missing files are silently skipped.
func mergeFile(path string, target *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}

	cfg, err := Parse(data, path)
	if err != nil {
		return err
	}

	for k, v := range cfg.Severities {
		target.Severities[k] = v
	}
	return nil
}
