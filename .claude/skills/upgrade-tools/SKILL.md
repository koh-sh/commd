---
name: upgrade-tools
description: |
  Upgrade dev tools managed by mise to latest versions and sync CI workflow files.
  Runs `mise run upgrade-tools`, then updates Go version in go.mod and tool versions
  in GitHub Actions workflows to match .mise.toml.
---

# upgrade-tools

Upgrade all dev tools and sync versions across local and CI.

## Steps

1. Run `mise run upgrade-tools` to update `.mise.toml` tools to their latest versions
2. Read `.mise.toml` to get the updated versions
3. Update the following files to match `.mise.toml`:

| mise tool | Target file | What to update |
|-----------|-------------|----------------|
| `go` | `go.mod` | `go X.Y.Z` directive (run `go mod tidy` after) |
| `"github:golangci/golangci-lint"` | `.github/workflows/golangci-lint.yml` | `version: vX.Y.Z` in golangci-lint-action |
| `"github:goreleaser/goreleaser"` | `.github/workflows/go-releaser.yml` | `version: vX.Y.Z` in goreleaser-action |
| `"github:goreleaser/goreleaser"` | `.github/workflows/go-releaser-check.yml` | `version: vX.Y.Z` in goreleaser-action |
| `bun` | `.github/workflows/e2e.yml` | `bun-version: "X.Y.Z"` in setup-bun |

4. Run `mise run ci` to verify everything passes

## Important

- CI versions use a `v` prefix (e.g., `v2.11.3`), mise versions do not (e.g., `2.11.3`). Add `v` prefix when writing to CI files.
- `bun-version` does NOT use a `v` prefix.
- Only update versions that actually changed. Skip files where the version is already correct.
- If `mise run ci` fails, fix the issue before finishing.
