package cmd

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/atotto/clipboard"
	"github.com/koh-sh/commd/internal/diff"
	"github.com/koh-sh/commd/internal/gitdiff"
	"github.com/koh-sh/commd/internal/markdown"
	"github.com/koh-sh/commd/internal/tui"
	"github.com/koh-sh/commd/internal/web"
)

// defaultBase is the git ref --diff compares against unless --base is given.
const defaultBase = "HEAD"

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

// Validate requires --output-path when --output=file, exactly one <file>
// unless --diff is set, --web for --port and --no-open, and a port number in
// 0-65535 (0, the default, picks a free port), and rejects --base and --track-viewed combinations that only make
// sense in one of the two modes.
func (r *ReviewCmd) Validate() error {
	if r.Output == "file" && r.OutputPath == "" {
		return fmt.Errorf("--output-path is required with --output file")
	}
	if !r.Web && (r.Port != 0 || r.NoOpen) {
		return fmt.Errorf("--port and --no-open require --web")
	}
	if r.Port < 0 || r.Port > 65535 {
		return fmt.Errorf("--port must be between 0 and 65535")
	}
	if r.Diff {
		if r.TrackViewed {
			return fmt.Errorf("--track-viewed cannot be combined with --diff")
		}
		return nil
	}
	if r.Base != "" {
		return fmt.Errorf("--base requires --diff")
	}
	if len(r.Files) != 1 {
		return fmt.Errorf("expected exactly one <file> (use --diff to review changed files)")
	}
	return nil
}

// Run executes the review subcommand.
func (r *ReviewCmd) Run() error {
	if r.Diff {
		return r.runDiff()
	}
	return r.runFile(r.Files[0])
}

// runFile reviews a single file in the rendered view.
func (r *ReviewCmd) runFile(path string) error {
	// Read file
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Parse document
	p, err := markdown.Parse(source)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	file := markdown.File{Path: path, Doc: p}
	if r.TrackViewed {
		file.Viewed = markdown.LoadViewedState(markdown.StatePath(path))
	}
	var status markdown.Status
	var result *markdown.ReviewResult
	if r.Web {
		status, result, err = r.reviewFileInBrowser(file)
	} else {
		status, result, err = r.reviewFileInTUI(file)
	}
	if err != nil {
		return err
	}

	// Save viewed state if tracking is enabled (the review updated it in place)
	if file.Viewed != nil {
		if err := markdown.SaveViewedState(markdown.StatePath(path), file.Viewed); err != nil {
			fmt.Fprintf(os.Stderr, "commd: warning: failed to save viewed state: %v\n", err)
		}
	}

	// Output review if submitted
	if status == markdown.StatusSubmitted && result != nil {
		output := markdown.FormatReview(result, p, path)
		if output == "" {
			return nil
		}

		if err := writeReviewOutput(output, r.Output, r.OutputPath); err != nil {
			return err
		}
	} else if status == markdown.StatusApproved {
		fmt.Fprintln(os.Stderr, "Approved.")
	}

	return nil
}

// reviewFileInTUI reviews a single file in the TUI.
func (r *ReviewCmd) reviewFileInTUI(f markdown.File) (markdown.Status, *markdown.ReviewResult, error) {
	app := tui.NewApp(f.Doc, tui.AppOptions{
		Theme:    r.Theme,
		FilePath: f.Path,
		Viewed:   f.Viewed,
	})
	result, err := runReviewApp(app, r.teaOpts)
	if err != nil {
		return "", nil, err
	}
	return result.Status, result.Review, nil
}

// reviewFileInBrowser reviews a single file in the browser.
func (r *ReviewCmd) reviewFileInBrowser(f markdown.File) (markdown.Status, *markdown.ReviewResult, error) {
	res, err := r.serveWeb(web.Review{
		Paths: []string{f.Path},
		Load:  func(string) (markdown.File, bool) { return f, true },
	})
	if err != nil {
		return "", nil, err
	}
	// An interrupted session has no result: treat it as quit.
	if len(res.Files) == 0 {
		return markdown.StatusCancelled, nil, nil
	}
	return res.Files[0].Status, res.Files[0].Review, nil
}

// appendReview adds the review of f unless f was quit (skipped).
func appendReview(reviews []markdown.FileReview, f markdown.File, status markdown.Status, review *markdown.ReviewResult) []markdown.FileReview {
	if status == markdown.StatusCancelled {
		return reviews
	}
	return append(reviews, markdown.FileReview{Path: f.Path, Doc: f.Doc, Review: review})
}

