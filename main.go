package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Suree33/gh-pr-todo/internal/config"
	ghclient "github.com/Suree33/gh-pr-todo/internal/github"
	"github.com/Suree33/gh-pr-todo/internal/output"
	"github.com/Suree33/gh-pr-todo/internal/todotype"
	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

func registerFlags(fs *pflag.FlagSet, repo *string, nameOnly, isCount, isHelp, noCIFail *bool, groupBy *types.GroupBy, sevFlag *severityFlag) {
	fs.StringVarP(repo, "repo", "R", "", "Select another repository using the [HOST/]OWNER/REPO format")
	fs.BoolVar(nameOnly, "name-only", false, "Display only names of the files containing TODO-style comments")
	fs.BoolVarP(isCount, "count", "c", false, "Display only the number of TODO-style comments")
	fs.BoolVarP(isHelp, "help", "h", false, "Display help information")
	fs.BoolVar(noCIFail, "no-ci-fail", false, "Disable non-zero exit when error-level TODOs are found in CI")
	fs.Var(groupBy, "group-by", "Group TODO-style comments by: \"file\" or \"type\"")
	fs.Var(sevFlag, "severity", "Override severity for one or more TODO types. Format: LEVEL=TYPE[,TYPE...] (e.g. --severity warning=TODO,HACK)")
}

func main() {
	// Use ContinueOnError so we can print a clear error and exit code 1
	// instead of pflag's default ExitOnError (exit code 2).
	pflag.CommandLine = pflag.NewFlagSet("gh pr-todo", pflag.ContinueOnError)
	pflag.CommandLine.SetOutput(io.Discard)

	var (
		repo     string
		nameOnly bool
		isCount  bool
		isHelp   bool
		noCIFail bool
		groupBy  = types.GroupByNone
		sevFlag  = newSeverityFlag()
	)
	registerFlags(pflag.CommandLine, &repo, &nameOnly, &isCount, &isHelp, &noCIFail, &groupBy, sevFlag)
	pflag.Usage = printUsage
	if err := pflag.CommandLine.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	args := pflag.Args()

	if isHelp {
		pflag.Usage()
		os.Exit(0)
	}

	var pr string
	switch len(args) {
	case 0:
		if repo != "" {
			fmt.Fprintf(color.Output, "%s%s\n", output.Red("✗"), " PR number, branch, or URL required when specifying repository\n")
			pflag.Usage()
			os.Exit(1)
		}
		pr = ""
	case 1:
		pr = args[0]
	default:
		fmt.Fprintf(color.Output, "%s%s\n", output.Red("✗"), " Too many arguments\n")
		pflag.Usage()
		os.Exit(1)
	}

	// Build policy with config and CLI overrides.
	// Precedence: default < config < CLI.
	policy := todotype.DefaultPolicy()

	userConfigDir := ""
	if dir, err := os.UserConfigDir(); err == nil {
		userConfigDir = dir
	}

	configRepo, configPR, useRemoteConfig := resolveConfigTarget(repo, pr)

	var (
		cfg config.Config
		err error
	)
	if useRemoteConfig {
		cfg, err = config.LoadGlobal(userConfigDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Config error:", err)
			os.Exit(1)
		}
		if len(cfg.Severities) > 0 {
			policy = policy.WithSeverities(cfg.Severities)
		}

		fetcher := ghclient.NewClient()
		cfg, err = config.LoadRemote(fetcher, configRepo, configPR)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Remote config error:", err)
			os.Exit(1)
		}
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error getting current directory:", err)
			os.Exit(1)
		}
		cfg, err = config.LoadLocal(cwd, userConfigDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Config error:", err)
			os.Exit(1)
		}
	}
	if len(cfg.Severities) > 0 {
		policy = policy.WithSeverities(cfg.Severities)
	}

	if len(sevFlag.assignments) > 0 {
		policy = policy.WithSeverities(sevFlag.assignments)
	}

	fetcher := ghclient.NewClient()
	gha := isGitHubActions()
	var result runResult
	switch {
	case nameOnly:
		result, err = runNameOnly(fetcher, repo, pr, policy)
	case isCount:
		result, err = runCount(fetcher, repo, pr, policy)
	default:
		result, err = runMain(fetcher, repo, pr, groupBy, gha, policy)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(exitCode(err, result.ciFailingCount, isCI(), noCIFail))
}

// runResult groups the total TODO count and the CI-failing count from a run.
type runResult struct {
	totalCount     int
	ciFailingCount int
}

// severityFlag accumulates --severity LEVEL=TYPE[,TYPE...] flag values.
// Each flag adds one or more type→severity assignments; later assignments
// for the same type (case-insensitive) replace earlier ones (last-wins).
type severityFlag struct {
	assignments map[string]todotype.Severity
}

func newSeverityFlag() *severityFlag {
	return &severityFlag{assignments: make(map[string]todotype.Severity)}
}

