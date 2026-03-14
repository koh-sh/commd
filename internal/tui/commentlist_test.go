package tui

import (
	"strings"
	"testing"

	"github.com/koh-sh/commd/internal/markdown"
)

func TestCommentListOpenClose(t *testing.T) {
	cl := NewCommentList()

	comments := []*markdown.ReviewComment{
		{SectionID: "S1", Action: markdown.ActionSuggestion, Body: "first"},
		{SectionID: "S1", Action: markdown.ActionIssue, Body: "second"},
	}

	cl.Open("S1", comments)
	if cl.SectionID() != "S1" {
		t.Errorf("sectionID = %s, want S1", cl.SectionID())
	}
	if cl.Cursor() != 0 {
		t.Errorf("cursor = %d, want 0", cl.Cursor())
	}

	cl.Close()
	if cl.comments != nil {
		t.Error("comments should be nil after Close")
	}
	if cl.Cursor() != 0 {
		t.Error("cursor should be reset to 0 after Close")
	}
}

func TestCommentListCursorAdjustOnOpen(t *testing.T) {
	cl := NewCommentList()

	// Set cursor beyond range
	cl.cursor = 5
	comments := []*markdown.ReviewComment{
		{Body: "only one"},
	}
	cl.Open("S1", comments)
	if cl.Cursor() != 0 {
		t.Errorf("cursor should be adjusted to 0, got %d", cl.Cursor())
	}
}

func TestCommentListCursorUpDown(t *testing.T) {
	cl := NewCommentList()
	comments := []*markdown.ReviewComment{
		{Body: "first"},
		{Body: "second"},
		{Body: "third"},
	}
	cl.Open("S1", comments)

	// At top, CursorUp should stay
	cl.CursorUp()
	if cl.Cursor() != 0 {
		t.Errorf("CursorUp at top: cursor = %d, want 0", cl.Cursor())
	}

	// Move down
	cl.CursorDown()
	if cl.Cursor() != 1 {
		t.Errorf("CursorDown: cursor = %d, want 1", cl.Cursor())
	}

	cl.CursorDown()
	if cl.Cursor() != 2 {
		t.Errorf("CursorDown: cursor = %d, want 2", cl.Cursor())
	}

	// At bottom, CursorDown should stay
	cl.CursorDown()
	if cl.Cursor() != 2 {
		t.Errorf("CursorDown at bottom: cursor = %d, want 2", cl.Cursor())
	}

	// Move up
	cl.CursorUp()
	if cl.Cursor() != 1 {
		t.Errorf("CursorUp: cursor = %d, want 1", cl.Cursor())
	}
}

func TestCommentListOpenEmptyComments(t *testing.T) {
	cl := NewCommentList()
	cl.cursor = 3
	cl.Open("S1", []*markdown.ReviewComment{})
	if cl.Cursor() != 0 {
		t.Errorf("cursor = %d, want 0 for empty comments", cl.Cursor())
	}
}

func TestCommentListRenderWithDecoration(t *testing.T) {
	cl := NewCommentList()
	comments := []*markdown.ReviewComment{
		{SectionID: "S1", Action: markdown.ActionSuggestion, Decoration: markdown.DecorationNonBlocking, Body: "decorated comment"},
	}
	cl.Open("S1", comments)

	styles := defaultStyles()
	output := cl.Render(80, 20, styles)

	if !strings.Contains(output, "non-blocking") {
		t.Error("render should contain decoration")
	}
	if !strings.Contains(output, "decorated comment") {
		t.Error("render should contain comment body")
	}
}

func TestCommentListRender(t *testing.T) {
	cl := NewCommentList()
	comments := []*markdown.ReviewComment{
		{SectionID: "S1", Action: markdown.ActionSuggestion, Body: "first comment"},
		{SectionID: "S1", Action: markdown.ActionIssue, Body: "second comment"},
	}
	cl.Open("S1", comments)

	styles := defaultStyles()
	output := cl.Render(80, 20, styles)

	if !strings.Contains(output, "S1") {
		t.Error("render should contain section ID")
	}
	if !strings.Contains(output, ">") {
		t.Error("render should contain cursor marker")
	}
	if !strings.Contains(output, "first comment") {
		t.Error("render should contain comment body")
	}
}
