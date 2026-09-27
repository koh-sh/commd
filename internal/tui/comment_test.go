package tui

import (
	"testing"

	"github.com/koh-sh/commd/internal/markdown"
)

func TestCommentEditorOpenClose(t *testing.T) {
	ce := NewCommentEditor()

	ce.Open("S1", nil)
	if ce.SectionID() != "S1" {
		t.Errorf("sectionID = %s, want S1", ce.SectionID())
	}

	ce.Close()
}

func TestCommentEditorOpenExisting(t *testing.T) {
	ce := NewCommentEditor()

	existing := &markdown.ReviewComment{
		SectionID: "S1",
		Action:    markdown.ActionIssue,
		Body:      "existing comment",
	}
	ce.Open("S1", existing)

	if ce.Label() != markdown.ActionIssue {
		t.Errorf("label = %s, want issue", ce.Label())
	}
}

func TestCommentEditorOpenNew(t *testing.T) {
	ce := NewCommentEditor()
	ce.Open("S1", nil)

	if ce.Label() != markdown.ActionQuestion {
		t.Errorf("default label = %s, want question", ce.Label())
	}
}

func TestCommentEditorCycleLabel(t *testing.T) {
	ce := NewCommentEditor()
	ce.Open("S1", nil)

	labels := make([]markdown.ActionType, 0)
	for range len(markdown.ActionLabels) {
		labels = append(labels, ce.Label())
		ce.CycleLabel()
	}
	// Should have cycled through all labels
	if len(labels) != len(markdown.ActionLabels) {
		t.Errorf("cycled %d labels, want %d", len(labels), len(markdown.ActionLabels))
	}
	// After full cycle, should be back to default
	if ce.Label() != markdown.DefaultAction {
		t.Errorf("after full cycle, label = %s, want %s", ce.Label(), markdown.DefaultAction)
	}
}

func TestCommentEditorCycleLabelReverse(t *testing.T) {
	ce := NewCommentEditor()
	ce.Open("S1", nil)

	initial := ce.Label()
	ce.CycleLabelReverse()
	// Cycle forward should return to initial
	ce.CycleLabel()
	if ce.Label() != initial {
		t.Errorf("after reverse+forward: label = %s, want %s", ce.Label(), initial)
	}

	// Full reverse cycle should return to start
	for range len(markdown.ActionLabels) {
		ce.CycleLabelReverse()
	}
	if ce.Label() != initial {
		t.Errorf("after full reverse cycle: label = %s, want %s", ce.Label(), initial)
	}
}

func TestCommentEditorLabelIndexFor(t *testing.T) {
	ce := NewCommentEditor()

	tests := []struct {
		action markdown.ActionType
		want   int
	}{
		{markdown.ActionSuggestion, 0},
		{markdown.ActionIssue, 1},
		{markdown.ActionQuestion, 2},
		{markdown.ActionType("unknown"), 0},
	}

	for _, tt := range tests {
		got := ce.labelIndexFor(tt.action)
		if got != tt.want {
			t.Errorf("labelIndexFor(%s) = %d, want %d", tt.action, got, tt.want)
		}
	}
}

func TestCommentEditorResult(t *testing.T) {
	tests := []struct {
		name     string
		existing *markdown.ReviewComment
		wantID   string
		wantEdit bool
	}{
		{name: "new comment", wantID: "", wantEdit: false},
		{
			name:     "edited comment keeps its ID",
			existing: &markdown.ReviewComment{ID: "c3", SectionID: "S1", Action: markdown.ActionQuestion, Body: "old"},
			wantID:   "c3",
			wantEdit: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ce := NewCommentEditor()
			ce.Open("S1", tt.existing)
			ce.textarea.SetValue("test comment")

			result := ce.Result()
			if result.ID != tt.wantID || ce.IsEdit() != tt.wantEdit {
				t.Errorf("ID = %q, IsEdit() = %v, want %q, %v", result.ID, ce.IsEdit(), tt.wantID, tt.wantEdit)
			}
			if result.SectionID != "S1" || result.Body != "test comment" ||
				result.Action != markdown.ActionQuestion || result.Decoration != markdown.DecorationNone {
				t.Errorf("result = %+v", result)
			}
		})
	}
}

func TestCommentEditorSide(t *testing.T) {
	tests := []struct {
		name string
		// setup mutates the editor before the assertion to simulate a prior
		// editing session whose side must not leak into the next Open.
		setup    func(ce *CommentEditor)
		existing *markdown.ReviewComment
		want     string
	}{
		{
			name:     "new comment resets stale side",
			setup:    func(ce *CommentEditor) { ce.OpenWithLines(3, 0, "LEFT") },
			existing: nil,
			want:     "",
		},
		{
			name:     "editing loads existing side",
			setup:    func(ce *CommentEditor) { ce.OpenWithLines(3, 0, "RIGHT") },
			existing: &markdown.ReviewComment{SectionID: "S1", Action: markdown.ActionIssue, Body: "b", StartLine: 5, Side: "LEFT"},
			want:     "LEFT",
		},
		{
			name:     "editing comment without side clears stale side",
			setup:    func(ce *CommentEditor) { ce.OpenWithLines(3, 0, "RIGHT") },
			existing: &markdown.ReviewComment{SectionID: "S1", Action: markdown.ActionIssue, Body: "b"},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ce := NewCommentEditor()
			tt.setup(ce)

			ce.Open("S1", tt.existing)
			ce.textarea.SetValue("body")

			result := ce.Result()
			if result.Side != tt.want {
				t.Errorf("Side = %q, want %q", result.Side, tt.want)
			}
		})
	}
}

func TestCommentEditorResultWithDecoration(t *testing.T) {
	ce := NewCommentEditor()
	ce.Open("S1", nil)
	ce.textarea.SetValue("test comment")
	ce.CycleDecoration() // None -> non-blocking

	result := ce.Result()
	if result.Decoration != markdown.DecorationNonBlocking {
		t.Errorf("decoration = %s, want non-blocking", result.Decoration)
	}
}