func (s *severityFlag) String() string {
	return fmt.Sprintf("%v", s.assignments)
}

func (s *severityFlag) Set(val string) error {
	eq := strings.IndexByte(val, '=')
	if eq < 0 {
		return fmt.Errorf("invalid --severity %q: expected LEVEL=TYPE[,TYPE...] (e.g. warning=TODO,HACK)", val)
	}

	levelStr := strings.TrimSpace(val[:eq])
	typesStr := strings.TrimSpace(val[eq+1:])

	if levelStr == "" {
		return fmt.Errorf("invalid --severity %q: severity level is empty", val)
	}
	if typesStr == "" {
		return fmt.Errorf("invalid --severity %q: type list is empty", val)
	}

	var severity todotype.Severity
	switch strings.ToLower(levelStr) {
	case "notice":
		severity = todotype.SeverityNotice
	case "warning":
		severity = todotype.SeverityWarning
	case "error":
		severity = todotype.SeverityError
	default:
		return fmt.Errorf("invalid severity level %q in --severity %q: allowed values are notice, warning, error", levelStr, val)
	}

	pending := make(map[string]todotype.Severity)
	for _, typ := range strings.Split(typesStr, ",") {
		t := strings.TrimSpace(typ)
		if t == "" {
			return fmt.Errorf("invalid --severity %q: type name is empty", val)
		}
		if strings.ContainsRune(t, '=') {
			return fmt.Errorf("invalid --severity %q: type name %q must not contain '='", val, t)
		}
		// Normalize type to uppercase for last-wins semantics across flags.
		pending[strings.ToUpper(t)] = severity
	}
	for todoType, severity := range pending {
		s.assignments[todoType] = severity
	}

	return nil
}

func (s *severityFlag) Type() string { return "severity" }

// newRunResult computes a runResult from a TODO slice using the given policy.
func newRunResult(todos []types.TODO, policy todotype.Policy) runResult {
	return runResult{
		totalCount:     len(todos),
		ciFailingCount: policy.CountCIFailing(todos),
	}
}

func exitCode(err error, ciFailingCount int, ci, noCIFail bool) int {
	if err != nil {
		return 1
	}
	if ci && !noCIFail && ciFailingCount > 0 {
		return 1
	}
	return 0
}

func isCI() bool {
	if isGitHubActions() {
		return true
	}
	v := strings.TrimSpace(os.Getenv("CI"))
	ok, err := strconv.ParseBool(v)
	return err == nil && ok
}

func isGitHubActions() bool {
	v := strings.TrimSpace(os.Getenv("GITHUB_ACTIONS"))
	ok, err := strconv.ParseBool(v)
	return err == nil && ok
}

