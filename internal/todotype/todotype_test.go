package todotype

import (
	"reflect"
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
		{" FIXME ", SeverityNotice},
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
		{"FIXME", false},
		{"HACK", false},
		{"XXX", false},
		{"BUG", false},
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
			want: 0,
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
			want: 0,
		},
		{
			name: "lowercase mixed",
			todos: []types.TODO{
				{Type: "todo"},
				{Type: "fixme"},
				{Type: "note"},
				{Type: "bug"},
			},
			want: 0,
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

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	t.Run("DefaultPolicy().SeverityFor returns defaults", func(t *testing.T) {
		tests := []struct {
			todoType string
			want     Severity
		}{
			{"TODO", SeverityNotice},
			{"FIXME", SeverityWarning},
			{"HACK", SeverityWarning},
			{"XXX", SeverityWarning},
			{"BUG", SeverityWarning},
			{"NOTE", SeverityNotice},
			{"unknown", SeverityNotice},
		}
		for _, tt := range tests {
			if got := p.SeverityFor(tt.todoType); got != tt.want {
				t.Fatalf("SeverityFor(%q) = %q, want %q", tt.todoType, got, tt.want)
			}
		}
	})

	t.Run("DefaultPolicy().IsCIFailing matches error-only severity", func(t *testing.T) {
		tests := []struct {
			todoType string
			want     bool
		}{
			{"TODO", false},
			{"NOTE", false},
			{"FIXME", false},
			{"HACK", false},
			{"XXX", false},
			{"BUG", false},
			{"unknown", false},
		}
		for _, tt := range tests {
			if got := p.IsCIFailing(tt.todoType); got != tt.want {
				t.Fatalf("IsCIFailing(%q) = %v, want %v", tt.todoType, got, tt.want)
			}
		}
	})

	t.Run("DefaultPolicy().CountCIFailing computes correctly", func(t *testing.T) {
		todos := []types.TODO{
			{Type: "TODO"},
			{Type: "FIXME"},
			{Type: "NOTE"},
			{Type: "BUG"},
		}
		if got := p.CountCIFailing(todos); got != 0 {
			t.Fatalf("CountCIFailing() = %d, want 0", got)
		}
	})
}

func TestPolicyWithSeverity(t *testing.T) {
	t.Run("override single type", func(t *testing.T) {
		p := DefaultPolicy().WithSeverity("TODO", SeverityWarning)
		if got := p.SeverityFor("TODO"); got != SeverityWarning {
			t.Fatalf("SeverityFor(TODO) = %q, want %q", got, SeverityWarning)
		}
		// unchanged types preserved
		if got := p.SeverityFor("FIXME"); got != SeverityWarning {
			t.Fatalf("SeverityFor(FIXME) = %q, want %q", got, SeverityWarning)
		}
		// CI fail: warning does NOT fail CI by default (only error does)
		if p.IsCIFailing("TODO") {
			t.Fatal("IsCIFailing(TODO) should be false after override to warning")
		}
	})

	t.Run("override to error severity", func(t *testing.T) {
		p := DefaultPolicy().WithSeverity("TODO", SeverityError)
		if got := p.SeverityFor("TODO"); got != SeverityError {
			t.Fatalf("SeverityFor(TODO) = %q, want %q", got, SeverityError)
		}
		// CI fail: error severity should also fail
		if !p.IsCIFailing("TODO") {
			t.Fatal("IsCIFailing(TODO) should be true after override to error")
		}
	})

	t.Run("add custom TODO type", func(t *testing.T) {
		p := DefaultPolicy().WithSeverity("REVIEW", SeverityWarning)
		if got := p.SeverityFor("REVIEW"); got != SeverityWarning {
			t.Fatalf("SeverityFor(REVIEW) = %q, want %q", got, SeverityWarning)
		}
		if got := p.SeverityFor("review"); got != SeverityWarning {
			t.Fatalf("SeverityFor(review) = %q, want %q", got, SeverityWarning)
		}
		// Default types still work
		if got := p.SeverityFor("TODO"); got != SeverityNotice {
			t.Fatalf("SeverityFor(TODO) = %q, want %q", got, SeverityNotice)
		}
	})

	t.Run("WithSeverities bulk override", func(t *testing.T) {
		p := DefaultPolicy().WithSeverities(map[string]Severity{
			"TODO":   SeverityWarning,
			"REVIEW": SeverityError,
		})
		if got := p.SeverityFor("TODO"); got != SeverityWarning {
			t.Fatalf("SeverityFor(TODO) = %q, want %q", got, SeverityWarning)
		}
		if got := p.SeverityFor("REVIEW"); got != SeverityError {
			t.Fatalf("SeverityFor(REVIEW) = %q, want %q", got, SeverityError)
		}
		if got := p.SeverityFor("FIXME"); got != SeverityWarning {
			t.Fatalf("SeverityFor(FIXME) = %q, want %q", got, SeverityWarning)
		}
	})

	t.Run("immutability: original unchanged after override", func(t *testing.T) {
		orig := DefaultPolicy()
		_ = orig.WithSeverity("TODO", SeverityWarning)
		if got := orig.SeverityFor("TODO"); got != SeverityNotice {
			t.Fatalf("original policy SeverityFor(TODO) = %q, want %q (unchanged)", got, SeverityNotice)
		}
	})

	t.Run("override with lowercase type", func(t *testing.T) {
		p := DefaultPolicy().WithSeverity("todo", SeverityWarning)
		if got := p.SeverityFor("TODO"); got != SeverityWarning {
			t.Fatalf("SeverityFor(TODO) = %q, want %q after setting lowercased key", got, SeverityWarning)
		}
	})

	t.Run("normalization does not trim spaces", func(t *testing.T) {
		p := DefaultPolicy().WithSeverity(" review ", SeverityWarning)
		if got := p.SeverityFor("REVIEW"); got != SeverityNotice {
			t.Fatalf("SeverityFor(REVIEW) = %q, want %q without trimming", got, SeverityNotice)
		}
		if got := p.SeverityFor(" review "); got != SeverityWarning {
			t.Fatalf("SeverityFor(\" review \") = %q, want %q", got, SeverityWarning)
		}
	})
}

