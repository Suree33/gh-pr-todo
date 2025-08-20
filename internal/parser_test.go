package internal

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/Suree33/gh-pr-todo/pkg/types"
)

func TestParseDiff(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []types.TODO
	}{
		{
			name: "Basic TODO comment with // style",
			input: `diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -1,3 +1,4 @@
 package main
 
+// TODO: implement this function
 func main() {`,
			expected: []types.TODO{
				{
					Filename: "test.go",
					Line:     3,
					Comment:  "// TODO: implement this function",
					Type:     "TODO",
				},
			},
		},
		{
			name: "Multiple comment formats (//、#、<!--、;、/*)",
			input: `diff --git a/multi.go b/multi.go
index 1234567..abcdefg 100644
--- a/multi.go
+++ b/multi.go
@@ -1,5 +1,10 @@
 package main

+// TODO: Go style comment
+# TODO: Shell style comment  
+<!-- TODO: HTML style comment -->
+; TODO: Assembly style comment
+/* TODO: C style comment
 func main() {`,
			expected: []types.TODO{
				{
					Filename: "multi.go",
					Line:     2,
					Comment:  "// TODO: Go style comment",
					Type:     "TODO",
				},
				{
					Filename: "multi.go",
					Line:     3,
					Comment:  "# TODO: Shell style comment",
					Type:     "TODO",
				},
				{
					Filename: "multi.go",
					Line:     4,
					Comment:  "<!-- TODO: HTML style comment -->",
					Type:     "TODO",
				},
				{
					Filename: "multi.go",
					Line:     5,
					Comment:  "; TODO: Assembly style comment",
					Type:     "TODO",
				},
				{
					Filename: "multi.go",
					Line:     6,
					Comment:  "/* TODO: C style comment",
					Type:     "TODO",
				},
			},
		},
		{
			name: "All TODO types (TODO、FIXME、HACK、NOTE、XXX、BUG)",
			input: `diff --git a/types.go b/types.go
index 1234567..abcdefg 100644
--- a/types.go
+++ b/types.go
@@ -1,6 +1,12 @@
 package main

+// TODO: implement feature
+// FIXME: fix this bug
+// HACK: temporary workaround
+// NOTE: important information
+// XXX: dangerous code
+// BUG: known issue
 func main() {`,
			expected: []types.TODO{
				{
					Filename: "types.go",
					Line:     2,
					Comment:  "// TODO: implement feature",
					Type:     "TODO",
				},
				{
					Filename: "types.go",
					Line:     3,
					Comment:  "// FIXME: fix this bug",
					Type:     "FIXME",
				},
				{
					Filename: "types.go",
					Line:     4,
					Comment:  "// HACK: temporary workaround",
					Type:     "HACK",
				},
				{
					Filename: "types.go",
					Line:     5,
					Comment:  "// NOTE: important information",
					Type:     "NOTE",
				},
				{
					Filename: "types.go",
					Line:     6,
					Comment:  "// XXX: dangerous code",
					Type:     "XXX",
				},
				{
					Filename: "types.go",
					Line:     7,
					Comment:  "// BUG: known issue",
					Type:     "BUG",
				},
			},
		},
		{
			name: "Case insensitive test",
			input: `diff --git a/case.go b/case.go
index 1234567..abcdefg 100644
--- a/case.go
+++ b/case.go
@@ -1,4 +1,8 @@
 package main

+// todo: lowercase
+// TODO: uppercase
+// Todo: mixed case
+// tOdO: weird case
 func main() {`,
			expected: []types.TODO{
				{
					Filename: "case.go",
					Line:     2,
					Comment:  "// todo: lowercase",
					Type:     "TODO",
				},
				{
					Filename: "case.go",
					Line:     3,
					Comment:  "// TODO: uppercase",
					Type:     "TODO",
				},
				{
					Filename: "case.go",
					Line:     4,
					Comment:  "// Todo: mixed case",
					Type:     "TODO",
				},
				{
					Filename: "case.go",
					Line:     5,
					Comment:  "// tOdO: weird case",
					Type:     "TODO",
				},
			},
		},
		{
			name: "TODO comments without colon",
			input: `diff --git a/nocolon.go b/nocolon.go
index 1234567..abcdefg 100644
--- a/nocolon.go
+++ b/nocolon.go
@@ -1,3 +1,5 @@
 package main

+// TODO implement this
+// FIXME repair the bug
 func main() {`,
			expected: []types.TODO{
				{
					Filename: "nocolon.go",
					Line:     2,
					Comment:  "// TODO implement this",
					Type:     "TODO",
				},
				{
					Filename: "nocolon.go",
					Line:     3,
					Comment:  "// FIXME repair the bug",
					Type:     "FIXME",
				},
			},
		},
		{
			name: "Multiple hunks (line number calculation test)",
			input: `diff --git a/multi_hunk.go b/multi_hunk.go
index 1234567..abcdefg 100644
--- a/multi_hunk.go
+++ b/multi_hunk.go
@@ -5,6 +5,7 @@ func first() {
 }
 
 func second() {
+	// TODO: first hunk
 }
 
 @@ -15,6 +16,7 @@ func third() {
 }
 
 func fourth() {
+	// FIXME: second hunk
 }`,
			expected: []types.TODO{
				{
					Filename: "multi_hunk.go",
					Line:     8,
					Comment:  "// TODO: first hunk",
					Type:     "TODO",
				},
				{
					Filename: "multi_hunk.go",
					Line:     19,
					Comment:  "// FIXME: second hunk",
					Type:     "FIXME",
				},
			},
		},
		{
			name: "Multiple files diff",
			input: `diff --git a/file1.go b/file1.go
index 1234567..abcdefg 100644
--- a/file1.go
+++ b/file1.go
@@ -1,3 +1,4 @@
 package main

+// TODO: file1 task
 func main() {}

diff --git a/file2.go b/file2.go
index 7890123..defghij 100644
--- a/file2.go
+++ b/file2.go
@@ -1,3 +1,4 @@
 package utils

+// FIXME: file2 issue
 func helper() {}`,
			expected: []types.TODO{
				{
					Filename: "file1.go",
					Line:     3,
					Comment:  "// TODO: file1 task",
					Type:     "TODO",
				},
				{
					Filename: "file2.go",
					Line:     3,
					Comment:  "// FIXME: file2 issue",
					Type:     "FIXME",
				},
			},
		},
		{
			name:     "Empty diff",
			input:    "",
			expected: nil,
		},
		{
			name: "Diff with no TODO comments",
			input: `diff --git a/no_todo.go b/no_todo.go
index 1234567..abcdefg 100644
--- a/no_todo.go
+++ b/no_todo.go
@@ -1,3 +1,4 @@
 package main

+// regular comment
 func main() {}`,
			expected: nil,
		},
		{
			name: "TODO comment in deleted lines (ignored)",
			input: `diff --git a/deleted.go b/deleted.go
index 1234567..abcdefg 100644
--- a/deleted.go
+++ b/deleted.go
@@ -1,4 +1,3 @@
 package main

-// TODO: this will be ignored
 func main() {}`,
			expected: nil,
		},
		{
			name: "TODO comment in unchanged lines (ignored)",
			input: `diff --git a/unchanged.go b/unchanged.go
index 1234567..abcdefg 100644
--- a/unchanged.go
+++ b/unchanged.go
@@ -1,4 +1,4 @@
 package main

 // TODO: this will be ignored
+var x = 1
 func main() {}`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDiff(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseDiff() = %+v, expected %+v", result, tt.expected)
			}
		})
	}
}

func TestTodoRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		matches  bool
		todoType string
	}{
		{"Go style comment", "// TODO: implement this", true, "TODO"},
		{"Python style comment", "# FIXME: fix this bug", true, "FIXME"},
		{"HTML comment", "<!-- HACK: temporary fix -->", true, "HACK"},
		{"Assembly comment", "; NOTE: important info", true, "NOTE"},
		{"C style comment", "/* XXX: dangerous code", true, "XXX"},
		{"Bug comment", "// BUG: known issue", true, "BUG"},
		{"Lowercase todo", "// todo: case insensitive", true, "todo"},
		{"Mixed case", "// Todo: mixed case", true, "Todo"},
		{"No colon", "// TODO implement feature", true, "TODO"},
		{"Multiple spaces", "//    TODO:     spaced out", true, "TODO"},
		{"Tab indented", "\t// TODO: indented with tab", true, "TODO"},
		{"Not a todo", "// regular comment", false, ""},
		{"Empty line", "", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := todoRegex.FindStringSubmatch(tt.input)
			if tt.matches {
				if len(matches) < 2 {
					t.Errorf("Expected regex to match %q, but it didn't", tt.input)
					return
				}
				if matches[2] != tt.todoType {
					t.Errorf("Expected todo type %q, got %q", tt.todoType, matches[2])
				}
			} else {
				if len(matches) > 0 {
					t.Errorf("Expected regex not to match %q, but it matched", tt.input)
				}
			}
		})
	}
}

func TestHunkRegex(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		matches   bool
		startLine int
	}{
		{"Basic hunk", "@@ -1,3 +1,4 @@", true, 1},
		{"Single line", "@@ -5 +5,2 @@", true, 5},
		{"Large line numbers", "@@ -100,50 +150,75 @@", true, 150},
		{"With context", "@@ -10,5 +15,8 @@ func test() {", true, 15},
		{"Invalid hunk", "not a hunk header", false, 0},
		{"Partial hunk", "@@ -1,3", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := hunkRegex.FindStringSubmatch(tt.input)
			if tt.matches {
				if len(matches) < 2 {
					t.Errorf("Expected regex to match %q, but it didn't", tt.input)
					return
				}
				startLine := 0
				if len(matches[1]) > 0 {
					var err error
					startLine, err = strconv.Atoi(matches[1])
					if err != nil {
						t.Errorf("Failed to parse start line %q: %v", matches[1], err)
						return
					}
				}
				if startLine != tt.startLine {
					t.Errorf("Expected start line %d, got %d", tt.startLine, startLine)
				}
			} else {
				if len(matches) > 0 {
					t.Errorf("Expected regex not to match %q, but it matched", tt.input)
				}
			}
		})
	}
}
