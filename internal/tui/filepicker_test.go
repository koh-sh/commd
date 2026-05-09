package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFilePicker(t *testing.T) {
	tests := []struct {
		name          string
		files         []string
		keys          []tea.KeyPressMsg
		wantSelected  []string
		wantCancelled bool
	}{
		{
			name:         "enter with all selected by default",
			files:        []string{"a.md", "b.md"},
			keys:         []tea.KeyPressMsg{keyMsg("enter")},
			wantSelected: []string{"a.md", "b.md"},
		},
		{
			name:          "q cancels",
			files:         []string{"a.md"},
			keys:          []tea.KeyPressMsg{keyMsg("q")},
			wantCancelled: true,
		},
		{
			name:          "esc cancels",
			files:         []string{"a.md"},
			keys:          []tea.KeyPressMsg{keyMsg("esc")},
			wantCancelled: true,
		},
		{
			name:  "deselect first file with space",
			files: []string{"a.md", "b.md"},
			keys: []tea.KeyPressMsg{
				keyMsg(" "),     // deselect a.md
				keyMsg("enter"), // confirm
			},
			wantSelected: []string{"b.md"},
		},
		{
			name:  "navigate and toggle",
			files: []string{"a.md", "b.md", "c.md"},
			keys: []tea.KeyPressMsg{
				keyMsg(" "),     // deselect a.md (cursor at 0)
				keyMsg("j"),     // move to b.md
				keyMsg(" "),     // deselect b.md
				keyMsg("enter"), // confirm
			},
			wantSelected: []string{"c.md"},
		},
		{
			name:  "toggle all with a",
			files: []string{"a.md", "b.md"},
			keys: []tea.KeyPressMsg{
				keyMsg("a"),     // deselect all (all were selected)
				keyMsg("enter"), // confirm
			},
			wantSelected: nil,
		},
		{
			name:  "toggle all then reselect all",
			files: []string{"a.md", "b.md"},
			keys: []tea.KeyPressMsg{
				keyMsg("a"),     // deselect all
				keyMsg("a"),     // select all
				keyMsg("enter"), // confirm
			},
			wantSelected: []string{"a.md", "b.md"},
		},
		{
			name:  "cursor bounds up",
			files: []string{"a.md"},
			keys: []tea.KeyPressMsg{
				keyMsg("k"), // already at 0, no-op
				keyMsg("enter"),
			},
			wantSelected: []string{"a.md"},
		},
		{
			name:  "cursor bounds down",
			files: []string{"a.md"},
			keys: []tea.KeyPressMsg{
				keyMsg("j"), // already at last, no-op
				keyMsg("enter"),
			},
			wantSelected: []string{"a.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := NewFilePicker(tt.files)
			// Send window size
			var model tea.Model = fp
			model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

			for _, k := range tt.keys {
				model, _ = model.Update(k)
			}

			result := model.(*FilePicker).Result()
			if result.Cancelled != tt.wantCancelled {
				t.Errorf("Cancelled = %v, want %v", result.Cancelled, tt.wantCancelled)
			}
			if !tt.wantCancelled {
				if len(result.SelectedFiles) != len(tt.wantSelected) {
					t.Fatalf("len(SelectedFiles) = %d, want %d", len(result.SelectedFiles), len(tt.wantSelected))
				}
				for i, want := range tt.wantSelected {
					if result.SelectedFiles[i] != want {
						t.Errorf("SelectedFiles[%d] = %q, want %q", i, result.SelectedFiles[i], want)
					}
				}
			}
		})
	}
}

func TestFilePickerView(t *testing.T) {
	fp := NewFilePicker([]string{"README.md", "docs/guide.md"})
	fp.Init()

	// Set window size
	var model tea.Model = fp
	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 24})

	view := model.(*FilePicker).View().Content
	if view == "" {
		t.Fatal("View() should not be empty")
	}
	if !strings.Contains(view, "README.md") {
		t.Error("View should contain file names")
	}
	if !strings.Contains(view, "docs/guide.md") {
		t.Error("View should contain all file names")
	}

	// After quitting, View should be empty
	model, _ = model.Update(keyMsg("q"))
	quitView := model.(*FilePicker).View().Content
	if quitView != "" {
		t.Errorf("View after quit should be empty, got %q", quitView)
	}
}

// keyMsg is defined in app_test.go
