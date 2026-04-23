// Package output renders TODO results to the terminal.
package output

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/fatih/color"
)

// Exported SprintFuncs so other packages (e.g. main) can share the same palette
// for status markers without redefining them.
var (
	Bold    = color.New(color.Bold).SprintFunc()
	Green   = color.New(color.FgGreen).SprintFunc()
	Red     = color.New(color.FgRed).SprintFunc()
	Blue    = color.New(color.FgBlue).SprintFunc()
	Magenta = color.New(color.FgMagenta).SprintFunc()
)

// PrintTODOs renders the given TODOs using the requested grouping.
func PrintTODOs(todos []types.TODO, groupBy types.GroupBy) {
	switch groupBy {
	case types.GroupByNone:
		printFlat(todos)
	case types.GroupByFile:
		printGroupedByFile(todos)
	case types.GroupByType:
		printGroupedByType(todos)
	}
}

// PrintFileNames writes one unique filename per line for every TODO.
func PrintFileNames(todos []types.TODO) {
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

// PrintCount writes the number of TODOs to stdout.
func PrintCount(todos []types.TODO) {
	fmt.Fprintln(color.Output, len(todos))
}

func printFlat(todos []types.TODO) {
	for _, todo := range todos {
		fmt.Fprintf(color.Output, "* %s\n", Blue(todo.Filename+":"+strconv.Itoa(todo.Line)))
		fmt.Fprintf(color.Output, "  %s\n\n", todo.Comment)
	}
}

func printGroupedByFile(todos []types.TODO) {
	files := make(map[string][]types.TODO)
	maxLineNumberLen := 0
	for _, todo := range todos {
		files[todo.Filename] = append(files[todo.Filename], todo)
		if n := len(strconv.Itoa(todo.Line)); n > maxLineNumberLen {
			maxLineNumberLen = n
		}
	}
	for filename, todos := range files {
		fmt.Fprintf(color.Output, "* %s\n", Blue(filename))
		for _, todo := range todos {
			lineStr := strconv.Itoa(todo.Line)
			fmt.Fprintf(color.Output, "  %s%s: %s\n", strings.Repeat(" ", maxLineNumberLen-len(lineStr)), Green(lineStr), todo.Comment)
		}
		fmt.Println()
	}
}

func printGroupedByType(todos []types.TODO) {
	todoTypes := make(map[string][]types.TODO)
	for _, todo := range todos {
		todoTypes[todo.Type] = append(todoTypes[todo.Type], todo)
	}
	todoTypeKeys := slices.Collect(maps.Keys(todoTypes))
	slices.Sort(todoTypeKeys)
	for _, todoType := range todoTypeKeys {
		todos := todoTypes[todoType]
		fmt.Fprintf(color.Output, "%s%s%s\n", Bold("["), Bold(Magenta(todoType)), Bold("]"))
		for _, todo := range todos {
			fmt.Fprintf(color.Output, "* %s\n", Blue(todo.Filename+":"+strconv.Itoa(todo.Line)))
			fmt.Fprintf(color.Output, "  %s\n\n", todo.Comment)
		}
	}
}
