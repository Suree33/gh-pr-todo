package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	ghclient "github.com/Suree33/gh-pr-todo/internal/github"
	"github.com/Suree33/gh-pr-todo/internal/output"
	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

type cliFlags struct {
	repo     string
	nameOnly bool
	count    bool
	help     bool
	noCIFail bool
	groupBy  types.GroupBy
}

// registerFlags registers all CLI flags on fs and returns the bound values.
// main() and tests share this so help/usage text never drifts between them.
func registerFlags(fs *pflag.FlagSet) *cliFlags {
	f := &cliFlags{groupBy: types.GroupByNone}
	fs.StringVarP(&f.repo, "repo", "R", "", "Select another repository using the [HOST/]OWNER/REPO format")
	fs.BoolVar(&f.nameOnly, "name-only", false, "Display only names of the files containing TODO comments")
	fs.BoolVarP(&f.count, "count", "c", false, "Display only the number of TODO comments")
	fs.BoolVarP(&f.help, "help", "h", false, "Display help information")
	fs.BoolVar(&f.noCIFail, "no-ci-fail", false, "Disable non-zero exit when warning-level TODOs (FIXME, HACK, XXX, BUG) are found in CI")
	fs.Var(&f.groupBy, "group-by", "Group TODO comments by: \"file\" or \"type\"")
	return f
}

func main() {
	flags := registerFlags(pflag.CommandLine)
	pflag.Usage = printUsage
	pflag.Parse()
	args := pflag.Args()

	if flags.help {
		pflag.Usage()
		os.Exit(0)
	}

	var pr string
	switch len(args) {
	case 0:
		if flags.repo != "" {
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
		err            error
		ciFailingCount int
	)
	switch {
	case flags.nameOnly:
		ciFailingCount, err = runNameOnly(fetcher, flags.repo, pr)
	case flags.count:
		ciFailingCount, err = runCount(fetcher, flags.repo, pr)
	default:
		ciFailingCount, err = runMain(fetcher, flags.repo, pr, flags.groupBy, gha)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(exitCode(err, ciFailingCount, isCI(), flags.noCIFail))
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
	fmt.Fprintf(color.Output, "  %s\n", "CI               When truthy (e.g. \"1\", \"true\"), exits non-zero if any warning-level TODO")
	fmt.Fprintf(color.Output, "  %s\n", "                 (FIXME, HACK, XXX, BUG) is found. Notice-level types (TODO, NOTE, ...) do not")
	fmt.Fprintf(color.Output, "  %s\n", "                 trigger a failure, matching the GitHub Actions workflow command severity.")
	fmt.Fprintf(color.Output, "  %s\n", "                 Override with --no-ci-fail.")
	fmt.Fprintf(color.Output, "  %s\n", "GITHUB_ACTIONS   When truthy, emits GitHub Actions workflow commands so each TODO appears as an annotation.")
	fmt.Fprintf(color.Output, "  %s\n", "                 Only emitted in the default mode; --count and --name-only stay machine-readable.")
	fmt.Fprintf(color.Output, "  %s\n\n", "                 Implies CI=true, so --no-ci-fail is required to suppress the non-zero exit.")
}

func runMain(fetcher ghclient.PRFetcher, repo, pr string, groupBy types.GroupBy, gha bool) (int, error) {
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
		return 0, err
	}
	fmt.Fprintf(color.Output, "%s%s\n", output.Green("✔"), fetchingMsg)

	if len(todos) == 0 {
		fmt.Fprintf(color.Output, "\nNo TODO comments found in the diff.\n")
		return 0, nil
	}

	fmt.Fprintf(color.Output, output.Bold("\nFound %d TODO comment(s)\n\n"), len(todos))
	output.PrintTODOs(todos, groupBy)
	if gha {
		output.PrintWorkflowCommands(todos)
	}
	return output.CountCIFailing(todos), nil
}

func runCount(fetcher ghclient.PRFetcher, repo, pr string) (int, error) {
	todos, err := ghclient.CollectTODOs(fetcher, repo, pr)
	if err != nil {
		return 0, err
	}
	output.PrintCount(todos)
	return output.CountCIFailing(todos), nil
}

func runNameOnly(fetcher ghclient.PRFetcher, repo, pr string) (int, error) {
	todos, err := ghclient.CollectTODOs(fetcher, repo, pr)
	if err != nil {
		return 0, err
	}
	output.PrintFileNames(todos)
	return output.CountCIFailing(todos), nil
}
