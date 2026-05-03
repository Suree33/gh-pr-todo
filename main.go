package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	ghclient "github.com/Suree33/gh-pr-todo/internal/github"
	"github.com/Suree33/gh-pr-todo/internal/output"
	"github.com/Suree33/gh-pr-todo/internal/todotype"
	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

func main() {
	var (
		repo     string
		nameOnly bool
		isCount  bool
		isHelp   bool
		noCIFail bool
		groupBy  = types.GroupByNone
	)
	pflag.StringVarP(&repo, "repo", "R", "", "Select another repository using the [HOST/]OWNER/REPO format")
	pflag.BoolVar(&nameOnly, "name-only", false, "Display only names of the files containing TODO comments")
	pflag.BoolVarP(&isCount, "count", "c", false, "Display only the number of TODO comments")
	pflag.BoolVarP(&isHelp, "help", "h", false, "Display help information")
	pflag.BoolVar(&noCIFail, "no-ci-fail", false, "Disable non-zero exit when error-level TODOs are found in CI")
	pflag.Var(&groupBy, "group-by", "Group TODO comments by: \"file\" or \"type\"")
	pflag.Usage = printUsage
	pflag.Parse()
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

	fetcher := ghclient.NewClient()
	gha := isGitHubActions()
	var (
		err    error
		result runResult
	)
	switch {
	case nameOnly:
		result, err = runNameOnly(fetcher, repo, pr)
	case isCount:
		result, err = runCount(fetcher, repo, pr)
	default:
		result, err = runMain(fetcher, repo, pr, groupBy, gha)
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

// newRunResult computes a runResult from a TODO slice.
func newRunResult(todos []types.TODO) runResult {
	return runResult{
		totalCount:     len(todos),
		ciFailingCount: todotype.CountCIFailing(todos),
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

func printUsage() {
	fmt.Fprintf(color.Output, "%s\n\n", "View TODO comments in the PR diff.")
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
}

func runMain(fetcher ghclient.PRFetcher, repo, pr string, groupBy types.GroupBy, gha bool) (runResult, error) {
	fetchingMsg := " Fetching PR diff..."
	var sp *spinner.Spinner
	if !gha {
		sp = spinner.New(spinner.CharSets[14], 40*time.Millisecond)
		sp.Suffix = fetchingMsg
		sp.Start()
	}

	todos, err := ghclient.CollectTODOs(fetcher, repo, pr)
	if sp != nil {
		sp.Stop()
	}

	if err != nil {
		fmt.Fprintf(color.Output, "%s%s\n", output.Red("✗"), fetchingMsg)
		return runResult{}, err
	}
	fmt.Fprintf(color.Output, "%s%s\n", output.Green("✔"), fetchingMsg)

	if len(todos) == 0 {
		fmt.Fprintf(color.Output, "\nNo TODO comments found in the diff.\n")
		return runResult{}, nil
	}

	fmt.Fprintf(color.Output, output.Bold("\nFound %d TODO comment(s)\n\n"), len(todos))
	output.PrintTODOs(todos, groupBy)
	if gha {
		output.PrintWorkflowCommands(todos)
	}
	return newRunResult(todos), nil
}

func runCount(fetcher ghclient.PRFetcher, repo, pr string) (runResult, error) {
	todos, err := ghclient.CollectTODOs(fetcher, repo, pr)
	if err != nil {
		return runResult{}, err
	}
	output.PrintCount(todos)
	return newRunResult(todos), nil
}

func runNameOnly(fetcher ghclient.PRFetcher, repo, pr string) (runResult, error) {
	todos, err := ghclient.CollectTODOs(fetcher, repo, pr)
	if err != nil {
		return runResult{}, err
	}
	output.PrintFileNames(todos)
	return newRunResult(todos), nil
}
