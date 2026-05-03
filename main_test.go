package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

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

// captureColorOutput redirects color.Output while fn runs and returns whatever
// was written there. It mutates the global color.Output and color.NoColor, so
// callers must not use t.Parallel().
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

// captureAll captures color.Output, os.Stdout, and os.Stderr while fn runs.
// It mutates these globals, so callers must not use t.Parallel().
func captureAll(t *testing.T, fn func()) (colorOut, stdout, stderr string) {
	t.Helper()
	stdout = captureStdout(t, func() {
		stderr = captureStderr(t, func() {
			colorOut = captureColorOutput(t, fn)
		})
	})
	return colorOut, stdout, stderr
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return capturePipe(t, &os.Stderr, fn)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return capturePipe(t, &os.Stdout, fn)
}

func capturePipe(t *testing.T, target **os.File, fn func()) string {
	t.Helper()
	original := *target
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	*target = w
	defer func() { *target = original }()

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
		wantStderr  string
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
		{
			name: "FetchChangedFileContents error logs warning and continues",
			fetcher: &stubFetcher{
				diff:     sampleDiff,
				files:    nil,
				filesErr: errors.New("contents failed"),
			},
			wantContain: []string{
				"Found 1 TODO comment(s)",
			},
			wantStderr: "Warning: could not fetch changed file contents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotErr error
			out, stdout, gotStderr := captureAll(t, func() {
				_, gotErr = runMain(tt.fetcher, "o/r", "1", tt.groupBy, false)
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
			if tt.wantStderr == "" && gotStderr != "" {
				t.Fatalf("runMain() unexpected stderr = %q", gotStderr)
			}
			if tt.wantStderr != "" && !strings.Contains(gotStderr, tt.wantStderr) {
				t.Fatalf("runMain() stderr = %q, expected to contain %q", gotStderr, tt.wantStderr)
			}
			if stdout != "" {
				t.Fatalf("runMain() unexpected os.Stdout write = %q", stdout)
			}
			if tt.fetcher.gotRepo != "o/r" || tt.fetcher.gotPR != "1" {
				t.Fatalf("fetcher received repo=%q pr=%q, expected o/r and 1", tt.fetcher.gotRepo, tt.fetcher.gotPR)
			}
		})
	}
}

func assertSilentChannels(t *testing.T, label, stdout, stderr string) {
	t.Helper()
	if stdout != "" {
		t.Fatalf("%s unexpected os.Stdout write = %q", label, stdout)
	}
	if stderr != "" {
		t.Fatalf("%s unexpected os.Stderr write = %q", label, stderr)
	}
}

func TestRunCount(t *testing.T) {
	t.Run("fetch error returned", func(t *testing.T) {
		fetcher := &stubFetcher{diffErr: errors.New("boom")}
		var err error
		out, stdout, stderr := captureAll(t, func() {
			_, err = runCount(fetcher, "", "")
		})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("runCount() error = %v, expected boom", err)
		}
		assertSilentChannels(t, "runCount()", stdout, stderr)
		if out != "" {
			t.Fatalf("runCount() unexpected color.Output = %q", out)
		}
	})

	t.Run("prints count on success", func(t *testing.T) {
		fetcher := &stubFetcher{
			diff:  sampleDiff,
			files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
		}
		var err error
		out, stdout, stderr := captureAll(t, func() {
			_, err = runCount(fetcher, "o/r", "1")
		})
		if err != nil {
			t.Fatalf("runCount() unexpected error = %v", err)
		}
		assertSilentChannels(t, "runCount()", stdout, stderr)
		if strings.TrimSpace(out) != "1" {
			t.Fatalf("runCount() output = %q, expected %q", out, "1")
		}
		if fetcher.gotRepo != "o/r" || fetcher.gotPR != "1" {
			t.Fatalf("fetcher received repo=%q pr=%q, expected o/r and 1", fetcher.gotRepo, fetcher.gotPR)
		}
	})
}

