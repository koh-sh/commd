package tui

import (
	"strings"
	"testing"

	"github.com/koh-sh/ccplan/internal/plan"
)

func TestCommentListOpenClose(t *testing.T) {
	cl := NewCommentList()

	comments := []*plan.ReviewComment{
		{StepID: "S1", Action: plan.ActionSuggestion, Body: "first"},
		{StepID: "S1", Action: plan.ActionIssue, Body: "second"},
	}

	cl.Open("S1", comments)
	if cl.StepID() != "S1" {
		t.Errorf("stepID = %s, want S1", cl.StepID())
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
	comments := []*plan.ReviewComment{
		{Body: "only one"},
	}
	cl.Open("S1", comments)
	if cl.Cursor() != 0 {
		t.Errorf("cursor should be adjusted to 0, got %d", cl.Cursor())
	}
}

func TestCommentListCursorUpDown(t *testing.T) {
	cl := NewCommentList()
	comments := []*plan.ReviewComment{
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
	cl.Open("S1", []*plan.ReviewComment{})
	if cl.Cursor() != 0 {
		t.Errorf("cursor = %d, want 0 for empty comments", cl.Cursor())
	}
}

func TestCommentListRenderWithDecoration(t *testing.T) {
	cl := NewCommentList()
	comments := []*plan.ReviewComment{
		{StepID: "S1", Action: plan.ActionSuggestion, Decoration: plan.DecorationNonBlocking, Body: "decorated comment"},
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
	comments := []*plan.ReviewComment{
		{StepID: "S1", Action: plan.ActionSuggestion, Body: "first comment"},
		{StepID: "S1", Action: plan.ActionIssue, Body: "second comment"},
	}
	cl.Open("S1", comments)

	styles := defaultStyles()
	output := cl.Render(80, 20, styles)

	if !strings.Contains(output, "S1") {
		t.Error("render should contain step ID")
	}
	if !strings.Contains(output, ">") {
		t.Error("render should contain cursor marker")
	}
	if !strings.Contains(output, "first comment") {
		t.Error("render should contain comment body")
	}
}