// runDiff reviews local git changes to Markdown files in the diff view.
// Without explicit files, the changed .md files are offered in a picker (the
// TUI one, or the browser's own with --web). The comments on every reviewed file
// are combined into one output.
func (r *ReviewCmd) runDiff() error {
	base := cmp.Or(r.Base, defaultBase)
	repo, err := gitdiff.Open(".")
	if err != nil {
		return err
	}
	if err := repo.VerifyRef(base); err != nil {
		return err
	}

	paths := r.Files
	var pick []string
	if len(paths) == 0 {
		changed, err := repo.ChangedMarkdownFiles(base)
		if err != nil {
			return err
		}
		if len(changed) == 0 {
			fmt.Fprintf(os.Stderr, "No changed Markdown files vs %s.\n", base)
			return nil
		}
		if r.Web {
			pick = changed
		} else {
			paths, err = pickFiles(changed, r.teaOpts)
			if err != nil || len(paths) == 0 {
				return err
			}
		}
	}

	var reviews []markdown.FileReview
	if r.Web {
		reviews, err = r.reviewDiffInBrowser(repo, base, web.Review{Pick: pick, Paths: paths})
	} else {
		reviews, err = r.reviewDiffInTUI(repo, base, paths)
	}
	if err != nil || len(reviews) == 0 {
		return err
	}

	var output string
	if len(reviews) == 1 {
		output = markdown.FormatReview(reviews[0].Review, reviews[0].Doc, reviews[0].Path)
	} else {
		output = markdown.FormatReviews(reviews)
	}
	if output == "" {
		fmt.Fprintln(os.Stderr, "Approved.")
		return nil
	}
	return writeReviewOutput(output, r.Output, r.OutputPath)
}

// loadDiffFile reads and parses path with its patch against base. A file
// that cannot be loaded or has no changes is reported and ok is false.
func loadDiffFile(repo *gitdiff.Repo, base, path string) (markdown.File, bool) {
	patch, err := repo.FilePatch(base, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		return markdown.File{}, false
	}
	if patch == "" {
		fmt.Fprintf(os.Stderr, "No changes in %s vs %s.\n", path, base)
		return markdown.File{}, false
	}
	source, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: reading %s: %v\n", path, err)
		return markdown.File{}, false
	}
	doc, err := markdown.Parse(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: parsing %s: %v\n", path, err)
		return markdown.File{}, false
	}
	return markdown.File{Path: path, Doc: doc, Diff: diff.ParsePatch(patch)}, true
}

// reviewDiffInTUI reviews the files one after another. Each file is read
// when its turn comes, so edits made while reviewing an earlier file are
// picked up. A file quit in the TUI is skipped; the others are returned.
func (r *ReviewCmd) reviewDiffInTUI(repo *gitdiff.Repo, base string, paths []string) ([]markdown.FileReview, error) {
	var reviews []markdown.FileReview
	for _, path := range paths {
		f, ok := loadDiffFile(repo, base, path)
		if !ok {
			continue
		}
		app := tui.NewApp(f.Doc, tui.AppOptions{
			Theme:     r.Theme,
			FilePath:  f.Path,
			MultiFile: len(paths) > 1,
			Diff:      tui.NewDiffData(f.Doc, f.Diff),
		})
		result, err := runReviewApp(app, r.teaOpts)
		if err != nil {
			return nil, err
		}
		reviews = appendReview(reviews, f, result.Status, result.Review)
	}
	return reviews, nil
}

// reviewDiffInBrowser reviews the files in the browser, one after another
// like the TUI: each is loaded when its turn comes, and a file quit
// (skipped) there is left out.
func (r *ReviewCmd) reviewDiffInBrowser(repo *gitdiff.Repo, base string, review web.Review) ([]markdown.FileReview, error) {
	review.Load = func(path string) (markdown.File, bool) { return loadDiffFile(repo, base, path) }
	res, err := r.serveWeb(review)
	if err != nil {
		return nil, err
	}
	var reviews []markdown.FileReview
	for _, f := range res.Files {
		reviews = appendReview(reviews, f.File, f.Status, f.Review)
	}
	return reviews, nil
}

// serveWeb runs a browser review until every file is finished or skipped
// there. Interrupting the command abandons the session with no result.
func (r *ReviewCmd) serveWeb(review web.Review) (web.Result, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	opts := web.Options{Port: r.Port, Theme: r.Theme, Log: os.Stderr}
	if !r.NoOpen {
		opts.Open = web.OpenBrowser
	}
	return web.Serve(ctx, review, opts)
}
