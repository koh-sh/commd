package tui

import tea "charm.land/bubbletea/v2"

// altScreenView wraps a rendered string in a tea.View with the alt-screen
// flag set. Centralizing this avoids each model duplicating the same setup
// in its View() method.
func altScreenView(s string) tea.View {
	v := tea.NewView(s)
	v.AltScreen = true
	return v
}

// paneBorderOverhead is the total cells consumed by a pane's rounded border
// in a single dimension (1 cell on each side).
const paneBorderOverhead = 2

// paneInnerSize returns the inner content size of a pane after subtracting
// the border. It applies to both width and height.
func paneInnerSize(paneSize int) int {
	return max(paneSize-paneBorderOverhead, 1)
}
