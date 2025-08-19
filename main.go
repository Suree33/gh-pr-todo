package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Suree33/gh-pr-todo/internal"
	"github.com/briandowns/spinner"
	"github.com/cli/go-gh/v2"
	"github.com/fatih/color"
)

var (
	green = color.New(color.FgGreen).SprintFunc()
	red   = color.New(color.FgRed).SprintFunc()
)

func main() {
	sp := spinner.New(spinner.CharSets[14], 40*time.Millisecond)
	fetchingMsg := " Fetching PR diff..."
	sp.Suffix = fetchingMsg
	sp.Start()

	stdOut, stdErr, err := gh.Exec("pr", "diff")
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

	fmt.Fprintf(color.Output, "\nFound %d TODO comment(s):\n\n", len(todos))
	for i, todo := range todos {
		fmt.Printf("%d. [%s] %s:%d\n", i+1, todo.Type, todo.Filename, todo.Line)
		fmt.Printf("   %s\n\n", todo.Comment)
	}
}
