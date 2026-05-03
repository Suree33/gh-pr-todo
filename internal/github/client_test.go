package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Suree33/gh-pr-todo/internal/todotype"
	"github.com/Suree33/gh-pr-todo/pkg/types"
)

// defaultTypes is used as the todoTypes argument in CollectTODOs calls.
var defaultTypes = todotype.DefaultTypes()

func TestNewClient(t *testing.T) {
	if NewClient() == nil {
		t.Fatal("NewClient() = nil, expected non-nil")
	}
}

func TestPRMetaHeadRepositoryNameWithOwner(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected string
	}{
		{
			name: "uses nameWithOwner from gh pr view payload",
			payload: `{
				"headRefOid": "b93811e17d1cb86894fc3196f00be046d483b26e",
				"headRepository": {
					"id": "R_kgDOPgbooQ",
					"name": "gh-pr-todo",
					"nameWithOwner": "Suree33/gh-pr-todo"
				}
			}`,
			expected: "Suree33/gh-pr-todo",
		},
		{
			name: "falls back to owner and name",
			payload: `{
				"headRefOid": "b93811e17d1cb86894fc3196f00be046d483b26e",
				"headRepository": {
					"name": "gh-pr-todo",
					"owner": {
						"login": "Suree33"
					}
				}
			}`,
			expected: "Suree33/gh-pr-todo",
		},
		{
			name: "returns empty when owner login missing",
			payload: `{
				"headRepository": {
					"name": "gh-pr-todo"
				}
			}`,
			expected: "",
		},
		{
			name: "returns empty when name missing",
			payload: `{
				"headRepository": {
					"owner": {"login": "Suree33"}
				}
			}`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var meta prMeta
			if err := json.Unmarshal([]byte(tt.payload), &meta); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if got := meta.headRepositoryNameWithOwner(); got != tt.expected {
				t.Fatalf("headRepositoryNameWithOwner() = %q, expected %q", got, tt.expected)
			}
		})
	}
}

// withGhExec swaps the package-level ghExec for the duration of a test.
// Tests that use this helper MUST NOT call t.Parallel(): the swap is a
// global mutation and would race with concurrent subtests.
func withGhExec(t *testing.T, fn func(args ...string) (bytes.Buffer, bytes.Buffer, error)) {
	t.Helper()
	original := ghExec
	ghExec = fn
	t.Cleanup(func() { ghExec = original })
}

// captureStderr redirects os.Stderr while fn runs and returns the captured
// output. Like withGhExec it mutates a global, so callers must not use
// t.Parallel().
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

func TestFetchDiff(t *testing.T) {
	tests := []struct {
		name        string
		repo        string
		pr          string
		stdout      string
		stderr      string
		execErr     error
		wantOut     string
		wantErr     string
		wantWarning string
		wantArgs    []string
	}{
		{
			name:     "success without repo or pr",
			stdout:   "diff --git a/x b/x\n",
			wantOut:  "diff --git a/x b/x\n",
			wantArgs: []string{"pr", "diff"},
		},
		{
			name:     "success with repo and pr",
			repo:     "owner/repo",
			pr:       "42",
			stdout:   "diff body",
			wantOut:  "diff body",
			wantArgs: []string{"pr", "diff", "-R", "owner/repo", "42"},
		},
		{
			name:    "error with stderr message",
			stderr:  "  bad credentials  ",
			execErr: errors.New("exit 1"),
			wantErr: "bad credentials",
		},
		{
			name:    "error with empty stderr falls back to err",
			execErr: errors.New("exit 1"),
			wantErr: "exit 1",
		},
		{
			name:        "stderr warning printed on success",
			stdout:      "ok",
			stderr:      "deprecation notice",
			wantOut:     "ok",
			wantWarning: "Warning: deprecation notice\n",
			wantArgs:    []string{"pr", "diff"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgs []string
			withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
				gotArgs = args
				return *bytes.NewBufferString(tt.stdout), *bytes.NewBufferString(tt.stderr), tt.execErr
			})

			c := NewClient()
			var got string
			var gotErr error
			stderrOut := captureStderr(t, func() {
				got, gotErr = c.FetchDiff(tt.repo, tt.pr)
			})

			if tt.wantErr != "" {
				if gotErr == nil {
					t.Fatalf("FetchDiff() error = nil, expected %q", tt.wantErr)
				}
				if gotErr.Error() != tt.wantErr {
					t.Fatalf("FetchDiff() error = %q, expected %q", gotErr.Error(), tt.wantErr)
				}
				if stderrOut != "" {
					t.Fatalf("stderr = %q, expected empty on error path", stderrOut)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("FetchDiff() unexpected error = %v", gotErr)
			}
			if got != tt.wantOut {
				t.Fatalf("FetchDiff() = %q, expected %q", got, tt.wantOut)
			}
			if tt.wantArgs != nil && !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("ghExec args = %v, expected %v", gotArgs, tt.wantArgs)
			}
			if tt.wantWarning != "" && !strings.Contains(stderrOut, tt.wantWarning) {
				t.Fatalf("stderr = %q, expected to contain %q", stderrOut, tt.wantWarning)
			}
			if tt.wantWarning == "" && stderrOut != "" {
				t.Fatalf("stderr = %q, expected empty", stderrOut)
			}
		})
	}
}

