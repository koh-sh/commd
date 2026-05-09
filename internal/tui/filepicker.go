package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

// FilePickerResult is the result of the file picker interaction.
type FilePickerResult struct {
	SelectedFiles []string
	Cancelled     bool
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
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc"))):
			fp.result.Cancelled = true
			fp.quitting = true
			return fp, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			var selected []string
			for i, f := range fp.files {
				if fp.selected[i] {
					selected = append(selected, f)
				}
			}
			fp.result.SelectedFiles = selected
			fp.quitting = true
			return fp, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			if fp.cursor < len(fp.files)-1 {
				fp.cursor++
				fp.ensureVisible()
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if fp.cursor > 0 {
				fp.cursor--
				fp.ensureVisible()
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("space"))):
			fp.selected[fp.cursor] = !fp.selected[fp.cursor]

		case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
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
	v := tea.NewView(fp.renderView())
	v.AltScreen = true
	return v
}

// renderView returns the rendered string content for the current state.
func (fp *FilePicker) renderView() string {
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

		raw := fmt.Sprintf("%s%s %s", cursor, check, fp.files[i])
		if pad := fp.width - runewidth.StringWidth(raw); pad > 0 {
			raw += strings.Repeat(" ", pad)
		}
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
