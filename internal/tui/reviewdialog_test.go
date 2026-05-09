package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestReviewDialog(t *testing.T) {
	tests := []struct {
		name        string
		summary     []string
		hasComments bool
		keys        []tea.KeyPressMsg
		wantAction  ReviewAction
		wantBody    string
	}{
		{
			name:        "no comments select approve",
			summary:     []string{"No comments"},
			hasComments: false,
			keys:        []tea.KeyPressMsg{keyMsg("enter"), keyMsg("ctrl+s")},
			wantAction:  ReviewActionApprove,
		},
		{
			name:        "no comments select exit",
			summary:     []string{"No comments"},
			hasComments: false,
			keys:        []tea.KeyPressMsg{keyMsg("j"), keyMsg("enter")},
			wantAction:  ReviewActionExit,
		},
		{
			name:        "has comments select comment with body",
			summary:     []string{"file.md: 2 comment(s)", "Total: 2 comment(s)"},
			hasComments: true,
			keys:        []tea.KeyPressMsg{keyMsg("enter"), keyMsg("x"), keyMsg("ctrl+s")},
			wantAction:  ReviewActionComment,
			wantBody:    "x",
		},
		{
			name:        "has comments select approve",
			summary:     []string{"file.md: 1 comment(s)"},
			hasComments: true,
			keys:        []tea.KeyPressMsg{keyMsg("j"), keyMsg("enter"), keyMsg("ctrl+s")},
			wantAction:  ReviewActionApprove,
		},
		{
			name:        "cancel with q",
			summary:     []string{"No comments"},
			hasComments: false,
			keys:        []tea.KeyPressMsg{keyMsg("q")},
			wantAction:  ReviewActionExit,
		},
		{
			name:        "cancel with esc",
			summary:     []string{"No comments"},
			hasComments: false,
			keys:        []tea.KeyPressMsg{keyMsg("esc")},
			wantAction:  ReviewActionExit,
		},
		{
			name:        "body mode esc returns to select",
			summary:     []string{"No comments"},
			hasComments: false,
			keys: []tea.KeyPressMsg{
				keyMsg("enter"), // enter body mode
				keyMsg("esc"),   // back to select
				keyMsg("j"),     // move to exit
				keyMsg("enter"), // select exit
			},
			wantAction: ReviewActionExit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewReviewDialog(tt.summary, tt.hasComments)
			var model tea.Model = d
			model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

			for _, k := range tt.keys {
				model, _ = model.Update(k)
			}

			result := model.(*ReviewDialog).Result()
			if result.Action != tt.wantAction {
				t.Errorf("Action = %d, want %d", result.Action, tt.wantAction)
			}
			if result.Body != tt.wantBody {
				t.Errorf("Body = %q, want %q", result.Body, tt.wantBody)
			}
		})
	}
}

func TestReviewDialogView(t *testing.T) {
	t.Run("select mode", func(t *testing.T) {
		d := NewReviewDialog([]string{"file.md: 1 comment(s)"}, true)
		d.Init()
		var model tea.Model = d
		model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

		view := model.(*ReviewDialog).View().Content
		if !strings.Contains(view, "file.md") {
			t.Error("View should contain file summary")
		}
		if !strings.Contains(view, "Comment") {
			t.Error("View should contain Comment option")
		}
	})

	t.Run("body mode", func(t *testing.T) {
		d := NewReviewDialog([]string{"file.md: 1 comment(s)"}, true)
		var model tea.Model = d
		model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		// Enter body mode
		model, _ = model.Update(keyMsg("enter"))

		view := model.(*ReviewDialog).View().Content
		if !strings.Contains(view, "Action:") {
			t.Error("View in body mode should show action label")
		}
		if !strings.Contains(view, "ctrl+s") {
			t.Error("View in body mode should show submit hint")
		}
	})

	t.Run("quitting", func(t *testing.T) {
		d := NewReviewDialog([]string{"No comments"}, false)
		var model tea.Model = d
		model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		model, _ = model.Update(keyMsg("q"))

		view := model.(*ReviewDialog).View().Content
		if view != "" {
			t.Errorf("View after quit should be empty, got %q", view)
		}
	})

	t.Run("cursor bounds", func(t *testing.T) {
		d := NewReviewDialog([]string{"x"}, false)
		var model tea.Model = d
		model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		// Move up past top
		model, _ = model.Update(keyMsg("k"))
		// Move down past bottom
		model, _ = model.Update(keyMsg("j"))
		model, _ = model.Update(keyMsg("j"))
		model, _ = model.Update(keyMsg("j"))

		// Verify no crash with cursor clamping
		_ = model.(*ReviewDialog).Result()
	})
}
