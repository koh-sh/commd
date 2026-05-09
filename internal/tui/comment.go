package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/koh-sh/commd/internal/markdown"
)

// CommentEditor wraps a textarea for entering review comments.
type CommentEditor struct {
	textarea   textarea.Model
	sectionID  string
	labelIndex int    // index into markdown.ActionLabels
	decoIndex  int    // index into markdown.DecorationLabels
	startLine  int    // 1-based start line (0 = section-level)
	endLine    int    // 1-based end line (0 = single line)
	side       string // "RIGHT" or "LEFT" (for PR diff)
}

// NewCommentEditor creates a new CommentEditor.
func NewCommentEditor() *CommentEditor {
	ta := textarea.New()
	ta.Placeholder = "Enter review comment... (Ctrl+S to save, Esc to cancel)"
	ta.ShowLineNumbers = false
	ta.SetHeight(5)
	ta.CharLimit = 0
	// Use virtual cursor (rendered as a character within the content) so the
	// cursor stays inside the textarea region instead of being placed at the
	// program's absolute (0,0) by Bubble Tea v2's real-cursor default.
	ta.SetVirtualCursor(true)

	return &CommentEditor{
		textarea: ta,
	}
}

// Open opens the comment editor for a section, optionally pre-filling with existing comment.
func (c *CommentEditor) Open(sectionID string, existing *markdown.ReviewComment) tea.Cmd {
	c.sectionID = sectionID

	if existing != nil {
		c.labelIndex = c.labelIndexFor(existing.Action)
		c.decoIndex = c.decorationIndexFor(existing.Decoration)
		c.textarea.SetValue(existing.Body)
		c.startLine = existing.StartLine
		c.endLine = existing.EndLine
	} else {
		c.labelIndex = c.labelIndexFor(markdown.DefaultAction)
		c.decoIndex = 0
		c.textarea.SetValue("")
		c.startLine = 0
		c.endLine = 0
	}

	return c.textarea.Focus()
}

// OpenWithLines opens the comment editor for a new line-level comment.
// existing must be nil; use Open directly when editing existing comments.
func (c *CommentEditor) OpenWithLines(sectionID string, existing *markdown.ReviewComment, startLine, endLine int, side string) tea.Cmd {
	cmd := c.Open(sectionID, existing)
	c.startLine = startLine
	c.endLine = endLine
	c.side = side
	return cmd
}

// labelIndexFor returns the index of the given action in ActionLabels.
func (c *CommentEditor) labelIndexFor(action markdown.ActionType) int {
	return indexInSlice(markdown.ActionLabels, action)
}

// Close closes the comment editor.
func (c *CommentEditor) Close() {
	c.textarea.Blur()
}

// SectionID returns the section ID being edited.
func (c *CommentEditor) SectionID() string {
	return c.sectionID
}

// Label returns the current action label.
func (c *CommentEditor) Label() markdown.ActionType {
	return markdown.ActionLabels[c.labelIndex]
}

// CycleLabel cycles to the next action label.
func (c *CommentEditor) CycleLabel() {
	c.labelIndex = (c.labelIndex + 1) % len(markdown.ActionLabels)
}

// CycleLabelReverse cycles to the previous action label.
func (c *CommentEditor) CycleLabelReverse() {
	c.labelIndex = (c.labelIndex - 1 + len(markdown.ActionLabels)) % len(markdown.ActionLabels)
}

// DecorationLabel returns the current decoration.
func (c *CommentEditor) DecorationLabel() markdown.Decoration {
	return markdown.DecorationLabels[c.decoIndex]
}

// CycleDecoration cycles to the next decoration.
func (c *CommentEditor) CycleDecoration() {
	c.decoIndex = (c.decoIndex + 1) % len(markdown.DecorationLabels)
}

// FormatLabel returns the combined action and decoration label for display.
func (c *CommentEditor) FormatLabel() string {
	return markdown.FormatActionLabel(
		markdown.ActionLabels[c.labelIndex],
		markdown.DecorationLabels[c.decoIndex],
	)
}

// FormatLineRef returns a line reference string for display.
func (c *CommentEditor) FormatLineRef() string {
	return markdown.FormatLineRef(c.startLine, c.endLine)
}

// decorationIndexFor returns the index of the given decoration in DecorationLabels.
func (c *CommentEditor) decorationIndexFor(deco markdown.Decoration) int {
	return indexInSlice(markdown.DecorationLabels, deco)
}

// indexInSlice returns the index of val in slice, or 0 if not found.
func indexInSlice[T comparable](slice []T, val T) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return 0
}

// Result returns the review comment from the editor content.
// Returns nil if the body is empty.
func (c *CommentEditor) Result() *markdown.ReviewComment {
	body := strings.TrimSpace(c.textarea.Value())

	if body == "" {
		return nil
	}

	return &markdown.ReviewComment{
		SectionID:  c.sectionID,
		Action:     markdown.ActionLabels[c.labelIndex],
		Decoration: markdown.DecorationLabels[c.decoIndex],
		Body:       body,
		StartLine:  c.startLine,
		EndLine:    c.endLine,
		Side:       c.side,
	}
}

// Update handles tea messages for the textarea.
func (c *CommentEditor) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.textarea, cmd = c.textarea.Update(msg)
	return cmd
}

// View renders the comment editor.
func (c *CommentEditor) View() string {
	return c.textarea.View()
}

// SetWidth sets the width of the textarea.
func (c *CommentEditor) SetWidth(w int) {
	c.textarea.SetWidth(w)
}
