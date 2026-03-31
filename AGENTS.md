# AGENTS.md

## Architecture

This is a GitHub CLI extension (`gh pr-todo`) that extracts TODO/FIXME/HACK/NOTE/XXX/BUG comments from PR diffs.

- `main.go` — CLI entrypoint; uses `pflag` for flags, `go-gh/v2` to call `gh pr diff`, `spinner`/`color` for terminal UI.
- `internal/parser.go` — `ParseDiff()` parses unified diff output and extracts TODO comments via regex.
- `pkg/types/` — shared types: `TODO` struct and `GroupBy` enum (implements `pflag.Value`).

## Build / Lint / Test

- Build: `go build -v ./...`
- Run `main.go`: `go run .` (or `go run . --help` to show usage)
- Lint: `golangci-lint run` (config: `.golangci.toml`, golangci-lint v2, formatters: gofmt)
- Test all: `go test -v ./...`
- Test single: `go test -v -run TestName ./internal/` (only `internal/` has tests)

## Code Style

- Go: use stdlib over third-party helpers.
- Imports: stdlib block, then third-party, separated by blank line.
- Formatting: `gofmt`. No comments unless explaining non-obvious logic.
- Error handling: print to stderr and return early; no `log.Fatal` or panics.
- Tests: table-driven with `t.Run`, use `reflect.DeepEqual` for struct comparison.
- Naming: exported types in `pkg/types/`, unexported helpers in `internal/`.
