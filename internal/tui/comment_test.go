package tui

import (
	"testing"

	"github.com/koh-sh/ccplan/internal/plan"
)

func TestCommentEditorOpenClose(t *testing.T) {
	ce := NewCommentEditor()

	ce.Open("S1", nil)
	if ce.StepID() != "S1" {
		t.Errorf("stepID = %s, want S1", ce.StepID())
	}

	ce.Close()
}

func TestCommentEditorOpenExisting(t *testing.T) {
	ce := NewCommentEditor()

	existing := &plan.ReviewComment{
		StepID: "S1",
		Action: plan.ActionIssue,
		Body:   "existing comment",
	}
	ce.Open("S1", existing)

	if ce.Label() != plan.ActionIssue {
		t.Errorf("label = %s, want issue", ce.Label())
	}
}

func TestCommentEditorOpenNew(t *testing.T) {
	ce := NewCommentEditor()
	ce.Open("S1", nil)

	if ce.Label() != plan.ActionQuestion {
		t.Errorf("default label = %s, want question", ce.Label())
	}
}

func TestCommentEditorCycleLabel(t *testing.T) {
	ce := NewCommentEditor()
	ce.Open("S1", nil)

	labels := make([]plan.ActionType, 0)
	for range len(plan.ActionLabels) {
		labels = append(labels, ce.Label())
		ce.CycleLabel()
	}
	// Should have cycled through all labels
	if len(labels) != len(plan.ActionLabels) {
		t.Errorf("cycled %d labels, want %d", len(labels), len(plan.ActionLabels))
	}
	// After full cycle, should be back to default
	if ce.Label() != plan.DefaultAction {
		t.Errorf("after full cycle, label = %s, want %s", ce.Label(), plan.DefaultAction)
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
	for range len(plan.ActionLabels) {
		ce.CycleLabelReverse()
	}
	if ce.Label() != initial {
		t.Errorf("after full reverse cycle: label = %s, want %s", ce.Label(), initial)
	}
}

func TestCommentEditorLabelIndexFor(t *testing.T) {
	ce := NewCommentEditor()

	tests := []struct {
		action plan.ActionType
		want   int
	}{
		{plan.ActionSuggestion, 0},
		{plan.ActionIssue, 1},
		{plan.ActionQuestion, 2},
		{plan.ActionType("unknown"), 0},
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
		if result.StepID != "S1" {
			t.Errorf("stepID = %s, want S1", result.StepID)
		}
		if result.Body != "test comment" {
			t.Errorf("body = %s, want 'test comment'", result.Body)
		}
		if result.Action != plan.ActionQuestion {
			t.Errorf("action = %s, want question", result.Action)
		}
		if result.Decoration != plan.DecorationNone {
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
	if ce.DecorationLabel() != plan.DecorationNone {
		t.Errorf("default decoration = %s, want empty", ce.DecorationLabel())
	}

	decos := make([]plan.Decoration, 0)
	for range len(plan.DecorationLabels) {
		decos = append(decos, ce.DecorationLabel())
		ce.CycleDecoration()
	}
	if len(decos) != len(plan.DecorationLabels) {
		t.Errorf("cycled %d decorations, want %d", len(decos), len(plan.DecorationLabels))
	}
	// After full cycle, should be back to DecorationNone
	if ce.DecorationLabel() != plan.DecorationNone {
		t.Errorf("after full cycle, decoration = %s, want empty", ce.DecorationLabel())
	}
}

func TestCommentEditorDecorationIndexFor(t *testing.T) {
	ce := NewCommentEditor()

	tests := []struct {
		deco plan.Decoration
		want int
	}{
		{plan.DecorationNone, 0},
		{plan.DecorationNonBlocking, 1},
		{plan.DecorationBlocking, 2},
		{plan.DecorationIfMinor, 3},
		{plan.Decoration("unknown"), 0},
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
		action plan.ActionType
		deco   plan.Decoration
		want   string
	}{
		{"no decoration", plan.ActionSuggestion, plan.DecorationNone, "suggestion"},
		{"with non-blocking", plan.ActionIssue, plan.DecorationNonBlocking, "issue (non-blocking)"},
		{"with blocking", plan.ActionSuggestion, plan.DecorationBlocking, "suggestion (blocking)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ce := NewCommentEditor()
			existing := &plan.ReviewComment{
				StepID:     "S1",
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

	existing := &plan.ReviewComment{
		StepID:     "S1",
		Action:     plan.ActionIssue,
		Decoration: plan.DecorationBlocking,
		Body:       "blocking comment",
	}
	ce.Open("S1", existing)

	if ce.Label() != plan.ActionIssue {
		t.Errorf("label = %s, want issue", ce.Label())
	}
	if ce.DecorationLabel() != plan.DecorationBlocking {
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
	if result.Decoration != plan.DecorationNonBlocking {
		t.Errorf("decoration = %s, want non-blocking", result.Decoration)
	}
}
