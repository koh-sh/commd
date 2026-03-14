package tui

import (
	"fmt"
	"strings"

	"github.com/koh-sh/commd/internal/markdown"
)

// CommentList manages the comment list view for a section.
type CommentList struct {
	sectionID string
	comments  []*markdown.ReviewComment
	cursor    int
}

// NewCommentList creates a new CommentList.
func NewCommentList() *CommentList {
	return &CommentList{}
}

// Open opens the comment list for a section.
func (cl *CommentList) Open(sectionID string, comments []*markdown.ReviewComment) {
	cl.sectionID = sectionID
	cl.comments = comments
	if cl.cursor >= len(comments) {
		cl.cursor = len(comments) - 1
	}
	if cl.cursor < 0 {
		cl.cursor = 0
	}
}

// Close closes the comment list.
func (cl *CommentList) Close() {
	cl.comments = nil
	cl.cursor = 0
}

// SectionID returns the section ID.
func (cl *CommentList) SectionID() string {
	return cl.sectionID
}

// Cursor returns the current cursor position.
func (cl *CommentList) Cursor() int {
	return cl.cursor
}

// CursorUp moves the cursor up.
func (cl *CommentList) CursorUp() {
	if cl.cursor > 0 {
		cl.cursor--
	}
}

// CursorDown moves the cursor down.
func (cl *CommentList) CursorDown() {
	if cl.cursor < len(cl.comments)-1 {
		cl.cursor++
	}
}

// Render renders the comment list.
func (cl *CommentList) Render(width, height int, styles Styles) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render(fmt.Sprintf("Comments on %s", cl.sectionID)))
	sb.WriteString("\n\n")

	for i, c := range cl.comments {
		prefix := "  "
		if i == cl.cursor {
			prefix = "> "
		}

		header := fmt.Sprintf("%s#%d [%s]", prefix, i+1, c.FormatLabel())
		if i == cl.cursor {
			sb.WriteString(styles.SelectedSection.Render(header))
		} else {
			sb.WriteString(styles.NormalSection.Render(header))
		}
		sb.WriteString("\n")

		// Show body preview (first line, truncated)
		if c.Body != "" {
			bodyLine := strings.SplitN(c.Body, "\n", 2)[0]
			bodyLine = truncate(bodyLine, width-6)
			sb.WriteString(styles.NormalSection.Render("    " + bodyLine))
			sb.WriteString("\n")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
