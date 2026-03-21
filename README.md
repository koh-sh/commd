# commd

Interactive Markdown reviewer with TUI.
Add review comments at section level or line level using [Conventional Comments](https://conventionalcomments.org/) and output structured feedback.

> Formerly **ccplan** — see [Migration from ccplan](#migration-from-ccplan) for upgrade instructions.

https://github.com/user-attachments/assets/55defd1d-c28c-473b-95ea-9e427f6a4266

## Install

### mise

```bash
mise use -g github:koh-sh/commd
```

### go install

```bash
go install github.com/koh-sh/commd@latest
```

### Pre-built binary

Download the latest release from the [Releases page](https://github.com/koh-sh/commd/releases).

## Usage

### `commd review`

Display a Markdown file in a 2-pane TUI and add review comments to each section.

```bash
commd review path/to/document.md

# Output review to a file
commd review --output file --output-path ./review.md document.md

# Output to stdout
commd review --output stdout document.md
```

| Flag | Description |
|------|-------------|
| `--output` | Output method: `clipboard` (default), `stdout`, `file` |
| `--output-path` | File path for `--output file` |
| `--theme` | Color theme: `dark` (default), `light` |
| `--track-viewed` | Persist viewed state to sidecar file (`.reviewed.json`) for change detection across sessions |

When `--track-viewed` is enabled, commd saves which sections you've marked as viewed in a `.reviewed.json` sidecar file. On subsequent runs, viewed marks are restored automatically. If a section's content has changed, its viewed mark is cleared (detected via content hash).

### `commd version`

Show the current version.

```bash
commd version
```

## TUI Key Bindings

### Normal Mode

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate sections (left pane) or lines (right pane, raw view) |
| `gg` / `G` | Jump to first / last |
| `Ctrl+D` / `Ctrl+U` | Half page down / up |
| `Ctrl+F` / `Ctrl+B` | Full page down / up |
| `l` / `h` / `→` / `←` | Expand / collapse (left pane) / Scroll right / left (right pane) |
| `H` / `L` | Scroll to start / end (right pane) |
| `>` / `<` | Resize left pane wider / narrower |
| `Enter` | Toggle expand/collapse |
| `Tab` | Switch focus between panes |
| `f` | Toggle full view / section view |
| `r` | Toggle raw source view (with line numbers) / rendered view |
| `c` | Add comment (section-level in rendered view, line-level in raw view) |
| `C` | Manage comments (edit/delete) |
| `V` | Start visual line selection (raw view, right pane) |
| `v` | Toggle viewed mark |
| `/` | Search sections |
| `s` | Submit review and exit |
| `q` / `Ctrl+C` | Quit |
| `?` | Show help |

### Comment Mode

| Key | Action |
|-----|--------|
| `Tab` | Cycle comment label (forward) |
| `Shift+Tab` | Cycle comment label (reverse) |
| `Ctrl+D` | Cycle decoration (none, non-blocking, blocking, if-minor) |
| `Ctrl+S` | Save comment |
| `Esc` | Cancel |

### Comment List Mode

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate comments |
| `e` | Edit selected comment |
| `d` | Delete selected comment |
| `Esc` | Back to normal mode |

### Status Bar

The status bar shows key hints and a progress indicator: `[X/Y viewed]` for sections marked as viewed, and `[N comments]` when comments have been added.

### Search Mode

| Key | Action |
|-----|--------|
| Type text | Incremental filter (searches ID, title, and body) |
| `j` / `k` | Navigate results |
| `Enter` | Confirm search |
| `Esc` | Cancel search |

## Mermaid Diagram Rendering

Fenced `` ```mermaid `` code blocks are automatically converted to ASCII art in the detail pane. If rendering fails (e.g. unsupported diagram type), the original source is shown as-is.

## Raw Source View

Press `r` to switch the right pane to raw source view with line numbers. In this mode:

- **Line-level commenting**: Press `c` to comment on the cursor line
- **Visual selection**: Press `V`, move with `j`/`k` to select a range, then `c` to comment
- **Section navigation**: `j`/`k` at the edge of a section automatically moves to the adjacent section
- Press `f` to toggle between section view (only selected section's lines) and full file view
- Press `r` again to return to rendered view

Both section-level comments (from rendered view) and line-level comments (from raw view) can coexist in the same session.

## Review Output Format

The review output generated on submit uses [Conventional Comments](https://conventionalcomments.org/) labels:

```markdown
# Review

Please review and address the following comments on: /path/to/document.md

## Overview
[note] Add a performance metrics section.

## S1.1: JWT verification
[suggestion (non-blocking)] Switch to HS256. Load the key from an environment variable.

## S2: Update routing
[issue (blocking)] Not needed; the existing implementation covers this.

---

`L15` [question] Is this variable used?

`L20-L25` [suggestion] Extract this block into a helper function.
```

Section-level comments (including Overview for the preamble) are grouped under section headings. Line-level comments appear below a `---` divider with inline code line references.

Labels: `suggestion`, `issue`, `question` (default), `nitpick`, `todo`, `thought`, `note`, `praise`, `chore`

Decorations: `non-blocking`, `blocking`, `if-minor` — cycle with `Ctrl+D` in comment mode

## Claude Code Integration

commd can be used as a Claude Code PostToolUse hook to review plan files interactively during plan mode.

### `commd cchook`

Run as a Claude Code PostToolUse (Write|Edit) hook. Detects writes to plan files and launches the review TUI to enable a feedback loop.

```bash
# Called automatically by Claude Code hook (no manual invocation needed)
commd cchook
```

| Flag | Description |
|------|-------------|
| `--spawner` | Terminal multiplexer: `auto` (default), `wezterm`, `tmux` |
| `--theme` | Color theme: `dark` (default), `light` |

> **Note:** Currently only WezTerm is supported as a terminal multiplexer spawner. tmux support is not yet implemented. `auto` will try WezTerm first, then fall back to running in the same terminal.

### `commd cclocate`

Locate plan file paths from a Claude Code transcript JSONL. This command is primarily used internally by `commd cchook` to resolve plan file paths during hook execution.

```bash
commd cclocate --transcript ~/.claude/projects/.../session.jsonl

# List all plan files found in a transcript
commd cclocate --transcript session.jsonl --all

# Read hook JSON input from stdin to resolve the plan file
commd cclocate --stdin
```

### Hook Setup

Add the following to `.claude/settings.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "commd cchook",
            "timeout": 600
          }
        ]
      }
    ]
  }
}
```

The hook only activates in plan mode and launches the review TUI when a file under `plansDirectory` is written. The hook automatically enables `--track-viewed`.

- **submitted** (exit 2): Sends review comments to Claude via stderr, prompting plan revision
- **approved / cancelled** (exit 0): Continues normally

Set `CC_PLAN_REVIEW_SKIP=1` to temporarily disable the hook.

## Development

Dev tools are managed by [mise](https://mise.jdx.dev/). Run `mise install` to set up the toolchain (includes Go linters, formatters, and bun).

```bash
mise run ci    # Run full CI pipeline (fmt, lint, build, test)
mise run e2e   # Run E2E tests (builds binary, then runs bun test in e2e/)
```

E2E tests use [tuistory](https://github.com/remorses/tuistory) to drive the TUI in a virtual terminal.

## Migration from ccplan

commd was formerly known as **ccplan**. If you are upgrading:

| Item | Before | After |
|------|--------|-------|
| Binary | `ccplan` | `commd` |
| Subcommand | `ccplan review` | `commd review` |
| Subcommand | `ccplan hook` | `commd cchook` |
| Subcommand | `ccplan locate` | `commd cclocate` |
| Hook config | `"command": "ccplan hook"` | `"command": "commd cchook"` |
| Environment variable | `PLAN_REVIEW_SKIP=1` | `CC_PLAN_REVIEW_SKIP=1` |
| go install | `github.com/koh-sh/ccplan` | `github.com/koh-sh/commd` |
| mise | `github:koh-sh/ccplan` | `github:koh-sh/commd` |

For mise users upgrading:

```bash
mise uninstall github:koh-sh/ccplan
mise use -g github:koh-sh/commd
```
