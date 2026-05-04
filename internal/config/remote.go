package config

import (
	"fmt"
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

// LoadRemote loads config files from remote repository references.
// Precedence: PR head > PR base > default branch. Within each scope,
// .github/gh-pr-todo.yml replaces .gh-pr-todo.yml entirely.
func LoadRemote(fetcher RemoteConfigFetcher, repo, pr string) (Config, error) {
	refs, err := fetcher.FetchRemoteConfigRefs(repo, pr)
	if err != nil {
		return Config{}, fmt.Errorf("fetching remote refs: %w", err)
	}

	candidates := []struct {
		repo  string
		ref   string
		scope string
	}{
		{repo: refs.HeadRepo, ref: refs.HeadRefOid, scope: "head"},
		{repo: refs.BaseRepo, ref: refs.BaseBranchRef, scope: "base"},
		{repo: refs.DefaultRepo, ref: refs.DefaultBranchRef, scope: "default"},
	}

	for _, candidate := range candidates {
		if candidate.repo == "" || candidate.ref == "" {
			continue
		}

		cfg, found, err := loadRemoteRef(fetcher, candidate.repo, candidate.ref, candidate.scope)
		if err != nil {
			return Config{}, err
		}
		if found {
			return cfg, nil
		}
	}

	return Config{}, nil
}

func loadRemoteRef(fetcher RemoteConfigFetcher, repoName, ref, scope string) (Config, bool, error) {
	for _, path := range []string{".github/gh-pr-todo.yml", ".gh-pr-todo.yml"} {
		data, found, err := fetcher.FetchFileAtRef(repoName, path, ref)
		if err != nil {
			return Config{}, false, fmt.Errorf("fetching %s from %s at %s (%s): %w", path, repoName, ref, scope, err)
		}
		if found {
			source := fmt.Sprintf("%s:%s:%s", repoName, ref, path)
			cfg, err := Parse(data, source)
			if err != nil {
				return Config{}, true, err
			}
			return cfg, true, nil
		}
	}
	return Config{}, false, nil
}
