package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	ghclient "github.com/Suree33/gh-pr-todo/internal/github"
	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

type stubFetcher struct {
	diff      string
	diffErr   error
	files     map[string][]byte
	filesErr  error
	gotRepo   string
	gotPR     string
	gotDiffFC string
}

func (s *stubFetcher) FetchDiff(repo, pr string) (string, error) {
	s.gotRepo, s.gotPR = repo, pr
	return s.diff, s.diffErr
}

func (s *stubFetcher) FetchChangedFileContents(repo, pr, diff string) (map[string][]byte, error) {
	s.gotDiffFC = diff
	return s.files, s.filesErr
}

// withFetcher swaps the package-level newFetcher for the duration of a test.
// Tests that use this helper MUST NOT call t.Parallel(): the swap is a global
// mutation and would race with concurrent subtests.
func withFetcher(t *testing.T, f ghclient.PRFetcher) {
	t.Helper()
	original := newFetcher
	newFetcher = func() ghclient.PRFetcher { return f }
	t.Cleanup(func() { newFetcher = original })
}

// captureColorOutput redirects color.Output and os.Stderr while fn runs and
// returns whatever was written to color.Output. Like withFetcher it mutates
// globals, so callers must not use t.Parallel().
func captureColorOutput(t *testing.T, fn func()) string {
	t.Helper()
	originalColorOutput := color.Output
	originalNoColor := color.NoColor
	color.NoColor = true
	var buf bytes.Buffer
	color.Output = &buf
	defer func() {
		color.Output = originalColorOutput
		color.NoColor = originalNoColor
	}()

	fn()
	return buf.String()
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = original }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("w.Close() error = %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("r.Close() error = %v", err)
	}
	return string(out)
}

const sampleDiff = `diff --git a/foo.go b/foo.go
index 0000000..1111111 100644
--- a/foo.go
+++ b/foo.go
@@ -1,1 +1,2 @@
 package foo
+// TODO: add bar
`

func TestRunMain(t *testing.T) {
	tests := []struct {
		name        string
		fetcher     *stubFetcher
		groupBy     types.GroupBy
		wantErr     string
		wantContain []string
	}{
		{
			name:    "fetch error returned",
			fetcher: &stubFetcher{diffErr: errors.New("diff failed")},
			wantErr: "diff failed",
			wantContain: []string{
				"Fetching PR diff...",
			},
		},
		{
			name: "no TODOs prints message",
			fetcher: &stubFetcher{
				diff:  "",
				files: map[string][]byte{},
			},
			wantContain: []string{
				"Fetching PR diff...",
				"No TODO comments found in the diff.",
			},
		},
		{
			name: "TODOs printed flat",
			fetcher: &stubFetcher{
				diff:  sampleDiff,
				files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
			},
			groupBy: types.GroupByNone,
			wantContain: []string{
				"Found 1 TODO comment(s)",
				"foo.go:2",
				"// TODO: add bar",
			},
		},
		{
			name: "TODOs grouped by file",
			fetcher: &stubFetcher{
				diff:  sampleDiff,
				files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
			},
			groupBy: types.GroupByFile,
			wantContain: []string{
				"Found 1 TODO comment(s)",
				"foo.go",
				"2: // TODO: add bar",
			},
		},
		{
			name: "TODOs grouped by type",
			fetcher: &stubFetcher{
				diff:  sampleDiff,
				files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
			},
			groupBy: types.GroupByType,
			wantContain: []string{
				"Found 1 TODO comment(s)",
				"[TODO]",
				"foo.go:2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFetcher(t, tt.fetcher)
			var gotErr error
			out := captureColorOutput(t, func() {
				_ = captureStderr(t, func() {
					gotErr = runMain("o/r", "1", tt.groupBy)
				})
			})

			if tt.wantErr != "" {
				if gotErr == nil || gotErr.Error() != tt.wantErr {
					t.Fatalf("runMain() error = %v, expected %q", gotErr, tt.wantErr)
				}
			} else if gotErr != nil {
				t.Fatalf("runMain() unexpected error = %v", gotErr)
			}
			for _, want := range tt.wantContain {
				if !strings.Contains(out, want) {
					t.Fatalf("runMain() output = %q, expected to contain %q", out, want)
				}
			}
			if tt.fetcher.gotRepo != "o/r" || tt.fetcher.gotPR != "1" {
				t.Fatalf("fetcher received repo=%q pr=%q, expected o/r and 1", tt.fetcher.gotRepo, tt.fetcher.gotPR)
			}
		})
	}
}