func TestRunNameOnly(t *testing.T) {
	t.Run("fetch error returned", func(t *testing.T) {
		fetcher := &stubFetcher{diffErr: errors.New("boom")}
		var err error
		out, stdout, stderr := captureAll(t, func() {
			_, err = runNameOnly(fetcher, "", "")
		})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("runNameOnly() error = %v, expected boom", err)
		}
		assertSilentChannels(t, "runNameOnly()", stdout, stderr)
		if out != "" {
			t.Fatalf("runNameOnly() unexpected color.Output = %q", out)
		}
	})

	t.Run("prints file names on success", func(t *testing.T) {
		fetcher := &stubFetcher{
			diff:  sampleDiff,
			files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
		}
		var err error
		out, stdout, stderr := captureAll(t, func() {
			_, err = runNameOnly(fetcher, "o/r", "1")
		})
		if err != nil {
			t.Fatalf("runNameOnly() unexpected error = %v", err)
		}
		assertSilentChannels(t, "runNameOnly()", stdout, stderr)
		if strings.TrimSpace(out) != "foo.go" {
			t.Fatalf("runNameOnly() output = %q, expected %q", out, "foo.go")
		}
		if fetcher.gotRepo != "o/r" || fetcher.gotPR != "1" {
			t.Fatalf("fetcher received repo=%q pr=%q, expected o/r and 1", fetcher.gotRepo, fetcher.gotPR)
		}
	})

	t.Run("no TODOs prints nothing", func(t *testing.T) {
		fetcher := &stubFetcher{diff: "", files: map[string][]byte{}}
		var err error
		out, stdout, stderr := captureAll(t, func() {
			_, err = runNameOnly(fetcher, "", "")
		})
		if err != nil {
			t.Fatalf("runNameOnly() unexpected error = %v", err)
		}
		assertSilentChannels(t, "runNameOnly()", stdout, stderr)
		if out != "" {
			t.Fatalf("runNameOnly() output = %q, expected empty", out)
		}
	})
}

func TestIsCI(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  bool
	}{
		{name: "unset returns false", set: false, want: false},
		{name: "empty returns false", value: "", set: true, want: false},
		{name: "CI=1 returns true", value: "1", set: true, want: true},
		{name: "CI=true returns true", value: "true", set: true, want: true},
		{name: "CI=True (mixed case) returns true", value: "True", set: true, want: true},
		{name: "CI=TRUE (uppercase) returns true", value: "TRUE", set: true, want: true},
		{name: "CI=t returns true", value: "t", set: true, want: true},
		{name: "CI=T returns true", value: "T", set: true, want: true},
		{name: " CI= surrounded by whitespace returns true", value: "  true  ", set: true, want: true},
		{name: "CI=false returns false", value: "false", set: true, want: false},
		{name: "CI=0 returns false", value: "0", set: true, want: false},
		{name: "CI=yes returns false", value: "yes", set: true, want: false},
		{name: "CI=on returns false", value: "on", set: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_ACTIONS", "")
			if tt.set {
				t.Setenv("CI", tt.value)
			} else {
				if err := os.Unsetenv("CI"); err != nil {
					t.Fatalf("os.Unsetenv() error = %v", err)
				}
				t.Cleanup(func() { _ = os.Unsetenv("CI") })
			}
			if got := isCI(); got != tt.want {
				t.Fatalf("isCI() = %v, expected %v (CI set=%v value=%q)", got, tt.want, tt.set, tt.value)
			}
		})
	}
}

func TestIsCIPromotedByGitHubActions(t *testing.T) {
	tests := []struct {
		name        string
		ci          string
		ciSet       bool
		githubValue string
		want        bool
	}{
		{name: "GITHUB_ACTIONS=true forces CI true even when CI unset", ciSet: false, githubValue: "true", want: true},
		{name: "GITHUB_ACTIONS=true forces CI true even when CI=false", ci: "false", ciSet: true, githubValue: "true", want: true},
		{name: "GITHUB_ACTIONS=1 forces CI true even when CI=0", ci: "0", ciSet: true, githubValue: "1", want: true},
		{name: "GITHUB_ACTIONS=false does not force CI true", ci: "false", ciSet: true, githubValue: "false", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ciSet {
				t.Setenv("CI", tt.ci)
			} else {
				if err := os.Unsetenv("CI"); err != nil {
					t.Fatalf("os.Unsetenv() error = %v", err)
				}
				t.Cleanup(func() { _ = os.Unsetenv("CI") })
			}
			t.Setenv("GITHUB_ACTIONS", tt.githubValue)
			if got := isCI(); got != tt.want {
				t.Fatalf("isCI() = %v, expected %v (CI set=%v value=%q, GITHUB_ACTIONS=%q)", got, tt.want, tt.ciSet, tt.ci, tt.githubValue)
			}
		})
	}
}

func TestIsGitHubActions(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  bool
	}{
		{name: "unset returns false", set: false, want: false},
		{name: "empty returns false", value: "", set: true, want: false},
		{name: "GITHUB_ACTIONS=true returns true", value: "true", set: true, want: true},
		{name: "GITHUB_ACTIONS=1 returns true", value: "1", set: true, want: true},
		{name: "GITHUB_ACTIONS=false returns false", value: "false", set: true, want: false},
		{name: "GITHUB_ACTIONS=0 returns false", value: "0", set: true, want: false},
		{name: "GITHUB_ACTIONS surrounded by whitespace returns true", value: "  true  ", set: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv("GITHUB_ACTIONS", tt.value)
			} else {
				if err := os.Unsetenv("GITHUB_ACTIONS"); err != nil {
					t.Fatalf("os.Unsetenv() error = %v", err)
				}
				t.Cleanup(func() { _ = os.Unsetenv("GITHUB_ACTIONS") })
			}
			if got := isGitHubActions(); got != tt.want {
				t.Fatalf("isGitHubActions() = %v, expected %v (GITHUB_ACTIONS set=%v value=%q)", got, tt.want, tt.set, tt.value)
			}
		})
	}
}

