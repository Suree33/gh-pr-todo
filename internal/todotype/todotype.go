package todotype

import (
	"strings"

	"github.com/Suree33/gh-pr-todo/pkg/types"
)

// IsWarning reports whether a TODO-style marker should be treated as a
// GitHub Actions warning annotation.
func IsWarning(todoType string) bool {
	switch strings.ToUpper(todoType) {
	case "FIXME", "HACK", "XXX", "BUG":
		return true
	default:
		return false
	}
}

func CountWarnings(todos []types.TODO) int {
	count := 0
	for _, todo := range todos {
		if IsWarning(todo.Type) {
			count++
		}
	}
	return count
}
