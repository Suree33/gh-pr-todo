package config

import (
	"fmt"

	"github.com/Suree33/gh-pr-todo/internal/todotype"
)

// RemoteConfigFetcher is the interface for fetching remote config files
// from a GitHub repository.
type RemoteConfigFetcher interface {
	// FetchRemoteConfigRefs returns the relevant refs for remote config loading.
	FetchRemoteConfigRefs(repo, pr string) (RemoteConfigRefs, error)
	// FetchFileAtRef fetches a file from the given repo at a specific ref.
	// Returns (nil, false, nil) if the file is not found (404).
	FetchFileAtRef(repo, path, ref string) ([]byte, bool, error)
}

// RemoteConfigRefs holds repository references used for remote config loading.
type RemoteConfigRefs struct {
	DefaultBranchRef string // e.g. "main"
	DefaultRepo      string // e.g. "owner/repo"
	BaseBranchRef    string // e.g. "main" (empty if no PR)
	BaseRepo         string // e.g. "owner/repo" (empty if no PR)
	HeadRefOid       string // e.g. "abc123def" (empty if no PR)
	HeadRepo         string // e.g. "forkuser/repo" (empty if no PR)
}

// LoadRemote loads and merges config files from remote repository references.
// Precedence within remote config (later wins): default branch < PR base < PR head.
// The caller is responsible for applying global config before this result and
// CLI overrides after it. For each scope, .gh-pr-todo.yml is loaded first, then
// .github/gh-pr-todo.yml (narrower path wins within each scope).
func LoadRemote(fetcher RemoteConfigFetcher, repo, pr string) (Config, error) {
	merged := Config{Severities: make(map[string]todotype.Severity)}

	refs, err := fetcher.FetchRemoteConfigRefs(repo, pr)
	if err != nil {
		return merged, fmt.Errorf("fetching remote refs: %w", err)
	}

	// Helper to try loading both config paths at a given ref
	loadRef := func(repoName, ref, scope string) error {
		for _, path := range []string{".gh-pr-todo.yml", ".github/gh-pr-todo.yml"} {
			data, found, err := fetcher.FetchFileAtRef(repoName, path, ref)
			if err != nil {
				return fmt.Errorf("fetching %s from %s at %s (%s): %w", path, repoName, ref, scope, err)
			}
			if found {
				source := fmt.Sprintf("%s:%s:%s", repoName, ref, path)
				cfg, err := Parse(data, source)
				if err != nil {
					return err
				}
				for k, v := range cfg.Severities {
					merged.Severities[k] = v
				}
			}
		}
		return nil
	}

	// Precedence: default branch < PR base < PR head
	if refs.DefaultBranchRef != "" && refs.DefaultRepo != "" {
		if err := loadRef(refs.DefaultRepo, refs.DefaultBranchRef, "default"); err != nil {
			return merged, err
		}
	}

	if refs.BaseBranchRef != "" && refs.BaseRepo != "" {
		if err := loadRef(refs.BaseRepo, refs.BaseBranchRef, "base"); err != nil {
			return merged, err
		}
	}

	if refs.HeadRefOid != "" && refs.HeadRepo != "" {
		if err := loadRef(refs.HeadRepo, refs.HeadRefOid, "head"); err != nil {
			return merged, err
		}
	}

	return merged, nil
}
