package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Suree33/gh-pr-todo/internal/config"
	"github.com/Suree33/gh-pr-todo/internal/todotype"
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

// noteOnlyDiff is a diff with a NOTE comment (no warning-level tokens).
const noteOnlyDiff = `diff --git a/note.go b/note.go
index 0000000..1111111 100644
--- a/note.go
+++ b/note.go
@@ -1,1 +1,2 @@
 package note
+// NOTE: test note
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
				"No TODO-style comments found in the diff.",
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
				"Found 1 TODO-style comment(s)",
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
				"Found 1 TODO-style comment(s)",
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
				"Found 1 TODO-style comment(s)",
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
				"Found 1 TODO-style comment(s)",
			},
			wantStderr: "Warning: could not fetch changed file contents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotErr error
			out, stdout, gotStderr := captureAll(t, func() {
				_, gotErr = runMain(tt.fetcher, "o/r", "1", tt.groupBy, false, todotype.DefaultPolicy())
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
			_, err = runCount(fetcher, "", "", todotype.DefaultPolicy())
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
			_, err = runCount(fetcher, "o/r", "1", todotype.DefaultPolicy())
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
			_, err = runNameOnly(fetcher, "", "", todotype.DefaultPolicy())
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
			_, err = runNameOnly(fetcher, "o/r", "1", todotype.DefaultPolicy())
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
			_, err = runNameOnly(fetcher, "", "", todotype.DefaultPolicy())
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
			_, _ = runMain(fetcher, "o/r", "1", types.GroupByNone, true, todotype.DefaultPolicy())
		})
		if !strings.Contains(out, wantLine) {
			t.Fatalf("runMain(gha=true) output = %q, expected to contain %q", out, wantLine)
		}
	})

	t.Run("runMain does not emit when gha=false", func(t *testing.T) {
		out, _, _ := captureAll(t, func() {
			_, _ = runMain(fetcher, "o/r", "1", types.GroupByNone, false, todotype.DefaultPolicy())
		})
		if strings.Contains(out, "::notice ") || strings.Contains(out, "::warning ") || strings.Contains(out, "::error ") {
			t.Fatalf("runMain(gha=false) unexpectedly emitted workflow command: %q", out)
		}
	})

	t.Run("runCount stdout stays plain", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		out, _, _ := captureAll(t, func() {
			_, _ = runCount(fetcher, "o/r", "1", todotype.DefaultPolicy())
		})
		if strings.Contains(out, "::notice") || strings.Contains(out, "::warning") || strings.Contains(out, "::error") {
			t.Fatalf("runCount must not emit workflow commands; got %q", out)
		}
	})

	t.Run("runNameOnly stdout stays plain", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		out, _, _ := captureAll(t, func() {
			_, _ = runNameOnly(fetcher, "o/r", "1", todotype.DefaultPolicy())
		})
		if strings.Contains(out, "::notice") || strings.Contains(out, "::warning") || strings.Contains(out, "::error") {
			t.Fatalf("runNameOnly must not emit workflow commands; got %q", out)
		}
	})
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		ciFailingCount int
		ci             bool
		noCIFail       bool
		want           int
	}{
		{name: "error returns 1", err: errors.New("boom"), want: 1},
		{name: "error in CI still 1", err: errors.New("boom"), ci: true, ciFailingCount: 5, want: 1},
		{name: "no error no TODOs returns 0", want: 0},
		{name: "no error ci-failing TODOs not in CI returns 0", ciFailingCount: 3, want: 0},
		{name: "no error ci-failing TODOs in CI returns 1", ciFailingCount: 3, ci: true, want: 1},
		{name: "no error notice-only TODOs in CI returns 0", ciFailingCount: 0, ci: true, want: 0},
		{name: "no error ci-failing TODOs in CI with no-ci-fail returns 0", ciFailingCount: 3, ci: true, noCIFail: true, want: 0},
		{name: "no error zero TODOs in CI returns 0", ciFailingCount: 0, ci: true, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCode(tt.err, tt.ciFailingCount, tt.ci, tt.noCIFail); got != tt.want {
				t.Fatalf("exitCode(err=%v, ciFailingCount=%d, ci=%v, noCIFail=%v) = %d, expected %d",
					tt.err, tt.ciFailingCount, tt.ci, tt.noCIFail, got, tt.want)
			}
		})
	}
}

func TestRunFunctionsReturnCIFailingCount(t *testing.T) {
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

	tests := []struct {
		name          string
		fetcher       *stubFetcher
		wantTotal     int
		wantCIFailing int
	}{
		{
			name:          "no TODOs returns 0",
			fetcher:       &stubFetcher{diff: "", files: map[string][]byte{}},
			wantTotal:     0,
			wantCIFailing: 0,
		},
		{
			name: "one notice TODO returns 0",
			fetcher: &stubFetcher{
				diff:  sampleDiff,
				files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
			},
			wantTotal:     1,
			wantCIFailing: 0,
		},
		{
			name: "notice only (NOTE) returns 0",
			fetcher: &stubFetcher{
				diff:  noteOnlyDiff,
				files: map[string][]byte{"note.go": []byte("package note\n// NOTE: test note\n")},
			},
			wantTotal:     1,
			wantCIFailing: 0,
		},
		{
			name:          "notice and warning TODOs returns 0 CI failures (warning is not error)",
			fetcher:       &stubFetcher{diff: twoTODOsDiff, files: twoTODOsFiles},
			wantTotal:     2,
			wantCIFailing: 0,
		},
	}

	for _, tt := range tests {
		t.Run("runMain/"+tt.name, func(t *testing.T) {
			var result runResult
			var gotErr error
			_, _, _ = captureAll(t, func() {
				result, gotErr = runMain(tt.fetcher, "o/r", "1", types.GroupByNone, false, todotype.DefaultPolicy())
			})
			if gotErr != nil {
				t.Fatalf("runMain() unexpected error = %v", gotErr)
			}
			if result.totalCount != tt.wantTotal {
				t.Fatalf("runMain() totalCount = %d, expected %d (ciFailingCount=%d)", result.totalCount, tt.wantTotal, result.ciFailingCount)
			}
			if result.ciFailingCount != tt.wantCIFailing {
				t.Fatalf("runMain() ciFailingCount = %d, expected %d", result.ciFailingCount, tt.wantCIFailing)
			}
		})

		t.Run("runCount/"+tt.name, func(t *testing.T) {
			var result runResult
			var gotErr error
			_, _, _ = captureAll(t, func() {
				result, gotErr = runCount(tt.fetcher, "o/r", "1", todotype.DefaultPolicy())
			})
			if gotErr != nil {
				t.Fatalf("runCount() unexpected error = %v", gotErr)
			}
			if result.totalCount != tt.wantTotal {
				t.Fatalf("runCount() totalCount = %d, expected %d (ciFailingCount=%d)", result.totalCount, tt.wantTotal, result.ciFailingCount)
			}
			if result.ciFailingCount != tt.wantCIFailing {
				t.Fatalf("runCount() ciFailingCount = %d, expected %d", result.ciFailingCount, tt.wantCIFailing)
			}
		})

		t.Run("runNameOnly/"+tt.name, func(t *testing.T) {
			var result runResult
			var gotErr error
			_, _, _ = captureAll(t, func() {
				result, gotErr = runNameOnly(tt.fetcher, "o/r", "1", todotype.DefaultPolicy())
			})
			if gotErr != nil {
				t.Fatalf("runNameOnly() unexpected error = %v", gotErr)
			}
			if result.totalCount != tt.wantTotal {
				t.Fatalf("runNameOnly() totalCount = %d, expected %d (ciFailingCount=%d)", result.totalCount, tt.wantTotal, result.ciFailingCount)
			}
			if result.ciFailingCount != tt.wantCIFailing {
				t.Fatalf("runNameOnly() ciFailingCount = %d, expected %d", result.ciFailingCount, tt.wantCIFailing)
			}
		})
	}
}

func TestRunFunctionsReturnZeroOnError(t *testing.T) {
	checkZero := func(t *testing.T, label string, result runResult) {
		t.Helper()
		if result.totalCount != 0 {
			t.Fatalf("%s totalCount = %d, expected 0", label, result.totalCount)
		}
		if result.ciFailingCount != 0 {
			t.Fatalf("%s ciFailingCount = %d, expected 0", label, result.ciFailingCount)
		}
	}

	t.Run("runMain", func(t *testing.T) {
		fetcher := &stubFetcher{diffErr: errors.New("boom")}
		var result runResult
		var gotErr error
		_, _, _ = captureAll(t, func() {
			result, gotErr = runMain(fetcher, "", "", types.GroupByNone, false, todotype.DefaultPolicy())
		})
		if gotErr == nil {
			t.Fatalf("runMain() expected error, got nil")
		}
		checkZero(t, "runMain()", result)
	})

	t.Run("runCount", func(t *testing.T) {
		fetcher := &stubFetcher{diffErr: errors.New("boom")}
		var result runResult
		var gotErr error
		_, _, _ = captureAll(t, func() {
			result, gotErr = runCount(fetcher, "", "", todotype.DefaultPolicy())
		})
		if gotErr == nil {
			t.Fatalf("runCount() expected error, got nil")
		}
		checkZero(t, "runCount()", result)
	})

	t.Run("runNameOnly", func(t *testing.T) {
		fetcher := &stubFetcher{diffErr: errors.New("boom")}
		var result runResult
		var gotErr error
		_, _, _ = captureAll(t, func() {
			result, gotErr = runNameOnly(fetcher, "", "", todotype.DefaultPolicy())
		})
		if gotErr == nil {
			t.Fatalf("runNameOnly() expected error, got nil")
		}
		checkZero(t, "runNameOnly()", result)
	})
}

func TestResolveConfigTarget(t *testing.T) {
	t.Run("explicit repo uses remote config", func(t *testing.T) {
		repo, pr, remote := resolveConfigTarget("owner/repo", "123")
		if !remote || repo != "owner/repo" || pr != "123" {
			t.Fatalf("resolveConfigTarget() = (%q, %q, %v)", repo, pr, remote)
		}
	})

	t.Run("github PR URL uses remote config", func(t *testing.T) {
		repo, pr, remote := resolveConfigTarget("", "https://github.com/owner/repo/pull/123")
		if !remote || repo != "owner/repo" || pr != "123" {
			t.Fatalf("resolveConfigTarget() = (%q, %q, %v)", repo, pr, remote)
		}
	})

	t.Run("host-qualified PR URL preserves host", func(t *testing.T) {
		repo, pr, remote := resolveConfigTarget("", "https://github.example.com/owner/repo/pull/123")
		if !remote || repo != "github.example.com/owner/repo" || pr != "123" {
			t.Fatalf("resolveConfigTarget() = (%q, %q, %v)", repo, pr, remote)
		}
	})

	t.Run("non-PR URL stays local", func(t *testing.T) {
		repo, pr, remote := resolveConfigTarget("", "https://github.com/owner/repo/issues/123")
		if remote || repo != "" || pr != "https://github.com/owner/repo/issues/123" {
			t.Fatalf("resolveConfigTarget() = (%q, %q, %v)", repo, pr, remote)
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
		sevFlag  = newSeverityFlag()
		ignFlag  = newIgnoreFlag()
	)
	registerFlags(pflag.CommandLine, &repo, &nameOnly, &isCount, &isHelp, &noCIFail, &groupBy, sevFlag, ignFlag)

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
		"View TODO-style comments in the PR diff.",
		"USAGE",
		"gh pr-todo [<number> | <url> | <branch>] [flags]",
		"gh pr-todo init [--repo | --global] [--force]",
		"FLAGS",
		"--repo",
		"Display only names of the files containing TODO-style comments",
		"--name-only",
		"Display only the number of TODO-style comments",
		"--count",
		"--help",
		"--group-by",
		"--severity",
		"--ignore",
		"--no-ci-fail",
		"ENVIRONMENT",
		"CI",
		"GITHUB_ACTIONS",
		"error-level TODO is found.",
		"By default, no built-in",
		"keyword maps to error-level",
		"SEVERITY OVERRIDES",
		"LEVEL=TYPE[,TYPE...]",
		"workflow annotation levels and CI exits",
		"warning=TODO,HACK",
		"CONFIGURATION",
		"Severity overrides and ignored types can be configured",
		"custom types are detected alongside the built-in markers.",
		"Schema:",
		"severity:",
		"notice|warning|error: [TYPE...]",
		"ignore:",
		"- TYPE",
		"Empty lists are allowed and ignored; a type may not appear under multiple severity levels.",
		"user config dir/gh-pr-todo/config.yml",
		".gh-pr-todo.yml",
		".github/gh-pr-todo.yml",
		"remote config replaces global config when found",
		"--group-by",
		"Group TODO-style comments by: \"file\" or \"type\"",
		"remote default branch config",
		"remote PR base branch config",
		"remote PR head branch config",
		"CLI --severity and --ignore flags (highest priority)",
	}
	for _, want := range wantContain {
		if !strings.Contains(out, want) {
			t.Fatalf("printUsage() output = %q, expected to contain %q", out, want)
		}
	}
}

func TestPrintInitUsage(t *testing.T) {
	initFS := pflag.NewFlagSet("gh pr-todo init", pflag.ContinueOnError)
	initFS.Bool("force", false, "Overwrite existing config file")
	initFS.Bool("repo", false, "Create repo config at <repo>/.gh-pr-todo.yml without prompting")
	initFS.Bool("global", false, "Create global config at user config dir/gh-pr-todo/config.yml without prompting")
	initFS.BoolP("help", "h", false, "Display help information")

	var out string
	stdout := captureStdout(t, func() {
		out = captureColorOutput(t, func() {
			printInitUsage(initFS)
		})
	})

	if stdout != "" {
		t.Fatalf("printInitUsage() leaked to os.Stdout: %q", stdout)
	}
	wantContain := []string{
		"gh pr-todo init [--repo | --global] [--force]",
		"Create repo config at <repo>/.gh-pr-todo.yml without prompting",
		"Create global config at user config dir/gh-pr-todo/config.yml without prompting",
		"--repo: Project (.gh-pr-todo.yml)",
		"--global: user config dir/gh-pr-todo/config.yml",
		"Use --force to overwrite",
	}
	for _, want := range wantContain {
		if !strings.Contains(out, want) {
			t.Fatalf("printInitUsage() output = %q, expected to contain %q", out, want)
		}
	}
}

func TestSeverityFlagInvalid(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "no equals sign", value: "warning"},
		{name: "empty level", value: "=TODO"},
		{name: "empty types", value: "warning="},
		{name: "empty type in list", value: "warning=TODO,"},
		{name: "type contains equals after level separator", value: "warning==TODO"},
		{name: "type contains equals in list", value: "warning=TODO=FIXME"},
		{name: "invalid severity", value: "invalid=TODO"},
		{name: "unknown level", value: "critical=TODO"},
		{name: "just equals", value: "="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSeverityFlag()
			err := s.Set(tt.value)
			if err == nil {
				t.Fatalf("severityFlag.Set(%q) expected error, got nil", tt.value)
			}
		})
	}
}

func TestSeverityFlagParsing(t *testing.T) {
	t.Run("single type", func(t *testing.T) {
		s := newSeverityFlag()
		if err := s.Set("warning=TODO"); err != nil {
			t.Fatalf("Set(warning=TODO) unexpected error: %v", err)
		}
		if got := s.assignments["TODO"]; got != todotype.SeverityWarning {
			t.Fatalf("assignments[TODO] = %q, want %q", got, todotype.SeverityWarning)
		}
	})

	t.Run("multiple types", func(t *testing.T) {
		s := newSeverityFlag()
		if err := s.Set("error=FIXME,BUG"); err != nil {
			t.Fatalf("Set(error=FIXME,BUG) unexpected error: %v", err)
		}
		if got := s.assignments["FIXME"]; got != todotype.SeverityError {
			t.Fatalf("assignments[FIXME] = %q, want %q", got, todotype.SeverityError)
		}
		if got := s.assignments["BUG"]; got != todotype.SeverityError {
			t.Fatalf("assignments[BUG] = %q, want %q", got, todotype.SeverityError)
		}
	})

	t.Run("last wins across repeated flags", func(t *testing.T) {
		s := newSeverityFlag()
		if err := s.Set("warning=TODO"); err != nil {
			t.Fatalf("Set(warning=TODO) unexpected error: %v", err)
		}
		if err := s.Set("error=TODO"); err != nil {
			t.Fatalf("Set(error=TODO) unexpected error: %v", err)
		}
		if got := s.assignments["TODO"]; got != todotype.SeverityError {
			t.Fatalf("assignments[TODO] after last-wins = %q, want %q", got, todotype.SeverityError)
		}
	})

	t.Run("type case normalization", func(t *testing.T) {
		s := newSeverityFlag()
		if err := s.Set("error=fixme"); err != nil {
			t.Fatalf("Set(error=fixme) unexpected error: %v", err)
		}
		if got := s.assignments["FIXME"]; got != todotype.SeverityError {
			t.Fatalf("assignments[FIXME] = %q, want %q", got, todotype.SeverityError)
		}
	})

	t.Run("whitespace trimming", func(t *testing.T) {
		s := newSeverityFlag()
		if err := s.Set("  warning  =  TODO  ,  HACK  "); err != nil {
			t.Fatalf("Set with whitespace unexpected error: %v", err)
		}
		if got := s.assignments["TODO"]; got != todotype.SeverityWarning {
			t.Fatalf("assignments[TODO] with whitespace = %q, want %q", got, todotype.SeverityWarning)
		}
		if got := s.assignments["HACK"]; got != todotype.SeverityWarning {
			t.Fatalf("assignments[HACK] with whitespace = %q, want %q", got, todotype.SeverityWarning)
		}
	})

	t.Run("custom type", func(t *testing.T) {
		s := newSeverityFlag()
		if err := s.Set("warning=PERF"); err != nil {
			t.Fatalf("Set(warning=PERF) unexpected error: %v", err)
		}
		if got := s.assignments["PERF"]; got != todotype.SeverityWarning {
			t.Fatalf("assignments[PERF] = %q, want %q", got, todotype.SeverityWarning)
		}
	})

	t.Run("severity case normalization", func(t *testing.T) {
		s := newSeverityFlag()
		if err := s.Set("WARNING=TODO"); err != nil {
			t.Fatalf("Set(WARNING=TODO) unexpected error: %v", err)
		}
		if got := s.assignments["TODO"]; got != todotype.SeverityWarning {
			t.Fatalf("assignments[TODO] = %q, want %q", got, todotype.SeverityWarning)
		}

		s2 := newSeverityFlag()
		if err := s2.Set("Error=BUG"); err != nil {
			t.Fatalf("Set(Error=BUG) unexpected error: %v", err)
		}
		if got := s2.assignments["BUG"]; got != todotype.SeverityError {
			t.Fatalf("assignments[BUG] = %q, want %q", got, todotype.SeverityError)
		}
	})

	t.Run("invalid input does not mutate existing assignments", func(t *testing.T) {
		s := newSeverityFlag()
		if err := s.Set("warning=TODO"); err != nil {
			t.Fatalf("Set(warning=TODO) unexpected error: %v", err)
		}
		if err := s.Set("error=BUG,=FIXME"); err == nil {
			t.Fatal("Set(error=BUG,=FIXME) expected error, got nil")
		}
		if got := s.assignments["TODO"]; got != todotype.SeverityWarning {
			t.Fatalf("assignments[TODO] after invalid Set = %q, want %q", got, todotype.SeverityWarning)
		}
		if _, ok := s.assignments["BUG"]; ok {
			t.Fatal("assignments[BUG] was added by invalid Set; want no mutation on error")
		}
	})
}

func TestSeverityOverrideAffectsCIFailCount(t *testing.T) {
	fetcher := &stubFetcher{
		diff:  sampleDiff,
		files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
	}

	t.Run("default policy TODO is notice → no CI fail", func(t *testing.T) {
		var result runResult
		var err error
		_, _, _ = captureAll(t, func() {
			result, err = runMain(fetcher, "o/r", "1", types.GroupByNone, false, todotype.DefaultPolicy())
		})
		if err != nil {
			t.Fatalf("runMain() unexpected error = %v", err)
		}
		if result.ciFailingCount != 0 {
			t.Fatalf("ciFailingCount = %d, want 0", result.ciFailingCount)
		}
	})

	t.Run("TODO overridden to error affects all run modes", func(t *testing.T) {
		policy := todotype.DefaultPolicy().WithSeverity("TODO", todotype.SeverityError)

		var result runResult
		var err error
		_, _, _ = captureAll(t, func() {
			result, err = runMain(fetcher, "o/r", "1", types.GroupByNone, false, policy)
		})
		if err != nil {
			t.Fatalf("runMain() unexpected error = %v", err)
		}
		if result.ciFailingCount != 1 {
			t.Fatalf("runMain() ciFailingCount = %d, want 1 (TODO overridden to error)", result.ciFailingCount)
		}
		if result.totalCount != 1 {
			t.Fatalf("runMain() totalCount = %d, want 1", result.totalCount)
		}

		var countResult runResult
		countOut, countStdout, countStderr := captureAll(t, func() {
			countResult, err = runCount(fetcher, "o/r", "1", policy)
		})
		if err != nil {
			t.Fatalf("runCount() unexpected error = %v", err)
		}
		assertSilentChannels(t, "runCount()", countStdout, countStderr)
		if strings.TrimSpace(countOut) != "1" {
			t.Fatalf("runCount() output = %q, want %q", countOut, "1")
		}
		if countResult.ciFailingCount != 1 {
			t.Fatalf("runCount() ciFailingCount = %d, want 1 (TODO overridden to error)", countResult.ciFailingCount)
		}

		var nameOnlyResult runResult
		nameOnlyOut, nameOnlyStdout, nameOnlyStderr := captureAll(t, func() {
			nameOnlyResult, err = runNameOnly(fetcher, "o/r", "1", policy)
		})
		if err != nil {
			t.Fatalf("runNameOnly() unexpected error = %v", err)
		}
		assertSilentChannels(t, "runNameOnly()", nameOnlyStdout, nameOnlyStderr)
		if strings.TrimSpace(nameOnlyOut) != "foo.go" {
			t.Fatalf("runNameOnly() output = %q, want %q", nameOnlyOut, "foo.go")
		}
		if nameOnlyResult.ciFailingCount != 1 {
			t.Fatalf("runNameOnly() ciFailingCount = %d, want 1 (TODO overridden to error)", nameOnlyResult.ciFailingCount)
		}
	})

	t.Run("TODO overridden to warning → no CI fail (warning is not error)", func(t *testing.T) {
		policy := todotype.DefaultPolicy().WithSeverity("TODO", todotype.SeverityWarning)
		var result runResult
		var err error
		_, _, _ = captureAll(t, func() {
			result, err = runMain(fetcher, "o/r", "1", types.GroupByNone, false, policy)
		})
		if err != nil {
			t.Fatalf("runMain() unexpected error = %v", err)
		}
		if result.ciFailingCount != 0 {
			t.Fatalf("ciFailingCount = %d, want 0 (warning should not fail CI)", result.ciFailingCount)
		}
	})

	t.Run("NOTE overridden to error → CI fail count is 1", func(t *testing.T) {
		// NOTE is normally notice-level; override to error to trigger CI fail.
		fetcher := &stubFetcher{
			diff:  noteOnlyDiff,
			files: map[string][]byte{"note.go": []byte("package note\n// NOTE: important note\n")},
		}

		policy := todotype.DefaultPolicy().WithSeverity("NOTE", todotype.SeverityError)
		var result runResult
		var err error
		_, _, _ = captureAll(t, func() {
			result, err = runMain(fetcher, "o/r", "1", types.GroupByNone, false, policy)
		})
		if err != nil {
			t.Fatalf("runMain() unexpected error = %v", err)
		}
		if result.ciFailingCount != 1 {
			t.Fatalf("ciFailingCount = %d, want 1 (NOTE overridden to error)", result.ciFailingCount)
		}
		if result.totalCount != 1 {
			t.Fatalf("totalCount = %d, want 1", result.totalCount)
		}
	})
}

func TestSeverityOverrideAffectsWorkflowAnnotation(t *testing.T) {
	fetcher := &stubFetcher{
		diff:  sampleDiff,
		files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
	}

	t.Run("TODO overridden to warning → ::warning annotation", func(t *testing.T) {
		policy := todotype.DefaultPolicy().WithSeverity("TODO", todotype.SeverityWarning)
		out, _, _ := captureAll(t, func() {
			_, _ = runMain(fetcher, "o/r", "1", types.GroupByNone, true, policy)
		})
		wantLine := "::warning file=foo.go,line=2,title=TODO::// TODO: add bar"
		if !strings.Contains(out, wantLine) {
			t.Fatalf("runMain output = %q, expected to contain %q", out, wantLine)
		}
		if strings.Contains(out, "::notice ") {
			t.Fatalf("runMain output should not contain ::notice with warning override: %q", out)
		}
	})

	t.Run("TODO overridden to error → ::error annotation", func(t *testing.T) {
		policy := todotype.DefaultPolicy().WithSeverity("TODO", todotype.SeverityError)
		out, _, _ := captureAll(t, func() {
			_, _ = runMain(fetcher, "o/r", "1", types.GroupByNone, true, policy)
		})
		wantLine := "::error file=foo.go,line=2,title=TODO::// TODO: add bar"
		if !strings.Contains(out, wantLine) {
			t.Fatalf("runMain output = %q, expected to contain %q", out, wantLine)
		}
	})
}

// Mixed diff with both TODO and NOTE on added lines.
const mixedDiff = `diff --git a/foo.go b/foo.go
index 0000000..1111111 100644
--- a/foo.go
+++ b/foo.go
@@ -1,1 +1,3 @@
 package foo
