package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/Suree33/gh-pr-todo/internal/todotype"
)

// fakeFetcher implements RemoteConfigFetcher for testing.
type fakeFetcher struct {
	refs           RemoteConfigRefs
	refsErr        error
	fileContents   map[string]map[string][]byte // [repo+":"+ref][path]content
	fetchFileError error
}

func (f *fakeFetcher) FetchRemoteConfigRefs(repo, pr string) (RemoteConfigRefs, error) {
	return f.refs, f.refsErr
}

func (f *fakeFetcher) FetchFileAtRef(repo, path, ref string) ([]byte, bool, error) {
	if f.fetchFileError != nil {
		return nil, false, f.fetchFileError
	}
	key := repo + ":" + ref
	if contents, ok := f.fileContents[key]; ok {
		if data, ok := contents[path]; ok {
			return data, true, nil
		}
	}
	return nil, false, nil
}

func TestLoadRemote(t *testing.T) {
	t.Run("fetch refs error returns error", func(t *testing.T) {
		fetcher := &fakeFetcher{refsErr: errors.New("network error")}
		_, err := LoadRemote(fetcher, "owner/repo", "1")
		if err == nil {
			t.Fatal("LoadRemote() expected error, got nil")
		}
	})

	t.Run("no remote config files returns empty", func(t *testing.T) {
		fetcher := &fakeFetcher{
			refs: RemoteConfigRefs{
				DefaultBranchRef: "main",
				DefaultRepo:      "owner/repo",
			},
			fileContents: make(map[string]map[string][]byte),
		}
		cfg, err := LoadRemote(fetcher, "owner/repo", "")
		if err != nil {
			t.Fatalf("LoadRemote() unexpected error: %v", err)
		}
		if len(cfg.Severities) != 0 {
			t.Fatalf("expected empty config, got %v", cfg.Severities)
		}
	})

	t.Run("default branch config applies", func(t *testing.T) {
		fetcher := &fakeFetcher{
			refs: RemoteConfigRefs{
				DefaultBranchRef: "main",
				DefaultRepo:      "owner/repo",
			},
			fileContents: map[string]map[string][]byte{
				"owner/repo:main": {
					".github/gh-pr-todo.yml": []byte("severity:\n  warning:\n    - TODO\n"),
				},
			},
		}
		cfg, err := LoadRemote(fetcher, "owner/repo", "")
		if err != nil {
			t.Fatalf("LoadRemote() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityWarning {
			t.Fatalf("expected TODO=warning, got %v", cfg.Severities)
		}
	})

	t.Run(".github/gh-pr-todo.yml replaces .gh-pr-todo.yml within same scope", func(t *testing.T) {
		fetcher := &fakeFetcher{
			refs: RemoteConfigRefs{
				DefaultBranchRef: "main",
				DefaultRepo:      "owner/repo",
			},
			fileContents: map[string]map[string][]byte{
				"owner/repo:main": {
					".gh-pr-todo.yml":        []byte("severity:\n  notice:\n    - TODO\n"),
					".github/gh-pr-todo.yml": []byte("severity:\n  warning:\n    - TODO\n"),
				},
			},
		}
		cfg, err := LoadRemote(fetcher, "owner/repo", "")
		if err != nil {
			t.Fatalf("LoadRemote() unexpected error: %v", err)
		}
		// .github/gh-pr-todo.yml replaces .gh-pr-todo.yml within scope
		if cfg.Severities["TODO"] != todotype.SeverityWarning {
			t.Fatalf("expected TODO=warning (.github wins), got %v", cfg.Severities)
		}
		// TODO=notice from .gh-pr-todo.yml should NOT survive
		if len(cfg.Severities) != 1 {
			t.Fatalf("expected exactly 1 severity (TODO=warning), got %v", cfg.Severities)
		}
	})

	t.Run("valid .github config replaces invalid broad config within same scope", func(t *testing.T) {
		fetcher := &fakeFetcher{
			refs: RemoteConfigRefs{
				DefaultBranchRef: "main",
				DefaultRepo:      "owner/repo",
			},
			fileContents: map[string]map[string][]byte{
				"owner/repo:main": {
					".gh-pr-todo.yml":        []byte("severity:\n  critical:\n    - TODO\n"),
					".github/gh-pr-todo.yml": []byte("severity:\n  warning:\n    - TODO\n"),
				},
			},
		}
		cfg, err := LoadRemote(fetcher, "owner/repo", "")
		if err != nil {
			t.Fatalf("LoadRemote() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityWarning {
			t.Fatalf("expected TODO=warning (.github wins), got %v", cfg.Severities)
		}
	})

	t.Run("PR head replaces base replaces default (whole config)", func(t *testing.T) {
		fetcher := &fakeFetcher{
			refs: RemoteConfigRefs{
				DefaultBranchRef: "main",
				DefaultRepo:      "owner/repo",
				BaseBranchRef:    "release",
				BaseRepo:         "owner/repo",
				HeadRefOid:       "abc123",
				HeadRepo:         "forkuser/repo",
			},
			fileContents: map[string]map[string][]byte{
				"owner/repo:main": {
					".github/gh-pr-todo.yml": []byte("severity:\n  notice:\n    - TODO\n  warning:\n    - FIXME\n"),
				},
				"owner/repo:release": {
					".github/gh-pr-todo.yml": []byte("severity:\n  warning:\n    - TODO\n"),
				},
				"forkuser/repo:abc123": {
					".github/gh-pr-todo.yml": []byte("severity:\n  error:\n    - TODO\n"),
				},
			},
		}
		cfg, err := LoadRemote(fetcher, "owner/repo", "42")
		if err != nil {
			t.Fatalf("LoadRemote() unexpected error: %v", err)
		}
		// Head replaces base which replaced default: only TODO=error, no FIXME
		if cfg.Severities["TODO"] != todotype.SeverityError {
			t.Fatalf("expected TODO=error (head wins), got %v", cfg.Severities)
		}
		// FIXME from default should NOT survive head replacement
		if _, exists := cfg.Severities["FIXME"]; exists {
			t.Fatalf("FIXME should not survive head replacement, but got %v", cfg.Severities["FIXME"])
		}
		if len(cfg.Severities) != 1 {
			t.Fatalf("expected exactly 1 severity (TODO=error), got %v", cfg.Severities)
		}
	})

	t.Run("valid head config replaces invalid default config", func(t *testing.T) {
		fetcher := &fakeFetcher{
			refs: RemoteConfigRefs{
				DefaultBranchRef: "main",
				DefaultRepo:      "owner/repo",
				HeadRefOid:       "abc123",
				HeadRepo:         "forkuser/repo",
			},
			fileContents: map[string]map[string][]byte{
				"owner/repo:main": {
					".github/gh-pr-todo.yml": []byte("severity:\n  critical:\n    - TODO\n"),
				},
				"forkuser/repo:abc123": {
					".github/gh-pr-todo.yml": []byte("severity:\n  error:\n    - TODO\n"),
				},
			},
		}
		cfg, err := LoadRemote(fetcher, "owner/repo", "42")
		if err != nil {
			t.Fatalf("LoadRemote() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityError {
			t.Fatalf("expected TODO=error from head config, got %v", cfg.Severities)
		}
	})

	t.Run("fetch error returns error", func(t *testing.T) {
		fetcher := &fakeFetcher{
			refs: RemoteConfigRefs{
				DefaultBranchRef: "main",
				DefaultRepo:      "owner/repo",
			},
			fetchFileError: errors.New("network error"),
		}
		_, err := LoadRemote(fetcher, "owner/repo", "")
		if err == nil {
			t.Fatal("LoadRemote() expected error, got nil")
		}
	})

	t.Run("parse error returns source-rich error", func(t *testing.T) {
		fetcher := &fakeFetcher{
			refs: RemoteConfigRefs{
				DefaultBranchRef: "main",
				DefaultRepo:      "owner/repo",
			},
			fileContents: map[string]map[string][]byte{
				"owner/repo:main": {
					".github/gh-pr-todo.yml": []byte("severity:\n  critical:\n    - TODO\n"),
				},
			},
		}
		_, err := LoadRemote(fetcher, "owner/repo", "")
		if err == nil {
			t.Fatal("LoadRemote() expected error, got nil")
		}
		if got := err.Error(); !strings.Contains(got, "owner/repo:main:.github/gh-pr-todo.yml") || !strings.Contains(got, "invalid severity key") {
			t.Fatalf("LoadRemote() error = %q", got)
		}
	})

	t.Run("missing default refs skips scope", func(t *testing.T) {
		fetcher := &fakeFetcher{
			refs: RemoteConfigRefs{
				// Only head refs, no default
				HeadRefOid: "abc123",
				HeadRepo:   "forkuser/repo",
			},
			fileContents: map[string]map[string][]byte{
				"forkuser/repo:abc123": {
					".github/gh-pr-todo.yml": []byte("severity:\n  error:\n    - TODO\n"),
				},
			},
		}
		cfg, err := LoadRemote(fetcher, "owner/repo", "42")
		if err != nil {
			t.Fatalf("LoadRemote() unexpected error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityError {
			t.Fatalf("expected TODO=error, got %v", cfg.Severities)
		}
	})
}
