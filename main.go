package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Suree33/gh-pr-todo/internal"
	"github.com/briandowns/spinner"
	"github.com/cli/go-gh/v2"
	"github.com/fatih/color"
)

var (
	bold  = color.New(color.Bold).SprintFunc()
	green = color.New(color.FgGreen).SprintFunc()
	red   = color.New(color.FgRed).SprintFunc()
	blue  = color.New(color.FgBlue).SprintFunc()
)

func main() {
	var repo string
	flag.StringVar(&repo, "R", "", "Select another repository using the [HOST/]OWNER/REPO format")
	flag.StringVar(&repo, "repo", "", "Select another repository using the [HOST/]OWNER/REPO format")
	flag.Usage = func() {
		fmt.Fprintf(color.Output, "%s\n\n", "View TODO comments in the PR diff.")
		fmt.Fprintf(color.Output, "%s\n", bold("USAGE"))
		fmt.Fprintf(color.Output, "  %s\n\n", "gh pr-todo [<number> | <url> | <branch>] [flags]")
		fmt.Fprintf(color.Output, "%s\n", bold("FLAGS"))
		fmt.Fprintf(color.Output, "  %s %s\n", "-R, --repo", "[HOST/]OWNER/REPO")
		fmt.Fprintf(color.Output, "      %s\n", "Select another repository using the [HOST/]OWNER/REPO format")
		fmt.Fprintf(color.Output, "  %s\n\n", "-h, --help")
	}
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		runMain(&repo, nil)
		return
	} else if len(args) == 1 {
		runMain(&repo, &args[0])
		return
	} else {
		fmt.Fprintf(color.Output, "%s%s\n", red("✗"), " Too many arguments\n")
		flag.Usage()
		os.Exit(1)
	}
}

func runMain(repo *string, pr *string) {
	sp := spinner.New(spinner.CharSets[14], 40*time.Millisecond)
	fetchingMsg := " Fetching PR diff..."
	sp.Suffix = fetchingMsg
	sp.Start()

	args := []string{"pr", "diff"}
	if repo != nil {
		args = append(args, "-R", *repo)
	}
	if pr != nil {
		args = append(args, *pr)
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
	for _, todo := range todos {
		fmt.Fprintf(color.Output, "* %s\n", blue(todo.Filename+":"+strconv.Itoa(todo.Line)))
		fmt.Fprintf(color.Output, "  %s\n\n", todo.Comment)
	}
}