+// TODO: add bar
+// NOTE: important note
`

func TestIgnoreFlagInvalid(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "empty type", value: "NOTE,"},
		{name: "type contains equals", value: "NOTE=FIXME"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newIgnoreFlag()
			err := f.Set(tt.value)
			if err == nil {
				t.Fatalf("ignoreFlag.Set(%q) expected error, got nil", tt.value)
			}
		})
	}
}

func TestIgnoreFlagParsing(t *testing.T) {
	t.Run("single type", func(t *testing.T) {
		f := newIgnoreFlag()
		if err := f.Set("NOTE"); err != nil {
			t.Fatalf("Set(NOTE) unexpected error: %v", err)
		}
		if len(f.types) != 1 || f.types[0] != "NOTE" {
			t.Fatalf("types = %v, want [NOTE]", f.types)
		}
	})

	t.Run("multiple types", func(t *testing.T) {
		f := newIgnoreFlag()
		if err := f.Set("NOTE,HACK"); err != nil {
			t.Fatalf("Set(NOTE,HACK) unexpected error: %v", err)
		}
		want := []string{"NOTE", "HACK"}
		if len(f.types) != 2 || f.types[0] != "NOTE" || f.types[1] != "HACK" {
			t.Fatalf("types = %v, want %v", f.types, want)
		}
	})

	t.Run("repeated flags accumulate", func(t *testing.T) {
		f := newIgnoreFlag()
		if err := f.Set("NOTE"); err != nil {
			t.Fatalf("Set(NOTE) unexpected error: %v", err)
		}
		if err := f.Set("HACK"); err != nil {
			t.Fatalf("Set(HACK) unexpected error: %v", err)
		}
		want := []string{"NOTE", "HACK"}
		if len(f.types) != 2 || f.types[0] != "NOTE" || f.types[1] != "HACK" {
			t.Fatalf("types = %v, want %v", f.types, want)
		}
	})

	t.Run("case normalization", func(t *testing.T) {
		f := newIgnoreFlag()
		if err := f.Set("note"); err != nil {
			t.Fatalf("Set(note) unexpected error: %v", err)
		}
		if len(f.types) != 1 || f.types[0] != "NOTE" {
			t.Fatalf("types = %v, want [NOTE]", f.types)
		}
	})

	t.Run("whitespace trimming", func(t *testing.T) {
		f := newIgnoreFlag()
		if err := f.Set("  NOTE  ,  HACK  "); err != nil {
			t.Fatalf("Set with whitespace unexpected error: %v", err)
		}
		want := []string{"NOTE", "HACK"}
		if len(f.types) != 2 || f.types[0] != "NOTE" || f.types[1] != "HACK" {
			t.Fatalf("types = %v, want %v", f.types, want)
		}
	})
}

func TestIgnoredTypesExcludeFromOutput(t *testing.T) {
	mixedFetcher := &stubFetcher{
		diff: mixedDiff,
		files: map[string][]byte{
			"foo.go": []byte("package foo\n// TODO: add bar\n// NOTE: important note\n"),
		},
	}

	ignoreNOTE := todotype.DefaultPolicy().WithIgnoredTypes([]string{"NOTE"})

	t.Run("default output excludes ignored NOTE", func(t *testing.T) {
		var result runResult
		var err error
		out, _, _ := captureAll(t, func() {
			result, err = runMain(mixedFetcher, "o/r", "1", types.GroupByNone, false, ignoreNOTE)
		})
		if err != nil {
			t.Fatalf("runMain() unexpected error: %v", err)
		}
		if strings.Contains(out, "NOTE") {
			t.Fatalf("output should not contain NOTE when ignored: %q", out)
		}
		if !strings.Contains(out, "TODO: add bar") {
			t.Fatalf("output should contain TODO: %q", out)
		}
		if result.totalCount != 1 {
			t.Fatalf("totalCount = %d, want 1 (only TODO)", result.totalCount)
		}
	})

	t.Run("--count excludes ignored NOTE", func(t *testing.T) {
		var result runResult
		var err error
		out, _, _ := captureAll(t, func() {
			result, err = runCount(mixedFetcher, "o/r", "1", ignoreNOTE)
		})
		if err != nil {
			t.Fatalf("runCount() unexpected error: %v", err)
		}
		if strings.TrimSpace(out) != "1" {
			t.Fatalf("runCount() output = %q, want %q", out, "1")
		}
		if result.totalCount != 1 {
			t.Fatalf("totalCount = %d, want 1", result.totalCount)
		}
	})

	t.Run("--name-only excludes ignored NOTE", func(t *testing.T) {
		// Both markers are in foo.go, so file should still appear
		var err error
		out, _, _ := captureAll(t, func() {
			_, err = runNameOnly(mixedFetcher, "o/r", "1", ignoreNOTE)
		})
		if err != nil {
			t.Fatalf("runNameOnly() unexpected error: %v", err)
		}
		if strings.TrimSpace(out) != "foo.go" {
			t.Fatalf("runNameOnly() output = %q, want %q", out, "foo.go")
		}
	})

	t.Run("group-by type excludes ignored NOTE", func(t *testing.T) {
		var result runResult
		var err error
		out, _, _ := captureAll(t, func() {
			result, err = runMain(mixedFetcher, "o/r", "1", types.GroupByType, false, ignoreNOTE)
		})
		if err != nil {
			t.Fatalf("runMain() unexpected error: %v", err)
		}
		if strings.Contains(out, "[NOTE]") {
			t.Fatalf("group-by output should not contain [NOTE] section: %q", out)
		}
		if !strings.Contains(out, "[TODO]") {
			t.Fatalf("group-by output should contain [TODO] section: %q", out)
		}
		if result.totalCount != 1 {
			t.Fatalf("totalCount = %d, want 1", result.totalCount)
		}
	})

	t.Run("workflow annotations exclude ignored NOTE", func(t *testing.T) {
		var err error
		out, _, _ := captureAll(t, func() {
			_, err = runMain(mixedFetcher, "o/r", "1", types.GroupByNone, true, ignoreNOTE)
		})
		if err != nil {
			t.Fatalf("runMain() unexpected error: %v", err)
		}
		if strings.Contains(out, "::notice file=foo.go,line=3,title=NOTE") {
			t.Fatalf("workflow output should not contain NOTE annotation: %q", out)
		}
		if !strings.Contains(out, "::notice file=foo.go,line=2,title=TODO") {
			t.Fatalf("workflow output should contain TODO annotation: %q", out)
		}
	})

	t.Run("CI failure count excludes ignored types", func(t *testing.T) {
		// NOTE overridden to error would normally fail CI, but if NOTE is ignored it should not
		policy := todotype.DefaultPolicy().WithSeverity("NOTE", todotype.SeverityError).WithIgnoredTypes([]string{"NOTE"})
		var result runResult
		var err error
		_, _, _ = captureAll(t, func() {
			result, err = runMain(mixedFetcher, "o/r", "1", types.GroupByNone, false, policy)
		})
		if err != nil {
			t.Fatalf("runMain() unexpected error: %v", err)
		}
		if result.ciFailingCount != 0 {
			t.Fatalf("ciFailingCount = %d, want 0 (NOTE ignored overrides error severity)", result.ciFailingCount)
		}
		// TODO should still be detected
		if result.totalCount != 1 {
			t.Fatalf("totalCount = %d, want 1 (only TODO)", result.totalCount)
		}
	})
}

func TestInitPathOptions(t *testing.T) {
	repoPath := filepath.Join("repo", ".gh-pr-todo.yml")
	globalPath := filepath.Join("config", "gh-pr-todo", "config.yml")

	t.Run("includes repo and global when both are available", func(t *testing.T) {
		options := initPathOptions(repoPath, nil, globalPath, nil)
		if len(options) != 2 {
			t.Fatalf("len(options) = %d, want 2", len(options))
		}
		if options[0].Key != "Project (.gh-pr-todo.yml)" {
			t.Fatalf("first option label = %q, want Project label", options[0].Key)
		}
		if options[0].Value != repoPath {
			t.Fatalf("first option = %q, want repo path %q", options[0].Value, repoPath)
		}
		wantGlobalLabel := "Global (" + globalPath + ")"
		if options[1].Key != wantGlobalLabel {
			t.Fatalf("second option label = %q, want %q", options[1].Key, wantGlobalLabel)
		}
		if options[1].Value != globalPath {
			t.Fatalf("second option = %q, want global path %q", options[1].Value, globalPath)
		}
	})

	t.Run("hides repo option outside git repo", func(t *testing.T) {
		options := initPathOptions(repoPath, errors.New("requires git repo"), globalPath, nil)
		if len(options) != 1 {
			t.Fatalf("len(options) = %d, want 1", len(options))
		}
		if options[0].Value != globalPath {
			t.Fatalf("only option = %q, want global path %q", options[0].Value, globalPath)
		}
	})
}

func TestChooseInitPathTextLabels(t *testing.T) {
	repoPath := filepath.Join("repo", ".gh-pr-todo.yml")
	globalPath := filepath.Join("config", "gh-pr-todo", "config.yml")

	t.Run("available locations include scoped labels", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := chooseInitPathText(strings.NewReader("1\n"), &buf, repoPath, nil, globalPath, nil)
		if err != nil {
			t.Fatalf("chooseInitPathText() unexpected error: %v", err)
		}
		output := buf.String()
		if !strings.Contains(output, "1) Project (.gh-pr-todo.yml)") {
			t.Fatalf("output = %q, expected project label", output)
		}
		if !strings.Contains(output, "2) Global ("+globalPath+")") {
			t.Fatalf("output = %q, expected global label", output)
		}
	})

	t.Run("unavailable repo location includes scoped message", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := chooseInitPathText(strings.NewReader("2\n"), &buf, repoPath, errors.New("not inside a Git repository"), globalPath, nil)
		if err != nil {
			t.Fatalf("chooseInitPathText() unexpected error: %v", err)
		}
		output := buf.String()
		if !strings.Contains(output, "1) Project (unavailable: not inside a Git repository)") {
			t.Fatalf("output = %q, expected unavailable project label", output)
		}
	})

	t.Run("unavailable global location includes scoped message", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := chooseInitPathText(strings.NewReader("1\n"), &buf, repoPath, nil, globalPath, errors.New("user config directory is empty"))
		if err != nil {
			t.Fatalf("chooseInitPathText() unexpected error: %v", err)
		}
		output := buf.String()
		if !strings.Contains(output, "2) Global (unavailable: user config directory not available)") {
			t.Fatalf("output = %q, expected unavailable global label", output)
		}
	})

	t.Run("no available locations returns before prompting", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := chooseInitPathText(strings.NewReader(""), &buf, repoPath, errors.New("not inside a Git repository"), "", errors.New("user config directory is empty"))
		if err == nil || !strings.Contains(err.Error(), "no config file location available") {
			t.Fatalf("chooseInitPathText() error = %v, want no location error", err)
		}
		if strings.Contains(buf.String(), "Enter selection") {
			t.Fatalf("output = %q, expected no prompt", buf.String())
		}
	})
}

func TestShouldUseInteractivePrompt(t *testing.T) {
	if shouldUseInteractivePrompt(strings.NewReader("1\n"), io.Discard) {
		t.Fatal("shouldUseInteractivePrompt() = true for non-file streams, want false")
	}
	if shouldUseInteractivePrompt(os.Stdin, &bytes.Buffer{}) {
		t.Fatal("shouldUseInteractivePrompt() = true for non-terminal output, want false")
	}
}

func TestInitTargetFromFlags(t *testing.T) {
	tests := []struct {
		name       string
		repoFlag   bool
		globalFlag bool
		want       initTarget
		wantErr    string
	}{
		{name: "prompt", want: initTargetPrompt},
		{name: "repo", repoFlag: true, want: initTargetRepo},
		{name: "global", globalFlag: true, want: initTargetGlobal},
		{name: "conflict", repoFlag: true, globalFlag: true, wantErr: "cannot use --repo and --global together"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := initTargetFromFlags(tt.repoFlag, tt.globalFlag)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("initTargetFromFlags() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("initTargetFromFlags() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("initTargetFromFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunInit(t *testing.T) {
	t.Run("selection 1 creates repo config", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}

		var buf bytes.Buffer
		in := strings.NewReader("1\n")
		err := runInit(in, &buf, repoRoot, t.TempDir(), false, initTargetPrompt)
		if err != nil {
			t.Fatalf("runInit() unexpected error: %v", err)
		}

		wantPath := filepath.Join(repoRoot, ".gh-pr-todo.yml")
		if _, err := os.Stat(wantPath); err != nil {
			t.Fatalf("expected file at %s: %v", wantPath, err)
		}
		if !strings.Contains(buf.String(), "Created") {
			t.Fatalf("output = %q, expected success message", buf.String())
		}
	})

	t.Run("selection 2 creates global config", func(t *testing.T) {
		userConfigDir := t.TempDir()
		var buf bytes.Buffer
		in := strings.NewReader("2\n")
		err := runInit(in, &buf, t.TempDir(), userConfigDir, false, initTargetPrompt)
		if err != nil {
			t.Fatalf("runInit() unexpected error: %v", err)
		}

		wantPath := filepath.Join(userConfigDir, "gh-pr-todo", "config.yml")
		if _, err := os.Stat(wantPath); err != nil {
			t.Fatalf("expected file at %s: %v", wantPath, err)
		}
		if !strings.Contains(buf.String(), "Created") {
			t.Fatalf("output = %q, expected success message", buf.String())
		}
	})

	t.Run("repo target creates repo config without prompt", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}

		var buf bytes.Buffer
		err := runInit(strings.NewReader(""), &buf, repoRoot, t.TempDir(), false, initTargetRepo)
		if err != nil {
			t.Fatalf("runInit() unexpected error: %v", err)
		}

		wantPath := filepath.Join(repoRoot, ".gh-pr-todo.yml")
		if _, err := os.Stat(wantPath); err != nil {
			t.Fatalf("expected file at %s: %v", wantPath, err)
		}
		if strings.Contains(buf.String(), "Enter selection") {
			t.Fatalf("output = %q, expected no prompt", buf.String())
		}
	})

	t.Run("global target creates global config without prompt", func(t *testing.T) {
		userConfigDir := t.TempDir()
		var buf bytes.Buffer
		err := runInit(strings.NewReader(""), &buf, filepath.Join(t.TempDir(), "missing"), userConfigDir, false, initTargetGlobal)
		if err != nil {
			t.Fatalf("runInit() unexpected error: %v", err)
		}

		wantPath := filepath.Join(userConfigDir, "gh-pr-todo", "config.yml")
		if _, err := os.Stat(wantPath); err != nil {
			t.Fatalf("expected file at %s: %v", wantPath, err)
		}
		if strings.Contains(buf.String(), "Enter selection") {
			t.Fatalf("output = %q, expected no prompt", buf.String())
		}
	})

	t.Run("repo target outside repo returns error without prompt", func(t *testing.T) {
		var buf bytes.Buffer
		err := runInit(strings.NewReader(""), &buf, t.TempDir(), t.TempDir(), false, initTargetRepo)
		if err == nil || !strings.Contains(err.Error(), "Git repository") {
			t.Fatalf("runInit() error = %v, want Git repository error", err)
		}
		if strings.Contains(buf.String(), "Enter selection") {
			t.Fatalf("output = %q, expected no prompt", buf.String())
		}
	})

	t.Run("global target without user config dir returns error without prompt", func(t *testing.T) {
		var buf bytes.Buffer
		err := runInit(strings.NewReader(""), &buf, filepath.Join(t.TempDir(), "missing"), "", false, initTargetGlobal)
		if err == nil || !strings.Contains(err.Error(), "user config directory not available") {
			t.Fatalf("runInit() error = %v, want user config dir error", err)
		}
		if strings.Contains(buf.String(), "Enter selection") {
			t.Fatalf("output = %q, expected no prompt", buf.String())
		}
	})

	t.Run("selection without trailing newline is accepted", func(t *testing.T) {
		userConfigDir := t.TempDir()
		var buf bytes.Buffer
		in := strings.NewReader("2")
		err := runInit(in, &buf, t.TempDir(), userConfigDir, false, initTargetPrompt)
		if err != nil {
			t.Fatalf("runInit() unexpected error: %v", err)
		}

		wantPath := filepath.Join(userConfigDir, "gh-pr-todo", "config.yml")
		if _, err := os.Stat(wantPath); err != nil {
			t.Fatalf("expected file at %s: %v", wantPath, err)
		}
	})

	t.Run("existing file without force returns error", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		// Create the config file first
		configPath := filepath.Join(repoRoot, ".gh-pr-todo.yml")
		if err := os.WriteFile(configPath, []byte("original"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		var buf bytes.Buffer
		in := strings.NewReader("1\n")
		err := runInit(in, &buf, repoRoot, t.TempDir(), false, initTargetPrompt)
		if err == nil {
			t.Fatal("runInit() expected error for existing file without force")
		}
		if !strings.Contains(err.Error(), "--force") {
			t.Fatalf("runInit() error = %q, expected to mention --force", err.Error())
		}
		// Content preserved
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}
		if string(data) != "original" {
			t.Fatalf("expected preserved content %q, got %q", "original", string(data))
		}
	})

	t.Run("existing narrow repo config returns error", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		narrowDir := filepath.Join(repoRoot, ".github")
		if err := os.MkdirAll(narrowDir, 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		narrowPath := filepath.Join(narrowDir, "gh-pr-todo.yml")
		if err := os.WriteFile(narrowPath, []byte("original"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		var buf bytes.Buffer
		in := strings.NewReader("1\n")
		err := runInit(in, &buf, repoRoot, t.TempDir(), false, initTargetPrompt)
		if err == nil {
			t.Fatal("runInit() expected error for existing narrow repo config")
		}
		if !strings.Contains(err.Error(), narrowPath) || !strings.Contains(err.Error(), "takes precedence") {
			t.Fatalf("runInit() error = %q, expected narrow config precedence message", err.Error())
		}
		rootPath := filepath.Join(repoRoot, ".gh-pr-todo.yml")
		if _, err := os.Stat(rootPath); !os.IsNotExist(err) {
			t.Fatalf("root config should not be created, stat err = %v", err)
		}
	})

	t.Run("existing file with force overwrites", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		// Create the config file first
		configPath := filepath.Join(repoRoot, ".gh-pr-todo.yml")
		if err := os.WriteFile(configPath, []byte("original"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		var buf bytes.Buffer
		in := strings.NewReader("1\n")
		err := runInit(in, &buf, repoRoot, t.TempDir(), true, initTargetPrompt)
		if err != nil {
			t.Fatalf("runInit() with force unexpected error: %v", err)
		}
		// Content should be default config now
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}
		cfg, err := config.Parse(data, configPath)
		if err != nil {
			t.Fatalf("Parse(overwritten) error: %v", err)
		}
		if cfg.Severities["TODO"] != todotype.SeverityNotice {
			t.Fatalf("expected TODO=notice after overwrite, got %v", cfg.Severities["TODO"])
		}
	})

	t.Run("invalid selection returns error", func(t *testing.T) {
		var buf bytes.Buffer
		in := strings.NewReader("3\n")
		err := runInit(in, &buf, t.TempDir(), t.TempDir(), false, initTargetPrompt)
		if err == nil {
			t.Fatal("runInit() expected error for invalid selection")
		}
		if !strings.Contains(err.Error(), "invalid selection") {
			t.Fatalf("runInit() error = %q, expected 'invalid selection'", err.Error())
		}
	})

	t.Run("EOF returns error", func(t *testing.T) {
		var buf bytes.Buffer
		in := strings.NewReader("") // EOF on first read
		err := runInit(in, &buf, t.TempDir(), t.TempDir(), false, initTargetPrompt)
		if err == nil {
			t.Fatal("runInit() expected error for EOF")
		}
	})

	t.Run("empty input returns error", func(t *testing.T) {
		var buf bytes.Buffer
		in := strings.NewReader("\n") // Just newline = empty after trim
		err := runInit(in, &buf, t.TempDir(), t.TempDir(), false, initTargetPrompt)
		if err == nil {
			t.Fatal("runInit() expected error for empty input")
		}
		if !strings.Contains(err.Error(), "no input") {
			t.Fatalf("runInit() error = %q, expected 'no input'", err.Error())
		}
	})

	t.Run("local selection outside repo returns error", func(t *testing.T) {
		var buf bytes.Buffer
		in := strings.NewReader("1\n")
		cwd := t.TempDir() // No .git directory
		err := runInit(in, &buf, cwd, t.TempDir(), false, initTargetPrompt)
		if err == nil {
			t.Fatal("runInit() expected error for local outside repo")
		}
		if !strings.Contains(err.Error(), "Git repository") {
			t.Fatalf("runInit() error = %q, expected 'Git repository'", err.Error())
		}
	})
}
