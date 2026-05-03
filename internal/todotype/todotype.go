// Package todotype provides the default classification for TODO-style
// comment types. It is the single source of truth for severity and
// CI-failure policy.
package todotype

import (
	"strings"

	"github.com/Suree33/gh-pr-todo/pkg/types"
)

// Severity represents the GitHub Actions annotation severity level.
type Severity string

const (
	SeverityNotice  Severity = "notice"
	SeverityWarning Severity = "warning"
)

// SeverityFor returns the default annotation severity for a TODO type.
// FIXME, HACK, XXX, BUG → warning. All others (TODO, NOTE, unknown) → notice.
func SeverityFor(todoType string) Severity {
	switch strings.ToUpper(todoType) {
	case "FIXME", "HACK", "XXX", "BUG":
		return SeverityWarning
	default:
		return SeverityNotice
	}
}

// IsCIFailing reports whether a TODO of the given type should cause a
// non-zero exit in CI. It mirrors the default severity: warning-level
// types fail, notice-level types do not.
func IsCIFailing(todoType string) bool {
	return SeverityFor(todoType) == SeverityWarning
}

// CountCIFailing returns the number of TODOs whose type maps to a
// CI-failing severity.
func CountCIFailing(todos []types.TODO) int {
	n := 0
	for _, t := range todos {
		if IsCIFailing(t.Type) {
			n++
		}
	}
	return n
}