func TestPolicyCountCIFailingWithOverrides(t *testing.T) {
	t.Run("custom type with warning severity does not fail CI", func(t *testing.T) {
		p := DefaultPolicy().WithSeverity("OPTIMIZE", SeverityWarning)
		todos := []types.TODO{
			{Type: "OPTIMIZE"},
			{Type: "TODO"},
		}
		if got := p.CountCIFailing(todos); got != 0 {
			t.Fatalf("CountCIFailing() = %d, want 0", got)
		}
	})

	t.Run("custom type with notice severity does not fail CI", func(t *testing.T) {
		p := DefaultPolicy().WithSeverity("OPTIMIZE", SeverityNotice)
		todos := []types.TODO{
			{Type: "OPTIMIZE"},
		}
		if got := p.CountCIFailing(todos); got != 0 {
			t.Fatalf("CountCIFailing() = %d, want 0", got)
		}
	})

	t.Run("custom type with error severity fails CI", func(t *testing.T) {
		p := DefaultPolicy().WithSeverity("OPTIMIZE", SeverityError)
		todos := []types.TODO{
			{Type: "OPTIMIZE"},
		}
		if got := p.CountCIFailing(todos); got != 1 {
			t.Fatalf("CountCIFailing() = %d, want 1", got)
		}
	})
}

func TestPackageLevelFunctionsUseDefaultPolicy(t *testing.T) {
	// Ensures package-level functions delegate to DefaultPolicy
	if got := SeverityFor("FIXME"); got != DefaultPolicy().SeverityFor("FIXME") {
		t.Fatal("SeverityFor does not match DefaultPolicy")
	}
	if got := IsCIFailing("FIXME"); got != DefaultPolicy().IsCIFailing("FIXME") {
		t.Fatal("IsCIFailing does not match DefaultPolicy")
	}
}

func TestSeverityErrorConstant(t *testing.T) {
	if SeverityError != "error" {
		t.Fatalf("SeverityError = %q, want 'error'", SeverityError)
	}
}

func TestDefaultTypes(t *testing.T) {
	got := DefaultTypes()
	want := []string{"BUG", "FIXME", "HACK", "NOTE", "TODO", "XXX"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultTypes() = %v, want %v", got, want)
	}
	// Ensure returned slice is a copy (mutating it should not affect global)
	got[0] = "CHANGED"
	if DefaultTypes()[0] == "CHANGED" {
		t.Fatal("DefaultTypes() should return a fresh copy")
	}
}

func TestPolicyTypes(t *testing.T) {
	t.Run("default policy returns built-in types", func(t *testing.T) {
		p := DefaultPolicy()
		got := p.Types()
		want := []string{"BUG", "FIXME", "HACK", "NOTE", "TODO", "XXX"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Types() = %v, want %v", got, want)
		}
	})

	t.Run("custom severity type appears in Types", func(t *testing.T) {
		p := DefaultPolicy().WithSeverity("REVIEW", SeverityWarning)
		got := p.Types()
		want := []string{"BUG", "FIXME", "HACK", "NOTE", "REVIEW", "TODO", "XXX"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Types() = %v, want %v", got, want)
		}
	})

	t.Run("multiple custom types", func(t *testing.T) {
		p := DefaultPolicy().WithSeverities(map[string]Severity{
			"SECURITY": SeverityError,
			"PERF":     SeverityWarning,
		})
		got := p.Types()
		want := []string{"BUG", "FIXME", "HACK", "NOTE", "PERF", "SECURITY", "TODO", "XXX"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Types() = %v, want %v", got, want)
		}
	})

}
