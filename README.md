# gh-pr-todo

[![CI](https://github.com/Suree33/gh-pr-todo/actions/workflows/ci.yml/badge.svg)](https://github.com/Suree33/gh-pr-todo/actions/workflows/ci.yml)
[![Downloads](https://img.shields.io/github/downloads/Suree33/gh-pr-todo/total?label=Downloads&color=blue)](https://github.com/Suree33/gh-pr-todo/releases)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/Suree33/gh-pr-todo)

A GitHub CLI extension that extracts TODO-style comments from pull request diffs, helping you track action items and reminders in your code changes.

## Features

- **Syntax-Aware Detection**: Uses Tree-sitter for accurate TODO-style comment detection in supported languages, with regex fallback for others
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

You can specify PR numbers, URLs, or branches, and you can target another repository with `-R/--repo` or a PR URL:

```bash
# Specify a specific PR number
gh pr-todo 123

# Specify a PR from a different repository
gh pr-todo 456 -R owner/repo

# Specify a PR by URL
gh pr-todo https://github.com/owner/repo/pull/789

# Specify a branch
gh pr-todo feature-branch

# Specify a branch from a different repository
gh pr-todo feature-branch -R owner/repo

# Display only names of the files containing TODO-style comments
gh pr-todo --name-only

# Display only the number of TODO-style comments
gh pr-todo -c

# Group TODO-style comments by file (or type)
gh pr-todo --group-by file

# Override severities for one or more TODO types
# Format: --severity LEVEL=TYPE[,TYPE...]
gh pr-todo --severity warning=TODO,HACK --severity error=FIXME
```

### Command Options

- `[<number> | <url> | <branch>]`: Specify a PR by number, URL, or branch name
- `-R, --repo [HOST/]OWNER/REPO`: Select another repository using the [HOST/]OWNER/REPO format (requires a PR number, URL, or branch argument)
- `--group-by`: Group TODO-style comments by `file` or `type`
- `--name-only`: Display only names of the files containing TODO-style comments
- `-c, --count`: Display only the number of TODO-style comments
- `--severity LEVEL=TYPE[,TYPE...]`: Override severity for one or more TODO types; repeatable, whitespace-tolerant, and last assignment wins for duplicate types
- `-h, --help`: Display help information
- `--no-ci-fail`: Disable non-zero exit when error-level TODOs are found in CI (see below)

### Configuration File

Severity overrides can be persisted in YAML configuration files. Config files use the following schema:

```yaml
severity:
  notice|warning|error: [TYPE...]
```

Example (`.github/gh-pr-todo.yml`):

```yaml
severity:
  warning:
    - TODO
  error:
    - FIXME
```

#### Config File Paths

Config files are loaded from these locations in order of increasing precedence:

1. User config dir `gh-pr-todo/config.yml` (global, shared across all repos; usually `~/.config/gh-pr-todo/config.yml` on Linux)
2. `<repo>/.gh-pr-todo.yml` (repository root)
3. `<repo>/.github/gh-pr-todo.yml` (narrower repo scope)
4. CLI `--severity` flag (highest priority)

Each file's severity overrides are merged into the previous level. A more specific file overrides the same keys from a broader file. Unspecified keys keep their previous value.

Configured TODO types are automatically detected in PR diffs alongside the built-in types. You can define custom types like `SECURITY` or `PERF` and they will be recognized.

#### Remote Config Precedence

When targeting another repository with `--repo` or a PR URL, local repository config files are replaced by remote configuration files with the following precedence (later wins):

1. Global local config (user config dir `gh-pr-todo/config.yml`)
2. Remote default branch config
3. Remote PR base branch config
4. Remote PR head branch config
5. CLI `--severity` flag (highest priority)

For each remote scope, both `.gh-pr-todo.yml` and `.github/gh-pr-todo.yml` are checked (the narrower path wins within each scope).

### CI Mode

When the `CI` environment variable is truthy (e.g. `1`, `true`, parsed via Go's `strconv.ParseBool`), `gh pr-todo` exits with status `1` if any **error-level** TODO-style comments are detected in the PR diff. By default, no built-in keyword type is mapped to error-level, so `gh pr-todo` does **not** fail CI based on default keywords alone. Use configuration files or `--severity` to promote recognized TODO keywords to `error` when you want CI failures, for example `--severity error=FIXME`. `GITHUB_ACTIONS=true` (set by the GitHub Actions runner) is treated as `CI=true` even when `CI` is missing or falsy.

```yaml
# GitHub Actions example — CI=true is set automatically
- run: gh pr-todo ${{ github.event.pull_request.number }}
```

Pass `--no-ci-fail` to suppress non-zero exit even when error-level TODOs exist:

```bash
gh pr-todo --count --severity error=FIXME --no-ci-fail
```

### GitHub Actions Annotations

When `GITHUB_ACTIONS=true` (set automatically by the GitHub Actions runner), `gh pr-todo` additionally emits [workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands) so each TODO appears as an annotation on the workflow run and pull request:

Default annotation severities:

- `TODO`, `NOTE` → `::notice` annotations
- `FIXME`, `HACK`, `XXX`, `BUG` → `::warning` annotations

You can override them with configuration files or `--severity`, for example:

- `--severity warning=TODO,NOTE`
- `--severity error=FIXME`

Annotations reflect the resolved severity of each keyword and are independent of CI exit behavior: warning annotations are displayed but do **not** cause a non-zero exit by default. Only error-level TODOs cause CI failure.

Each annotation is anchored to the file and line of the TODO, with the keyword used as the annotation title. Regular human-readable output is still printed, and the spinner is suppressed to keep Actions logs clean.

Workflow commands are only emitted in the default mode. The machine-readable modes `--count` and `--name-only` keep their plain output unchanged so that `count=$(gh pr-todo --count)` and similar shell pipelines stay reliable in Actions.

### Example Output

```
✔ Fetching PR diff...

Found 3 TODO-style comment(s)

* src/api/users.go:42
  // TODO: Add input validation for email format

* components/Header.tsx:15
  // FIXME: Memory leak in event listener cleanup

* docs/setup.md:8
  <!-- NOTE: Update this section after v2.0 release -->
```

## Supported Comment Formats

The tool recognizes TODO-style comments in various formats:

| Format              | Example                              |
| ------------------- | ------------------------------------ |
| **C-style**         | `// TODO: Fix this bug`              |
| **C-style block**   | `/* HACK: Quick fix for demo */`     |
| **Shell/Python**    | `# FIXME: Optimization needed`       |
| **HTML/XML**        | `<!-- NOTE: Review this section -->` |
| **Assembly/Config** | `; XXX: Temporary workaround`        |

## Supported Keywords

### Default Keywords

- `TODO`
- `FIXME`
- `HACK`
- `NOTE`
- `XXX`
- `BUG`

### Custom Keywords

Additional keywords can be defined via [configuration files](#configuration-file) or the `--severity` CLI flag. Any custom type assigned a severity will be detected in PR diffs alongside the default keywords. For example:

```yaml
# .github/gh-pr-todo.yml
severity:
  error:
    - SECURITY
  warning:
    - PERF
```

This configures `SECURITY` and `PERF` as recognized TODO markers.

#### Validation Rules

- Severity keys (`notice`, `warning`, `error`) are case-insensitive.
- Empty lists (`warning: []`) are allowed and ignored.
- A TODO type must not appear under multiple severity levels in the same file.
- The old `TYPE: level` format is not supported.

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
│   ├── config/
│   │   ├── config.go    # YAML config parsing and local loading
│   │   └── remote.go    # Remote config loading
│   ├── github/
│   │   └── client.go    # GitHub API client (diffs, file contents, remote config)
│   ├── output/
│   │   ├── printer.go   # Terminal output rendering
│   │   └── workflow.go  # GitHub Actions annotation commands
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
