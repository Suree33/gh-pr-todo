// Package github wraps the `gh` CLI and GitHub REST API to fetch pull request diffs and file contents.
package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Suree33/gh-pr-todo/internal"
	"github.com/Suree33/gh-pr-todo/internal/config"
	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/api"
)

var ghExec func(args ...string) (bytes.Buffer, bytes.Buffer, error) = gh.Exec

const maxConcurrentFileFetches = 8

const rawContentsAccept = "application/vnd.github.raw+json"

type restClient interface {
	Request(method, path string, body io.Reader) (*http.Response, error)
}

var newRESTClient = func(host string) (restClient, error) {
	options := api.ClientOptions{
		Headers:      map[string]string{"Accept": rawContentsAccept},
		LogIgnoreEnv: true,
	}
	if host != "" {
		options.Host = host
	}
	return api.NewRESTClient(options)
}

type PRFetcher interface {
	FetchDiff(repo, pr string) (string, error)
	FetchChangedFileContents(repo, pr, diffOutput string, todoTypes []string) (map[string][]byte, error)
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

type prMeta struct {
	HeadRefOid     string `json:"headRefOid"`
	HeadRepository struct {
		NameWithOwner string `json:"nameWithOwner"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	} `json:"headRepository"`
}

// Remote config fetch methods

// FetchRemoteConfigRefs returns repository and PR refs for remote config loading.
func (c *Client) FetchRemoteConfigRefs(repo, pr string) (config.RemoteConfigRefs, error) {
	refs := config.RemoteConfigRefs{}
	host, _ := splitHostRepo(repo)

	// Get repo default branch info
	args := []string{"repo", "view"}
	if repo != "" {
		args = append(args, repo)
	}
	args = append(args, "--json", "defaultBranchRef,nameWithOwner")
	out, _, err := ghExec(args...)
	if err != nil {
		return refs, err
	}

	var repoView struct {
		DefaultBranchRef struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
		NameWithOwner string `json:"nameWithOwner"`
	}
	if err := json.Unmarshal(out.Bytes(), &repoView); err != nil {
		return refs, err
	}
	refs.DefaultBranchRef = repoView.DefaultBranchRef.Name
	refs.DefaultRepo = withHost(host, repoView.NameWithOwner)

	// Get PR info if specified
	if pr != "" {
		args := []string{"pr", "view", "--json", "baseRefName,headRefOid,headRepository"}
		if repo != "" {
			args = append(args, "-R", repo)
		}
		args = append(args, pr)
		out, _, err := ghExec(args...)
		if err != nil {
			return refs, err
		}

		var prView struct {
			BaseRefName    string `json:"baseRefName"`
			HeadRefOid     string `json:"headRefOid"`
			HeadRepository struct {
				NameWithOwner string `json:"nameWithOwner"`
			} `json:"headRepository"`
		}
		if err := json.Unmarshal(out.Bytes(), &prView); err != nil {
			return refs, err
		}
		refs.BaseBranchRef = prView.BaseRefName
		refs.BaseRepo = refs.DefaultRepo
		refs.HeadRefOid = prView.HeadRefOid
		refs.HeadRepo = withHost(host, prView.HeadRepository.NameWithOwner)
	}

	return refs, nil
}

func (c *Client) fetchRawFileContent(client restClient, repo, path, ref string) (data []byte, err error) {
	_, repoPath := splitHostRepo(repo)
	segments := strings.Split(path, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	apiPath := fmt.Sprintf("repos/%s/contents/%s?ref=%s", repoPath, strings.Join(segments, "/"), url.QueryEscape(ref))
	response, err := client.Request(http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, err
	}
	if response == nil || response.Body == nil {
		return nil, fmt.Errorf("REST client returned an empty response")
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing REST response body: %w", closeErr))
		}
	}()
	return io.ReadAll(response.Body)
}

func isRESTNotFound(err error) bool {
	var httpErr *api.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound
}

// FetchFileAtRef fetches a file from the repository at a specific ref.
// Returns (nil, false, nil) when the file is not found (404).
func (c *Client) FetchFileAtRef(repo, path, ref string) ([]byte, bool, error) {
	host, _ := splitHostRepo(repo)
	client, err := newRESTClient(host)
	if err != nil {
		return nil, false, fmt.Errorf("fetching %s from %s at %s: %w", path, repo, ref, err)
	}

	data, err := c.fetchRawFileContent(client, repo, path, ref)
	if err != nil {
		if isRESTNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("fetching %s from %s at %s: %w", path, repo, ref, err)
	}
	return data, true, nil
}

func splitHostRepo(repo string) (string, string) {
	parts := strings.Split(repo, "/")
	if len(parts) == 3 {
		return parts[0], parts[1] + "/" + parts[2]
	}
	return "", repo
}

func withHost(host, repo string) string {
	if host == "" || repo == "" {
		return repo
	}
	return host + "/" + repo
}

func (m prMeta) headRepositoryNameWithOwner() string {
	if m.HeadRepository.NameWithOwner != "" {
		return m.HeadRepository.NameWithOwner
	}
	if m.HeadRepository.Owner.Login == "" || m.HeadRepository.Name == "" {
		return ""
	}
	return m.HeadRepository.Owner.Login + "/" + m.HeadRepository.Name
}

func (c *Client) FetchDiff(repo, pr string) (string, error) {
	args := []string{"pr", "diff"}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	if pr != "" {
		args = append(args, pr)
	}
	stdOut, stdErr, err := ghExec(args...)
	if err != nil {
		if msg := strings.TrimSpace(stdErr.String()); msg != "" {
			return "", fmt.Errorf("%s", msg)
		}
		return "", err
	}
	if stdErr.Len() > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", stdErr.String())
	}
	return stdOut.String(), nil
}

func (c *Client) FetchChangedFileContents(repo, pr, diffOutput string, todoTypes []string) (map[string][]byte, error) {
	paths := internal.ExtractPathsRequiringContents(diffOutput, todoTypes)
	if len(paths) == 0 {
		return make(map[string][]byte), nil
	}

	host, _ := splitHostRepo(repo)
	args := []string{"pr", "view", "--json", "headRefOid,headRepository"}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	if pr != "" {
		args = append(args, pr)
	}
	stdOut, _, err := ghExec(args...)
	if err != nil {
		return nil, err
	}

	var meta prMeta
	if err := json.Unmarshal(stdOut.Bytes(), &meta); err != nil {
		return nil, err
	}

	nwo := meta.headRepositoryNameWithOwner()
	sha := meta.HeadRefOid
	if nwo == "" || sha == "" {
		return nil, fmt.Errorf("could not determine PR head")
	}

	files := make(map[string][]byte, len(paths))
	if len(paths) == 0 {
		return files, nil
	}

	client, err := newRESTClient(host)
	if err != nil {
		return files, fmt.Errorf("creating GitHub REST client: %w", err)
	}

	var failedPaths []string
	type fetchResult struct {
		path string
		data []byte
		err  error
	}
	results := make(chan fetchResult, len(paths))
	semaphore := make(chan struct{}, maxConcurrentFileFetches)
	for _, p := range paths {
		go func(path string) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			data, err := c.fetchRawFileContent(client, withHost(host, nwo), path, sha)
			results <- fetchResult{path: path, data: data, err: err}
		}(p)
	}
	for range paths {
		result := <-results
		if result.err != nil {
			failedPaths = append(failedPaths, result.path)
			continue
		}
		files[result.path] = result.data
	}
	if len(failedPaths) > 0 {
		return files, fmt.Errorf("failed to fetch %d changed file(s)", len(failedPaths))
	}

	return files, nil
}

// CollectTODOs fetches and parses TODOs from a PR diff using the given
// fetcher and the specified TODO marker types.
func CollectTODOs(fetcher PRFetcher, repo, pr string, todoTypes []string) ([]types.TODO, error) {
	diffOutput, err := fetcher.FetchDiff(repo, pr)
	if err != nil {
		return nil, err
	}

	files, err := fetcher.FetchChangedFileContents(repo, pr, diffOutput, todoTypes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not fetch changed file contents; falling back to diff-only parsing where needed: %v\n", err)
	}
	if files == nil {
		files = make(map[string][]byte)
	}

	return internal.ParseDiffWithContentsAndTypes(diffOutput, files, todoTypes), nil
}
