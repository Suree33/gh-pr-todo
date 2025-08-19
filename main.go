package main

import (
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

	fmt.Fprintf(color.Output, bold("\nFound %d TODO comment(s)\n\n"), len(todos))
	for _, todo := range todos {
		fmt.Fprintf(color.Output, "* %s\n", blue(todo.Filename+":"+strconv.Itoa(todo.Line)))
		fmt.Fprintf(color.Output, "    %s\n\n", todo.Comment)
	}
}