func TestFetchRemoteConfigRefs(t *testing.T) {
	var calls [][]string
	withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
		copied := append([]string(nil), args...)
		calls = append(calls, copied)
		switch len(calls) {
		case 1:
			return *bytes.NewBufferString(`{"defaultBranchRef":{"name":"main"},"nameWithOwner":"owner/repo"}`), bytes.Buffer{}, nil
		case 2:
			return *bytes.NewBufferString(`{"baseRefName":"release","headRefOid":"abc123","headRepository":{"nameWithOwner":"fork/repo"}}`), bytes.Buffer{}, nil
		default:
			return bytes.Buffer{}, bytes.Buffer{}, errors.New("unexpected call")
		}
	})

	refs, err := NewClient().FetchRemoteConfigRefs("github.example.com/owner/repo", "42")
	if err != nil {
		t.Fatalf("FetchRemoteConfigRefs() unexpected error: %v", err)
	}
	wantCalls := [][]string{
		{"repo", "view", "github.example.com/owner/repo", "--json", "defaultBranchRef,nameWithOwner"},
		{"pr", "view", "--json", "baseRefName,headRefOid,headRepository", "-R", "github.example.com/owner/repo", "42"},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("ghExec calls = %v, expected %v", calls, wantCalls)
	}
	if refs.DefaultRepo != "github.example.com/owner/repo" || refs.BaseRepo != "github.example.com/owner/repo" || refs.HeadRepo != "github.example.com/fork/repo" {
		t.Fatalf("refs repos = default %q base %q head %q", refs.DefaultRepo, refs.BaseRepo, refs.HeadRepo)
	}
	if refs.DefaultBranchRef != "main" || refs.BaseBranchRef != "release" || refs.HeadRefOid != "abc123" {
		t.Fatalf("refs = %+v", refs)
	}
}

