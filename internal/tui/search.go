package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// SearchBar wraps a textinput for section searching.
type SearchBar struct {
	input textinput.Model
}

// NewSearchBar creates a new SearchBar.
func NewSearchBar() *SearchBar {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.CharLimit = 100
	ti.SetVirtualCursor(true)

	return &SearchBar{
		input: ti,
	}
}

// Open activates the search bar.
func (s *SearchBar) Open() tea.Cmd {
	s.input.SetValue("")
	return s.input.Focus()
}

// Close deactivates the search bar.
func (s *SearchBar) Close() {
	s.input.Blur()
}

// Query returns the current search query.
func (s *SearchBar) Query() string {
	return s.input.Value()
}

// Update handles tea messages for the textinput.
func (s *SearchBar) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return cmd
}

// View renders the search bar.
func (s *SearchBar) View() string {
	return s.input.View()
}
