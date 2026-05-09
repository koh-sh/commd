package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// keyMsg builds a tea.KeyPressMsg from a textual key spec like "enter",
// "ctrl+s", "tab", or a single rune. Shared across tui tests.
func keyMsg(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "space", " ":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	}
	if rest, ok := strings.CutPrefix(s, "ctrl+"); ok && len(rest) == 1 {
		return tea.KeyPressMsg{Code: rune(rest[0]), Mod: tea.ModCtrl}
	}
	if len(s) == 1 {
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	r := []rune(s)
	if len(r) == 1 {
		return tea.KeyPressMsg{Code: r[0], Text: s}
	}
	return tea.KeyPressMsg{}
}