func TestFetchFileAtRef(t *testing.T) {
	t.Run("fetches raw content with escaped path and ref", func(t *testing.T) {
		var gotArgs []string
		withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
			gotArgs = args
			return *bytes.NewBufferString("severity:\n  TODO: error\n"), bytes.Buffer{}, nil
		})

		got, found, err := NewClient().FetchFileAtRef("owner/repo", ".github/gh pr-todo.yml", "feature/config")
		if err != nil {
			t.Fatalf("FetchFileAtRef() unexpected error: %v", err)
		}
		if !found {
			t.Fatal("FetchFileAtRef() found = false, expected true")
		}
		if string(got) != "severity:\n  TODO: error\n" {
			t.Fatalf("FetchFileAtRef() data = %q", got)
		}
		wantArgs := []string{"api", "repos/owner/repo/contents/.github/gh%20pr-todo.yml?ref=feature%2Fconfig", "-H", "Accept: application/vnd.github.raw+json"}
		if !reflect.DeepEqual(gotArgs, wantArgs) {
			t.Fatalf("ghExec args = %v, expected %v", gotArgs, wantArgs)
		}
	})

	t.Run("host-qualified repo passes hostname to gh api", func(t *testing.T) {
		var gotArgs []string
		withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
			gotArgs = args
			return *bytes.NewBufferString("severity: {}\n"), bytes.Buffer{}, nil
		})

		_, found, err := NewClient().FetchFileAtRef("github.example.com/owner/repo", ".gh-pr-todo.yml", "main")
		if err != nil {
			t.Fatalf("FetchFileAtRef() unexpected error: %v", err)
		}
		if !found {
			t.Fatal("FetchFileAtRef() found = false, expected true")
		}
		wantArgs := []string{"api", "repos/owner/repo/contents/.gh-pr-todo.yml?ref=main", "-H", "Accept: application/vnd.github.raw+json", "--hostname", "github.example.com"}
		if !reflect.DeepEqual(gotArgs, wantArgs) {
			t.Fatalf("ghExec args = %v, expected %v", gotArgs, wantArgs)
		}
	})

	t.Run("not found returns found false without error", func(t *testing.T) {
		withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
			return bytes.Buffer{}, *bytes.NewBufferString("HTTP 404: Not Found"), errors.New("exit 1")
		})

		got, found, err := NewClient().FetchFileAtRef("owner/repo", ".gh-pr-todo.yml", "main")
		if err != nil {
			t.Fatalf("FetchFileAtRef() unexpected error: %v", err)
		}
		if found {
			t.Fatal("FetchFileAtRef() found = true, expected false")
		}
		if got != nil {
			t.Fatalf("FetchFileAtRef() data = %q, expected nil", got)
		}
	})
}

const sampleDiff = `diff --git a/foo.go b/foo.go
index 0000000..1111111 100644
--- a/foo.go
+++ b/foo.go
@@ -1,1 +1,2 @@
 package foo
+// TODO: add bar
`

const twoFileDiff = `diff --git a/foo.go b/foo.go
index 0000000..1111111 100644
--- a/foo.go
+++ b/foo.go
@@ -1,1 +1,2 @@
 package foo
+// TODO: add bar
diff --git a/bar.go b/bar.go
index 0000000..2222222 100644
--- a/bar.go
+++ b/bar.go
@@ -1,1 +1,2 @@
 package bar
+// TODO: add baz
`

