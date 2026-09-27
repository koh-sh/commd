package markdown

import (
	"fmt"
	"strings"
)

// maxQuoteLines caps how many quoted source lines follow a line-level
// comment. Enough to locate the target; a long visual selection is elided.
const maxQuoteLines = 5

// reviewHeader is the opening of every review output.
const reviewHeader = "# Review\n\n"

// FormatReview formats a ReviewResult for a single file as a Markdown string.
// Section-level comments are grouped under section headings.
// Line-level comments are listed separately by line number.
func FormatReview(result *ReviewResult, d *Document, filePath string) string {
	if len(result.Comments) == 0 {
		return ""
	}

	target := filePath
	if target == "" {
		target = "the file"
	}

	var sb strings.Builder
	sb.WriteString(reviewHeader)
	fmt.Fprintf(&sb, "Please review and address the following comments on: %s\n", target)
	writeComments(&sb, result, d, 2)
	return sb.String()
}

// FormatReviews formats reviews for several files as one Markdown document.
// Each file gets its own "## path" heading with section headings nested one
// level deeper. Files without comments are omitted; returns "" if none have any.
func FormatReviews(files []FileResult) string {
	var sb strings.Builder
	for _, f := range files {
		if f.Review == nil || len(f.Review.Comments) == 0 {
			continue
		}
		if sb.Len() == 0 {
			sb.WriteString(reviewHeader)
			sb.WriteString("Please review and address the following comments.\n")
		}
		fmt.Fprintf(&sb, "\n## %s\n", f.Path)
		writeComments(&sb, f.Review, f.Doc, 3)
	}
	return sb.String()
}

// writeComments writes the section-level groups and line-level list for one
// file. headingLevel is the Markdown heading level used for section groups.
func writeComments(sb *strings.Builder, result *ReviewResult, d *Document, headingLevel int) {
	// Separate section-level and line-level comments.
	type group struct {
		title    string
		comments []ReviewComment
	}
	var sectionOrder []string
	sectionGroups := make(map[string]*group)
	var lineComments []ReviewComment

	for _, c := range result.Comments {
		if c.StartLine > 0 {
			lineComments = append(lineComments, c)
			continue
		}
		g, ok := sectionGroups[c.SectionID]
		if !ok {
			var title string
			if c.SectionID == OverviewSectionID {
				title = OverviewTitle
			} else {
				section := d.FindSection(c.SectionID)
				title = c.SectionID
				if section != nil {
					title = fmt.Sprintf("%s: %s", c.SectionID, section.Title)
				}
			}
			g = &group{title: title}
			sectionGroups[c.SectionID] = g
			sectionOrder = append(sectionOrder, c.SectionID)
		}
		g.comments = append(g.comments, c)
	}

	heading := strings.Repeat("#", headingLevel)

	// Section-level comments grouped by section
	for _, id := range sectionOrder {
		g := sectionGroups[id]
		fmt.Fprintf(sb, "\n%s %s\n", heading, g.title)
		for _, c := range g.comments {
			fmt.Fprintf(sb, "[%s] %s\n", c.FormatLabel(), c.Body)
			// A line comment whose lines were edited away keeps its quote.
			writeQuote(sb, c.Quote)
		}
	}

	// Line-level comments with inline code decoration, separated by a divider
	if len(lineComments) > 0 {
		sb.WriteString("\n---\n")
		for _, c := range lineComments {
			fmt.Fprintf(sb, "\n`%s` [%s] %s\n", c.DisplayLineRef(), c.FormatLabel(), c.Body)
			writeQuote(sb, c.Quote)
		}
	}
}

// writeQuote writes the quoted source lines as a Markdown blockquote,
// eliding beyond maxQuoteLines.
func writeQuote(sb *strings.Builder, quote []string) {
	for i, line := range quote {
		if i == maxQuoteLines {
			sb.WriteString("> ...\n")
			return
		}
		fmt.Fprintf(sb, "> %s\n", line)
	}
}
