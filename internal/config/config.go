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

// Config holds parsed severity overrides and ignored types from configuration files.
type Config struct {
	Severities map[string]todotype.Severity
	Ignored    map[string]bool
	Found      bool // true if at least one config file was found and parsed
}

// File represents the YAML configuration file schema.
type File struct {
	Severity map[string][]string `yaml:"severity"`
	Ignore   []string            `yaml:"ignore"`
}

// Parse parses YAML config data and validates severity values and ignore list.
// source is used in error messages to identify the file origin.
func Parse(data []byte, source string) (Config, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return Config{}, fmt.Errorf("%s: invalid YAML: %w", source, err)
	}

	cfg := Config{Found: true}

	// Parse severity overrides
	if len(f.Severity) > 0 {
		severities := make(map[string]todotype.Severity)
		for levelStr, typeNames := range f.Severity {
			normalizedLevel := strings.ToLower(strings.TrimSpace(levelStr))

			var sev todotype.Severity
			switch normalizedLevel {
			case "notice":
				sev = todotype.SeverityNotice
			case "warning":
				sev = todotype.SeverityWarning
			case "error":
				sev = todotype.SeverityError
			default:
				return Config{}, fmt.Errorf("%s: invalid severity key %q: allowed values are notice, warning, error",
					source, levelStr)
			}

			for _, typeName := range typeNames {
				normalizedType := strings.TrimSpace(typeName)
				if normalizedType == "" {
					return Config{}, fmt.Errorf("%s: type name is empty in severity %q", source, normalizedLevel)
				}
				normalizedType = strings.ToUpper(normalizedType)

				if existingSev, exists := severities[normalizedType]; exists && existingSev != sev {
					return Config{}, fmt.Errorf("%s: type %q appears under multiple severity levels (%s and %s)",
						source, normalizedType, existingSev, sev)
				}
				severities[normalizedType] = sev
			}
		}
		cfg.Severities = severities
	}

	// Parse ignore list
	if len(f.Ignore) > 0 {
		ignored := make(map[string]bool)
		for _, t := range f.Ignore {
			normalized := strings.ToUpper(strings.TrimSpace(t))
			if normalized == "" {
				return Config{}, fmt.Errorf("%s: type name is empty in ignore list", source)
			}
			ignored[normalized] = true
		}
		cfg.Ignored = ignored
	}

	return cfg, nil
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

// DefaultConfigYAML returns the default configuration YAML content matching
// the runtime default severity policy.
func DefaultConfigYAML() []byte {
	return []byte(`severity:
  notice:
    - TODO
    - NOTE
  warning:
    - FIXME
    - HACK
    - XXX
    - BUG
  error: []
ignore: []
`)
}

// GlobalPath returns the global config file path for the given user config
// directory. Returns an error if userConfigDir is empty.
func GlobalPath(userConfigDir string) (string, error) {
	if userConfigDir == "" {
		return "", fmt.Errorf("user config directory is empty")
	}
	return filepath.Join(userConfigDir, "gh-pr-todo", "config.yml"), nil
}

// RepoNarrowPath returns the path to .github/gh-pr-todo.yml at the root of
// the repository containing cwd. Returns an error if cwd is not inside a
// Git repository.
func RepoNarrowPath(cwd string) (string, error) {
	repoRoot, found := discoverRepoRoot(cwd)
	if !found {
		return "", fmt.Errorf("not inside a Git repository")
	}
	return filepath.Join(repoRoot, ".github", "gh-pr-todo.yml"), nil
}

// WriteDefault writes the default config YAML to path. If force is false,
// it refuses to overwrite an existing file. Parent directories are created
// as needed.
func WriteDefault(path string, force bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", path, err)
	}

	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	f, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%s already exists; use --force to overwrite", path)
		}
		return fmt.Errorf("creating %s: %w", path, err)
	}

	if _, err := f.Write(DefaultConfigYAML()); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", path, err)
	}
	return nil
}

// LoadGlobal loads the user-level global config file if it exists.
// Returns an empty/nil-map Config if the file does not exist.
func LoadGlobal(userConfigDir string) (Config, error) {
	if userConfigDir == "" {
		return Config{}, nil
	}
	globalPath, err := GlobalPath(userConfigDir)
	if err != nil {
		return Config{}, nil
	}
	data, err := os.ReadFile(globalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("reading %s: %w", globalPath, err)
	}
	return Parse(data, globalPath)
}

// LoadLocal loads config from global, repo root, and repo .github paths
// with whole-file replacement: each existing file replaces the entire
// previous config. Precedence (later wins):
// global < repo root .gh-pr-todo.yml < repo .github/gh-pr-todo.yml.
// If any repo config exists, global content does not survive.
// Missing files are silently ignored; parse errors are returned.
func LoadLocal(cwd, userConfigDir string) (Config, error) {
	// Repository root discovery happens before global loading so repo config can
	// replace global config without reading it at all.
	repoRoot, found := discoverRepoRoot(cwd)
	if !found {
		return LoadGlobal(userConfigDir)
	}

	// Narrower scope config replaces the repo-root config entirely, so prefer it
	// before reading or parsing the broader repo-root file.
	narrowPath := filepath.Join(repoRoot, ".github", "gh-pr-todo.yml")
	cfg, exists, err := loadFile(narrowPath)
	if err != nil {
		return Config{}, err
	}
	if exists {
		return cfg, nil
	}

	rootPath := filepath.Join(repoRoot, ".gh-pr-todo.yml")
	cfg, exists, err = loadFile(rootPath)
	if err != nil {
		return Config{}, err
	}
	if exists {
		return cfg, nil
	}

	return LoadGlobal(userConfigDir)
}

// loadFile reads and parses a config file at path. Missing files return
// exists=false and no error.
func loadFile(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, nil
		}
		return Config{}, false, fmt.Errorf("reading %s: %w", path, err)
	}
	cfg, err := Parse(data, path)
	if err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}
