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
	pflag.BoolVar(&noCIFail, "no-ci-fail", false, "Disable non-zero exit when TODOs are found in CI")
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
		err   error
		count int
	)
	switch {
	case nameOnly:
		count, err = runNameOnly(fetcher, repo, pr, gha)
	case isCount:
		count, err = runCount(fetcher, repo, pr, gha)
	default:
		count, err = runMain(fetcher, repo, pr, groupBy, gha)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(exitCode(err, count, isCI(), noCIFail))
}

func exitCode(err error, count int, ci, noCIFail bool) int {
	if err != nil {
		return 1
	}
	if ci && !noCIFail && count > 0 {
		return 1
	}
	return 0
}

func isCI() bool {
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
	fmt.Fprintf(color.Output, "  %s\n", "CI               When truthy (e.g. \"1\", \"true\"), exits non-zero if any TODO is found.")
	fmt.Fprintf(color.Output, "  %s\n", "                 Override with --no-ci-fail.")
	fmt.Fprintf(color.Output, "  %s\n\n", "GITHUB_ACTIONS   When truthy, also emits GitHub Actions workflow commands so each TODO appears as an annotation.")
}

func runMain(fetcher ghclient.PRFetcher, repo, pr string, groupBy types.GroupBy, gha bool) (int, error) {
	sp := spinner.New(spinner.CharSets[14], 40*time.Millisecond)
	fetchingMsg := " Fetching PR diff..."
	sp.Suffix = fetchingMsg
	sp.Start()

	todos, err := ghclient.CollectTODOs(fetcher, repo, pr)
	sp.Stop()

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
	return len(todos), nil
}

func runCount(fetcher ghclient.PRFetcher, repo, pr string, gha bool) (int, error) {
	todos, err := ghclient.CollectTODOs(fetcher, repo, pr)
	if err != nil {
		return 0, err
	}
	output.PrintCount(todos)
	if gha {
		output.PrintWorkflowCommands(todos)
	}
	return len(todos), nil
}

func runNameOnly(fetcher ghclient.PRFetcher, repo, pr string, gha bool) (int, error) {
	todos, err := ghclient.CollectTODOs(fetcher, repo, pr)
	if err != nil {
		return 0, err
	}
	output.PrintFileNames(todos)
	if gha {
		output.PrintWorkflowCommands(todos)
	}
	return len(todos), nil
}
