package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/koh-sh/commd/internal/markdown"
)

// CommentEditor wraps a textarea for entering review comments.
type CommentEditor struct {
	textarea   textarea.Model
	sectionID  string
	labelIndex int // index into markdown.ActionLabels
	decoIndex  int // index into markdown.DecorationLabels
}

// NewCommentEditor creates a new CommentEditor.
func NewCommentEditor() *CommentEditor {
	ta := textarea.New()
	ta.Placeholder = "Enter review comment... (Ctrl+S to save, Esc to cancel)"
	ta.ShowLineNumbers = false
	ta.SetHeight(5)
	ta.CharLimit = 0

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
	} else {
		c.labelIndex = c.labelIndexFor(markdown.DefaultAction)
		c.decoIndex = 0
		c.textarea.SetValue("")
	}

	return c.textarea.Focus()
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
