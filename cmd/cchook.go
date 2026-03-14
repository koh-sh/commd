package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/koh-sh/commd/internal/cchook"
	"github.com/koh-sh/commd/internal/pane"
)

// Run executes the hook subcommand.
func (h *HookCmd) Run() error {
	os.Exit(h.runExit(os.Stdin))
	return nil // unreachable
}

// runExit executes the hook logic and returns the exit code.
// Errors always return exit code 0 so that hook failures never block the
// Claude Code workflow. Only a successful review submission returns exit
// code 2 (feedback signal).
func (h *HookCmd) runExit(r io.Reader) int {
	input, err := cchook.ParseInput(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "commd cchook: failed to parse input: %v\n", err)
		return 0
	}

	spawner := pane.ByName(h.Spawner)

	exitCode, err := cchook.Run(input, cchook.RunConfig{
		Spawner: spawner,
		Theme:   h.Theme,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "commd cchook: %v\n", err)
		return 0
	}

	return exitCode
}
