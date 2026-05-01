package types

import "testing"

func TestGroupBy_Set(t *testing.T) {
	tests := []struct {
		name    string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var g GroupBy
			err := g.Set(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Set(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && g != tt.want {
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
