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
mise run e2e-cov                        # Run all E2E tests and report the web UI's JS coverage
mise run lint                           # Run golangci-lint with --fix
mise run fmt                            # Format with gofumpt
mise run fix                            # Run go fix (modernize)
mise run tidy                           # Run go mod tidy -v
mise run vuln                           # Run go mod verify + govulncheck
mise run install-skills                 # Install tuistory Claude Code skill
go test -v ./internal/markdown           # Run tests for a specific package
go test -run TestParsePreamble ./internal/markdown  # Run a single test
```

`mise run e2e` builds the binary and runs all E2E tests in `e2e/` using bun + tuistory, and Playwright for the web UI (full suite). `mise run e2e-basic` runs only critical-path E2E tests and is included in `mise run ci`.

Linter config: `.golangci.yml` (enabled: asciicheck, gocritic, misspell, nolintlint, predeclared, unconvert; formatters: gci, gofumpt).

## Architecture

### Entry Point & CLI

`main.go` → `cmd/cli.go`: Kong struct-based CLI with 5 subcommands (review, pr, cclocate, cchook, version). Cross-field constraints are enforced via `Validate()` methods on each command struct (called by Kong post-parse). The GitHub client is wired with `kong.BindToProvider(ghclient.NewClient)` and lazily injected only into commands that take `*ghclient.Client` in their `Run()` signature (currently `PRCmd`).

### Package Layout

- **`internal/markdown/`** — Core domain. Markdown parsing via goldmark AST (not regex, to avoid `#` in code blocks being misinterpreted as headings). Data models (`Document`, `Section`, `ReviewComment`), review output formatting.
- **`internal/tui/`** — Bubble Tea TUI. 2-pane layout: `SectionList` (left) + `DetailPane` (right). Mode-based state machine: `ModeNormal` → `ModeComment` → `ModeCommentList` → `ModeConfirm` → `ModeHelp` → `ModeSearch` → `ModeLineSelect` (visual line selection in raw view).
- **`internal/cclocate/`** — Plan file discovery from Claude Code transcript JSONL files. `plansDirectory` resolution chain: `.claude/settings.local.json` → `.claude/settings.json` → `~/.claude/settings.json` → `~/.claude/plans/`.
- **`internal/cchook/`** — Claude Code hook orchestration. Parses stdin JSON, validates `permission_mode == "plan"`, resolves the plan file (`PreToolUse`/`ExitPlanMode`: injected `tool_input.planFilePath`; `PostToolUse`/`Write|Edit`: `file_path` under `plansDirectory`), spawns review in a pane, returns exit code 0 (continue) or 2 (feedback / deny ExitPlanMode).
- **`internal/diff/`** — Unified diff parsing (`ParsePatch`, `StripHeader`, `AddedFilePatch`) shared by PR mode and local diff mode. Source-agnostic.
- **`internal/gitdiff/`** — Local git queries for `review --diff`: changed/untracked `.md` listing and per-file patches via the `git` binary.
- **`internal/github/`** — GitHub API client for PR operations via `google/go-github`. PR URL parsing (`ParsePRURL`), changed file listing (`ListMDFiles`), content fetching (`FetchFileContent`), and PR review submission (`SubmitReview`, `BuildPRReview`). Comment mapping from commd's `ReviewComment` to GitHub's `DraftReviewComment`.
- **`internal/web/`** — Browser review UI (`review --web`), behaviorally the same as the TUI. `Serve()` listens on 127.0.0.1, prints/opens a URL carrying a session token (URL fragment → `X-Commd-Token` header on every `/api/` call), and blocks until every file is finished or skipped, or ctx is cancelled. `session.go` runs the flow (optional picker → one file at a time, each loaded via `Review.Load` when its turn comes); `file.go` holds one file's comments (addressed by random ID), viewed marks, and builds `markdown.ReviewResult` in TUI order; `view.go` builds the state JSON and renders section HTML with goldmark (raw HTML escaped), rewriting relative image paths to `/assets/{seq}/...?t=<token>`, which `serveAsset` serves from the document's directory only (`os.Root`, image types only, CSP sandbox). `static/*.js` (vanilla JS ES modules, embedded with `go:embed`; entry `main.js`, then `state` / `api` / `render` / `actions` / `keys` / `mouse` / `dom`) mirrors the TUI's modes, key bindings, dialogs and status bar (without pane focus), plus mouse support.
- **`internal/mermaid/`** — `RenderBlocks` converts fenced mermaid blocks to ASCII art; shared by the TUI detail pane and the web renderer.
- **`internal/pane/`** — Terminal multiplexer abstraction. `PaneSpawner` interface with WezTerm and Direct (fallback) implementations. tmux is stub only (`ByName("tmux")` returns DirectSpawner). `AutoDetect()` tries WezTerm → Direct. WezTerm spawner uses pixel dimensions for split direction (right 50% or bottom 80%).

