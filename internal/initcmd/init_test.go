package initcmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Suree33/gh-pr-todo/internal/config"
	"github.com/Suree33/gh-pr-todo/internal/todotype"
)

func TestCommandExecuteUsesTextFallback(t *testing.T) {
	userConfigDir := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer
	usedText := false
	usedInteractive := false

	cmd := Command{
		In:            strings.NewReader("2\n"),
		Out:           &out,
		ErrOut:        &errOut,
		UsageOut:      &out,
		Getwd:         func() (string, error) { return t.TempDir(), nil },
		UserConfigDir: func() (string, error) { return userConfigDir, nil },
		chooser: chooser{
			useInteractive: func(in io.Reader, out io.Writer) bool { return false },
			interactive: func(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error) (string, error) {
				usedInteractive = true
				return "", nil
			},
			text: func(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error) (string, error) {
				usedText = true
				return globalPath, nil
			},
		},
	}

	if code := cmd.Execute(nil); code != 0 {
		t.Fatalf("Execute() exit code = %d, want 0 (stderr=%q)", code, errOut.String())
	}
	if !usedText {
		t.Fatal("Execute() did not use text fallback chooser")
	}
	if usedInteractive {
		t.Fatal("Execute() unexpectedly used interactive chooser")
	}
	wantPath := filepath.Join(userConfigDir, "gh-pr-todo", "config.yml")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file at %s: %v", wantPath, err)
	}
}

func TestCommandExecuteUsesInteractiveChooser(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	usedText := false
	usedInteractive := false

	cmd := Command{
		In:            strings.NewReader("ignored"),
		Out:           &out,
		ErrOut:        &errOut,
		UsageOut:      &out,
		Getwd:         func() (string, error) { return repoRoot, nil },
		UserConfigDir: func() (string, error) { return t.TempDir(), nil },
		chooser: chooser{
			useInteractive: func(in io.Reader, out io.Writer) bool { return true },
			interactive: func(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error) (string, error) {
				usedInteractive = true
				return repoPath, nil
			},
			text: func(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error) (string, error) {
				usedText = true
				return "", nil
			},
		},
	}

	if code := cmd.Execute(nil); code != 0 {
		t.Fatalf("Execute() exit code = %d, want 0 (stderr=%q)", code, errOut.String())
	}
	if !usedInteractive {
		t.Fatal("Execute() did not use interactive chooser")
	}
	if usedText {
		t.Fatal("Execute() unexpectedly used text chooser")
	}
	wantPath := filepath.Join(repoRoot, ".gh-pr-todo.yml")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file at %s: %v", wantPath, err)
	}
}

func TestCommandExecuteRepoFlagWithForce(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	configPath := filepath.Join(repoRoot, ".gh-pr-todo.yml")
	if err := os.WriteFile(configPath, []byte("original"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := Command{
		In:            strings.NewReader(""),
		Out:           &out,
		ErrOut:        &errOut,
		UsageOut:      &out,
		Getwd:         func() (string, error) { return repoRoot, nil },
		UserConfigDir: func() (string, error) { return t.TempDir(), nil },
	}

	if code := cmd.Execute([]string{"--repo", "--force"}); code != 0 {
		t.Fatalf("Execute() exit code = %d, want 0 (stderr=%q)", code, errOut.String())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	cfg, err := config.Parse(data, configPath)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cfg.Severities["TODO"] != todotype.SeverityNotice {
		t.Fatalf("TODO severity = %v, want %v", cfg.Severities["TODO"], todotype.SeverityNotice)
	}
}

func TestCommandExecuteGlobalFlagSkipsGetwd(t *testing.T) {
	userConfigDir := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer
	getwdCalled := false

	cmd := Command{
		In:       strings.NewReader(""),
		Out:      &out,
		ErrOut:   &errOut,
		UsageOut: &out,
		Getwd: func() (string, error) {
			getwdCalled = true
			return "", nil
		},
		UserConfigDir: func() (string, error) { return userConfigDir, nil },
	}

	if code := cmd.Execute([]string{"--global"}); code != 0 {
		t.Fatalf("Execute() exit code = %d, want 0 (stderr=%q)", code, errOut.String())
	}
	if getwdCalled {
		t.Fatal("Execute() called Getwd() for --global target")
	}
	wantPath := filepath.Join(userConfigDir, "gh-pr-todo", "config.yml")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file at %s: %v", wantPath, err)
	}
}

func TestCommandExecuteHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	var usage bytes.Buffer
	cmd := Command{Out: &out, ErrOut: &errOut, UsageOut: &usage}

	if code := cmd.Execute([]string{"--help"}); code != 0 {
		t.Fatalf("Execute() exit code = %d, want 0", code)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	if !strings.Contains(usage.String(), "gh pr-todo init [--repo | --global] [--force]") {
		t.Fatalf("usage = %q, expected init usage", usage.String())
	}
}