func resolveConfigTarget(repo, pr string) (string, string, bool) {
	if repo != "" {
		return repo, pr, true
	}
	parsed, err := url.Parse(pr)
	if err != nil || parsed.Host == "" {
		return "", pr, false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "pull" || parts[0] == "" || parts[1] == "" || parts[3] == "" {
		return "", pr, false
	}
	repo = parts[0] + "/" + parts[1]
	if parsed.Host != "github.com" {
		repo = parsed.Host + "/" + repo
	}
	return repo, parts[3], true
}

func printUsage() {
	fmt.Fprintf(color.Output, "%s\n\n", "View TODO-style comments in the PR diff.")
	fmt.Fprintf(color.Output, "%s\n", output.Bold("USAGE"))
	fmt.Fprintf(color.Output, "  %s\n\n", "gh pr-todo [<number> | <url> | <branch>] [flags]")
	fmt.Fprintf(color.Output, "%s\n", output.Bold("FLAGS"))
	maxLen := 0
	pflag.VisitAll(func(f *pflag.Flag) {
		nameLen := len(f.Name)
		if f.Shorthand != "" {
			nameLen += len(f.Shorthand) + 2
		}
		if nameLen > maxLen {
			maxLen = nameLen
		}
	})
	pflag.VisitAll(func(f *pflag.Flag) {
		if f.Shorthand != "" {
			fmt.Fprintf(color.Output, "  -%-*s, --%-*s %s\n", len(f.Shorthand), f.Shorthand, maxLen+2, f.Name, f.Usage)
		} else {
			fmt.Fprintf(color.Output, "      --%-*s %s\n", maxLen+2, f.Name, f.Usage)
		}
	})
	fmt.Fprintln(color.Output)
	fmt.Fprintf(color.Output, "%s\n", output.Bold("ENVIRONMENT"))
	fmt.Fprintf(color.Output, "  %s\n", "CI               When truthy (e.g. \"1\", \"true\"), exits non-zero if any")
	fmt.Fprintf(color.Output, "  %s\n", "                 error-level TODO is found. By default, no built-in")
	fmt.Fprintf(color.Output, "  %s\n", "                 keyword maps to error-level, so CI does not fail.")
	fmt.Fprintf(color.Output, "  %s\n", "                 Use --no-ci-fail to disable even if error-level types exist.")
	fmt.Fprintf(color.Output, "  %s\n", "GITHUB_ACTIONS   When truthy, emits GitHub Actions workflow annotations.")
	fmt.Fprintf(color.Output, "  %s\n", "                 Implies CI=true; --no-ci-fail suppresses error-level exits.")
	fmt.Fprintf(color.Output, "  %s\n\n", "                 Only emitted in the default mode; --count and --name-only stay machine-readable.")
	fmt.Fprintf(color.Output, "%s\n", output.Bold("SEVERITY OVERRIDES"))
	fmt.Fprintf(color.Output, "  %s\n", "Use --severity LEVEL=TYPE[,TYPE...] to override severities.")
	fmt.Fprintf(color.Output, "  %s\n", "Affects workflow annotation levels and CI exits for error-level types.")
	fmt.Fprintf(color.Output, "  %s\n\n", "Example: --severity warning=TODO,HACK --severity error=FIXME")
	fmt.Fprintf(color.Output, "%s\n", output.Bold("CONFIGURATION"))
	fmt.Fprintf(color.Output, "  %s\n", "Severity overrides can be configured in YAML config files.")
	fmt.Fprintf(color.Output, "  %s\n", "Configured custom types are detected alongside the built-in markers.")
	fmt.Fprintf(color.Output, "  %s\n", "Schema: severity:\n    TYPE: notice|warning|error")
	fmt.Fprintf(color.Output, "  %s\n", "Config file paths and precedence (later wins):")
	fmt.Fprintf(color.Output, "  %s\n", "  1. user config dir/gh-pr-todo/config.yml (global)")
	fmt.Fprintf(color.Output, "  %s\n", "  2. <repo>/.gh-pr-todo.yml (repo root)")
	fmt.Fprintf(color.Output, "  %s\n", "  3. <repo>/.github/gh-pr-todo.yml (narrower repo config)")
	fmt.Fprintf(color.Output, "  %s\n", "  4. CLI --severity flag (highest priority)")
	fmt.Fprintf(color.Output, "  %s\n", "When targeting another repository with --repo or a PR URL,")
	fmt.Fprintf(color.Output, "  %s\n", "repo-local configs are replaced by remote configs:")
	fmt.Fprintf(color.Output, "  %s\n", "  1. global config")
	fmt.Fprintf(color.Output, "  %s\n", "  2. remote default branch config")
	fmt.Fprintf(color.Output, "  %s\n", "  3. remote PR base branch config")
	fmt.Fprintf(color.Output, "  %s\n", "  4. remote PR head branch config")
	fmt.Fprintf(color.Output, "  %s\n", "  5. CLI --severity flag (highest priority)")
	fmt.Fprintf(color.Output, "  %s\n\n", "Example config:\n# .github/gh-pr-todo.yml\nseverity:\n  TODO: warning\n  FIXME: error")
}

func runMain(fetcher ghclient.PRFetcher, repo, pr string, groupBy types.GroupBy, gha bool, policy todotype.Policy) (runResult, error) {
	fetchingMsg := " Fetching PR diff..."
	var sp *spinner.Spinner
	if !gha {
		sp = spinner.New(spinner.CharSets[14], 40*time.Millisecond)
		sp.Suffix = fetchingMsg
		sp.Start()
	}

	todos, err := ghclient.CollectTODOs(fetcher, repo, pr, policy.Types())
	if sp != nil {
		sp.Stop()
	}

	if err != nil {
		fmt.Fprintf(color.Output, "%s%s\n", output.Red("✗"), fetchingMsg)
		return runResult{}, err
	}
	fmt.Fprintf(color.Output, "%s%s\n", output.Green("✔"), fetchingMsg)

	if len(todos) == 0 {
		fmt.Fprintf(color.Output, "\nNo TODO-style comments found in the diff.\n")
		return runResult{}, nil
	}

	fmt.Fprintf(color.Output, output.Bold("\nFound %d TODO-style comment(s)\n\n"), len(todos))
	output.PrintTODOs(todos, groupBy)
	if gha {
		output.PrintWorkflowCommands(todos, policy)
	}
	return newRunResult(todos, policy), nil
}

func runCount(fetcher ghclient.PRFetcher, repo, pr string, policy todotype.Policy) (runResult, error) {
	todos, err := ghclient.CollectTODOs(fetcher, repo, pr, policy.Types())
	if err != nil {
		return runResult{}, err
	}
	output.PrintCount(todos)
	return newRunResult(todos, policy), nil
}

func runNameOnly(fetcher ghclient.PRFetcher, repo, pr string, policy todotype.Policy) (runResult, error) {
	todos, err := ghclient.CollectTODOs(fetcher, repo, pr, policy.Types())
	if err != nil {
		return runResult{}, err
	}
	output.PrintFileNames(todos)
	return newRunResult(todos, policy), nil
}
