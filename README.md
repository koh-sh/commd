# ccplan

A CLI tool for reviewing Claude Code plan files in an interactive TUI.
Add inline review comments to each step and send feedback back to Claude Code.

https://github.com/user-attachments/assets/55defd1d-c28c-473b-95ea-9e427f6a4266

## Install

### mise

```bash
mise use -g github:koh-sh/ccplan
```

### go install

```bash
go install github.com/koh-sh/ccplan@latest
```

### Pre-built binary

Download the latest release from the [Releases page](https://github.com/koh-sh/ccplan/releases).

## Subcommands

### `ccplan review`

Display a plan file in a 2-pane TUI and add review comments to each step.

```bash
ccplan review path/to/plan.md

# Output review to a file
ccplan review --output file --output-path ./review.md plan.md

# Output to stdout
ccplan review --output stdout plan.md
```

| Flag | Description |
|------|-------------|
| `--output` | Output method: `clipboard` (default), `stdout`, `file` |
| `--output-path` | File path for `--output file` |
| `--theme` | Color theme: `dark` (default), `light` |
| `--track-viewed` | Persist viewed state to sidecar file (`.reviewed.json`) for change detection across sessions |

When `--track-viewed` is enabled, ccplan saves which steps you've marked as viewed in a `<plan-file>.reviewed.json` sidecar file. On subsequent runs, viewed marks are restored automatically. If a step's content has changed, its viewed mark is cleared (detected via content hash). The hook automatically enables this flag.

### `ccplan locate`

Locate plan file paths from a Claude Code transcript JSONL. This command is primarily used internally by `ccplan hook` to resolve plan file paths during hook execution.

```bash
ccplan locate --transcript ~/.claude/projects/.../session.jsonl

# List all plan files found in a transcript
ccplan locate --transcript session.jsonl --all

# Read hook JSON input from stdin to resolve the plan file
ccplan locate --stdin
```

### `ccplan hook`

Run as a Claude Code PostToolUse (Write|Edit) hook. Detects writes to plan files and launches the review TUI to enable a feedback loop.

```bash
# Called automatically by Claude Code hook (no manual invocation needed)
ccplan hook
```

| Flag | Description |
|------|-------------|
| `--spawner` | Terminal multiplexer: `auto` (default), `wezterm`, `tmux` |
| `--theme` | Color theme: `dark` (default), `light` |

> **Note:** Currently only WezTerm is supported as a terminal multiplexer spawner. tmux support is not yet implemented. `auto` will try WezTerm first, then fall back to running in the same terminal.

### `ccplan version`

Show the current version.

```bash
ccplan version
```

## Claude Code Hook Setup

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
            "command": "ccplan hook",
            "timeout": 600
          }
        ]
      }
    ]
  }
}
```

The hook only activates in plan mode and launches the review TUI when a file under `plansDirectory` is written.

- **submitted** (exit 2): Sends review comments to Claude via stderr, prompting plan revision
- **approved / cancelled** (exit 0): Continues normally

Set `PLAN_REVIEW_SKIP=1` to temporarily disable the hook.

## TUI Key Bindings

### Normal Mode

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate steps |
| `gg` / `G` | Jump to first / last step |
| `l` / `h` / `→` / `←` | Expand / collapse (left pane) / Scroll right / left (right pane) |
| `H` / `L` | Scroll to start / end (right pane) |
| `>` / `<` | Resize left pane wider / narrower |
| `Enter` | Toggle expand/collapse |
| `Tab` | Switch focus between panes |
| `f` | Toggle full view / section view (right pane) |
| `c` | Add comment |
| `C` | Manage comments (edit/delete) |
| `v` | Toggle viewed mark |
| `/` | Search steps |
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

The status bar shows key hints and a progress indicator: `[X/Y viewed]` for steps marked as viewed, and `[N comments]` when comments have been added.

### Search Mode

| Key | Action |
|-----|--------|
| Type text | Incremental filter (searches ID, title, and body) |
| `j` / `k` | Navigate results |
| `Enter` | Confirm search |
| `Esc` | Cancel search |

## Mermaid Diagram Rendering

Fenced `` ```mermaid `` code blocks in plan files are automatically converted to ASCII art in the detail pane. If rendering fails (e.g. unsupported diagram type), the original source is shown as-is.

## Review Output Format

The review output generated on submit uses [Conventional Comments](https://conventionalcomments.org/) labels:

```markdown
# Plan Review

Please review and address the following comments on: /path/to/plan.md

## S1.1: JWT verification
[suggestion (non-blocking)] Switch to HS256. Load the key from an environment variable.

## S2: Update routing
[issue (blocking)] Not needed; the existing implementation covers this.

## S3: Add tests
[question] Is the coverage target 80% or 90%?
```

Labels: `suggestion`, `issue`, `question` (default), `nitpick`, `todo`, `thought`, `note`, `praise`, `chore`

Decorations: `non-blocking`, `blocking`, `if-minor` — cycle with `Ctrl+D` in comment mode
