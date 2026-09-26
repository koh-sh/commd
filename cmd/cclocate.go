package cmd

import (
	"fmt"
	"os"

	"github.com/koh-sh/commd/internal/cclocate"
)

// Validate requires exactly one of --transcript or --stdin; with --stdin the
// transcript path comes from the hook input, so a --transcript given alongside
// it would be silently ignored. The path resolved from stdin is checked in
// Run, not here.
func (l *LocateCmd) Validate() error {
	if l.Transcript == "" && !l.Stdin {
		return fmt.Errorf("--transcript or --stdin is required")
	}
	if l.Transcript != "" && l.Stdin {
		return fmt.Errorf("--transcript and --stdin are mutually exclusive")
	}
	return nil
}

// Run executes the locate subcommand.
func (l *LocateCmd) Run() error {
	fmt.Fprintln(os.Stderr, "commd: cclocate is deprecated and will be removed in a future release")

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