func TestFetchChangedFileContents(t *testing.T) {
	metaJSON := `{"headRefOid":"abc123","headRepository":{"nameWithOwner":"o/r"}}`

	t.Run("returns pr view exec error", func(t *testing.T) {
		withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
			return bytes.Buffer{}, bytes.Buffer{}, errors.New("boom")
		})
		c := NewClient()
		got, err := c.FetchChangedFileContents("", "", sampleDiff)
		if err == nil || err.Error() != "boom" {
			t.Fatalf("expected boom error, got %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil files, got %v", got)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
			return *bytes.NewBufferString("not-json"), bytes.Buffer{}, nil
		})
		c := NewClient()
		_, err := c.FetchChangedFileContents("", "", sampleDiff)
		if err == nil {
			t.Fatal("expected json error, got nil")
		}
	})

	t.Run("empty meta", func(t *testing.T) {
		withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
			return *bytes.NewBufferString(`{}`), bytes.Buffer{}, nil
		})
		c := NewClient()
		_, err := c.FetchChangedFileContents("", "", sampleDiff)
		if err == nil || err.Error() != "could not determine PR head" {
			t.Fatalf("expected PR head error, got %v", err)
		}
	})

	t.Run("success with repo and pr", func(t *testing.T) {
		var calls [][]string
		withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
			calls = append(calls, args)
			if args[0] == "pr" {
				return *bytes.NewBufferString(metaJSON), bytes.Buffer{}, nil
			}
			return *bytes.NewBufferString("file contents"), bytes.Buffer{}, nil
		})
		c := NewClient()
		got, err := c.FetchChangedFileContents("o/r", "1", sampleDiff)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got["foo.go"]) != "file contents" {
			t.Fatalf("got %v", got)
		}
		if len(calls) != 2 {
			t.Fatalf("expected 2 ghExec calls, got %d: %v", len(calls), calls)
		}
		expectedFirst := []string{"pr", "view", "--json", "headRefOid,headRepository", "-R", "o/r", "1"}
		if !reflect.DeepEqual(calls[0], expectedFirst) {
			t.Fatalf("first call args = %v, expected %v", calls[0], expectedFirst)
		}
		expectedSecond := []string{"api", "repos/o/r/contents/foo.go?ref=abc123", "-H", "Accept: application/vnd.github.raw+json"}
		if !reflect.DeepEqual(calls[1], expectedSecond) {
			t.Fatalf("second call args = %v, expected %v", calls[1], expectedSecond)
		}
	})

	t.Run("host-qualified repo passes hostname when fetching changed file contents", func(t *testing.T) {
		metaJSON := `{"headRefOid":"feature/sha","headRepository":{"nameWithOwner":"o/r"}}`
		var calls [][]string
		withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
			copied := append([]string(nil), args...)
			calls = append(calls, copied)
			if args[0] == "pr" {
				return *bytes.NewBufferString(metaJSON), bytes.Buffer{}, nil
			}
			return *bytes.NewBufferString("file contents"), bytes.Buffer{}, nil
		})

		_, err := NewClient().FetchChangedFileContents("github.example.com/o/r", "1", sampleDiff)
		if err != nil {
			t.Fatalf("FetchChangedFileContents() unexpected error: %v", err)
		}
		if len(calls) != 2 {
			t.Fatalf("expected 2 calls, got %d: %v", len(calls), calls)
		}
		want := []string{"api", "repos/o/r/contents/foo.go?ref=feature%2Fsha", "-H", "Accept: application/vnd.github.raw+json", "--hostname", "github.example.com"}
		if !reflect.DeepEqual(calls[1], want) {
			t.Fatalf("second call args = %v, expected %v", calls[1], want)
		}
	})

	t.Run("partial failure returns error and partial files", func(t *testing.T) {
		withGhExec(t, func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
			if args[0] == "pr" {
				return *bytes.NewBufferString(metaJSON), bytes.Buffer{}, nil
			}
			if strings.Contains(args[1], "/foo.go") {
				return *bytes.NewBufferString("foo contents"), bytes.Buffer{}, nil
			}
			return bytes.Buffer{}, bytes.Buffer{}, errors.New("404")
		})
		c := NewClient()
		got, err := c.FetchChangedFileContents("", "", twoFileDiff)
		if err == nil || !strings.Contains(err.Error(), "failed to fetch 1") {
			t.Fatalf("expected failed-to-fetch error, got %v", err)
		}
		if string(got["foo.go"]) != "foo contents" {
			t.Fatalf("expected foo.go to be present with successful contents, got map=%v", got)
		}
		if _, ok := got["bar.go"]; ok {
			t.Fatalf("expected bar.go to be absent (fetch failed), got map=%v", got)
		}
	})
}

type stubFetcher struct {
	diff          string
	diffErr       error
	files         map[string][]byte
	filesErr      error
	gotRepoFD     string
	gotPRFD       string
	gotRepoFC     string
	gotPRFC       string
	gotDiffFC     string
	fetchFCCalled bool
}

func (s *stubFetcher) FetchDiff(repo, pr string) (string, error) {
	s.gotRepoFD, s.gotPRFD = repo, pr
	return s.diff, s.diffErr
}

func (s *stubFetcher) FetchChangedFileContents(repo, pr, diff string) (map[string][]byte, error) {
	s.fetchFCCalled = true
	s.gotRepoFC, s.gotPRFC, s.gotDiffFC = repo, pr, diff
	return s.files, s.filesErr
}

