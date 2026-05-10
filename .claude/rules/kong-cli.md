---
paths:
  - "cmd/**/*.go"
  - "main.go"
---

# Kong CLI conventions

This project uses Kong (`github.com/alecthomas/kong`) for the CLI surface. The patterns below were established by deliberate refactoring and must be preserved.

## Dispatch with `Run(...) error`

- Every subcommand struct must implement a `Run(...) error` method. `main.go` dispatches via `ctx.Run(...)`.
- Do not add `switch ctx.Command()` branching in `main.go`. The README's "switch on the command string" pattern is fragile and explicitly avoided here.

## Cross-field validation goes in `Validate()`, not `Run()`

- For constraints that involve more than one field (e.g. "`--output-path` required when `--output=file`", "`--transcript` or `--stdin` required"), implement `Validate() error` on the command struct. Kong calls it after parsing, before `Run`.
- When moving a check into `Validate`, remove the equivalent guard from `Run` and any helper functions it called. Do not keep both — keeping defensive duplicates of a constraint already enforced upstream is dead code.
- Single-flag constraints (enum, required, type) should use Kong tags (`enum:""`, `required:""`, `type:"existingfile"`), not `Validate`.

## File layout

- `cmd/cli.go` is reserved for struct definitions, Kong tags, and the top-level `CLI` type. Do not put `Run` or `Validate` method bodies here.
- Place `Run` and `Validate` for a subcommand in its domain file (e.g. `cmd/review.go` for `ReviewCmd`, `cmd/pr.go` for `PRCmd`, `cmd/cclocate.go` for `LocateCmd`).
- godoc on `Validate` must state what is checked, not just "validates the command". Example: `// Validate requires --output-path when --output=file.`

## Dependency injection via `kong.BindToProvider`

- External dependencies built from environment state (e.g. `*ghclient.Client` from `GITHUB_TOKEN`) are wired in `main.go` with `kong.BindToProvider(constructor)` and received as `Run(dep *T) error` parameters.
- Do not add a private struct field on the command (e.g. `client *ghclient.Client`) just to support test overrides. Tests should call `cmd.Run(stub)` directly.
- `kong.BindToProvider` is lazy: the provider only runs when a `Run` method requests its return type, so adding a provider for one command does not impose its setup cost (or required env vars) on other commands.

## Test conventions

- Test cross-field constraints by calling `Validate()` on the struct, not by invoking `Run`. Share one table-driven test across commands using a `validator interface{ Validate() error }`.
- Do not use `/tmp` paths in tests. Use a literal like `"any/path"` when only the field shape matters, or `t.TempDir()` when a real file is needed.
- When a test must call `Run` with a dependency that would normally be injected by Kong, pass it as a function argument (e.g. `p.Run(client)`). Comment any `Run(nil)` call with the reason nil is safe.
