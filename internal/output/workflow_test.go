package output

import (
	"testing"

	"github.com/Suree33/gh-pr-todo/pkg/types"
)

func TestPrintWorkflowCommands(t *testing.T) {
	todos := []types.TODO{
		{Filename: "a.go", Line: 5, Comment: "// TODO: a", Type: "TODO"},
		{Filename: "b.go", Line: 20, Comment: "// FIXME: b", Type: "FIXME"},
		{Filename: "c.go", Line: 7, Comment: "// HACK: c", Type: "HACK"},
		{Filename: "d.go", Line: 9, Comment: "// XXX: d", Type: "XXX"},
		{Filename: "e.go", Line: 11, Comment: "// BUG: e", Type: "BUG"},
		{Filename: "f.go", Line: 1, Comment: "// NOTE: f", Type: "NOTE"},
	}

	want := "::notice file=a.go,line=5,title=TODO::// TODO: a\n" +
		"::warning file=b.go,line=20,title=FIXME::// FIXME: b\n" +
		"::warning file=c.go,line=7,title=HACK::// HACK: c\n" +
		"::warning file=d.go,line=9,title=XXX::// XXX: d\n" +
		"::warning file=e.go,line=11,title=BUG::// BUG: e\n" +
		"::notice file=f.go,line=1,title=NOTE::// NOTE: f\n"

	got := captureOutput(t, func() {
		PrintWorkflowCommands(todos)
	})
	if got != want {
		t.Fatalf("PrintWorkflowCommands() output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestPrintWorkflowCommandsEmpty(t *testing.T) {
	got := captureOutput(t, func() {
		PrintWorkflowCommands(nil)
	})
	if got != "" {
		t.Fatalf("PrintWorkflowCommands(nil) = %q, want empty", got)
	}
}

func TestEscapeWorkflowMessage(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"plain", "plain"},
		{"100% sure", "100%25 sure"},
		{"a\nb", "a%0Ab"},
		{"a\rb", "a%0Db"},
		{"a%b\nc\rd", "a%25b%0Ac%0Dd"},
		{"a:b,c", "a:b,c"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := escapeWorkflowMessage(tt.in); got != tt.want {
				t.Fatalf("escapeWorkflowMessage(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEscapeWorkflowProperty(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"plain", "plain"},
		{"a:b", "a%3Ab"},
		{"a,b", "a%2Cb"},
		{"a%b\nc:d,e\rf", "a%25b%0Ac%3Ad%2Ce%0Df"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := escapeWorkflowProperty(tt.in); got != tt.want {
				t.Fatalf("escapeWorkflowProperty(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWorkflowCommandFor(t *testing.T) {
	tests := []struct {
		todoType string
		want     string
	}{
		{"TODO", "notice"},
		{"todo", "notice"},
		{"NOTE", "notice"},
		{"FIXME", "warning"},
		{"fixme", "warning"},
		{"HACK", "warning"},
		{"XXX", "warning"},
		{"BUG", "warning"},
		{"unknown", "notice"},
	}
	for _, tt := range tests {
		t.Run(tt.todoType, func(t *testing.T) {
			if got := workflowCommandFor(tt.todoType); got != tt.want {
				t.Fatalf("workflowCommandFor(%q) = %q, want %q", tt.todoType, got, tt.want)
			}
		})
	}
}
