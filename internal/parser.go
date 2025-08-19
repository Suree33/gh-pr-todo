package internal

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Suree33/gh-pr-todo/pkg/types"
)

var (
	todoRegex = regexp.MustCompile(`(?i)//\s*(TODO|FIXME|HACK|NOTE|XXX|BUG):?\s*(.*)`)
	hunkRegex = regexp.MustCompile(`^@@\s+\-\d+(?:,\d+)?\s+\+(\d+)(?:,\d+)?\s+@@`)
)

// ParseDiff extracts TODO comments from git diff output
func ParseDiff(diffOutput string) []types.TODO {
	var todos []types.TODO
	lines := strings.Split(diffOutput, "\n")

	var currentFile string
	var lineNumber int

	for _, line := range lines {
		// TODO(human): Implement the core parsing logic here
		// This function should:
		// 1. Track current filename from diff headers (lines starting with "+++")
		// 2. Track line numbers from diff hunks (lines starting with "@@")
		// 3. Find TODO patterns in added lines (lines starting with "+" or " ")
		// 4. Extract the comment content and create TODO structs
		if after, ok := strings.CutPrefix(line, "+++ b/"); ok {
			currentFile = after
		} else if after, ok := strings.CutPrefix(line, "@@"); ok {
			if matches := hunkRegex.FindStringSubmatch(after); len(matches) > 1 {
				if startLine, err := strconv.Atoi(matches[1]); err == nil {
					lineNumber = startLine - 1 // -1 because we increment before processing
				}
			}
		} else if after, ok := strings.CutPrefix(line, "+"); ok {
			lineNumber++
			if matches := todoRegex.FindStringSubmatch(after); len(matches) > 2 {
				todos = append(todos, types.TODO{
					Filename: currentFile,
					Line:     lineNumber,
					Comment:  strings.TrimSpace(matches[2]),
					Type:     strings.ToUpper(matches[1]),
				})
			}
		} else if _, ok := strings.CutPrefix(line, " "); ok {
			lineNumber++
		}
	}

	return todos
}
