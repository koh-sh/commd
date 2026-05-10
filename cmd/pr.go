package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"

	tea "charm.land/bubbletea/v2"
	ghclient "github.com/koh-sh/commd/internal/github"
	"github.com/koh-sh/commd/internal/markdown"
	"github.com/koh-sh/commd/internal/tui"
)

// Validate ensures the PR URL is well-formed before Run is invoked.
func (p *PRCmd) Validate() error {
	_, err := ghclient.ParsePRURL(p.URL)
	return err
}

// Run executes the pr subcommand. The GitHub client is provided by Kong via
// BindToProvider; tests may call Run with a stub client directly. URL is
// guaranteed parseable here because Validate runs first.
func (p *PRCmd) Run(client *ghclient.Client) error {
	ctx := context.Background()

	ref, err := ghclient.ParsePRURL(p.URL)
	if err != nil {
		return err
	}

	// List changed .md files
	mdFiles, err := client.ListMDFiles(ctx, ref)
	if err != nil {
		return err
	}
	if len(mdFiles) == 0 {
		fmt.Fprintf(os.Stderr, "No Markdown files changed in PR #%d.\n", ref.Number)
		return nil
	}

	// Build path list and patch map
	paths := make([]string, len(mdFiles))
	patches := make(map[string]string)
	for i, f := range mdFiles {
		paths[i] = f.Path
		patches[f.Path] = f.Patch
	}

	// Select files to review
	var selectedPaths []string
	if p.File != "" {
		if !slices.Contains(paths, p.File) {
			return fmt.Errorf("file %q not found in PR #%d changed files", p.File, ref.Number)
		}
		selectedPaths = []string{p.File}
	} else {
		picker := tui.NewFilePicker(paths)
		finalModel, err := runTea(picker, p.teaOpts)
		if err != nil {
			return fmt.Errorf("running file picker: %w", err)
		}
		fp, ok := finalModel.(*tui.FilePicker)
		if !ok {
			return fmt.Errorf("unexpected model type: %T", finalModel)
		}
		result := fp.Result()
		if result.Cancelled || len(result.SelectedFiles) == 0 {
			return nil
		}
		selectedPaths = result.SelectedFiles
	}

	// Resolve head SHA once for all file fetches
	headSHA, err := client.GetHeadSHA(ctx, ref)
	if err != nil {
		return err
	}

	// Review each file
	var results []ghclient.FileReviewResult

	for i, path := range selectedPaths {
		fmt.Fprintf(os.Stderr, "Fetching %s (%d/%d)...\n", path, i+1, len(selectedPaths))

		source, err := client.FetchFileContent(ctx, ref, path, headSHA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			continue
		}

		doc, err := markdown.Parse(source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: parsing %s: %v\n", path, err)
			continue
		}

		// Parse diff for this file
		var diffData *tui.DiffData
		if patch := patches[path]; patch != "" {
			if diffInfo := ghclient.ParsePatch(patch); diffInfo != nil {
				lineMap, sideMap, typeMap := diffInfo.LineSideMap()
				diffData = &tui.DiffData{
					DisplayLines: diffInfo.FormatDiffLines(),
					LineMap:      lineMap,
					SideMap:      sideMap,
					TypeMap:      typeMap,
				}
			}
		}

		app := tui.NewApp(doc, tui.AppOptions{
			Theme:    p.Theme,
			FilePath: path,
			PRMode:   true,
			Diff:     diffData,
		})
		finalModel, err := runTea(app, p.teaOpts)
		if err != nil {
			return fmt.Errorf("running TUI for %s: %w", path, err)
		}

		reviewApp, ok := finalModel.(*tui.App)
		if !ok {
			return fmt.Errorf("unexpected model type: %T", finalModel)
		}

		appResult := reviewApp.Result()

		// Submitted or Approved = done with this file, Cancelled = skipped
		if appResult.Status == markdown.StatusSubmitted || appResult.Status == markdown.StatusApproved {
			results = append(results, ghclient.FileReviewResult{
				Path:   path,
				Doc:    doc,
				Review: appResult.Review,
			})
		}
	}

	// Show final dialog only if at least one file was reviewed (not all skipped)
	if len(results) == 0 {
		return nil
	}
	return p.showFinalDialog(ctx, client, ref, results)
}

