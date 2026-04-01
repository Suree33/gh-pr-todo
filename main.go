package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Suree33/gh-pr-todo/internal"
	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/briandowns/spinner"
	"github.com/cli/go-gh/v2"
	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

var (
	bold    = color.New(color.Bold).SprintFunc()
	green   = color.New(color.FgGreen).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	blue    = color.New(color.FgBlue).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
)

func main() {
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
	pflag.Usage = func() {
		fmt.Fprintf(color.Output, "%s\n\n", "View TODO comments in the PR diff.")
		fmt.Fprintf(color.Output, "%s\n", bold("USAGE"))
		fmt.Fprintf(color.Output, "  %s\n\n", "gh pr-todo [<number> | <url> | <branch>] [flags]")
		fmt.Fprintf(color.Output, "%s\n", bold("FLAGS"))
		maxLen := 0
		pflag.VisitAll(func(f *pflag.Flag) {
			nameLen := len(f.Name)
			if f.Shorthand != "" {
				nameLen += len(f.Shorthand) + 2
			}
			if nameLen > maxLen {
				maxLen = nameLen
			}
		})
		pflag.VisitAll(func(f *pflag.Flag) {
			if f.Shorthand != "" {
				fmt.Fprintf(color.Output, "  -%-*s, --%-*s %s\n", len(f.Shorthand), f.Shorthand, maxLen+2, f.Name, f.Usage)
			} else {
				fmt.Fprintf(color.Output, "      --%-*s %s\n", maxLen+2, f.Name, f.Usage)
			}
		})
		fmt.Println()
	}
	pflag.Parse()
	args := pflag.Args()

	if isHelp {
		pflag.Usage()
		os.Exit(0)
	}

	var pr string
	switch len(args) {
	case 0:
		if repo != "" {
			fmt.Fprintf(color.Output, "%s%s\n", red("✗"), " PR number, branch, or URL required when specifying repository\n")
			pflag.Usage()
			os.Exit(1)
		}
		pr = ""
	case 1:
		pr = args[0]
	default:
		fmt.Fprintf(color.Output, "%s%s\n", red("✗"), " Too many arguments\n")
		pflag.Usage()
		os.Exit(1)
	}

	if nameOnly {
		runNameOnly(repo, pr)
	} else if isCount {
		runCount(repo, pr)
	} else {
		runMain(repo, pr, groupBy)
	}
}

func collectTODOs(repo, pr string) ([]types.TODO, error) {
	stdOut, stdErr, err := fetchPRDiff(repo, pr)
	if err != nil {
		if msg := strings.TrimSpace(stdErr.String()); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, err
	}
	if stdErr.Len() > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", stdErr.String())
	}

	diffOutput := stdOut.String()
	files, err := fetchChangedFileContents(repo, pr, diffOutput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not fetch changed file contents; falling back to diff-only parsing where needed: %v\n", err)
	}
	if files == nil {
		files = make(map[string][]byte)
	}

	return internal.ParseDiffWithContents(diffOutput, files), nil
}

func runMain(repo string, pr string, groupBy types.GroupBy) {
	sp := spinner.New(spinner.CharSets[14], 40*time.Millisecond)
	fetchingMsg := " Fetching PR diff..."
	sp.Suffix = fetchingMsg
	sp.Start()

	todos, err := collectTODOs(repo, pr)
	sp.Stop()

	if err == nil {
		fmt.Fprintf(color.Output, "%s%s\n", green("✔"), fetchingMsg)
	} else {
		fmt.Fprintf(color.Output, "%s%s\n", red("✗"), fetchingMsg)
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if len(todos) == 0 {
		fmt.Fprintf(color.Output, "\nNo TODO comments found in the diff.\n")
		return
	}

	fmt.Fprintf(color.Output, bold("\nFound %d TODO comment(s)\n\n"), len(todos))
	switch groupBy {
	case types.GroupByNone:
		for _, todo := range todos {
			fmt.Fprintf(color.Output, "* %s\n", blue(todo.Filename+":"+strconv.Itoa(todo.Line)))
			fmt.Fprintf(color.Output, "  %s\n\n", todo.Comment)
		}
	case types.GroupByFile:
		files := make(map[string][]types.TODO)
		maxLineNumberLen := 0
		for _, todo := range todos {
			files[todo.Filename] = append(files[todo.Filename], todo)
			if len(strconv.Itoa(todo.Line)) > maxLineNumberLen {
				maxLineNumberLen = len(strconv.Itoa(todo.Line))
			}
		}
		for filename, todos := range files {
			fmt.Fprintf(color.Output, "* %s\n", blue(filename))
			for _, todo := range todos {
				fmt.Fprintf(color.Output, "  %s%s: %s\n", strings.Repeat(" ", maxLineNumberLen-int(len(strconv.Itoa(todo.Line)))), green(strconv.Itoa(todo.Line)), todo.Comment)
			}
			fmt.Println()
		}
	case types.GroupByType:
		todoTypes := make(map[string][]types.TODO)
		for _, todo := range todos {
			todoTypes[todo.Type] = append(todoTypes[todo.Type], todo)
		}
		todoTypeKeys := slices.Collect(maps.Keys(todoTypes))
		slices.Sort(todoTypeKeys)
		for _, todoType := range todoTypeKeys {
			todos := todoTypes[todoType]
			fmt.Fprintf(color.Output, "%s%s%s\n", bold("["), bold(magenta(todoType)), bold("]"))
			for _, todo := range todos {
				fmt.Fprintf(color.Output, "* %s\n", blue(todo.Filename+":"+strconv.Itoa(todo.Line)))
				fmt.Fprintf(color.Output, "  %s\n\n", todo.Comment)
			}
		}
	}
}

func runCount(repo string, pr string) {
	todos, err := collectTODOs(repo, pr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	fmt.Fprintln(color.Output, len(todos))
}

func runNameOnly(repo string, pr string) {
	todos, err := collectTODOs(repo, pr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if len(todos) == 0 {
		return
	}

	files := make(map[string]struct{})
	for _, todo := range todos {
		files[todo.Filename] = struct{}{}
	}

	for file := range files {
		fmt.Fprintln(color.Output, file)
	}
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

func fetchChangedFileContents(repo, pr, diffOutput string) (map[string][]byte, error) {
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

	paths := extractChangedPaths(diffOutput)
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

func extractChangedPaths(diffOutput string) []string {
	var paths []string
	seen := make(map[string]struct{})
	for _, line := range strings.Split(diffOutput, "\n") {
		if after, ok := strings.CutPrefix(line, "+++ b/"); ok {
			p := path.Clean(after)
			if _, exists := seen[p]; !exists {
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
	}
	return paths
}

func fetchPRDiff(repo, pr string) (bytes.Buffer, bytes.Buffer, error) {
	args := []string{"pr", "diff"}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	if pr != "" {
		args = append(args, pr)
	}
	stdOut, stdErr, err := gh.Exec(args...)
	return stdOut, stdErr, err
}
