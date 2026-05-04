package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Suree33/gh-pr-todo/internal/todotype"
)

func TestParse(t *testing.T) {
	t.Run("empty data returns empty config", func(t *testing.T) {
		cfg, err := Parse([]byte{}, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if cfg.Severities != nil {
			t.Fatalf("expected nil Severities, got %v", cfg.Severities)
		}
		if cfg.Ignored != nil {
			t.Fatalf("expected nil Ignored, got %v", cfg.Ignored)
		}
		if !cfg.Found {
			t.Fatal("expected Found=true after Parse")
		}
	})

	t.Run("empty severity block returns empty config", func(t *testing.T) {
		cfg, err := Parse([]byte("severity:"), "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if cfg.Severities != nil {
			t.Fatalf("expected nil Severities, got %v", cfg.Severities)
		}
		if cfg.Ignored != nil {
			t.Fatalf("expected nil Ignored, got %v", cfg.Ignored)
		}
	})

	t.Run("valid config with all severities", func(t *testing.T) {
		data := []byte("severity:\n  notice:\n    - TODO\n  warning:\n    - FIXME\n  error:\n    - BUG\n")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		want := map[string]todotype.Severity{
			"TODO":  todotype.SeverityNotice,
			"FIXME": todotype.SeverityWarning,
			"BUG":   todotype.SeverityError,
		}
		if !reflect.DeepEqual(cfg.Severities, want) {
			t.Fatalf("Parse() = %v, want %v", cfg.Severities, want)
		}
		if !cfg.Found {
			t.Fatal("expected Found=true")
		}
	})

	t.Run("type names normalized to uppercase", func(t *testing.T) {
		data := []byte("severity:\n  error:\n    - todo\n  warning:\n    - FixMe\n")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityError {
			t.Fatalf("expected TODO=error, got %v", cfg.Severities["TODO"])
		}
		if cfg.Severities["FIXME"] != todotype.SeverityWarning {
			t.Fatalf("expected FIXME=warning, got %v", cfg.Severities["FIXME"])
		}
	})

	t.Run("severity keys case-insensitive", func(t *testing.T) {
		data := []byte("severity:\n  ERROR:\n    - TODO\n  Warning:\n    - FIXME\n  Notice:\n    - HACK\n")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityError {
			t.Fatalf("expected TODO=error, got %v", cfg.Severities["TODO"])
		}
		if cfg.Severities["FIXME"] != todotype.SeverityWarning {
			t.Fatalf("expected FIXME=warning, got %v", cfg.Severities["FIXME"])
		}
		if cfg.Severities["HACK"] != todotype.SeverityNotice {
			t.Fatalf("expected HACK=notice, got %v", cfg.Severities["HACK"])
		}
	})

	t.Run("custom type names allowed", func(t *testing.T) {
		data := []byte("severity:\n  error:\n    - SECURITY\n  warning:\n    - PERF\n")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if cfg.Severities["SECURITY"] != todotype.SeverityError {
			t.Fatalf("expected SECURITY=error, got %v", cfg.Severities["SECURITY"])
		}
		if cfg.Severities["PERF"] != todotype.SeverityWarning {
			t.Fatalf("expected PERF=warning, got %v", cfg.Severities["PERF"])
		}
	})

	t.Run("invalid severity key returns error", func(t *testing.T) {
		data := []byte("severity:\n  critical:\n    - TODO\n")
		_, err := Parse(data, "test")
		if err == nil {
			t.Fatal("Parse() expected error for invalid severity key, got nil")
		}
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		data := []byte("severity: [unclosed")
		_, err := Parse(data, "test")
		if err == nil {
			t.Fatal("Parse() expected error for invalid YAML, got nil")
		}
	})

	t.Run("empty type name returns error", func(t *testing.T) {
		data := []byte("severity:\n  warning:\n    - \"\"\n")
		_, err := Parse(data, "test")
		if err == nil {
			t.Fatal("Parse() expected error for empty type, got nil")
		}
	})

	t.Run("error message includes source path", func(t *testing.T) {
		_, err := Parse([]byte("severity:\n  critical:\n    - TODO\n"), "/path/to/config.yml")
		if err == nil || !strings.Contains(err.Error(), "/path/to/config.yml") {
			t.Fatalf("Parse() error = %v, expected to contain source path", err)
		}
	})

	t.Run("empty lists allowed as no-op", func(t *testing.T) {
		data := []byte("severity:\n  warning: []\n")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if len(cfg.Severities) != 0 {
			t.Fatalf("expected empty config for empty list, got %v", cfg.Severities)
		}
	})

	t.Run("old format rejected", func(t *testing.T) {
		data := []byte("severity:\n  TODO: warning\n")
		_, err := Parse(data, "test")
		if err == nil {
			t.Fatal("Parse() expected error for old format, got nil")
		}
	})

	t.Run("duplicate type in same severity is no-op", func(t *testing.T) {
		data := []byte("severity:\n  warning:\n    - TODO\n    - todo\n")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityWarning {
			t.Fatalf("expected TODO=warning, got %v", cfg.Severities)
		}
	})

	t.Run("duplicate type across severity levels returns error", func(t *testing.T) {
		data := []byte("severity:\n  warning:\n    - TODO\n  error:\n    - TODO\n")
		_, err := Parse(data, "test")
		if err == nil {
			t.Fatal("Parse() expected error for duplicate type across severity levels, got nil")
		}
	})

	t.Run("duplicate detection is case-insensitive", func(t *testing.T) {
		data := []byte("severity:\n  warning:\n    - TODO\n  error:\n    - todo\n")
		_, err := Parse(data, "test")
		if err == nil {
			t.Fatal("Parse() expected error for case-insensitive duplicate type, got nil")
		}
	})

	t.Run("ignore list parses to normalized set", func(t *testing.T) {
		data := []byte("ignore:\n  - NOTE\n  - HACK\n  - todo\n")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if len(cfg.Severities) != 0 {
			t.Fatalf("expected no severities, got %v", cfg.Severities)
		}
		if !cfg.Ignored["NOTE"] || !cfg.Ignored["HACK"] || !cfg.Ignored["TODO"] {
			t.Fatalf("expected ignored={NOTE,HACK,TODO}, got %v", cfg.Ignored)
		}
		// Should not have extra entries beyond what was specified
		if len(cfg.Ignored) != 3 {
			t.Fatalf("expected exactly 3 ignored types, got %v", cfg.Ignored)
		}
	})

	t.Run("ignore list case normalization", func(t *testing.T) {
		data := []byte("ignore:\n  - note\n  - Hack\n")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if !cfg.Ignored["NOTE"] || !cfg.Ignored["HACK"] {
			t.Fatalf("expected ignored={NOTE,HACK}, got %v", cfg.Ignored)
		}
	})

	t.Run("empty ignore list results in nil map", func(t *testing.T) {
		data := []byte("ignore: []")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if cfg.Ignored != nil {
			t.Fatalf("expected nil Ignored for empty list, got %v", cfg.Ignored)
		}
	})

	t.Run("ignore with only severity has nil Ignored", func(t *testing.T) {
		data := []byte("severity:\n  warning:\n    - TODO\n")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if cfg.Ignored != nil {
			t.Fatalf("expected nil Ignored when no ignore key, got %v", cfg.Ignored)
		}
		if cfg.Severities["TODO"] != todotype.SeverityWarning {
			t.Fatalf("expected TODO=warning, got %v", cfg.Severities["TODO"])
		}
	})

	t.Run("ignore empty type name returns error", func(t *testing.T) {
		data := []byte("ignore:\n  - \"\"")
		_, err := Parse(data, "test")
		if err == nil || !strings.Contains(err.Error(), "type name is empty in ignore list") {
			t.Fatalf("Parse() expected error for empty ignore type, got %v", err)
		}
	})

	t.Run("error message for ignore includes source path", func(t *testing.T) {
		_, err := Parse([]byte("ignore:\n  - \"\""), "/my/config.yml")
		if err == nil || !strings.Contains(err.Error(), "/my/config.yml") {
			t.Fatalf("Parse() error = %v, expected to contain source path", err)
		}
	})

	t.Run("combination of severity and ignore", func(t *testing.T) {
		data := []byte("severity:\n  warning:\n    - TODO\nignore:\n  - NOTE")
		cfg, err := Parse(data, "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityWarning {
			t.Fatalf("expected TODO=warning, got %v", cfg.Severities["TODO"])
		}
		if !cfg.Ignored["NOTE"] {
			t.Fatalf("expected NOTE ignored, got %v", cfg.Ignored)
		}
	})
}

func TestDefaultConfigYAMLParsesToRuntimeDefaults(t *testing.T) {
	t.Run("default YAML parses without error", func(t *testing.T) {
		data := DefaultConfigYAML()
		cfg, err := Parse(data, "default")
		if err != nil {
			t.Fatalf("Parse(DefaultConfigYAML()) unexpected error: %v", err)
		}

		// TODO and NOTE should be notice
		if cfg.Severities["TODO"] != todotype.SeverityNotice {
			t.Fatalf("expected TODO=notice, got %v", cfg.Severities["TODO"])
		}
		if cfg.Severities["NOTE"] != todotype.SeverityNotice {
			t.Fatalf("expected NOTE=notice, got %v", cfg.Severities["NOTE"])
		}

		// FIXME, HACK, XXX, BUG should be warning
		for _, typ := range []string{"FIXME", "HACK", "XXX", "BUG"} {
			if cfg.Severities[typ] != todotype.SeverityWarning {
				t.Fatalf("expected %s=warning, got %v", typ, cfg.Severities[typ])
			}
		}

		// No type should have error severity
		for typ, sev := range cfg.Severities {
			if sev == todotype.SeverityError {
				t.Fatalf("expected no error-level types in default, but %s is error", typ)
			}
		}

		// ignore: [] should result in nil Ignored map (no ignored types)
		if cfg.Ignored != nil {
			t.Fatalf("expected nil Ignored for default config, got %v", cfg.Ignored)
		}
	})
}

func TestGlobalPath(t *testing.T) {
	t.Run("empty userConfigDir returns error", func(t *testing.T) {
		_, err := GlobalPath("")
		if err == nil {
			t.Fatal("GlobalPath(\"\") expected error")
		}
	})

	t.Run("valid userConfigDir returns path", func(t *testing.T) {
		dir := t.TempDir()
		path, err := GlobalPath(dir)
		if err != nil {
			t.Fatalf("GlobalPath(%q) unexpected error: %v", dir, err)
		}
		want := filepath.Join(dir, "gh-pr-todo", "config.yml")
		if path != want {
			t.Fatalf("GlobalPath(%q) = %q, want %q", dir, path, want)
		}
	})
}

func TestRepoNarrowPath(t *testing.T) {
	t.Run("non-repo directory errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := RepoNarrowPath(tmpDir)
		if err == nil {
			t.Fatal("RepoNarrowPath() expected error for non-repo directory")
		}
	})

	t.Run("repo root returns .github/gh-pr-todo.yml", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		path, err := RepoNarrowPath(repoRoot)
		if err != nil {
			t.Fatalf("RepoNarrowPath() unexpected error: %v", err)
		}
		want := filepath.Join(repoRoot, ".github", "gh-pr-todo.yml")
		if path != want {
			t.Fatalf("RepoNarrowPath() = %q, want %q", path, want)
		}
	})

	t.Run("nested subdirectory inside repo still returns root path", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		subdir := filepath.Join(repoRoot, "a", "b", "c")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		path, err := RepoNarrowPath(subdir)
		if err != nil {
			t.Fatalf("RepoNarrowPath() unexpected error: %v", err)
		}
		want := filepath.Join(repoRoot, ".github", "gh-pr-todo.yml")
		if path != want {
			t.Fatalf("RepoNarrowPath() = %q, want %q", path, want)
		}
	})
}

