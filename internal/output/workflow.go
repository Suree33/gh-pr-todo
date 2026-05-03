package output

import (
	"fmt"
	"strings"

	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/fatih/color"
)

// PrintWorkflowCommands writes a GitHub Actions workflow command annotation
// for each TODO so that they show up in the PR/check-run UI.
//
// See https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands
func PrintWorkflowCommands(todos []types.TODO) {
	for _, todo := range todos {
		fmt.Fprintf(color.Output, "::%s file=%s,line=%d,title=%s::%s\n",
			workflowCommandFor(todo.Type),
			escapeWorkflowProperty(todo.Filename),
			todo.Line,
			escapeWorkflowProperty(todo.Type),
			escapeWorkflowMessage(todo.Comment),
		)
	}
}

func workflowCommandFor(todoType string) string {
	switch strings.ToUpper(todoType) {
	case "FIXME", "HACK", "XXX", "BUG":
		return "warning"
	default:
		return "notice"
	}
}

// IsCIFailing reports whether a TODO of the given type should cause a
// non-zero exit in CI. It mirrors the severity used in GitHub Actions
// workflow commands: warning-level types fail, notice-level types do not.
func IsCIFailing(todoType string) bool {
	return workflowCommandFor(todoType) == "warning"
}

// CountCIFailing returns the number of TODOs whose type maps to a
// warning-level workflow command.
func CountCIFailing(todos []types.TODO) int {
	n := 0
	for _, t := range todos {
		if IsCIFailing(t.Type) {
			n++
		}
	}
	return n
}

func escapeWorkflowMessage(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

func escapeWorkflowProperty(s string) string {
	s = escapeWorkflowMessage(s)
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}
