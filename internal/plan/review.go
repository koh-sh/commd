package plan

import (
	"fmt"
	"strings"
)

// FormatReview formats a ReviewResult as a Markdown string.
// Comments on the same step are grouped under a single heading.
// Format: ## {StepID}: {StepTitle}\n[{ActionType}] {Body}
func FormatReview(result *ReviewResult, p *Plan, filePath string) string {
	if len(result.Comments) == 0 {
		return ""
	}

	target := filePath
	if target == "" {
		target = "the file"
	}

	// Group comments by StepID, preserving first-seen order.
	type group struct {
		title    string
		comments []ReviewComment
	}
	var order []string
	groups := make(map[string]*group)

	for _, c := range result.Comments {
		g, ok := groups[c.StepID]
		if !ok {
			step := p.FindStep(c.StepID)
			title := c.StepID
			if step != nil {
				title = fmt.Sprintf("%s: %s", c.StepID, step.Title)
			}
			g = &group{title: title}
			groups[c.StepID] = g
			order = append(order, c.StepID)
		}
		g.comments = append(g.comments, c)
	}

	var sb strings.Builder
	sb.WriteString("# Plan Review\n\n")
	fmt.Fprintf(&sb, "Please review and address the following comments on: %s\n", target)

	for _, id := range order {
		g := groups[id]
		fmt.Fprintf(&sb, "\n## %s\n", g.title)
		for _, c := range g.comments {
			fmt.Fprintf(&sb, "[%s] %s\n", c.FormatLabel(), c.Body)
		}
	}

	return sb.String()
}
