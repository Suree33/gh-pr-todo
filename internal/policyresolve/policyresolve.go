package policyresolve

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/Suree33/gh-pr-todo/internal/config"
	"github.com/Suree33/gh-pr-todo/internal/todotype"
)

// Target describes which config source should be used for a run.
type Target struct {
	Repo      string
	PR        string
	UseRemote bool
}

// Options contains the inputs required to resolve a ready-to-use TODO policy.
type Options struct {
	Target        Target
	CWD           string
	UserConfigDir string
	CLISeverities map[string]todotype.Severity
	CLIIgnored    []string
}

// ResolveTarget determines whether config should be loaded locally or from a
// remote repository, including PR URL normalization.
func ResolveTarget(repo, pr string) Target {
	if repo != "" {
		return Target{Repo: repo, PR: pr, UseRemote: true}
	}

	parsed, err := url.Parse(pr)
	if err != nil || parsed.Host == "" {
		return Target{PR: pr}
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "pull" || parts[0] == "" || parts[1] == "" || parts[3] == "" {
		return Target{PR: pr}
	}

	repo = parts[0] + "/" + parts[1]
	if parsed.Host != "github.com" {
		repo = parsed.Host + "/" + repo
	}

	return Target{Repo: repo, PR: parts[3], UseRemote: true}
}

// Resolve loads configuration from the appropriate source and applies config
// and CLI overrides on top of the default TODO policy.
func Resolve(fetcher config.RemoteConfigFetcher, opts Options) (todotype.Policy, error) {
	cfg, err := loadConfig(fetcher, opts)
	if err != nil {
		return todotype.Policy{}, err
	}

	policy := todotype.DefaultPolicy()
	if len(cfg.Severities) > 0 {
		policy = policy.WithSeverities(cfg.Severities)
	}
	if len(opts.CLISeverities) > 0 {
		policy = policy.WithSeverities(opts.CLISeverities)
	}

	ignoredSet := make(map[string]bool, len(cfg.Ignored)+len(opts.CLIIgnored))
	for todoType := range cfg.Ignored {
		ignoredSet[todoType] = true
	}
	for _, todoType := range opts.CLIIgnored {
		ignoredSet[strings.ToUpper(todoType)] = true
	}
	if len(ignoredSet) > 0 {
		ignored := make([]string, 0, len(ignoredSet))
		for todoType := range ignoredSet {
			ignored = append(ignored, todoType)
		}
		sort.Strings(ignored)
		policy = policy.WithIgnoredTypes(ignored)
	}

	return policy, nil
}

func loadConfig(fetcher config.RemoteConfigFetcher, opts Options) (config.Config, error) {
	if opts.Target.UseRemote {
		if fetcher == nil {
			return config.Config{}, fmt.Errorf("remote config fetcher is required")
		}

		remoteCfg, err := config.LoadRemote(fetcher, opts.Target.Repo, opts.Target.PR)
		if err != nil {
			return config.Config{}, err
		}
		if remoteCfg.Found {
			return remoteCfg, nil
		}
		return config.LoadGlobal(opts.UserConfigDir)
	}

	return config.LoadLocal(opts.CWD, opts.UserConfigDir)
}