func TestCollectTODOs(t *testing.T) {
	t.Run("FetchDiff error returned", func(t *testing.T) {
		s := &stubFetcher{diffErr: errors.New("diff failed")}
		var (
			todos []types.TODO
			err   error
		)
		stderrOut := captureStderr(t, func() {
			todos, err = CollectTODOs(s, "o/r", "1", defaultTypes)
		})
		if err == nil || err.Error() != "diff failed" {
			t.Fatalf("expected diff failed error, got %v", err)
		}
		if todos != nil {
			t.Fatalf("expected nil todos, got %v", todos)
		}
		if stderrOut != "" {
			t.Fatalf("stderr = %q, expected empty", stderrOut)
		}
		if s.fetchFCCalled {
			t.Fatal("FetchChangedFileContents should not be called when FetchDiff fails")
		}
	})

	expectedTODO := types.TODO{
		Filename: "foo.go",
		Line:     2,
		Comment:  "// TODO: add bar",
		Type:     "TODO",
	}

	t.Run("FetchChangedFileContents error logs warning and continues", func(t *testing.T) {
		s := &stubFetcher{
			diff:     sampleDiff,
			files:    nil,
			filesErr: errors.New("contents failed"),
		}
		var todos []types.TODO
		var err error
		stderrOut := captureStderr(t, func() {
			todos, err = CollectTODOs(s, "o/r", "2", defaultTypes)
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stderrOut, "Warning: could not fetch changed file contents") {
			t.Fatalf("expected warning on stderr, got %q", stderrOut)
		}
		if !reflect.DeepEqual(todos, []types.TODO{expectedTODO}) {
			t.Fatalf("todos = %#v, expected %#v", todos, []types.TODO{expectedTODO})
		}
	})

	t.Run("success with files", func(t *testing.T) {
		s := &stubFetcher{
			diff:  sampleDiff,
			files: map[string][]byte{"foo.go": []byte("package foo\n// TODO: add bar\n")},
		}
		var (
			todos []types.TODO
			err   error
		)
		stderrOut := captureStderr(t, func() {
			todos, err = CollectTODOs(s, "o/r", "3", defaultTypes)
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stderrOut != "" {
			t.Fatalf("stderr = %q, expected empty", stderrOut)
		}
		if !reflect.DeepEqual(todos, []types.TODO{expectedTODO}) {
			t.Fatalf("todos = %#v, expected %#v", todos, []types.TODO{expectedTODO})
		}
		if s.gotRepoFD != "o/r" || s.gotPRFD != "3" {
			t.Fatalf("FetchDiff received repo=%q pr=%q", s.gotRepoFD, s.gotPRFD)
		}
		if s.gotRepoFC != "o/r" || s.gotPRFC != "3" {
			t.Fatalf("FetchChangedFileContents received repo=%q pr=%q", s.gotRepoFC, s.gotPRFC)
		}
		if s.gotDiffFC != sampleDiff {
			t.Fatalf("FetchChangedFileContents received diff %q", s.gotDiffFC)
		}
	})

	t.Run("custom marker type flows through collection", func(t *testing.T) {
		diff := "diff --git a/security.go b/security.go\n" +
			"index 0000000..1111111 100644\n" +
			"--- a/security.go\n" +
			"+++ b/security.go\n" +
			"@@ -1,1 +1,2 @@\n" +
			" package security\n" +
			"+// SECURITY: review token handling\n"
		s := &stubFetcher{
			diff:  diff,
			files: map[string][]byte{"security.go": []byte("package security\n// SECURITY: review token handling\n")},
		}
		todos, err := CollectTODOs(s, "o/r", "4", []string{"TODO", "SECURITY"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []types.TODO{{
			Filename: "security.go",
			Line:     2,
			Comment:  "// SECURITY: review token handling",
			Type:     "SECURITY",
		}}
		if !reflect.DeepEqual(todos, want) {
			t.Fatalf("todos = %#v, expected %#v", todos, want)
		}
	})
}
