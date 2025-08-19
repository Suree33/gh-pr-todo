package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cli/go-gh/v2"
	"github.com/Suree33/gh-pr-todo/internal"
)

func main() {
	// TODO: Add support for custom diff sources
	stdOut, stdErr, err := gh.Exec("pr", "diff")
	if err != nil {
		log.Fatal(err)
	}
	
	if stdErr.Len() > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", stdErr.String())
	}
	
	todos := internal.ParseDiff(stdOut.String())
	
	if len(todos) == 0 {
		fmt.Println("No TODO comments found in the diff.")
		return
	}
	
	fmt.Printf("Found %d TODO comment(s):\n\n", len(todos))
	for i, todo := range todos {
		fmt.Printf("%d. [%s] %s:%d\n", i+1, todo.Type, todo.Filename, todo.Line)
		fmt.Printf("   %s\n\n", todo.Comment)
	}
}
