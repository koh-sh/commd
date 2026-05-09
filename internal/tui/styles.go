package tui

import "charm.land/lipgloss/v2"

// Theme constants.
const (
	ThemeDark  = "dark"
	ThemeLight = "light"
)

// Styles holds all the lipgloss styles for the TUI.
type Styles struct {
	ActiveBorder   lipgloss.Style
	InactiveBorder lipgloss.Style

	// Section list
	Title           lipgloss.Style
	SelectedSection lipgloss.Style
	NormalSection   lipgloss.Style
	SectionBadge    lipgloss.Style
	ViewedBadge     lipgloss.Style

	// Status bar
	StatusBar lipgloss.Style
	StatusKey lipgloss.Style

	// Comment
	CommentBorder lipgloss.Style

	// Line mode
	LineGutter   lipgloss.Style
	LineCursor   lipgloss.Style
	LineSelected lipgloss.Style

	// Diff mode
	DiffAdded   lipgloss.Style
	DiffRemoved lipgloss.Style
}

// colorPalette defines the color values for a theme.
type colorPalette struct {
	activeBorder    string
	inactiveBorder  string
	title           string
	selectedSection string
	normalSection   string
	sectionBadge    string
	viewedBadge     string
	statusBar       string
	statusKey       string
	commentBorder   string
	lineGutter      string
	lineCursorBg    string
	lineSelectedBg  string
	diffAddedFg     string
	diffRemovedFg   string
}

var (
	darkPalette = colorPalette{
		activeBorder:    "62",
		inactiveBorder:  "240",
		title:           "170",
		selectedSection: "212",
		normalSection:   "252",
		sectionBadge:    "170",
		viewedBadge:     "82",
		statusBar:       "240",
		statusKey:       "62",
		commentBorder:   "62",
		lineGutter:      "240",
		lineCursorBg:    "236",
		lineSelectedBg:  "235",
		diffAddedFg:     "114",
		diffRemovedFg:   "210",
	}

	lightPalette = colorPalette{
		activeBorder:    "33",
		inactiveBorder:  "250",
		title:           "130",
		selectedSection: "33",
		normalSection:   "236",
		sectionBadge:    "130",
		viewedBadge:     "28",
		statusBar:       "245",
		statusKey:       "33",
		commentBorder:   "33",
		lineGutter:      "245",
		lineCursorBg:    "254",
		lineSelectedBg:  "253",
		diffAddedFg:     "28",
		diffRemovedFg:   "160",
	}
)

func buildStyles(p colorPalette) Styles {
	return Styles{
		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(p.activeBorder)),
		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(p.inactiveBorder)),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(p.title)),
		SelectedSection: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(p.selectedSection)),
		NormalSection: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.normalSection)),
		SectionBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.sectionBadge)),
		ViewedBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.viewedBadge)),
		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.statusBar)),
		StatusKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.statusKey)).
			Bold(true),
		CommentBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(p.commentBorder)),
		LineGutter: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.lineGutter)),
		LineCursor: lipgloss.NewStyle().
			Background(lipgloss.Color(p.lineCursorBg)),
		LineSelected: lipgloss.NewStyle().
			Background(lipgloss.Color(p.lineSelectedBg)),
		DiffAdded: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.diffAddedFg)),
		DiffRemoved: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.diffRemovedFg)),
	}
}

// stylesForTheme returns styles for the given theme.
func stylesForTheme(theme string) Styles {
	if theme == ThemeLight {
		return buildStyles(lightPalette)
	}
	return buildStyles(darkPalette)
}
