package tui

import (
	"testing"
)

// defaultStyles returns the default dark theme styles for use in tests.
func defaultStyles() Styles {
	return buildStyles(darkPalette)
}

func TestStylesForTheme(t *testing.T) {
	tests := []struct {
		theme    string
		wantDark bool // true if expected to use dark palette
	}{
		{"dark", true},
		{"light", false},
		{"", true}, // empty defaults to dark
	}

	for _, tt := range tests {
		t.Run(tt.theme, func(t *testing.T) {
			s := stylesForTheme(tt.theme)

			if s.Title.GetBold() != true {
				t.Error("Title style should be bold")
			}
			if s.StatusKey.GetBold() != true {
				t.Error("StatusKey style should be bold")
			}

			// Verify dark and light produce different colors
			dark := stylesForTheme("dark")
			light := stylesForTheme("light")
			darkFg := dark.SelectedSection.GetForeground()
			lightFg := light.SelectedSection.GetForeground()
			if darkFg == lightFg {
				t.Error("dark and light themes should produce different SelectedSection foreground colors")
			}
		})
	}
}
