package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/koh-sh/commd/internal/cchook"
	"github.com/koh-sh/commd/internal/pane"
)

// ExitCodeError carries a non-zero process exit code out of a command so
// that main can exit with it once Kong has returned, instead of the command
// calling os.Exit itself (which would skip deferred cleanup and bypass
// Kong's own error handling).
type ExitCodeError struct {
	Code int
}

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// Run executes the hook subcommand.
func (h *HookCmd) Run() error {
	if code := h.runExit(os.Stdin); code != 0 {
		return ExitCodeError{Code: code}
	}
	return nil
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

	return cchook.Run(context.Background(), input, cchook.RunConfig{
		Spawner: spawner,
		Theme:   h.Theme,
	})
}