func TestRunFunctionsEmitWorkflowCommands(t *testing.T) {
	fetcher := &stubFetcher{
		diff:  sampleDiff,
		files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
	}
	wantLine := "::notice file=foo.go,line=2,title=TODO::// TODO: add bar"

	t.Run("runMain emits when gha=true", func(t *testing.T) {
		out, _, _ := captureAll(t, func() {
			_, _ = runMain(fetcher, "o/r", "1", types.GroupByNone, true)
		})
		if !strings.Contains(out, wantLine) {
			t.Fatalf("runMain(gha=true) output = %q, expected to contain %q", out, wantLine)
		}
	})

	t.Run("runMain does not emit when gha=false", func(t *testing.T) {
		out, _, _ := captureAll(t, func() {
			_, _ = runMain(fetcher, "o/r", "1", types.GroupByNone, false)
		})
		if strings.Contains(out, "::notice ") || strings.Contains(out, "::warning ") {
			t.Fatalf("runMain(gha=false) unexpectedly emitted workflow command: %q", out)
		}
	})

	t.Run("runCount stdout stays plain", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		out, _, _ := captureAll(t, func() {
			_, _ = runCount(fetcher, "o/r", "1")
		})
		if strings.Contains(out, "::notice") || strings.Contains(out, "::warning") {
			t.Fatalf("runCount must not emit workflow commands; got %q", out)
		}
	})

	t.Run("runNameOnly stdout stays plain", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		out, _, _ := captureAll(t, func() {
			_, _ = runNameOnly(fetcher, "o/r", "1")
		})
		if strings.Contains(out, "::notice") || strings.Contains(out, "::warning") {
			t.Fatalf("runNameOnly must not emit workflow commands; got %q", out)
		}
	})
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		count    int
		ci       bool
		noCIFail bool
		want     int
	}{
		{name: "error returns 1", err: errors.New("boom"), want: 1},
		{name: "error in CI still 1", err: errors.New("boom"), ci: true, count: 5, want: 1},
		{name: "no error no TODOs returns 0", want: 0},
		{name: "no error TODOs not in CI returns 0", count: 3, want: 0},
		{name: "no error TODOs in CI returns 1", count: 3, ci: true, want: 1},
		{name: "no error TODOs in CI with no-ci-fail returns 0", count: 3, ci: true, noCIFail: true, want: 0},
		{name: "no error zero TODOs in CI returns 0", count: 0, ci: true, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCode(tt.err, tt.count, tt.ci, tt.noCIFail); got != tt.want {
				t.Fatalf("exitCode(err=%v, count=%d, ci=%v, noCIFail=%v) = %d, expected %d",
					tt.err, tt.count, tt.ci, tt.noCIFail, got, tt.want)
			}
		})
	}
}

