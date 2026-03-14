package cmd

import (
	"fmt"
	"os"

	"github.com/koh-sh/commd/internal/cclocate"
)

// Run executes the locate subcommand.
func (l *LocateCmd) Run() error {
	opts := cclocate.Options{
		TranscriptPath: l.Transcript,
		CWD:            l.CWD,
		All:            l.All,
	}

	// If --stdin, read hook input from stdin
	if l.Stdin {
		input, err := cclocate.ParseHookInput(os.Stdin)
		if err != nil {
			return fmt.Errorf("parsing stdin: %w", err)
		}
		opts.TranscriptPath = input.TranscriptPath
		if input.CWD != "" {
			opts.CWD = input.CWD
		}
	}

	if opts.TranscriptPath == "" {
		return fmt.Errorf("--transcript or --stdin is required")
	}

	paths, err := cclocate.LocatePlanFile(opts)
	if err != nil {
		return fmt.Errorf("locating plan file: %w", err)
	}

	if len(paths) == 0 {
		plansDir := cclocate.ResolvePlansDir(opts.CWD)
		return fmt.Errorf("no plan file found (plansDirectory: %s)", plansDir)
	}

	for _, p := range paths {
		fmt.Println(p)
	}

	return nil
}
