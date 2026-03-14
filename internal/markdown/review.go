package markdown

import (
	"fmt"
	"strings"
)

// FormatReview formats a ReviewResult as a Markdown string.
// Comments on the same section are grouped under a single heading.
// Format: ## {SectionID}: {SectionTitle}\n[{ActionType}] {Body}
func FormatReview(result *ReviewResult, d *Document, filePath string) string {
	if len(result.Comments) == 0 {
		return ""
	}

	target := filePath
	if target == "" {
		target = "the file"
	}

	// Group comments by SectionID, preserving first-seen order.
	type group struct {
		title    string
		comments []ReviewComment
	}
	var order []string
	groups := make(map[string]*group)

	for _, c := range result.Comments {
		g, ok := groups[c.SectionID]
		if !ok {
			section := d.FindSection(c.SectionID)
			title := c.SectionID
			if section != nil {
				title = fmt.Sprintf("%s: %s", c.SectionID, section.Title)
			}
			g = &group{title: title}
			groups[c.SectionID] = g
			order = append(order, c.SectionID)
		}
		g.comments = append(g.comments, c)
	}

	var sb strings.Builder
	sb.WriteString("# Review\n\n")
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
