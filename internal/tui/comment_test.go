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
	t.Run("with text", func(t *testing.T) {
		ce := NewCommentEditor()
		ce.Open("S1", nil)
		ce.textarea.SetValue("test comment")

		result := ce.Result()
		if result == nil {
			t.Fatal("result should not be nil")
			return
		}
		if result.SectionID != "S1" {
			t.Errorf("sectionID = %s, want S1", result.SectionID)
		}
		if result.Body != "test comment" {
			t.Errorf("body = %s, want 'test comment'", result.Body)
		}
		if result.Action != markdown.ActionQuestion {
			t.Errorf("action = %s, want question", result.Action)
		}
		if result.Decoration != markdown.DecorationNone {
			t.Errorf("decoration = %s, want empty", result.Decoration)
		}
	})

	t.Run("empty text", func(t *testing.T) {
		ce := NewCommentEditor()
		ce.Open("S1", nil)
		ce.textarea.SetValue("")

		if ce.Result() != nil {
			t.Error("result should be nil for empty text")
		}
	})

	t.Run("whitespace only", func(t *testing.T) {
		ce := NewCommentEditor()
		ce.Open("S1", nil)
		ce.textarea.SetValue("   \n  ")

		if ce.Result() != nil {
			t.Error("result should be nil for whitespace only")
		}
	})
}

func TestCommentEditorCycleDecoration(t *testing.T) {
	ce := NewCommentEditor()
	ce.Open("S1", nil)

	// Default should be DecorationNone
	if ce.DecorationLabel() != markdown.DecorationNone {
		t.Errorf("default decoration = %s, want empty", ce.DecorationLabel())
	}

	decos := make([]markdown.Decoration, 0)
	for range len(markdown.DecorationLabels) {
		decos = append(decos, ce.DecorationLabel())
		ce.CycleDecoration()
	}
	if len(decos) != len(markdown.DecorationLabels) {
		t.Errorf("cycled %d decorations, want %d", len(decos), len(markdown.DecorationLabels))
	}
	// After full cycle, should be back to DecorationNone
	if ce.DecorationLabel() != markdown.DecorationNone {
		t.Errorf("after full cycle, decoration = %s, want empty", ce.DecorationLabel())
	}
}

func TestCommentEditorDecorationIndexFor(t *testing.T) {
	ce := NewCommentEditor()

	tests := []struct {
		deco markdown.Decoration
		want int
	}{
		{markdown.DecorationNone, 0},
		{markdown.DecorationNonBlocking, 1},
		{markdown.DecorationBlocking, 2},
		{markdown.DecorationIfMinor, 3},
		{markdown.Decoration("unknown"), 0},
	}

	for _, tt := range tests {
		got := ce.decorationIndexFor(tt.deco)
		if got != tt.want {
			t.Errorf("decorationIndexFor(%s) = %d, want %d", tt.deco, got, tt.want)
		}
	}
}

func TestCommentEditorFormatLabel(t *testing.T) {
	tests := []struct {
		name   string
		action markdown.ActionType
		deco   markdown.Decoration
		want   string
	}{
		{"no decoration", markdown.ActionSuggestion, markdown.DecorationNone, "suggestion"},
		{"with non-blocking", markdown.ActionIssue, markdown.DecorationNonBlocking, "issue (non-blocking)"},
		{"with blocking", markdown.ActionSuggestion, markdown.DecorationBlocking, "suggestion (blocking)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ce := NewCommentEditor()
			existing := &markdown.ReviewComment{
				SectionID:  "S1",
				Action:     tt.action,
				Decoration: tt.deco,
				Body:       "body",
			}
			ce.Open("S1", existing)
			got := ce.FormatLabel()
			if got != tt.want {
				t.Errorf("FormatLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommentEditorOpenExistingWithDecoration(t *testing.T) {
	ce := NewCommentEditor()

	existing := &markdown.ReviewComment{
		SectionID:  "S1",
		Action:     markdown.ActionIssue,
		Decoration: markdown.DecorationBlocking,
		Body:       "blocking comment",
	}
	ce.Open("S1", existing)

	if ce.Label() != markdown.ActionIssue {
		t.Errorf("label = %s, want issue", ce.Label())
	}
	if ce.DecorationLabel() != markdown.DecorationBlocking {
		t.Errorf("decoration = %s, want blocking", ce.DecorationLabel())
	}
}

func TestCommentEditorResultWithDecoration(t *testing.T) {
	ce := NewCommentEditor()
	ce.Open("S1", nil)
	ce.textarea.SetValue("test comment")
	ce.CycleDecoration() // None -> non-blocking

	result := ce.Result()
	if result == nil {
		t.Fatal("result should not be nil")
		return
	}
	if result.Decoration != markdown.DecorationNonBlocking {
		t.Errorf("decoration = %s, want non-blocking", result.Decoration)
	}
}
