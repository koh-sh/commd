package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// FilePickerResult is the result of the file picker interaction.
type FilePickerResult struct {
	SelectedFiles []string
	Cancelled     bool
}

// filePickerKeyMap holds the file picker's key bindings, built once at
// construction like App's KeyMap rather than on every key press.
type filePickerKeyMap struct {
	Cancel    key.Binding
	Confirm   key.Binding
	Down      key.Binding
	Up        key.Binding
	Toggle    key.Binding
	ToggleAll key.Binding
}

func newFilePickerKeyMap() filePickerKeyMap {
	return filePickerKeyMap{
		Cancel:    key.NewBinding(key.WithKeys("q", "esc", "ctrl+c")),
		Confirm:   key.NewBinding(key.WithKeys("enter")),
		Down:      key.NewBinding(key.WithKeys("j", "down")),
		Up:        key.NewBinding(key.WithKeys("k", "up")),
		Toggle:    key.NewBinding(key.WithKeys("space")),
		ToggleAll: key.NewBinding(key.WithKeys("a")),
	}
}

// FilePicker is a Bubble Tea model for selecting files from a list.
type FilePicker struct {
	files        []string
	selected     map[int]bool
	cursor       int
	scrollOffset int
	width        int
	height       int
	result       FilePickerResult
	quitting     bool
	keymap       filePickerKeyMap
}

// NewFilePicker creates a file picker with the given file list.
func NewFilePicker(files []string) *FilePicker {
	selected := make(map[int]bool)
	// Select all by default
	for i := range files {
		selected[i] = true
	}
	return &FilePicker{
		files:    files,
		selected: selected,
		keymap:   newFilePickerKeyMap(),
	}
}

// Result returns the file picker result.
func (fp *FilePicker) Result() FilePickerResult {
	return fp.result
}

// Init implements tea.Model.
func (fp *FilePicker) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (fp *FilePicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		fp.width = msg.Width
		fp.height = msg.Height
		return fp, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, fp.keymap.Cancel):
			fp.result.Cancelled = true
			fp.quitting = true
			return fp, tea.Quit

		case key.Matches(msg, fp.keymap.Confirm):
			var selected []string
			for i, f := range fp.files {
				if fp.selected[i] {
					selected = append(selected, f)
				}
			}
			fp.result.SelectedFiles = selected
			fp.quitting = true
			return fp, tea.Quit

		case key.Matches(msg, fp.keymap.Down):
			if fp.cursor < len(fp.files)-1 {
				fp.cursor++
				fp.ensureVisible()
			}

		case key.Matches(msg, fp.keymap.Up):
			if fp.cursor > 0 {
				fp.cursor--
				fp.ensureVisible()
			}

		case key.Matches(msg, fp.keymap.Toggle):
			fp.selected[fp.cursor] = !fp.selected[fp.cursor]

		case key.Matches(msg, fp.keymap.ToggleAll):
			allSelected := fp.allSelected()
			for i := range fp.files {
				fp.selected[i] = !allSelected
			}
		}
	}

	return fp, nil
}

// visibleFileCount returns the number of file lines that fit in the viewport.
// Reserves space for title (2 lines) and help (2 lines: blank + help text).
func (fp *FilePicker) visibleFileCount() int {
	return max(fp.height-4, 1)
}

// ensureVisible adjusts scrollOffset so the cursor is within the visible area.
func (fp *FilePicker) ensureVisible() {
	visible := fp.visibleFileCount()
	if fp.cursor < fp.scrollOffset {
		fp.scrollOffset = fp.cursor
	}
	if fp.cursor >= fp.scrollOffset+visible {
		fp.scrollOffset = fp.cursor - visible + 1
	}
}

// View implements tea.Model.
func (fp *FilePicker) View() tea.View {
	return altScreenView(fp.renderFilePicker())
}

// renderFilePicker returns the rendered string content for the current state.
func (fp *FilePicker) renderFilePicker() string {
	if fp.quitting {
		return ""
	}

	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Render("Select Markdown files to review")

	b.WriteString(title + "\n")
	b.WriteString(strings.Repeat("─", min(fp.width, 60)) + "\n")

	visible := fp.visibleFileCount()
	end := min(fp.scrollOffset+visible, len(fp.files))

	for i := fp.scrollOffset; i < end; i++ {
		cursor := "  "
		if i == fp.cursor {
			cursor = "▸ "
		}

		check := "[ ]"
		if fp.selected[i] {
			check = "[✓]"
		}

		raw := fitToWidth(fmt.Sprintf("%s%s %s", cursor, check, fp.files[i]), fp.width)
		style := lipgloss.NewStyle()
		if i == fp.cursor {
			style = style.Foreground(lipgloss.Color("14"))
		}
		b.WriteString(style.Render(raw) + "\n")
	}

	b.WriteString("\n")
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Render("↑/↓ navigate • space toggle • a all • enter confirm • q cancel")
	b.WriteString(help)

	return b.String()
}

func (fp *FilePicker) allSelected() bool {
	for i := range fp.files {
		if !fp.selected[i] {
			return false
		}
	}
	return true
}
