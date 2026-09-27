package github

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v84/github"
	"github.com/koh-sh/commd/internal/diff"
	"github.com/koh-sh/commd/internal/markdown"
)

// PRReviewComment represents a single inline comment on a PR.
type PRReviewComment struct {
	Path      string
	Body      string
	Line      int
	StartLine int    // 0 = single line
	Side      string // "RIGHT" or "LEFT"
}

// MapComment converts a commd ReviewComment to a GitHub PR review comment.
// Overview (file-level) comments are skipped — not supported by the GitHub API.
func MapComment(c markdown.ReviewComment, path string, doc *markdown.Document) *PRReviewComment {
	if c.SectionID == markdown.OverviewSectionID {
		return nil
	}

	body := formatCommentBody(c)

	side := diff.SideRight
	if c.Side != "" {
		side = c.Side
	}

	// Line-level comment
	if c.StartLine > 0 {
		rc := &PRReviewComment{
			Path: path,
			Body: body,
			Line: c.StartLine,
			Side: side,
		}
		if c.EndLine > 0 && c.EndLine != c.StartLine {
			rc.StartLine = c.StartLine
			rc.Line = c.EndLine
		}
		return rc
	}

	// Section-level comment: map to section heading line
	section := doc.FindSection(c.SectionID)
	if section == nil || section.StartLine == 0 {
		return nil
	}

	return &PRReviewComment{
		Path: path,
		Body: body,
		Line: section.StartLine,
		Side: side,
	}
}

// BuildPRReview builds a GitHub PR review request from file review results.
func BuildPRReview(results []markdown.FileResult, event, body string) *gh.PullRequestReviewRequest {
	var comments []*gh.DraftReviewComment

	for _, fr := range results {
		if fr.Review == nil {
			continue
		}
		for _, c := range fr.Review.Comments {
			mapped := MapComment(c, fr.Path, fr.Doc)
			if mapped == nil {
				continue
			}
			dc := &gh.DraftReviewComment{
				Path: new(mapped.Path),
				Body: new(mapped.Body),
				Line: new(mapped.Line),
				Side: new(mapped.Side),
			}
			if mapped.StartLine > 0 {
				dc.StartLine = new(mapped.StartLine)
				dc.StartSide = new(mapped.Side)
			}
			comments = append(comments, dc)
		}
	}

	review := &gh.PullRequestReviewRequest{
		Event:    new(event),
		Comments: comments,
	}
	if body != "" {
		review.Body = new(body)
	}
	return review
}

// SubmitReview posts a PR review with inline comments.
func (c *Client) SubmitReview(ctx context.Context, ref *PRRef, review *gh.PullRequestReviewRequest) error {
	_, _, err := c.inner.PullRequests.CreateReview(ctx, ref.Owner, ref.Repo, ref.Number, review)
	if err != nil {
		return fmt.Errorf("submitting PR review: %w", err)
	}
	return nil
}

// formatCommentBody formats a ReviewComment body for GitHub display.
func formatCommentBody(c markdown.ReviewComment) string {
	return fmt.Sprintf("**[%s]** %s", c.FormatLabel(), c.Body)
}
