package types

import (
	"strings"
	"testing"
)

func TestGroupBy_Set(t *testing.T) {
	tests := []struct {
		name    string
		initial GroupBy
		input   string
		want    GroupBy
		wantErr bool
	}{
		{name: "file lowercase", input: "file", want: GroupByFile},
		{name: "type lowercase", input: "type", want: GroupByType},
		{name: "file mixed case", input: "FILE", want: GroupByFile},
		{name: "type mixed case", input: "Type", want: GroupByType},
		{name: "invalid", input: "bogus", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "invalid does not mutate existing value", initial: GroupByFile, input: "bogus", want: GroupByFile, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := tt.initial
			err := g.Set(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Set(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				msg := err.Error()
				for _, want := range []string{tt.input, "--group-by", `"file"`, `"type"`} {
					if !strings.Contains(msg, want) {
						t.Errorf("Set(%q) error %q does not contain %q", tt.input, msg, want)
					}
				}
				if g != tt.want {
					t.Errorf("Set(%q) mutated value to %q, want %q", tt.input, g, tt.want)
				}
				return
			}
			if g != tt.want {
				t.Errorf("Set(%q) = %q, want %q", tt.input, g, tt.want)
			}
		})
	}
}

func TestGroupBy_String(t *testing.T) {
	tests := []struct {
		name string
		g    GroupBy
		want string
	}{
		{name: "none", g: GroupByNone, want: ""},
		{name: "file", g: GroupByFile, want: "file"},
		{name: "type", g: GroupByType, want: "type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.g.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGroupBy_Type(t *testing.T) {
	var g GroupBy
	if got := g.Type(); got != "group-by" {
		t.Errorf("Type() = %q, want %q", got, "group-by")
	}
}
