package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/koh-sh/commd/internal/markdown"
	"github.com/koh-sh/commd/internal/tui"
)

// writeReviewOutput writes the review output using the specified method.
func writeReviewOutput(output, mode, outputPath string) error {
	switch mode {
	case "clipboard":
		if err := clipboard.WriteAll(output); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to copy to clipboard: %v\n", err)
			fmt.Fprintf(os.Stderr, "Use --output stdout or --output file instead.\n")
			// Still print to stdout as fallback
			fmt.Print(output)
		} else {
			fmt.Fprintln(os.Stderr, "Review copied to clipboard.")
		}
	case "stdout":
		fmt.Print(output)
	case "file":
		if outputPath == "" {
			return fmt.Errorf("--output-path is required with --output file")
		}
		if _, err := os.Stat(outputPath); errors.Is(err, os.ErrNotExist) {
			// Output file was deleted (possibly due to hook timeout). Fall back to clipboard.
			if err := clipboard.WriteAll(output); err != nil {
				fmt.Fprintf(os.Stderr, "Output file %s was deleted (possibly due to hook timeout). Failed to copy to clipboard: %v\n", outputPath, err)
				fmt.Print(output)
			} else {
				fmt.Fprintf(os.Stderr, "Output file %s was deleted (possibly due to hook timeout). Review copied to clipboard.\n", outputPath)
			}
		} else if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		} else {
			fmt.Fprintf(os.Stderr, "Review written to %s\n", outputPath)
		}
	}
	return nil
}

// Run executes the review subcommand.
func (r *ReviewCmd) Run() error {
	// Read file
	source, err := os.ReadFile(r.File)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Parse document
	p, err := markdown.Parse(source)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	// Create and run TUI
	app := tui.NewApp(p, tui.AppOptions{
		Theme:       r.Theme,
		FilePath:    r.File,
		TrackViewed: r.TrackViewed,
	})
	opts := append([]tea.ProgramOption{tea.WithAltScreen()}, r.teaOpts...)
	prog := tea.NewProgram(app, opts...)
	finalModel, err := prog.Run()
	if err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	app, ok := finalModel.(*tui.App)
	if !ok {
		return fmt.Errorf("unexpected model type: %T", finalModel)
	}

	// Save viewed state if tracking is enabled
	if r.TrackViewed {
		if vs := app.ViewedState(); vs != nil {
			statePath := markdown.StatePath(r.File)
			if err := markdown.SaveViewedState(statePath, vs); err != nil {
				fmt.Fprintf(os.Stderr, "commd: warning: failed to save viewed state: %v\n", err)
			}
		}
	}

	result := app.Result()

	// Output review if submitted
	if result.Status == markdown.StatusSubmitted && result.Review != nil {
		output := markdown.FormatReview(result.Review, p, r.File)
		if output == "" {
			return nil
		}

		if err := writeReviewOutput(output, r.Output, r.OutputPath); err != nil {
			return err
		}
	} else if result.Status == markdown.StatusApproved {
		fmt.Fprintln(os.Stderr, "Approved.")
	}

	return nil
}
