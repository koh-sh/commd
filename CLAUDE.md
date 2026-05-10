# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

commd is a Go CLI tool for reviewing Markdown files in an interactive TUI. It parses Markdown files, displays them in a 2-pane interface, and supports inline commenting with feedback integration via Claude Code hooks.

## Absolute Rule

**`mise run ci` must always pass.** Before finishing any code change, run `mise run ci` and confirm all steps succeed. This is non-negotiable — do not leave the codebase in a state where `mise run ci` fails.

`mise run ci` runs: `fmt` → `fix` → `lint` → `build` → `cov` (test with coverage) → `e2e-basic` (basic E2E tests). If any step fails, fix it before considering the task complete. Run `mise run vuln` separately to check for known vulnerabilities (`go mod verify` + `govulncheck`).

## Build & Test Commands

Dev tools (Go, golangci-lint, tparse, gofumpt, octocov, goreleaser, bun) are managed by mise (`.mise.toml`). Run `mise install` to set up the toolchain.

```bash
mise run ci                             # Run full CI pipeline (MUST pass)
mise run build                          # Build binary
mise run test                           # Run all tests with tparse
mise run e2e                            # Run all E2E tests (full suite, manual)
mise run e2e-basic                      # Run basic E2E tests (included in ci)
mise run lint                           # Run golangci-lint with --fix
mise run fmt                            # Format with gofumpt
mise run fix                            # Run go fix (modernize)
mise run tidy                           # Run go mod tidy -v
mise run vuln                           # Run go mod verify + govulncheck
mise run install-skills                 # Install tuistory Claude Code skill
go test -v ./internal/markdown           # Run tests for a specific package
go test -run TestParsePreamble ./internal/markdown  # Run a single test
```

`mise run e2e` builds the binary and runs all E2E tests in `e2e/` using bun + tuistory (full suite). `mise run e2e-basic` runs only critical-path E2E tests and is included in `mise run ci`.

Linter config: `.golangci.yml` (enabled: asciicheck, gocritic, misspell, nolintlint, predeclared, unconvert; formatters: gci, gofumpt).

## Architecture

### Entry Point & CLI

`main.go` → `cmd/cli.go`: Kong struct-based CLI with 5 subcommands (review, pr, cclocate, cchook, version). Cross-field constraints are enforced via `Validate()` methods on each command struct (called by Kong post-parse). The GitHub client is wired with `kong.BindToProvider(ghclient.NewClient)` and lazily injected only into commands that take `*ghclient.Client` in their `Run()` signature (currently `PRCmd`).

### Package Layout

- **`internal/markdown/`** — Core domain. Markdown parsing via goldmark AST (not regex, to avoid `#` in code blocks being misinterpreted as headings). Data models (`Document`, `Section`, `ReviewComment`), review output formatting.
- **`internal/tui/`** — Bubble Tea TUI. 2-pane layout: `SectionList` (left) + `DetailPane` (right). Mode-based state machine: `ModeNormal` → `ModeComment` → `ModeCommentList` → `ModeConfirm` → `ModeHelp` → `ModeSearch`.
- **`internal/cclocate/`** — Plan file discovery from Claude Code transcript JSONL files. `plansDirectory` resolution chain: `.claude/settings.local.json` → `.claude/settings.json` → `~/.claude/settings.json` → `~/.claude/plans/`.
- **`internal/cchook/`** — PostToolUse hook orchestration. Parses stdin JSON from Claude Code, validates `permission_mode == "plan"`, spawns review in a pane, returns exit code 0 (continue) or 2 (feedback).
- **`internal/github/`** — GitHub API client for PR operations via `google/go-github`. PR URL parsing (`ParsePRURL`), changed file listing (`ListMDFiles`), content fetching (`FetchFileContent`), and PR review submission (`SubmitReview`, `BuildPRReview`). Comment mapping from commd's `ReviewComment` to GitHub's `DraftReviewComment`.
- **`internal/pane/`** — Terminal multiplexer abstraction. `PaneSpawner` interface with WezTerm and Direct (fallback) implementations. tmux is stub only (`ByName("tmux")` returns DirectSpawner). `AutoDetect()` tries WezTerm → Direct. WezTerm spawner uses pixel dimensions for split direction (right 50% or bottom 80%).

### Key Data Flow

**Review**: `cmd/review.go` → `markdown.Parse(source)` → `tui.NewApp(doc)` → Bubble Tea loop → `markdown.FormatReview(result.Review, doc, filePath)` → clipboard/file/stdout

**PR Review**: `cmd/pr.go` → `PRCmd.Validate()` (parses URL) → `Run(client)` (client injected by Kong via `BindToProvider`) → `github.ListMDFiles()` → file picker (multi-select) → for each file: `github.FetchFileContent()` → `markdown.Parse()` → `tui.NewApp()` → `github.BuildPRReview()` → `github.SubmitReview()`

**Hook**: Claude Code (PostToolUse: Write|Edit) → stdin JSON → `cchook.Run()` → `pane.SpawnAndWait(commd review ...)` → temp file IPC → exit 0 or 2

## Development Rules

- Use `go doc` to look up library APIs and usage (more accurate than web searches)
- Do not use Python for complex scripting. Use shell scripts instead
- Do not use `/tmp`. Keep temporary files within the project directory

## Implementation Guidelines

### Code

- Follow Go idioms and best practices
- Keep design style consistent across subcommands (Kong struct tags, error handling patterns, etc.)
- Follow the DRY principle and eliminate duplicate code
- Keep package and function responsibilities single-purpose. Do not mix multiple responsibilities
- Keep functions unexported (lowercase) unless they are referenced externally
- Use function and variable names that clearly represent their responsibilities
- Add comments for code or logic that is not immediately obvious

### Tests

- Write all tests in table-driven test format
- Do not write meaningless tests that only exist to inflate coverage
- When implementing or modifying features, add E2E tests in `e2e/tests/`
- Consider the feature's criticality and add to basic tier (`.mise.toml` `e2e-basic` task) if appropriate

### Documentation

- Keep the README clear and concise for users
- Update the README when code specifications change

## Key Design Decisions

- **goldmark AST walk** for Markdown parsing instead of regex to correctly handle `#` inside code fences
- **Mermaid → ASCII rendering** via `mermaid-ascii` library; fenced `\`\`\`mermaid` blocks are converted to ASCII art in the detail pane, with fallback to raw source on error
- **Content-hash based viewed tracking** (`state.go`): sections are tracked by title → SHA-256 hash of title+body; if content changes, the viewed mark auto-clears
- **PostToolUse (Write|Edit) hook** instead of Stop hook, because Stop doesn't fire during plan mode
- **Temp files for IPC** between hook process and review subprocess (which runs in a separate pane)
- **Hook always exits 0 on error** — never breaks the Claude Code workflow; uses `CC_PLAN_REVIEW_SKIP=1` env var to disable

## Testing Patterns

### Go Unit Tests

- Golden file tests using `testdata/` directories (e.g., `internal/markdown/testdata/basic.md`, `code-block-hash.md`)
- Test helper `readTestdata(t, name)` for loading fixtures
- TUI tests validate render bounds, scroll behavior, and cursor position

### E2E Tests

E2E tests in `e2e/` use tuistory (terminal UI automation) with bun:test:

- `launchCommd()` helper spawns the built binary in a virtual terminal and waits for Bubble Tea to render
- Each test runs with a fixed terminal size (120×36 by default)
- Snapshot tests (`snapshot.test.ts`) capture full screen state for layout regression detection
- Fixtures reuse `internal/markdown/testdata/basic.md`
- `addComment()` shared helper for comment creation sequences
