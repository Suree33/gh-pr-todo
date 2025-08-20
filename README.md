# gh-pr-todo

A GitHub CLI extension that extracts TODO comments from pull request diffs, helping you track action items and reminders in your code changes.

## ✨ Features

- 🔍 **Smart Detection**: Finds TODO, FIXME, HACK, NOTE, XXX, and BUG comments
- 🎨 **Beautiful Output**: Colorized terminal output with loading indicators
- 📋 **Multiple Formats**: Supports various comment styles (`//`, `#`, `<!--`, `;`, `/*`, markdown lists)
- ⚡ **Fast**: Efficient diff parsing with GitHub CLI integration
- 🎯 **PR-Focused**: Only shows comments from your current changes

## 🚀 Installation

```bash
gh ext install Suree33/gh-pr-todo
```

**Prerequisites:**
- [GitHub CLI](https://cli.github.com/) installed and authenticated
- Go 1.20.0 or later

## 📖 Usage

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
```

### Command Options

- `-R, --repo [HOST/]OWNER/REPO`: Select another repository using the [HOST/]OWNER/REPO format
- `[<number> | <url> | <branch>]`: Specify a PR by number, URL, or branch name

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

## 🔧 Supported Comment Formats

The tool recognizes TODO-style comments in various formats:

| Format | Example |
|--------|---------|
| **C-style** | `// TODO: Fix this bug` |
| **C-style block** | `/* HACK: Quick fix for demo */` |
| **Shell/Python** | `# FIXME: Optimization needed` |
| **HTML/XML** | `<!-- NOTE: Review this section -->` |
| **Assembly/Config** | `; XXX: Temporary workaround` |

## 🏗️ Supported Keywords

- `TODO`
- `FIXME`
- `HACK`
- `NOTE`
- `XXX`
- `BUG`

## 🛠️ Development

### Building from Source

```bash
git clone https://github.com/Suree33/gh-pr-todo.git
cd gh-pr-todo
go build -o gh-pr-todo main.go
```

### Project Structure

```
├── main.go              # CLI entry point
├── internal/
│   └── parser.go        # Diff parsing logic
├── pkg/
│   └── types/
│       └── todo.go      # TODO type definitions
└── scripts/             # Build scripts (if any)
```

### Dependencies

- [GitHub CLI Go library](https://github.com/cli/go-gh) - GitHub CLI integration
- [Spinner](https://github.com/briandowns/spinner) - Loading animations  
- [Color](https://github.com/fatih/color) - Terminal colors

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

## 🐛 Issues & Feature Requests

Found a bug or have a feature idea? Please open an issue on [GitHub](https://github.com/Suree33/gh-pr-todo/issues).
