package todotype

import "strings"

// ParseSeverity parses a configured severity level.
//
// The input is trimmed and matched case-insensitively so CLI flags and YAML
// config files share the same normalization behavior.
func ParseSeverity(value string) (Severity, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "notice":
		return SeverityNotice, true
	case "warning":
		return SeverityWarning, true
	case "error":
		return SeverityError, true
	default:
		return "", false
	}
}

// NormalizeConfiguredType normalizes a configured TODO marker type.
//
// The input is trimmed and uppercased. Validation such as rejecting empty
// strings or CLI-specific separator characters is left to the caller so each
// input path can preserve its existing error behavior.
func NormalizeConfiguredType(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// NormalizeConfiguredTypes normalizes a configured TODO marker type list while
// preserving order and duplicates for caller-specific handling.
func NormalizeConfiguredTypes(values []string) []string {
	normalized := make([]string, len(values))
	for i, value := range values {
		normalized[i] = NormalizeConfiguredType(value)
	}
	return normalized
}
