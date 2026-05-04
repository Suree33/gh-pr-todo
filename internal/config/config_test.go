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
		if len(cfg.Severities) != 0 {
			t.Fatalf("expected empty config, got %v", cfg.Severities)
		}
	})

	t.Run("empty severity block returns empty config", func(t *testing.T) {
		cfg, err := Parse([]byte("severity:"), "test")
		if err != nil {
			t.Fatalf("Parse() unexpected error: %v", err)
		}
		if len(cfg.Severities) != 0 {
			t.Fatalf("expected empty config, got %v", cfg.Severities)
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

	t.Run("repo root config overrides global", func(t *testing.T) {
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
		// Global TODO=error overridden by repo TODO=warning
		if cfg.Severities["TODO"] != todotype.SeverityWarning {
			t.Fatalf("expected TODO=warning (repo overrides global), got %v", cfg.Severities["TODO"])
		}
		// Global FIXME=notice preserved
		if cfg.Severities["FIXME"] != todotype.SeverityNotice {
			t.Fatalf("expected FIXME=notice (from global), got %v", cfg.Severities["FIXME"])
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
