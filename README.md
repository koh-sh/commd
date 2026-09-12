# commd

Interactive Markdown reviewer with TUI.
Add review comments at section level or line level using [Conventional Comments](https://conventionalcomments.org/) and output structured feedback.

> Formerly **ccplan** — see [Migration from ccplan](#migration-from-ccplan) for upgrade instructions.

![commd review demo](vhs/demo.gif)

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
`review` is the default command, so the subcommand name can be omitted.

```bash
commd review path/to/document.md

# Same as above (review is the default command)
commd path/to/document.md

# Output review to a file
commd review --output file --output-path ./review.md document.md

# Output to stdout
commd review --output stdout document.md

# Review local git changes in diff view (working tree vs HEAD)
commd review --diff document.md

# Pick from all changed/untracked .md files, comparing against a branch
commd review --diff --base main
```

| Flag | Description |
|------|-------------|
| `--output` | Output method: `clipboard` (default), `stdout`, `file` |
| `--output-path` | File path for `--output file` |
| `--theme` | Color theme: `dark` (default), `light` |
| `--track-viewed` | Persist viewed state to sidecar file (`.reviewed.json`) for change detection across sessions |
| `--diff` | Review local git changes in diff view. Without a file, pick from changed `.md` files |
| `--base` | Git ref to diff against (default: `HEAD`; requires `--diff`) |

When `--track-viewed` is enabled, commd saves which sections you've marked as viewed in a `.reviewed.json` sidecar file. On subsequent runs, viewed marks are restored automatically. If a section's content has changed, its viewed mark is cleared (detected via content hash).

#### Diff mode

`--diff` reviews what changed in the working tree instead of the whole file — for example, docs that Claude Code edited, before you commit them. It uses the same diff view as `commd pr`:

- The right pane opens in raw view showing the unified diff (`+`/`-` lines); press `r` for the rendered working-tree content
- Press `Tab` to focus the diff pane, then comment on added, removed, or context lines with `c` / `V`; section comments work in the rendered view
- Without a file argument, all changed and untracked `.md` files under the current directory are offered in a file picker and reviewed one by one; comments are combined into a single output
- Untracked files are shown as entirely added; unchanged files are skipped
- `--track-viewed` is not available in diff mode

### `commd pr`

Review Markdown files changed in a GitHub pull request. Comments are submitted as a GitHub PR Review with inline file comments.

```bash
# Interactive file picker for changed .md files
commd pr https://github.com/owner/repo/pull/123

# Review a specific file directly
commd pr https://github.com/owner/repo/pull/123 --file docs/README.md
```

| Flag | Description |
|------|-------------|
| `--file` | Review a specific file instead of showing the file picker |
| `--theme` | Color theme: `dark` (default), `light` |

**Authentication**: Requires a GitHub token via `GITHUB_TOKEN` environment variable or `gh auth login`.

**File size**: The GitHub Contents API only decodes files up to 1 MB. Larger files (up to GitHub's 100 MB limit) are fetched automatically via their raw download URL.

**File picker**: When `--file` is not specified, an interactive file picker shows all changed `.md` files. All files are selected by default. Use `space` to toggle, `a` to select/deselect all, `enter` to confirm, `q` or `esc` to cancel.

**Review flow**: After selecting files, you review them one by one. For each file you can add comments, then press `s` to finish or `q` to skip. After all files, a summary dialog lets you choose to approve, comment, or cancel the review.

**Submit behavior**: Comments are posted as a GitHub PR Review with inline comments on each file. If no comments are added, you can optionally approve the PR. Note: Overview (file-level) comments are not posted to GitHub due to API limitations — only section-level and line-level comments are submitted.

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
| `l` / `h` / `→` / `←` | Scroll the detail pane right / left |
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
| `↑` / `↓` | Navigate results |
| `Enter` | Confirm search |
| `Esc` | Cancel search |

## Mermaid Diagram Rendering

Fenced `` ```mermaid `` code blocks are automatically converted to ASCII art in the detail pane. If rendering fails (e.g. unsupported diagram type), the original source is shown as-is.

## Raw Source View

Press `r` to switch the right pane to raw source view with line numbers. In this mode:

- **Line-level commenting**: Press `c` to comment on the cursor line
- **Visual selection**: Press `V`, move with `j`/`k` to select a range, then `c` to comment. In diff mode (`commd pr` and `--diff`), a selection spanning both sides is automatically restricted to the cursor's side (old or new file lines) to satisfy GitHub's single-side comment requirement
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
> const token = parseToken(header)

`L20-L25` [suggestion] Extract this block into a helper function.
> if err := validate(input); err != nil {
>     return err
> }
```

Section-level comments (including Overview for the preamble) are grouped under section headings. Line-level comments appear below a `---` divider with inline code line references, followed by a quote of the commented source (up to 5 lines) so the target can still be found after the file changes. In diff mode, comments on removed lines are numbered by the old file and marked `(removed)`. When several files are reviewed with `--diff`, each file gets its own `## path` heading with section headings nested below it.

Labels: `suggestion`, `issue`, `question` (default), `nitpick`, `todo`, `thought`, `note`, `praise`, `chore`

Decorations: `non-blocking`, `blocking`, `if-minor` — cycle with `Ctrl+D` in comment mode

## Claude Code Integration

commd can be used as a Claude Code hook to review plan files interactively during plan mode.

### `commd cchook`

Run as a Claude Code hook. Launches the review TUI for the plan file to enable a feedback loop. Two trigger points are supported:

- **`PreToolUse` on `ExitPlanMode`** (recommended): fires once, when Claude has finished the plan and asks to leave plan mode. Claude Code injects the plan file path (`tool_input.planFilePath`), so no `plansDirectory` lookup is involved. Intermediate edits to the plan do not open a review.
- **`PostToolUse` on `Write|Edit`**: fires on every write to a file under `plansDirectory`, including edits made before the plan is complete.

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

Locate plan file paths from a Claude Code transcript JSONL. Useful for debugging the `PostToolUse` trigger's `plansDirectory` resolution; the `PreToolUse`/`ExitPlanMode` trigger receives the plan path directly and does not need it.

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
    "PreToolUse": [
      {
        "matcher": "ExitPlanMode",
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

The hook only activates in plan mode and automatically enables `--track-viewed`. The review opens *before* Claude Code's own plan approval dialog:

1. Claude writes the plan and calls `ExitPlanMode`
2. commd opens the plan for review
3. **submitted** (exit 2): the review is sent to Claude as the denial reason; Claude revises the plan in plan mode and calls `ExitPlanMode` again (back to step 2)
4. **approved / cancelled** (exit 0): Claude Code shows its usual plan approval dialog

To review on every plan file write instead (including intermediate edits), use the `PostToolUse` trigger. It requires the file to be under `plansDirectory`:

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

Set `CC_PLAN_REVIEW_SKIP=1` to temporarily disable the hook.

## Development

Dev tools are managed by [mise](https://mise.jdx.dev/). Run `mise install` to set up the toolchain (includes Go linters, formatters, and bun).

```bash
mise run ci         # Run full CI pipeline (fmt, fix, lint, build, cov, e2e-basic)
mise run e2e        # Run all E2E tests (full suite)
mise run e2e-basic  # Run basic E2E tests (critical path only, included in ci)
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
