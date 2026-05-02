# gh-pr-todo

[![CI](https://github.com/Suree33/gh-pr-todo/actions/workflows/ci.yml/badge.svg)](https://github.com/Suree33/gh-pr-todo/actions/workflows/ci.yml)
[![Downloads](https://img.shields.io/github/downloads/Suree33/gh-pr-todo/total?label=Downloads&color=blue)](https://github.com/Suree33/gh-pr-todo/releases)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/Suree33/gh-pr-todo)

A GitHub CLI extension that extracts TODO comments from pull request diffs, helping you track action items and reminders in your code changes.

## Features

- **Syntax-Aware Detection**: Uses Tree-sitter for accurate TODO detection in supported languages, with regex fallback for others
- **Beautiful Output**: Colorized terminal output with loading indicators
- **Multiple Formats**: Supports various comment styles (`//`, `#`, `<!--`, `;`, `/*`)
- **Fast**: Efficient diff parsing with GitHub CLI integration
- **PR-Focused**: Only shows comments from your current changes

## Installation

```bash
gh ext install Suree33/gh-pr-todo
```

**Prerequisites:**
- [GitHub CLI](https://cli.github.com/) installed and authenticated

## Usage

### Basic Usage

Navigate to your repository with an open pull request and run:

```bash
gh pr-todo
```

### Advanced Usage

You can specify different repositories, PR numbers, URLs, or branches just like `gh pr diff`:

```bash
# Specify a different repository
gh pr-todo -R owner/repo

# Specify a specific PR number
gh pr-todo 123

# Specify a PR from a different repository
gh pr-todo 456 -R owner/repo

# Specify a PR by URL
gh pr-todo https://github.com/owner/repo/pull/789

# Specify a branch
gh pr-todo feature-branch

# Display only names of the files containing TODO comments
gh pr-todo --name-only

# Display only the number of TODO comments
gh pr-todo -c

# Group TODO comments by file (or type)
gh pr-todo --group-by file
```

### Command Options

- `[<number> | <url> | <branch>]`: Specify a PR by number, URL, or branch name
- `-R, --repo [HOST/]OWNER/REPO`: Select another repository using the [HOST/]OWNER/REPO format
- `--group-by`: Group TODO comments by file or type
- `--name-only`: Display only names of the files containing TODO comments
- `-c, --count`: Display only the number of TODO comments
- `--no-ci-fail`: Disable non-zero exit when TODOs are found in CI (see below)

### CI Mode

When the `CI` environment variable is set to a truthy value (e.g. `1`, `true`, parsed via Go's `strconv.ParseBool`), `gh pr-todo` exits with status `1` if any TODO-style comments are detected in the PR diff. This makes it easy to fail a CI job when new TODOs slip into a pull request.

```yaml
# GitHub Actions example — CI=true is set automatically
- run: gh pr-todo ${{ github.event.pull_request.number }}
```

Pass `--no-ci-fail` to keep the informational behavior even in CI:

```bash
gh pr-todo --count --no-ci-fail
```

### GitHub Actions Annotations

When `GITHUB_ACTIONS=true` (set automatically by the GitHub Actions runner), `gh pr-todo` additionally emits [workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands) so each TODO appears as an annotation on the workflow run and pull request:

- `TODO`, `NOTE` → `::notice` annotations
- `FIXME`, `HACK`, `XXX`, `BUG` → `::warning` annotations

Each annotation is anchored to the file and line of the TODO, with the keyword used as the annotation title. Regular human-readable output is still printed, and the spinner is suppressed to keep Actions logs clean.

Workflow commands are only emitted in the default mode. The machine-readable modes `--count` and `--name-only` keep their plain output unchanged so that `count=$(gh pr-todo --count)` and similar shell pipelines stay reliable in Actions.

### Example Output

```
✔ Fetching PR diff...

Found 3 TODO comment(s)

* src/api/users.go:42
  // TODO: Add input validation for email format

* components/Header.tsx:15
  // FIXME: Memory leak in event listener cleanup

* docs/setup.md:8
  <!-- NOTE: Update this section after v2.0 release -->
```

## Supported Comment Formats

The tool recognizes TODO-style comments in various formats:

| Format | Example |
|--------|---------|
| **C-style** | `// TODO: Fix this bug` |
| **C-style block** | `/* HACK: Quick fix for demo */` |
| **Shell/Python** | `# FIXME: Optimization needed` |
| **HTML/XML** | `<!-- NOTE: Review this section -->` |
| **Assembly/Config** | `; XXX: Temporary workaround` |

## Supported Keywords

- `TODO`
- `FIXME`
- `HACK`
- `NOTE`
- `XXX`
- `BUG`

## Development

### Building from Source

```bash
git clone https://github.com/Suree33/gh-pr-todo.git
cd gh-pr-todo
go build -o gh-pr-todo .
```

### Project Structure

```
├── main.go              # CLI entry point
├── internal/
│   ├── github/
│   │   └── client.go    # GitHub API client (diffs & file contents)
│   ├── output/
│   │   └── printer.go   # Terminal output rendering
│   └── parser.go        # Diff parsing logic (Tree-sitter + regex)
├── pkg/
│   └── types/
│       ├── groupby.go   # GroupBy enum
│       └── todo.go      # TODO type definitions
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

## Issues & Feature Requests

Found a bug or have a feature idea? Please open an issue on [GitHub](https://github.com/Suree33/gh-pr-todo/issues).
