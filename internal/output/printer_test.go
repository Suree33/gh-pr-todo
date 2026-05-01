package output

import (
	"bytes"
	"testing"

	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/fatih/color"
)

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	prevOutput := color.Output
	prevNoColor := color.NoColor
	buf := &bytes.Buffer{}
	color.Output = buf
	color.NoColor = true
	t.Cleanup(func() {
		color.Output = prevOutput
		color.NoColor = prevNoColor
	})
	fn()
	return buf.String()
}

func TestPrintTODOs(t *testing.T) {
	todos := []types.TODO{
		{Filename: "a.go", Line: 5, Comment: "// TODO: a", Type: "TODO"},
		{Filename: "a.go", Line: 100, Comment: "// HACK: long line", Type: "HACK"},
		{Filename: "b.go", Line: 20, Comment: "// FIXME: b", Type: "FIXME"},
	}

	tests := []struct {
		name    string
		groupBy types.GroupBy
		want    string
	}{
		{
			name:    "GroupByNone",
			groupBy: types.GroupByNone,
			want: "* a.go:5\n  // TODO: a\n\n" +
				"* a.go:100\n  // HACK: long line\n\n" +
				"* b.go:20\n  // FIXME: b\n\n",
		},
		{
			name:    "GroupByFile",
			groupBy: types.GroupByFile,
			want: "* a.go\n" +
				"    5: // TODO: a\n" +
				"  100: // HACK: long line\n" +
				"\n" +
				"* b.go\n" +
				"   20: // FIXME: b\n" +
				"\n",
		},
		{
			name:    "GroupByType",
			groupBy: types.GroupByType,
			want: "[FIXME]\n" +
				"* b.go:20\n  // FIXME: b\n\n" +
				"[HACK]\n" +
				"* a.go:100\n  // HACK: long line\n\n" +
				"[TODO]\n" +
				"* a.go:5\n  // TODO: a\n\n",
		},
		{
			name:    "unknown groupBy is a no-op",
			groupBy: types.GroupBy("bogus"),
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captureOutput(t, func() { PrintTODOs(todos, tt.groupBy) })
			if got != tt.want {
				t.Errorf("output mismatch\n--- want ---\n%s\n--- got ---\n%s", tt.want, got)
			}
		})
	}
}

func TestPrintFileNames(t *testing.T) {
	tests := []struct {
		name  string
		todos []types.TODO
		want  string
	}{
		{
			name:  "empty",
			todos: nil,
			want:  "",
		},
		{
			name: "deduplicated and sorted",
			todos: []types.TODO{
				{Filename: "b.go", Line: 1},
				{Filename: "a.go", Line: 2},
				{Filename: "b.go", Line: 3},
			},
			want: "a.go\nb.go\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captureOutput(t, func() { PrintFileNames(tt.todos) })
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestPrintCount(t *testing.T) {
	tests := []struct {
		name  string
		todos []types.TODO
		want  string
	}{
		{"zero", nil, "0\n"},
		{"some", []types.TODO{{}, {}, {}}, "3\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captureOutput(t, func() { PrintCount(tt.todos) })
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