func TestRunCount(t *testing.T) {
	t.Run("fetch error returned", func(t *testing.T) {
		withFetcher(t, &stubFetcher{diffErr: errors.New("boom")})
		var err error
		_ = captureColorOutput(t, func() {
			_ = captureStderr(t, func() {
				err = runCount("", "")
			})
		})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("runCount() error = %v, expected boom", err)
		}
	})

	t.Run("prints count on success", func(t *testing.T) {
		withFetcher(t, &stubFetcher{
			diff:  sampleDiff,
			files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
		})
		var err error
		out := captureColorOutput(t, func() {
			_ = captureStderr(t, func() {
				err = runCount("o/r", "1")
			})
		})
		if err != nil {
			t.Fatalf("runCount() unexpected error = %v", err)
		}
		if strings.TrimSpace(out) != "1" {
			t.Fatalf("runCount() output = %q, expected %q", out, "1")
		}
	})
}

func TestRunNameOnly(t *testing.T) {
	t.Run("fetch error returned", func(t *testing.T) {
		withFetcher(t, &stubFetcher{diffErr: errors.New("boom")})
		var err error
		_ = captureColorOutput(t, func() {
			_ = captureStderr(t, func() {
				err = runNameOnly("", "")
			})
		})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("runNameOnly() error = %v, expected boom", err)
		}
	})

	t.Run("prints file names on success", func(t *testing.T) {
		withFetcher(t, &stubFetcher{
			diff:  sampleDiff,
			files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
		})
		var err error
		out := captureColorOutput(t, func() {
			_ = captureStderr(t, func() {
				err = runNameOnly("o/r", "1")
			})
		})
		if err != nil {
			t.Fatalf("runNameOnly() unexpected error = %v", err)
		}
		if strings.TrimSpace(out) != "foo.go" {
			t.Fatalf("runNameOnly() output = %q, expected %q", out, "foo.go")
		}
	})

	t.Run("no TODOs prints nothing", func(t *testing.T) {
		withFetcher(t, &stubFetcher{diff: "", files: map[string][]byte{}})
		var err error
		out := captureColorOutput(t, func() {
			_ = captureStderr(t, func() {
				err = runNameOnly("", "")
			})
		})
		if err != nil {
			t.Fatalf("runNameOnly() unexpected error = %v", err)
		}
		if out != "" {
			t.Fatalf("runNameOnly() output = %q, expected empty", out)
		}
	})
}

func TestPrintUsage(t *testing.T) {
	originalCommandLine := pflag.CommandLine
	pflag.CommandLine = pflag.NewFlagSet("gh pr-todo", pflag.ContinueOnError)
	t.Cleanup(func() { pflag.CommandLine = originalCommandLine })

	var (
		repo     string
		nameOnly bool
		isCount  bool
		isHelp   bool
		groupBy  = types.GroupByNone
	)
	pflag.StringVarP(&repo, "repo", "R", "", "Select another repository using the [HOST/]OWNER/REPO format")
	pflag.BoolVar(&nameOnly, "name-only", false, "Display only names of the files containing TODO comments")
	pflag.BoolVarP(&isCount, "count", "c", false, "Display only the number of TODO comments")
	pflag.BoolVarP(&isHelp, "help", "h", false, "Display help information")
	pflag.Var(&groupBy, "group-by", "Group TODO comments by: \"file\" or \"type\"")

	out := captureColorOutput(t, func() {
		printUsage()
	})

	wantContain := []string{
		"View TODO comments in the PR diff.",
		"USAGE",
		"gh pr-todo [<number> | <url> | <branch>] [flags]",
		"FLAGS",
		"--repo",
		"--name-only",
		"--count",
		"--help",
		"--group-by",
	}
	for _, want := range wantContain {
		if !strings.Contains(out, want) {
			t.Fatalf("printUsage() output = %q, expected to contain %q", out, want)
		}
	}
}