func TestRunFunctionsReturnTODOCount(t *testing.T) {
	twoTODOsDiff := `diff --git a/foo.go b/foo.go
index 0000000..1111111 100644
--- a/foo.go
+++ b/foo.go
@@ -1,1 +1,3 @@
 package foo
+// TODO: add bar
+// FIXME: fix baz
`
	twoTODOsFiles := map[string][]byte{
		"foo.go": []byte("package foo\n// TODO: add bar\n// FIXME: fix baz\n"),
	}

	noteOnlyDiff := `diff --git a/foo.go b/foo.go
index 0000000..1111111 100644
--- a/foo.go
+++ b/foo.go
@@ -1,1 +1,2 @@
 package foo
+// NOTE: just a note
`
	noteOnlyFiles := map[string][]byte{
		"foo.go": []byte("package foo\n// NOTE: just a note\n"),
	}

	tests := []struct {
		name      string
		fetcher   *stubFetcher
		wantCount int
	}{
		{
			name:      "no TODOs returns 0",
			fetcher:   &stubFetcher{diff: "", files: map[string][]byte{}},
			wantCount: 0,
		},
		{
			name: "notice-only TODO does not fail CI",
			fetcher: &stubFetcher{
				diff:  sampleDiff,
				files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
			},
			wantCount: 0,
		},
		{
			name:      "NOTE-only does not fail CI",
			fetcher:   &stubFetcher{diff: noteOnlyDiff, files: noteOnlyFiles},
			wantCount: 0,
		},
		{
			name:      "FIXME among TODOs counts as ci-failing",
			fetcher:   &stubFetcher{diff: twoTODOsDiff, files: twoTODOsFiles},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run("runMain/"+tt.name, func(t *testing.T) {
			var gotCount int
			var gotErr error
			_, _, _ = captureAll(t, func() {
				gotCount, gotErr = runMain(tt.fetcher, "o/r", "1", types.GroupByNone, false)
			})
			if gotErr != nil {
				t.Fatalf("runMain() unexpected error = %v", gotErr)
			}
			if gotCount != tt.wantCount {
				t.Fatalf("runMain() count = %d, expected %d", gotCount, tt.wantCount)
			}
		})

		t.Run("runCount/"+tt.name, func(t *testing.T) {
			var gotCount int
			var gotErr error
			_, _, _ = captureAll(t, func() {
				gotCount, gotErr = runCount(tt.fetcher, "o/r", "1")
			})
			if gotErr != nil {
				t.Fatalf("runCount() unexpected error = %v", gotErr)
			}
			if gotCount != tt.wantCount {
				t.Fatalf("runCount() count = %d, expected %d", gotCount, tt.wantCount)
			}
		})

		t.Run("runNameOnly/"+tt.name, func(t *testing.T) {
			var gotCount int
			var gotErr error
			_, _, _ = captureAll(t, func() {
				gotCount, gotErr = runNameOnly(tt.fetcher, "o/r", "1")
			})
			if gotErr != nil {
				t.Fatalf("runNameOnly() unexpected error = %v", gotErr)
			}
			if gotCount != tt.wantCount {
				t.Fatalf("runNameOnly() count = %d, expected %d", gotCount, tt.wantCount)
			}
		})
	}
}

func TestRunFunctionsReturnZeroCountOnError(t *testing.T) {
	t.Run("runMain", func(t *testing.T) {
		fetcher := &stubFetcher{diffErr: errors.New("boom")}
		var gotCount int
		var gotErr error
		_, _, _ = captureAll(t, func() {
			gotCount, gotErr = runMain(fetcher, "", "", types.GroupByNone, false)
		})
		if gotErr == nil {
			t.Fatalf("runMain() expected error, got nil")
		}
		if gotCount != 0 {
			t.Fatalf("runMain() count = %d, expected 0", gotCount)
		}
	})

	t.Run("runCount", func(t *testing.T) {
		fetcher := &stubFetcher{diffErr: errors.New("boom")}
		var gotCount int
		var gotErr error
		_, _, _ = captureAll(t, func() {
			gotCount, gotErr = runCount(fetcher, "", "")
		})
		if gotErr == nil {
			t.Fatalf("runCount() expected error, got nil")
		}
		if gotCount != 0 {
			t.Fatalf("runCount() count = %d, expected 0", gotCount)
		}
	})

	t.Run("runNameOnly", func(t *testing.T) {
		fetcher := &stubFetcher{diffErr: errors.New("boom")}
		var gotCount int
		var gotErr error
		_, _, _ = captureAll(t, func() {
			gotCount, gotErr = runNameOnly(fetcher, "", "")
		})
		if gotErr == nil {
			t.Fatalf("runNameOnly() expected error, got nil")
		}
		if gotCount != 0 {
			t.Fatalf("runNameOnly() count = %d, expected 0", gotCount)
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
		noCIFail bool
		groupBy  = types.GroupByNone
	)
	pflag.StringVarP(&repo, "repo", "R", "", "Select another repository using the [HOST/]OWNER/REPO format")
	pflag.BoolVar(&nameOnly, "name-only", false, "Display only names of the files containing TODO comments")
	pflag.BoolVarP(&isCount, "count", "c", false, "Display only the number of TODO comments")
	pflag.BoolVarP(&isHelp, "help", "h", false, "Display help information")
	pflag.BoolVar(&noCIFail, "no-ci-fail", false, "Disable non-zero exit when TODOs are found in CI")
	pflag.Var(&groupBy, "group-by", "Group TODO comments by: \"file\" or \"type\"")

	var out string
	stdout := captureStdout(t, func() {
		out = captureColorOutput(t, func() {
			printUsage()
		})
	})

	if stdout != "" {
		t.Fatalf("printUsage() leaked to os.Stdout: %q", stdout)
	}
	if !strings.HasSuffix(out, "\n\n") {
		t.Fatalf("printUsage() output should end with a blank line, got tail %q", out[max(0, len(out)-4):])
	}

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
		"--no-ci-fail",
		"ENVIRONMENT",
		"CI",
		"GITHUB_ACTIONS",
	}
	for _, want := range wantContain {
		if !strings.Contains(out, want) {
			t.Fatalf("printUsage() output = %q, expected to contain %q", out, want)
		}
	}
}
