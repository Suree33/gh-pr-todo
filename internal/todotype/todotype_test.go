package todotype

import (
	"testing"

	"github.com/Suree33/gh-pr-todo/pkg/types"
)

func TestSeverityFor(t *testing.T) {
	tests := []struct {
		todoType string
		want     Severity
	}{
		{"TODO", SeverityNotice},
		{"todo", SeverityNotice},
		{"NOTE", SeverityNotice},
		{"note", SeverityNotice},
		{"FIXME", SeverityWarning},
		{"fixme", SeverityWarning},
		{"HACK", SeverityWarning},
		{"hack", SeverityWarning},
		{"XXX", SeverityWarning},
		{"xxx", SeverityWarning},
		{"BUG", SeverityWarning},
		{"bug", SeverityWarning},
		{"unknown", SeverityNotice},
		{"", SeverityNotice},
	}
	for _, tt := range tests {
		t.Run(tt.todoType, func(t *testing.T) {
			if got := SeverityFor(tt.todoType); got != tt.want {
				t.Fatalf("SeverityFor(%q) = %q, want %q", tt.todoType, got, tt.want)
			}
		})
	}
}

func TestIsCIFailing(t *testing.T) {
	tests := []struct {
		todoType string
		want     bool
	}{
		{"TODO", false},
		{"todo", false},
		{"NOTE", false},
		{"FIXME", true},
		{"HACK", true},
		{"XXX", true},
		{"BUG", true},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.todoType, func(t *testing.T) {
			if got := IsCIFailing(tt.todoType); got != tt.want {
				t.Fatalf("IsCIFailing(%q) = %v, want %v", tt.todoType, got, tt.want)
			}
		})
	}
}

func TestCountCIFailing(t *testing.T) {
	tests := []struct {
		name  string
		todos []types.TODO
		want  int
	}{
		{name: "nil slice", todos: nil, want: 0},
		{name: "empty slice", todos: []types.TODO{}, want: 0},
		{
			name: "notice only",
			todos: []types.TODO{
				{Type: "TODO"},
				{Type: "NOTE"},
				{Type: "unknown"},
			},
			want: 0,
		},
		{
			name: "warning only",
			todos: []types.TODO{
				{Type: "FIXME"},
				{Type: "HACK"},
				{Type: "XXX"},
				{Type: "BUG"},
			},
			want: 4,
		},
		{
			name: "mixed notice and warning",
			todos: []types.TODO{
				{Type: "TODO"},
				{Type: "NOTE"},
				{Type: "FIXME"},
				{Type: "HACK"},
				{Type: "XXX"},
				{Type: "BUG"},
			},
			want: 4,
		},
		{
			name: "lowercase mixed",
			todos: []types.TODO{
				{Type: "todo"},
				{Type: "fixme"},
				{Type: "note"},
				{Type: "bug"},
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountCIFailing(tt.todos); got != tt.want {
				t.Fatalf("CountCIFailing() = %d, want %d", got, tt.want)
			}
		})
	}
}