### Key Data Flow

**Review**: `cmd/review.go` → `markdown.Parse(source)` → `tui.NewApp(doc)` → Bubble Tea loop → `markdown.FormatReview(result.Review, doc, filePath)` → clipboard/file/stdout

**Diff Review**: `cmd/review.go` (`--diff`) → `gitdiff.Open()` → `ChangedMarkdownFiles()` (when no file given) → file picker → for each file: `gitdiff.FilePatch()` → `diff.ParsePatch()` → `tui.NewDiffData()` → `tui.NewApp()` (diff view) → `markdown.FormatReview` / `FormatReviews` (multi-file) → clipboard/file/stdout

**Web Review**: `cmd/review.go` (`--web`, with or without `--diff`) → `web.Serve(web.Review{Pick|Paths, Load})` → browser ↔ JSON API (`/api/state`, `/api/pick`, `/api/files/{seq}/comments[/{id}]`, `/api/files/{seq}/viewed/{section}`, `/api/files/{seq}/finish`; every change returns the full state, while the read-only `/api/files/{seq}/search` returns the matching section IDs from `markdown.Document.SearchSections`, shared with the TUI) → `web.Result` (per-file status) → same output path as the TUI

**PR Review**: `cmd/pr.go` → `PRCmd.Validate()` (parses URL) → `Run(client)` (client injected by Kong via `BindToProvider`) → `github.ListMDFiles()` → file picker (multi-select) → for each file: `github.FetchFileContent()` → `markdown.Parse()` → `tui.NewApp()` → `github.BuildPRReview()` → `github.SubmitReview()`

**Hook**: Claude Code (PreToolUse: ExitPlanMode, or PostToolUse: Write|Edit) → stdin JSON → `cchook.Run()` → `pane.SpawnAndWait(commd review ...)` → temp file IPC → exit 0 or 2

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
- **Mermaid → ASCII rendering** via `mermaid-ascii` library; fenced `\`\`\`mermaid` blocks are converted to ASCII art in the TUI detail pane and web view (`internal/mermaid`), with fallback to raw source on error
- **Content-hash based viewed tracking** (`state.go`): sections are tracked by title → SHA-256 hash of title+body; if content changes, the viewed mark auto-clears
- **PreToolUse (ExitPlanMode) hook** as the recommended trigger: it fires exactly once per completed plan, before Claude Code's own approval dialog, and exit 2 denies the tool with the review as the reason so Claude revises in plan mode. **PostToolUse (Write|Edit)** remains as an alternative that fires on every plan write. A Stop hook is not an option because Stop doesn't fire during plan mode
- **Line comments carry a source quote** (`ReviewComment.Quote`) captured at save time; the output quotes it so Claude can locate the target after line numbers shift, and removed diff lines (which no longer exist in the file) stay readable
- **Temp files for IPC** between hook process and review subprocess (which runs in a separate pane)
- **Web review is a second front end over the same model**, not a TUI-in-a-browser (no terminal emulation), but it must behave like the TUI — same keys, modes, dialogs and flow; mouse support is additive. The one deliberate difference: no pane focus (vertical keys move the content and step between sections at its edges; the section list is 20% wide by default). The server owns all state (the page only renders JSON and calls the API) and returns the same `ReviewResult`, so output formatting stays in `markdown`. It binds to 127.0.0.1 only and requires the per-session token on the API, which also blocks cross-origin requests (custom header ⇒ preflight, never approved) and DNS rebinding. Vanilla JS with no build step keeps `go build` self-contained
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
- `review --web` is tested at two levels: `web.test.ts` drives the JSON API over HTTP, and `web-ui*.test.ts` drive the page in headless Chromium via Playwright (the `playwright` library under bun:test, not the Playwright test runner). Like the TUI tests, the critical paths (`web-ui.test.ts`) are in the basic tier and the rest (`web-ui-keys` / `web-ui-modes` / `web-ui-edge`) only in the full suite, which aims to cover every key binding and mode. Helpers: `launchWeb()` (`helpers/web.ts`), `useBrowser()` / `openPage()` / `eventually()` (`helpers/browser.ts`; wrap assertions after server-bound keys in `eventually()` since the page updates asynchronously). `mise run e2e-cov` collects V8 coverage of the page scripts (`COMMD_JS_COVERAGE=1`) and prints it per file (`bun scripts/js-coverage.ts --uncovered` lists missed lines). The e2e tasks run `bunx playwright install chromium` first
