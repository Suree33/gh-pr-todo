// Package todotype provides the default classification for TODO-style
// comment types. It is the single source of truth for severity and
// CI-failure policy.
package todotype

import (
	"sort"
	"strings"

	"github.com/Suree33/gh-pr-todo/pkg/types"
)

// Severity represents the GitHub Actions annotation severity level.
type Severity string

const (
	SeverityNotice  Severity = "notice"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Policy classifies TODO types into severities and CI-failing levels.
type Policy struct {
	severityByType      map[string]Severity
	ciFailingSeverities map[Severity]bool
	ignoredTypes        map[string]bool
}

// DefaultPolicy returns the default TODO type policy.
func DefaultPolicy() Policy {
	return Policy{
		severityByType: map[string]Severity{
			"FIXME": SeverityWarning,
			"HACK":  SeverityWarning,
			"XXX":   SeverityWarning,
			"BUG":   SeverityWarning,
		},
		ciFailingSeverities: map[Severity]bool{
			SeverityError: true,
		},
	}
}

// WithSeverity returns a copy of the policy with a severity override for one type.
func (p Policy) WithSeverity(todoType string, severity Severity) Policy {
	return p.WithSeverities(map[string]Severity{todoType: severity})
}

// WithSeverities returns a copy of the policy with severity overrides applied.
func (p Policy) WithSeverities(overrides map[string]Severity) Policy {
	clone := Policy{
		severityByType:      make(map[string]Severity, len(p.severityByType)+len(overrides)),
		ciFailingSeverities: make(map[Severity]bool, len(p.ciFailingSeverities)),
		ignoredTypes:        make(map[string]bool, len(p.ignoredTypes)),
	}
	for todoType, severity := range p.severityByType {
		clone.severityByType[todoType] = severity
	}
	for severity, failing := range p.ciFailingSeverities {
		clone.ciFailingSeverities[severity] = failing
	}
	for t := range p.ignoredTypes {
		clone.ignoredTypes[t] = true
	}
	for todoType, severity := range overrides {
		clone.severityByType[normalizeTodoType(todoType)] = severity
	}
	return clone
}

// WithIgnoredTypes returns a copy of the policy with the ignored type set
// replaced by the given types. Ignored types are excluded from Types() and
// therefore from detection, output, annotations, and CI failure counts.
func (p Policy) WithIgnoredTypes(types []string) Policy {
	clone := Policy{
		severityByType:      make(map[string]Severity, len(p.severityByType)),
		ciFailingSeverities: make(map[Severity]bool, len(p.ciFailingSeverities)),
		ignoredTypes:        make(map[string]bool, len(types)),
	}
	for todoType, severity := range p.severityByType {
		clone.severityByType[todoType] = severity
	}
	for severity, failing := range p.ciFailingSeverities {
		clone.ciFailingSeverities[severity] = failing
	}
	for _, t := range types {
		clone.ignoredTypes[normalizeTodoType(t)] = true
	}
	return clone
}

// SeverityFor returns the annotation severity for a TODO type.
func (p Policy) SeverityFor(todoType string) Severity {
	severity, ok := p.severityByType[normalizeTodoType(todoType)]
	if !ok {
		return SeverityNotice
	}
	return severity
}

// IsIgnored reports whether a TODO type is excluded from detection and reporting.
func (p Policy) IsIgnored(todoType string) bool {
	return p.ignoredTypes[normalizeTodoType(todoType)]
}

// IsCIFailing reports whether a TODO of the given type should cause a
// non-zero exit in CI. Ignored types never fail CI. By default, only
// error-level types fail; warning-level and notice-level types do not.
func (p Policy) IsCIFailing(todoType string) bool {
	if p.IsIgnored(todoType) {
		return false
	}
	return p.ciFailingSeverities[p.SeverityFor(todoType)]
}

// CountCIFailing returns the number of TODOs whose type maps to a
// CI-failing severity.
func (p Policy) CountCIFailing(todos []types.TODO) int {
	n := 0
	for _, t := range todos {
		if p.IsCIFailing(t.Type) {
			n++
		}
	}
	return n
}

// defaultTypes is the built-in TODO marker set.
var defaultTypes = []string{"BUG", "FIXME", "HACK", "NOTE", "TODO", "XXX"}

// DefaultTypes returns a copy of the built-in TODO marker types.
func DefaultTypes() []string {
	result := make([]string, len(defaultTypes))
	copy(result, defaultTypes)
	return result
}

// Types returns all TODO marker types known to this policy, excluding
// ignored types. Built-in markers and custom types added via severity
// overrides are included unless they are in the ignored set.
// The result is sorted alphabetically and normalized to uppercase.
func (p Policy) Types() []string {
	typeSet := make(map[string]bool)
	for _, t := range defaultTypes {
		if !p.ignoredTypes[t] {
			typeSet[t] = true
		}
	}
	for t := range p.severityByType {
		normalized := normalizeTodoType(t)
		if !p.ignoredTypes[normalized] {
			typeSet[normalized] = true
		}
	}
	result := make([]string, 0, len(typeSet))
	for t := range typeSet {
		result = append(result, t)
	}
	sort.Strings(result)
	return result
}

// defaultPolicy is a cached shared Policy used by the package-level
// wrappers to avoid rebuilding maps on every call. DefaultPolicy() still
// returns a fresh copy for callers who need a configurable instance.
var defaultPolicy = DefaultPolicy()

// SeverityFor returns the default annotation severity for a TODO type.
// FIXME, HACK, XXX, BUG → warning. All others (TODO, NOTE, unknown) → notice.
func SeverityFor(todoType string) Severity {
	return defaultPolicy.SeverityFor(todoType)
}

// IsCIFailing reports whether a TODO of the given type should cause a
// non-zero exit in CI. By default, only error-level types fail;
// warning-level and notice-level types do not.
func IsCIFailing(todoType string) bool {
	return defaultPolicy.IsCIFailing(todoType)
}

// CountCIFailing returns the number of TODOs whose type maps to a
// CI-failing severity.
func CountCIFailing(todos []types.TODO) int {
	return defaultPolicy.CountCIFailing(todos)
}

func normalizeTodoType(todoType string) string {
	return strings.ToUpper(todoType)
}