func TestWriteDefault(t *testing.T) {
	t.Run("creates file and parent directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "subdir", "config.yml")
		if err := WriteDefault(path, false); err != nil {
			t.Fatalf("WriteDefault() unexpected error: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file to exist at %s: %v", path, err)
		}
		// Verify content is valid YAML
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}
		if _, err := Parse(data, path); err != nil {
			t.Fatalf("Parse(written content) error: %v", err)
		}
	})

	t.Run("refuses to overwrite without force", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "config.yml")
		if err := WriteDefault(path, false); err != nil {
			t.Fatalf("first WriteDefault() unexpected error: %v", err)
		}
		err := WriteDefault(path, false)
		if err == nil {
			t.Fatal("WriteDefault() without force expected error for existing file")
		}
		if !strings.Contains(err.Error(), "--force") {
			t.Fatalf("WriteDefault() error = %q, expected to mention --force", err.Error())
		}
		// Content should be preserved
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}
		if _, err := Parse(data, path); err != nil {
			t.Fatalf("Parse(preserved content) error: %v", err)
		}
	})

	t.Run("overwrites with force", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "config.yml")
		// Write original content
		if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}
		// Overwrite with force
		if err := WriteDefault(path, true); err != nil {
			t.Fatalf("WriteDefault(force=true) unexpected error: %v", err)
		}
		// Content should be the default YAML now
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}
		cfg, err := Parse(data, path)
		if err != nil {
			t.Fatalf("Parse(overwritten content) error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityNotice {
			t.Fatalf("expected TODO=notice in overwritten file, got %v", cfg.Severities["TODO"])
		}
	})
}

