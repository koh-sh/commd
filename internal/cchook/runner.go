package cchook

import (
	"context"
	"fmt"
	"os"

	"github.com/koh-sh/commd/internal/cclocate"
	"github.com/koh-sh/commd/internal/pane"
)

// RunConfig holds configuration for the hook runner.
type RunConfig struct {
	Spawner pane.PaneSpawner
	Theme   string
}

// Run executes the hook orchestration flow.
// Returns exitCode: 0 = continue normally, 2 = feedback to Claude.
func Run(input *Input, cfg RunConfig) (int, error) {
	// Early returns
	if input.PermissionMode != permissionModePlan {
		return 0, nil
	}
	if os.Getenv("CC_PLAN_REVIEW_SKIP") == "1" {
		return 0, nil
	}

	// Determine plan file from tool_input
	if input.ToolInput == nil || input.ToolInput.FilePath == "" {
		return 0, nil
	}
	planFile := input.ToolInput.FilePath

	// Check if file is under plans directory
	plansDir := cclocate.ResolvePlansDir(input.CWD)
	if !cclocate.IsUnderDir(planFile, plansDir) {
		return 0, nil
	}

	// Check file exists
	if _, err := os.Stat(planFile); err != nil {
		return 0, nil
	}

	// Prepare temp file for IPC with review subprocess
	reviewFile, err := os.CreateTemp("", "commd-review-*.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "commd: failed to create temp review file: %v\n", err)
		return 0, nil
	}
	reviewPath := reviewFile.Name()
	reviewFile.Close()
	defer os.Remove(reviewPath)

	// Resolve commd binary path
	executable, err := os.Executable()
	if err != nil {
		executable = "commd"
	}

	// Build review args
	args := []string{
		"review",
		"--output", "file",
		"--output-path", reviewPath,
		"--theme", cfg.Theme,
		"--track-viewed",
		planFile,
	}

	// Spawn review in pane
	ctx := context.Background()
	spawner := cfg.Spawner
	err = spawner.SpawnAndWait(ctx, executable, args)
	if err != nil {
		// Fallback to direct if not already direct
		if spawner.Name() != pane.NameDirect {
			fmt.Fprintf(os.Stderr, "commd: %s spawn failed, falling back to direct: %v\n", spawner.Name(), err)
			direct := &pane.DirectSpawner{}
			err = direct.SpawnAndWait(ctx, executable, args)
		}
		if err != nil {
			return 0, nil
		}
	}

	// Read review result -- non-empty means submitted
	reviewBytes, err := os.ReadFile(reviewPath)
	if err != nil {
		return 0, nil
	}
	if review := string(reviewBytes); review != "" {
		fmt.Fprint(os.Stderr, review)
		return 2, nil
	}

	return 0, nil
}
