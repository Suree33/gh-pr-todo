// Package github wraps the `gh` CLI to fetch pull request diffs and file contents.
package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/Suree33/gh-pr-todo/internal"
	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/cli/go-gh/v2"
)

type PRFetcher interface {
	FetchDiff(repo, pr string) (string, error)
	FetchChangedFileContents(repo, pr, diffOutput string) (map[string][]byte, error)
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
	stdOut, stdErr, err := gh.Exec(args...)
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

func (c *Client) FetchChangedFileContents(repo, pr, diffOutput string) (map[string][]byte, error) {
	args := []string{"pr", "view", "--json", "headRefOid,headRepository"}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	if pr != "" {
		args = append(args, pr)
	}
	stdOut, _, err := gh.Exec(args...)
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

	paths := internal.ExtractChangedPaths(diffOutput)
	files := make(map[string][]byte, len(paths))
	var failedPaths []string
	for _, p := range paths {
		segments := strings.Split(p, "/")
		for i, s := range segments {
			segments[i] = url.PathEscape(s)
		}
		apiPath := fmt.Sprintf("repos/%s/contents/%s?ref=%s", nwo, strings.Join(segments, "/"), sha)
		out, _, err := gh.Exec("api", apiPath, "-H", "Accept: application/vnd.github.raw+json")
		if err != nil {
			failedPaths = append(failedPaths, p)
			continue
		}
		files[p] = out.Bytes()
	}
	if len(failedPaths) > 0 {
		return files, fmt.Errorf("failed to fetch %d changed file(s)", len(failedPaths))
	}

	return files, nil
}

func CollectTODOs(fetcher PRFetcher, repo, pr string) ([]types.TODO, error) {
	diffOutput, err := fetcher.FetchDiff(repo, pr)
	if err != nil {
		return nil, err
	}

	files, err := fetcher.FetchChangedFileContents(repo, pr, diffOutput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not fetch changed file contents; falling back to diff-only parsing where needed: %v\n", err)
	}
	if files == nil {
		files = make(map[string][]byte)
	}

	return internal.ParseDiffWithContents(diffOutput, files), nil
}
