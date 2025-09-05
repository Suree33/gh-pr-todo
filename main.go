package main

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Suree33/gh-pr-todo/internal"
	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/briandowns/spinner"
	"github.com/cli/go-gh/v2"
	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

var (
	bold    = color.New(color.Bold).SprintFunc()
	green   = color.New(color.FgGreen).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	blue    = color.New(color.FgBlue).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
)

func main() {
	var (
		repo     string
		nameOnly bool
		isCount  bool
		isHelp   bool
		groupBy  types.GroupBy = types.GroupByNone
	)
	pflag.StringVarP(&repo, "repo", "R", "", "Select another repository using the [HOST/]OWNER/REPO format")
	pflag.BoolVar(&nameOnly, "name-only", false, "Display only names of the files containing TODO comments")
	pflag.BoolVarP(&isCount, "count", "c", false, "Display only the number of TODO comments")
	pflag.BoolVarP(&isHelp, "help", "h", false, "Display help information")
	pflag.Var(&groupBy, "group-by", "Group TODO comments by: \"file\" or \"type\"")
	pflag.Usage = func() {
		fmt.Fprintf(color.Output, "%s\n\n", "View TODO comments in the PR diff.")
		fmt.Fprintf(color.Output, "%s\n", bold("USAGE"))
		fmt.Fprintf(color.Output, "  %s\n\n", "gh pr-todo [<number> | <url> | <branch>] [flags]")
		fmt.Fprintf(color.Output, "%s\n", bold("FLAGS"))
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
		fmt.Println()
	}
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
			fmt.Fprintf(color.Output, "%s%s\n", red("✗"), " PR number, branch, or URL required when specifying repository\n")
			pflag.Usage()
			os.Exit(1)
		}
		pr = ""
	case 1:
		pr = args[0]
	default:
		fmt.Fprintf(color.Output, "%s%s\n", red("✗"), " Too many arguments\n")
		pflag.Usage()
		os.Exit(1)
	}

	if nameOnly {
		runNameOnly(repo, pr)
	} else if isCount {
		runCount(repo, pr)
	} else {
		runMain(repo, pr, groupBy)
	}
}

func runMain(repo string, pr string, groupBy types.GroupBy) {
	sp := spinner.New(spinner.CharSets[14], 40*time.Millisecond)
	fetchingMsg := " Fetching PR diff..."
	sp.Suffix = fetchingMsg
	sp.Start()

	args := []string{"pr", "diff"}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	if pr != "" {
		args = append(args, pr)
	}
	stdOut, stdErr, err := gh.Exec(args...)
	sp.Stop()

	if err == nil {
		fmt.Fprintf(color.Output, "%s%s\n", green("✔"), fetchingMsg)
	} else {
		fmt.Fprintf(color.Output, "%s%s\n", red("✗"), fetchingMsg)
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if stdErr.Len() > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", stdErr.String())
	}

	todos := internal.ParseDiff(stdOut.String())

	if len(todos) == 0 {
		fmt.Fprintf(color.Output, "\nNo TODO comments found in the diff.\n")
		return
	}

	fmt.Fprintf(color.Output, bold("\nFound %d TODO comment(s)\n\n"), len(todos))
	switch groupBy {
	case types.GroupByNone:
		for _, todo := range todos {
			fmt.Fprintf(color.Output, "* %s\n", blue(todo.Filename+":"+strconv.Itoa(todo.Line)))
			fmt.Fprintf(color.Output, "  %s\n\n", todo.Comment)
		}
	case types.GroupByFile:
		files := make(map[string][]types.TODO)
		maxLineNumberLen := 0
		for _, todo := range todos {
			files[todo.Filename] = append(files[todo.Filename], todo)
			if len(strconv.Itoa(todo.Line)) > maxLineNumberLen {
				maxLineNumberLen = len(strconv.Itoa(todo.Line))
			}
		}
		for filename, todos := range files {
			fmt.Fprintf(color.Output, "* %s\n", blue(filename))
			for _, todo := range todos {
				fmt.Fprintf(color.Output, "  %s%s: %s\n", strings.Repeat(" ", maxLineNumberLen-int(len(strconv.Itoa(todo.Line)))), green(strconv.Itoa(todo.Line)), todo.Comment)
			}
			fmt.Println()
		}
	case types.GroupByType:
		todoTypes := make(map[string][]types.TODO)
		for _, todo := range todos {
			todoTypes[todo.Type] = append(todoTypes[todo.Type], todo)
		}
		todoTypeKeys := slices.Collect(maps.Keys(todoTypes))
		slices.Sort(todoTypeKeys)
		for _, todoType := range todoTypeKeys {
			todos := todoTypes[todoType]
			fmt.Fprintf(color.Output, "%s%s%s\n", bold("["), bold(magenta(todoType)), bold("]"))
			for _, todo := range todos {
				fmt.Fprintf(color.Output, "* %s\n", blue(todo.Filename+":"+strconv.Itoa(todo.Line)))
				fmt.Fprintf(color.Output, "  %s\n\n", todo.Comment)
			}
		}
	}
}

func runCount(repo string, pr string) {
	sp := spinner.New(spinner.CharSets[14], 40*time.Millisecond)
	fetchingMsg := " Fetching PR diff..."
	sp.Suffix = fetchingMsg
	sp.Start()

	args := []string{"pr", "diff"}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	if pr != "" {
		args = append(args, pr)
	}
	stdOut, stdErr, err := gh.Exec(args...)
	sp.Stop()

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if stdErr.Len() > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", stdErr.String())
	}

	todos := internal.ParseDiff(stdOut.String())

	fmt.Fprintln(color.Output, len(todos))
}

func runNameOnly(repo string, pr string) {
	sp := spinner.New(spinner.CharSets[14], 40*time.Millisecond)
	fetchingMsg := " Fetching PR diff..."
	sp.Suffix = fetchingMsg
	sp.Start()

	args := []string{"pr", "diff"}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	if pr != "" {
		args = append(args, pr)
	}
	stdOut, stdErr, err := gh.Exec(args...)
	sp.Stop()

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if stdErr.Len() > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", stdErr.String())
	}

	todos := internal.ParseDiff(stdOut.String())

	if len(todos) == 0 {
		return
	}

	files := make(map[string]struct{})
	for _, todo := range todos {
		files[todo.Filename] = struct{}{}
	}

	for file := range files {
		fmt.Fprintln(color.Output, file)
	}
}
