# VHS demo

Tape file for the README demo GIF, generated with
[VHS](https://github.com/charmbracelet/vhs).

## Prerequisites

- `vhs` itself is provisioned by `mise install` (pinned in `.mise.toml`).
  Install its runtime dependencies once:

  ```bash
  brew install ttyd ffmpeg
  ```

## Generate

```bash
mise run demo
```

Builds `commd` and runs `vhs vhs/demo.tape`. Output: `vhs/demo.gif`.

To check tape syntax without recording:

```bash
mise run demo-validate
```

## How the tape works

The recorded session walks through `commd review` against
`internal/markdown/testdata/basic.md`:

```
launch -> navigate -> add comment (question) -> add comment (suggestion)
       -> show help -> submit -> confirm
```

`--output stdout` is used so the rendered review markdown is printed to
the terminal after submission, instead of going to the clipboard.

A hidden setup block prepends the repo root to `PATH` (so commands appear
as `commd ...` rather than `./commd ...`) and exports `CI=true`. The
latter makes Bubble Tea skip the cursor/background-color queries it
issues on startup, which would otherwise add a ~5s delay under the VHS
PTY. The same trick is used in the bun-based E2E suite.

## Pacing

`Set PlaybackSpeed 1.5` and `Set TypingSpeed 80ms` give a comfortable
demo cadence. Tweak `PlaybackSpeed`, `TypingSpeed`, and the per-step
`Sleep` values in `vhs/demo.tape` if the pacing feels off.
