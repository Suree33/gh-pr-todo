package policyresolve

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Suree33/gh-pr-todo/internal/config"
	"github.com/Suree33/gh-pr-todo/internal/todotype"
	"github.com/Suree33/gh-pr-todo/pkg/types"
)

type fakeFetcher struct {
	refs         config.RemoteConfigRefs
	fileContents map[string]map[string][]byte
}

func (f *fakeFetcher) FetchRemoteConfigRefs(repo, pr string) (config.RemoteConfigRefs, error) {
	return f.refs, nil
}

func (f *fakeFetcher) FetchFileAtRef(repo, path, ref string) ([]byte, bool, error) {
	key := repo + ":" + ref
	if contents, ok := f.fileContents[key]; ok {
		if data, ok := contents[path]; ok {
			return data, true, nil
		}
	}
	return nil, false, nil
}

func TestResolveTarget(t *testing.T) {
	tests := []struct {
		name string
		repo string
		pr   string
		want Target
	}{
		{
			name: "explicit repo uses remote config",
			repo: "owner/repo",
			pr:   "123",
			want: Target{Repo: "owner/repo", PR: "123", UseRemote: true},
		},
		{
			name: "github PR URL uses remote config",
			pr:   "https://github.com/owner/repo/pull/123",
			want: Target{Repo: "owner/repo", PR: "123", UseRemote: true},
		},
		{
			name: "host-qualified PR URL preserves host",
			pr:   "https://github.example.com/owner/repo/pull/123",
			want: Target{Repo: "github.example.com/owner/repo", PR: "123", UseRemote: true},
		},
		{
			name: "non-PR URL stays local",
			pr:   "https://github.com/owner/repo/issues/123",
			want: Target{PR: "https://github.com/owner/repo/issues/123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveTarget(tt.repo, tt.pr); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveTarget() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	t.Run("local config produces ready-to-use policy", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, ".github"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".github", "gh-pr-todo.yml"), []byte("severity:\n  error:\n    - TODO\nignore:\n  - note\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		policy, err := Resolve(nil, Options{Target: ResolveTarget("", ""), CWD: repoRoot})
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if got := policy.SeverityFor("TODO"); got != todotype.SeverityError {
			t.Fatalf("SeverityFor(TODO) = %q, want %q", got, todotype.SeverityError)
		}
		if !policy.IsIgnored("NOTE") {
			t.Fatal("NOTE should be ignored")
		}
		if got := policy.Types(); reflect.DeepEqual(got, []string{"BUG", "FIXME", "HACK", "TODO", "XXX"}) == false {
			t.Fatalf("Types() = %v", got)
		}
	})

	t.Run("remote config uses PR head precedence", func(t *testing.T) {
		policy, err := Resolve(&fakeFetcher{
			refs: config.RemoteConfigRefs{
				DefaultBranchRef: "main",
				DefaultRepo:      "owner/repo",
				BaseBranchRef:    "release",
				BaseRepo:         "owner/repo",
				HeadRefOid:       "abc123",
				HeadRepo:         "fork/repo",
			},
			fileContents: map[string]map[string][]byte{
				"owner/repo:main": {
					".github/gh-pr-todo.yml": []byte("severity:\n  warning:\n    - BUG\n"),
				},
				"owner/repo:release": {
					".github/gh-pr-todo.yml": []byte("severity:\n  warning:\n    - FIXME\n"),
				},
				"fork/repo:abc123": {
					".github/gh-pr-todo.yml": []byte("severity:\n  error:\n    - TODO\n"),
				},
			},
		}, Options{Target: Target{Repo: "owner/repo", PR: "42", UseRemote: true}})
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if got := policy.SeverityFor("TODO"); got != todotype.SeverityError {
			t.Fatalf("SeverityFor(TODO) = %q, want %q", got, todotype.SeverityError)
		}
		if got := policy.SeverityFor("BUG"); got != todotype.SeverityWarning {
			t.Fatalf("SeverityFor(BUG) = %q, want built-in %q after head replacement", got, todotype.SeverityWarning)
		}
		if got := policy.SeverityFor("FIXME"); got != todotype.SeverityWarning {
			t.Fatalf("SeverityFor(FIXME) = %q, want built-in %q after head replacement", got, todotype.SeverityWarning)
		}
	})

	t.Run("global config is used when no remote config exists", func(t *testing.T) {
		userConfigDir := t.TempDir()
		globalDir := filepath.Join(userConfigDir, "gh-pr-todo")
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte("severity:\n  error:\n    - NOTE\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		policy, err := Resolve(&fakeFetcher{
			refs: config.RemoteConfigRefs{
				DefaultBranchRef: "main",
				DefaultRepo:      "owner/repo",
			},
			fileContents: map[string]map[string][]byte{},
		}, Options{
			Target:        Target{Repo: "owner/repo", PR: "42", UseRemote: true},
			UserConfigDir: userConfigDir,
		})
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if got := policy.SeverityFor("NOTE"); got != todotype.SeverityError {
			t.Fatalf("SeverityFor(NOTE) = %q, want %q", got, todotype.SeverityError)
		}
	})

	t.Run("CLI severity overrides config severity", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".gh-pr-todo.yml"), []byte("severity:\n  warning:\n    - TODO\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		policy, err := Resolve(nil, Options{
			Target: ResolveTarget("", ""),
			CWD:    repoRoot,
			CLISeverities: map[string]todotype.Severity{
				"TODO": todotype.SeverityError,
				"NOTE": todotype.SeverityWarning,
			},
		})
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if got := policy.SeverityFor("TODO"); got != todotype.SeverityError {
			t.Fatalf("SeverityFor(TODO) = %q, want %q", got, todotype.SeverityError)
		}
		if got := policy.SeverityFor("NOTE"); got != todotype.SeverityWarning {
			t.Fatalf("SeverityFor(NOTE) = %q, want %q", got, todotype.SeverityWarning)
		}
	})

	t.Run("ignore and severity settings are merged before building policy", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".gh-pr-todo.yml"), []byte("severity:\n  error:\n    - NOTE\nignore:\n  - HACK\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		policy, err := Resolve(nil, Options{
			Target: ResolveTarget("", ""),
			CWD:    repoRoot,
			CLISeverities: map[string]todotype.Severity{
				"HACK": todotype.SeverityError,
			},
			CLIIgnored: []string{"note"},
		})
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if !policy.IsIgnored("NOTE") || !policy.IsIgnored("HACK") {
			t.Fatalf("expected NOTE and HACK to be ignored")
		}
		if got := policy.Types(); !reflect.DeepEqual(got, []string{"BUG", "FIXME", "TODO", "XXX"}) {
			t.Fatalf("Types() = %v, want [BUG FIXME TODO XXX]", got)
		}
		if got := policy.CountCIFailing([]types.TODO{{Type: "NOTE"}, {Type: "HACK"}}); got != 0 {
			t.Fatalf("CountCIFailing() = %d, want 0", got)
		}
	})
}