func TestLoadGlobal(t *testing.T) {
	t.Run("empty user config dir is treated as no global config", func(t *testing.T) {
		cfg, err := LoadGlobal("")
		if err != nil {
			t.Fatalf("LoadGlobal() unexpected error: %v", err)
		}
		if len(cfg.Severities) != 0 {
			t.Fatalf("expected empty config, got %v", cfg.Severities)
		}
	})

	t.Run("global config applies", func(t *testing.T) {
		userConfigDir := t.TempDir()
		globalDir := filepath.Join(userConfigDir, "gh-pr-todo")
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte("severity:\n  error:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		cfg, err := LoadGlobal(userConfigDir)
		if err != nil {
			t.Fatalf("LoadGlobal() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityError {
			t.Fatalf("expected TODO=error, got %v", cfg.Severities)
		}
	})
}

func TestLoadLocal(t *testing.T) {
	t.Run("no config files returns empty config", func(t *testing.T) {
		tmpDir := t.TempDir()
		userConfigDir := t.TempDir()
		cfg, err := LoadLocal(tmpDir, userConfigDir)
		if err != nil {
			t.Fatalf("LoadLocal() unexpected error: %v", err)
		}
		if len(cfg.Severities) != 0 {
			t.Fatalf("expected empty config, got %v", cfg.Severities)
		}
	})

	t.Run("global config applies", func(t *testing.T) {
		userConfigDir := t.TempDir()
		globalDir := filepath.Join(userConfigDir, "gh-pr-todo")
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte("severity:\n  error:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		tmpDir := t.TempDir()
		cfg, err := LoadLocal(tmpDir, userConfigDir)
		if err != nil {
			t.Fatalf("LoadLocal() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityError {
			t.Fatalf("expected TODO=error, got %v", cfg.Severities)
		}
	})

	t.Run("repo root config replaces global entirely", func(t *testing.T) {
		userConfigDir := t.TempDir()
		globalDir := filepath.Join(userConfigDir, "gh-pr-todo")
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte("severity:\n  error:\n    - TODO\n  notice:\n    - FIXME\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		repoRoot := t.TempDir()
		if err := os.MkdirAll(repoRoot, 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		// Create .git dir to mark repo root
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".gh-pr-todo.yml"), []byte("severity:\n  warning:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		cfg, err := LoadLocal(repoRoot, userConfigDir)
		if err != nil {
			t.Fatalf("LoadLocal() unexpected error: %v", err)
		}
		// Repo config replaces global entirely: only TODO=warning
		if cfg.Severities["TODO"] != todotype.SeverityWarning {
			t.Fatalf("expected TODO=warning (repo replaces global), got %v", cfg.Severities["TODO"])
		}
		// FIXME from global should NOT survive whole-file replacement
		if _, exists := cfg.Severities["FIXME"]; exists {
			t.Fatalf("FIXME should not survive repo replacement, but got %v", cfg.Severities["FIXME"])
		}
	})

	t.Run("invalid global config is not read when repo config exists", func(t *testing.T) {
		userConfigDir := t.TempDir()
		globalDir := filepath.Join(userConfigDir, "gh-pr-todo")
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte("severity:\n  critical:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".gh-pr-todo.yml"), []byte("severity:\n  warning:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		cfg, err := LoadLocal(repoRoot, userConfigDir)
		if err != nil {
			t.Fatalf("LoadLocal() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityWarning {
			t.Fatalf("expected TODO=warning from repo config, got %v", cfg.Severities["TODO"])
		}
	})

	t.Run(".git file marks repo root", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(repoRoot, ".git"), []byte("gitdir: /tmp/worktree.git\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".gh-pr-todo.yml"), []byte("severity:\n  warning:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		cfg, err := LoadLocal(repoRoot, t.TempDir())
		if err != nil {
			t.Fatalf("LoadLocal() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityWarning {
			t.Fatalf("expected TODO=warning, got %v", cfg.Severities["TODO"])
		}
	})

	t.Run(".github/gh-pr-todo.yml overrides root .gh-pr-todo.yml", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".gh-pr-todo.yml"), []byte("severity:\n  warning:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, ".github"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".github", "gh-pr-todo.yml"), []byte("severity:\n  error:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		cfg, err := LoadLocal(repoRoot, t.TempDir())
		if err != nil {
			t.Fatalf("LoadLocal() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityError {
			t.Fatalf("expected TODO=error (.github wins), got %v", cfg.Severities["TODO"])
		}
	})

	t.Run("valid .github config replaces invalid root config", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".gh-pr-todo.yml"), []byte("severity:\n  critical:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, ".github"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".github", "gh-pr-todo.yml"), []byte("severity:\n  error:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		cfg, err := LoadLocal(repoRoot, t.TempDir())
		if err != nil {
			t.Fatalf("LoadLocal() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityError {
			t.Fatalf("expected TODO=error from .github config, got %v", cfg.Severities["TODO"])
		}
	})

	t.Run("no .git directory returns only global config", func(t *testing.T) {
		userConfigDir := t.TempDir()
		globalDir := filepath.Join(userConfigDir, "gh-pr-todo")
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte("severity:\n  error:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		// No .git directory in tmpDir
		tmpDir := t.TempDir()
		cfg, err := LoadLocal(tmpDir, userConfigDir)
		if err != nil {
			t.Fatalf("LoadLocal() unexpected error: %v", err)
		}
		if len(cfg.Severities) != 1 || cfg.Severities["TODO"] != todotype.SeverityError {
			t.Fatalf("expected only global config, got %v", cfg.Severities)
		}
	})

	t.Run("parse error returns error", func(t *testing.T) {
		userConfigDir := t.TempDir()
		globalDir := filepath.Join(userConfigDir, "gh-pr-todo")
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte("severity:\n  critical:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		_, err := LoadLocal(t.TempDir(), userConfigDir)
		if err == nil {
			t.Fatal("LoadLocal() expected error for invalid severity key, got nil")
		}
	})
}