// showFinalDialog shows the post-review dialog after all files have been reviewed.
func (p *PRCmd) showFinalDialog(ctx context.Context, client *ghclient.Client, ref *ghclient.PRRef, results []ghclient.FileReviewResult) error {
	// Build summary lines
	var summary []string
	totalComments := 0
	for _, r := range results {
		if r.Review == nil {
			continue
		}
		count := len(r.Review.Comments)
		totalComments += count
		if count > 0 {
			summary = append(summary, fmt.Sprintf("%s: %d comment(s)", r.Path, count))
		}
	}
	hasComments := totalComments > 0
	if hasComments {
		summary = append(summary, fmt.Sprintf("Total: %d comment(s)", totalComments))
	} else {
		summary = append(summary, "No comments")
	}

	dialog := tui.NewReviewDialog(summary, hasComments)
	finalModel, err := runTea(dialog, p.teaOpts)
	if err != nil {
		return fmt.Errorf("running review dialog: %w", err)
	}

	rd, ok := finalModel.(*tui.ReviewDialog)
	if !ok {
		return fmt.Errorf("unexpected model type: %T", finalModel)
	}
	result := rd.Result()
	switch result.Action {
	case tui.ReviewActionApprove:
		return p.submitReview(ctx, client, ref, results, "APPROVE", result.Body)
	case tui.ReviewActionComment:
		return p.submitReview(ctx, client, ref, results, "COMMENT", result.Body)
	default:
		fmt.Fprintln(os.Stderr, "Review cancelled.")
		return nil
	}
}

func (p *PRCmd) submitReview(ctx context.Context, client *ghclient.Client, ref *ghclient.PRRef, results []ghclient.FileReviewResult, event, body string) error {
	// Warn about overview comments (filtered by BuildPRReview/MapComment, not supported as inline PR comments)
	for _, r := range results {
		if r.Review == nil {
			continue
		}
		for _, c := range r.Review.Comments {
			if c.SectionID == markdown.OverviewSectionID {
				fmt.Fprintf(os.Stderr, "Warning: overview comment on %s skipped (not supported as inline PR comment)\n", r.Path)
			}
		}
	}

	review := ghclient.BuildPRReview(results, event, body)

	// Check if any comments remain after filtering
	if len(review.Comments) == 0 && body == "" && event == "COMMENT" {
		fmt.Fprintln(os.Stderr, "No comments to submit (all were overview-level).")
		return nil
	}

	if err := client.SubmitReview(ctx, ref, review); err != nil {
		// Fallback: print review to stderr to prevent data loss
		fmt.Fprintf(os.Stderr, "Review content:\n")
		for _, r := range results {
			if r.Review != nil {
				output := markdown.FormatReview(r.Review, r.Doc, r.Path)
				fmt.Fprint(os.Stderr, output)
			}
		}
		return err
	}

	if event == "APPROVE" {
		fmt.Fprintf(os.Stderr, "PR #%d approved.\n", ref.Number)
	} else {
		fmt.Fprintf(os.Stderr, "Review submitted to PR #%d.\n", ref.Number)
	}
	return nil
}

// runTea creates and runs a Bubble Tea program with alt screen and optional extra options.
// Alt screen mode is set via the View.AltScreen field in each model's View() method.
func runTea(model tea.Model, extraOpts []tea.ProgramOption) (tea.Model, error) {
	// Reset Line Feed/New Line Mode (LNM) so bare LF moves cursor down without
	// resetting the column. The renderer's cursor-down-and-back diff updates
	// rely on this; some terminal emulators default LNM to on, which makes
	// those updates land at the wrong column. VT100-compliant terminals already
	// have LNM off, so this is a no-op there.
	fmt.Print("\x1b[20l")
	return tea.NewProgram(model, extraOpts...).Run()
}
