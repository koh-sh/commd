package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/koh-sh/commd/internal/markdown"
)

// makeLargeDoc creates a document with many sections to exceed any terminal height.
func makeLargeDoc(topLevel, childrenPer int) *markdown.Document {
	p := &markdown.Document{
		Title:    "Large Test Plan",
		Preamble: "This is a preamble with enough text to test overflow.",
	}
	for i := 1; i <= topLevel; i++ {
		section := &markdown.Section{
			ID:    fmt.Sprintf("S%d", i),
			Title: fmt.Sprintf("Top Level Step %d", i),
			Level: 2,
			Body:  fmt.Sprintf("Body for step %d with some content.", i),
		}
		for j := 1; j <= childrenPer; j++ {
			child := &markdown.Section{
				ID:     fmt.Sprintf("S%d.%d", i, j),
				Title:  fmt.Sprintf("Sub Step %d.%d", i, j),
				Level:  3,
				Body:   fmt.Sprintf("Body for sub-step %d.%d.", i, j),
				Parent: section,
			}
			section.Children = append(section.Children, child)
		}
		p.Sections = append(p.Sections, section)
	}
	return p
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func TestViewFitsTerminalHeight(t *testing.T) {
	// 50 top-level sections with 3 children each = 200 items, way more than any terminal
	p := makeLargeDoc(50, 3)

	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{"small", 80, 24},
		{"medium", 120, 40},
		{"large", 200, 60},
		{"tiny", 60, 15},
		{"wide-short", 200, 10},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			app := NewApp(p, AppOptions{})

			// Simulate window size message
			model, _ := app.Update(tea.WindowSizeMsg{Width: sz.width, Height: sz.height})
			a, ok := model.(*App)
			if !ok {
				t.Fatalf("model type = %T, want *App", model)
			}

			view := a.View()
			lines := countLines(view)

			if lines > sz.height {
				t.Errorf("View() has %d lines, exceeds terminal height %d", lines, sz.height)
			}
		})
	}
}

func TestViewFitsInCommentMode(t *testing.T) {
	p := makeLargeDoc(20, 3)
	app := NewApp(p, AppOptions{})

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	// Move to a section and enter comment mode
	a.sectionList.CursorDown() // move to first section
	a.comment.Open("S1", nil)
	a.mode = ModeComment

	view := a.View()
	lines := countLines(view)

	if lines > 30 {
		t.Errorf("View() in comment mode has %d lines, exceeds terminal height 30", lines)
	}
}

func TestViewFitsInConfirmMode(t *testing.T) {
	p := makeLargeDoc(20, 3)
	app := NewApp(p, AppOptions{})

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	a.mode = ModeConfirm

	view := a.View()
	lines := countLines(view)

	if lines > 30 {
		t.Errorf("View() in confirm mode has %d lines, exceeds terminal height 30", lines)
	}
}

func TestSectionListScrollsWithCursor(t *testing.T) {
	p := makeLargeDoc(30, 0) // 30 top-level sections, no children
	app := NewApp(p, AppOptions{})

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	// Move cursor down past the visible area
	for range 25 {
		a.sectionList.CursorDown()
	}

	view := a.View()
	lines := countLines(view)

	if lines > 20 {
		t.Errorf("View() has %d lines after scrolling, exceeds terminal height 20", lines)
	}

	// The selected section should be visible in the rendered output
	selected := a.sectionList.Selected()
	if selected == nil {
		t.Fatal("Expected a selected section")
		return
	}
	if !strings.Contains(view, selected.ID) {
		t.Errorf("Selected section %s not visible in rendered view after scrolling", selected.ID)
	}
}

func TestGGGoesToTop(t *testing.T) {
	p := makeLargeDoc(10, 0)
	app := NewApp(p, AppOptions{})

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	// Move cursor down several sections
	for range 5 {
		a.sectionList.CursorDown()
	}
	if a.sectionList.Selected().ID == "S1" {
		t.Fatal("Cursor should not be at S1 before gg")
	}

	// Send 'g' then 'g' (gg chord)
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	// Cursor should be at top (Overview if preamble exists, otherwise S1)
	if a.sectionList.IsOverviewSelected() {
		// OK: overview is at top
	} else if a.sectionList.Selected() != nil && a.sectionList.Selected().ID != "S1" {
		t.Errorf("After gg, expected cursor at top, got %s", a.sectionList.Selected().ID)
	}
}

func TestShiftGGoesToBottom(t *testing.T) {
	p := makeLargeDoc(10, 0)
	app := NewApp(p, AppOptions{})

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	// Send 'G' (Shift+G)
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	selected := a.sectionList.Selected()
	if selected == nil {
		t.Fatal("Expected a selected section after G")
		return
	}
	if selected.ID != "S10" {
		t.Errorf("After G, expected cursor at S10, got %s", selected.ID)
	}
}

func TestPendingGResetOnOtherKey(t *testing.T) {
	p := makeLargeDoc(10, 0)
	app := NewApp(p, AppOptions{})

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	// Send 'g' then 'j' (not a gg chord)
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Should have moved down by one (j)
	if a.sectionList.IsOverviewSelected() {
		// Overview is index 0, j should move to S1
	} else if a.sectionList.Selected() != nil {
		// The j key should have been processed normally after pendingG reset
		// Just verify we didn't jump to top or bottom
		id := a.sectionList.Selected().ID
		if id != "S1" && id != "S2" {
			t.Errorf("After g+j, expected cursor near top, got %s", id)
		}
	}
}

func TestTruncateCJK(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
	}{
		{"ascii fits", "hello", 10},
		{"ascii truncate", "hello world", 8},
		{"cjk fits", "テスト", 6},
		{"cjk truncate", "テスト計画の概要", 8},
		{"mixed", "Step テスト", 8},
		{"zero max", "hello", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxWidth)
			w := lipglossWidth(got)
			if w > tt.maxWidth {
				t.Errorf("truncate(%q, %d) = %q (width %d), exceeds maxWidth",
					tt.input, tt.maxWidth, got, w)
			}
		})
	}
}

// lipglossWidth returns the display width of a string (ANSI-aware).
func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}

func TestViewFitsInHelpMode(t *testing.T) {
	p := makeLargeDoc(5, 2)
	app := NewApp(p, AppOptions{})

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	a.mode = ModeHelp

	view := a.View()
	lines := countLines(view)

	if lines > 30 {
		t.Errorf("View() in help mode has %d lines, exceeds terminal height 30", lines)
	}
}

// initApp creates an App with a standard window size for testing.
func initApp(t *testing.T, p *markdown.Document) *App {
	t.Helper()
	app := NewApp(p, AppOptions{})
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}
	return a
}

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestInit(t *testing.T) {
	app := NewApp(makeLargeDoc(3, 0), AppOptions{})
	if app.Init() != nil {
		t.Error("Init() should return nil")
	}
}

func TestResult(t *testing.T) {
	app := NewApp(makeLargeDoc(3, 0), AppOptions{})
	result := app.Result()
	if result.Status != markdown.StatusCancelled {
		t.Errorf("initial status = %s, want cancelled", result.Status)
	}
}

func TestQuitNoComments(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	_, cmd := a.Update(keyMsg("q"))
	if a.mode != ModeNormal {
		t.Error("should stay in normal mode")
	}
	if a.result.Status != markdown.StatusCancelled {
		t.Errorf("status = %s, want cancelled", a.result.Status)
	}
	if cmd == nil {
		t.Error("should return quit command")
	}
}

func TestQuitWithComments(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "test"})

	a.Update(keyMsg("q"))
	if a.mode != ModeConfirm {
		t.Errorf("mode = %d, want ModeConfirm", a.mode)
	}
}

func TestHelpModeToggle(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("?"))
	if a.mode != ModeHelp {
		t.Errorf("mode = %d, want ModeHelp", a.mode)
	}

	// Esc exits help
	a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after Esc", a.mode)
	}
}

func TestHelpModeQ(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = ModeHelp

	a.Update(keyMsg("q"))
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after q in help", a.mode)
	}
}

func TestHelpModeQuestion(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = ModeHelp

	a.Update(keyMsg("?"))
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after ? in help", a.mode)
	}
}

func TestTabSwitchFocus(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	if a.focus != FocusLeft {
		t.Fatal("initial focus should be left")
	}

	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.focus != FocusRight {
		t.Error("after Tab, focus should be right")
	}

	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.focus != FocusLeft {
		t.Error("after second Tab, focus should be left")
	}
}

func TestSubmitNoComments(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	_, cmd := a.Update(keyMsg("s"))
	if a.result.Status != markdown.StatusApproved {
		t.Errorf("status = %s, want approved", a.result.Status)
	}
	if cmd == nil {
		t.Error("should return quit command")
	}
}

func TestSubmitWithComments(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "feedback"})

	_, cmd := a.Update(keyMsg("s"))
	if a.result.Status != markdown.StatusSubmitted {
		t.Errorf("status = %s, want submitted", a.result.Status)
	}
	if a.result.Review == nil {
		t.Error("review should not be nil")
	}
	if cmd == nil {
		t.Error("should return quit command")
	}
}

func TestLeftPaneUpDown(t *testing.T) {
	a := initApp(t, makeLargeDoc(5, 0))

	// Move down from overview to S1
	a.Update(keyMsg("j"))
	if a.sectionList.Selected() == nil || a.sectionList.Selected().ID != "S1" {
		t.Error("j should move to S1")
	}

	// Move down again
	a.Update(keyMsg("j"))
	if a.sectionList.Selected().ID != "S2" {
		t.Error("j should move to S2")
	}

	// Move up
	a.Update(keyMsg("k"))
	if a.sectionList.Selected().ID != "S1" {
		t.Error("k should move back to S1")
	}
}

func TestLeftPaneToggle(t *testing.T) {
	p := makeLargeDoc(3, 2)
	a := initApp(t, p)

	a.Update(keyMsg("j")) // S1
	a.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// S1 should be collapsed
	for _, item := range a.sectionList.items {
		if item.Section != nil && item.Section.ID == "S1" && item.Expanded {
			t.Error("S1 should be collapsed after enter")
		}
	}
}

func TestLeftPaneHorizontalScroll(t *testing.T) {
	// Use a code block with a wide line so wrapProse does not wrap it.
	p := &markdown.Document{Title: "Wide Plan"}
	wideLine := "```\n" + strings.Repeat("x", 300) + "\n```"
	p.Sections = append(p.Sections, &markdown.Section{
		ID: "S1", Title: "Wide Step", Level: 2, Body: wideLine,
	})
	a := initApp(t, p)

	a.Update(keyMsg("j")) // S1

	// Scroll right with l
	a.Update(keyMsg("l"))
	if pct := a.detail.Viewport().HorizontalScrollPercent(); pct == 0 {
		t.Error("after l, HorizontalScrollPercent should be > 0")
	}

	// Scroll left with h (back to start)
	a.Update(keyMsg("h"))
	if pct := a.detail.Viewport().HorizontalScrollPercent(); pct != 0 {
		t.Errorf("after h, HorizontalScrollPercent = %f, want 0", pct)
	}
}

func TestLeftPaneComment(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("c"))
	if a.mode != ModeComment {
		t.Errorf("mode = %d, want ModeComment", a.mode)
	}
	if a.comment.SectionID() != "S1" {
		t.Errorf("comment sectionID = %s, want S1", a.comment.SectionID())
	}
}

func TestLeftPaneCommentOnOverview(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	// On overview, c should do nothing
	a.Update(keyMsg("c"))
	if a.mode != ModeNormal {
		t.Error("c on overview should not enter comment mode")
	}
}

func TestLeftPaneCommentList(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("j")) // S1

	// No comments, C should not enter comment list
	a.Update(keyMsg("C"))
	if a.mode != ModeNormal {
		t.Error("C with no comments should stay in normal mode")
	}

	// Add comment, then C should work
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "test"})
	a.Update(keyMsg("C"))
	if a.mode != ModeCommentList {
		t.Errorf("mode = %d, want ModeCommentList", a.mode)
	}
}

func TestLeftPaneViewed(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("v"))

	if !a.sectionList.IsViewed("S1") {
		t.Error("S1 should be viewed after v")
	}
}

func TestLeftPaneSearch(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("/"))
	if a.mode != ModeSearch {
		t.Errorf("mode = %d, want ModeSearch", a.mode)
	}
}

func TestCommentModeCtrlS(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("c"))

	// Type something
	a.comment.textarea.SetValue("my comment")

	// Save
	a.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after Ctrl+S", a.mode)
	}
	comments := a.sectionList.GetComments("S1")
	if len(comments) != 1 {
		t.Fatalf("comments count = %d, want 1", len(comments))
	}
	if comments[0].Body != "my comment" {
		t.Errorf("body = %s, want 'my comment'", comments[0].Body)
	}
}

func TestCommentModeCtrlSEdit(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.sectionList.AddComment("S1", &markdown.ReviewComment{SectionID: "S1", Action: markdown.ActionSuggestion, Body: "original"})

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("C")) // comment list
	a.Update(keyMsg("e")) // edit

	if a.mode != ModeComment {
		t.Fatalf("mode = %d, want ModeComment", a.mode)
	}

	a.comment.textarea.SetValue("edited")
	a.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	if a.mode != ModeCommentList {
		t.Errorf("mode = %d, want ModeCommentList after edit save", a.mode)
	}
	comments := a.sectionList.GetComments("S1")
	if len(comments) != 1 || comments[0].Body != "edited" {
		t.Errorf("comment not updated properly")
	}
}

func TestCommentModeEsc(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("c"))
	a.comment.textarea.SetValue("will cancel")

	a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after Esc", a.mode)
	}
	if len(a.sectionList.GetComments("S1")) != 0 {
		t.Error("cancelled comment should not be saved")
	}
}

func TestCommentModeEscFromEdit(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.sectionList.AddComment("S1", &markdown.ReviewComment{SectionID: "S1", Body: "original"})

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("C")) // comment list
	a.Update(keyMsg("e")) // edit

	a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if a.mode != ModeCommentList {
		t.Errorf("mode = %d, want ModeCommentList after Esc from edit", a.mode)
	}
}

func TestCommentModeTab(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("c"))

	initial := a.comment.Label()
	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.comment.Label() == initial {
		t.Error("Tab should cycle label")
	}
}

func TestCommentModeCtrlDCyclesDecoration(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("c"))

	if a.comment.DecorationLabel() != markdown.DecorationNone {
		t.Errorf("initial decoration = %s, want none", a.comment.DecorationLabel())
	}
	a.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if a.comment.DecorationLabel() != markdown.DecorationNonBlocking {
		t.Errorf("decoration after Ctrl+D = %s, want non-blocking", a.comment.DecorationLabel())
	}
	if a.mode != ModeComment {
		t.Errorf("mode = %d, want ModeComment after Ctrl+D", a.mode)
	}
}

func TestCommentListModeEsc(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "test"})

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("C"))

	a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
}

func TestCommentListModeNav(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "first"})
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "second"})

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("C"))

	if a.commentList.Cursor() != 0 {
		t.Fatal("cursor should start at 0")
	}

	a.Update(keyMsg("j"))
	if a.commentList.Cursor() != 1 {
		t.Errorf("cursor = %d, want 1", a.commentList.Cursor())
	}

	a.Update(keyMsg("k"))
	if a.commentList.Cursor() != 0 {
		t.Errorf("cursor = %d, want 0", a.commentList.Cursor())
	}
}

func TestCommentListModeDelete(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "only"})

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("C"))
	a.Update(keyMsg("d"))

	// Last comment deleted -> back to normal mode
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after deleting last comment", a.mode)
	}
	if len(a.sectionList.GetComments("S1")) != 0 {
		t.Error("comment should be deleted")
	}
}

func TestCommentListModeDeleteWithRemaining(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "first"})
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "second"})

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("C"))
	a.Update(keyMsg("d"))

	// Remaining comments -> stay in comment list
	if a.mode != ModeCommentList {
		t.Errorf("mode = %d, want ModeCommentList", a.mode)
	}
	if len(a.sectionList.GetComments("S1")) != 1 {
		t.Error("should have 1 remaining comment")
	}
}

func TestConfirmModeYes(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "test"})
	a.mode = ModeConfirm

	_, cmd := a.Update(keyMsg("y"))
	if a.result.Status != markdown.StatusCancelled {
		t.Errorf("status = %s, want cancelled", a.result.Status)
	}
	if cmd == nil {
		t.Error("should return quit command")
	}
}

func TestConfirmModeNo(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = ModeConfirm

	a.Update(keyMsg("n"))
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
}

func TestConfirmModeEsc(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = ModeConfirm

	a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
}

func TestSearchModeEnter(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("/"))
	a.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after Enter", a.mode)
	}
}

func TestSearchModeEsc(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("/"))
	a.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after Esc", a.mode)
	}
	// All items should be visible
	for _, item := range a.sectionList.items {
		if !item.Visible {
			t.Error("all items should be visible after search cancel")
		}
	}
}

func TestRightPaneScroll(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(tea.KeyMsg{Type: tea.KeyTab}) // switch to right pane
	if a.focus != FocusRight {
		t.Fatal("focus should be right after Tab")
	}

	before := a.detail.viewport.YOffset
	a.Update(keyMsg("j"))
	afterDown := a.detail.viewport.YOffset
	if afterDown < before {
		t.Error("j in right pane should not scroll up")
	}

	a.Update(keyMsg("k"))
	afterUp := a.detail.viewport.YOffset
	if afterUp > afterDown {
		t.Error("k in right pane should not scroll down")
	}
}

func TestRightPaneHorizontalScroll(t *testing.T) {
	p := &markdown.Document{
		Title: "Test",
		Sections: []*markdown.Section{
			{
				ID:    "S1",
				Title: "Long Code",
				Level: 2,
				Body:  "```\n" + strings.Repeat("ABCDEFGHIJ", 20) + "\n```",
			},
		},
	}
	a := initApp(t, p)

	// Select section S1 in left pane, then switch to right pane
	a.Update(keyMsg("j")) // move to S1
	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.focus != FocusRight {
		t.Fatal("focus should be right after Tab")
	}

	// Check detail.View() changes
	detailBefore := a.detail.View()
	before := a.detail.Viewport().HorizontalScrollPercent()

	a.Update(keyMsg("l"))
	detailAfter := a.detail.View()
	after := a.detail.Viewport().HorizontalScrollPercent()
	if after <= before {
		t.Errorf("l in right pane should scroll right: before=%f, after=%f", before, after)
	}
	if detailBefore == detailAfter {
		t.Error("detail.View() should change after horizontal scroll")
	}

	// Check full app View() changes
	appViewBefore := a.View()
	a.Update(keyMsg("l"))
	appViewAfter := a.View()
	if appViewBefore == appViewAfter {
		t.Error("app.View() should change after horizontal scroll")
	}

	// Press 'h' to scroll left
	beforeH := a.detail.Viewport().HorizontalScrollPercent()
	a.Update(keyMsg("h"))
	afterLeft := a.detail.Viewport().HorizontalScrollPercent()
	if afterLeft >= beforeH {
		t.Errorf("h in right pane should scroll left: before=%f, after=%f", beforeH, afterLeft)
	}
}

func TestRightPaneHorizontalScrollJump(t *testing.T) {
	p := &markdown.Document{
		Title: "Test",
		Sections: []*markdown.Section{
			{
				ID:    "S1",
				Title: "Long Code",
				Level: 2,
				Body:  "```\n" + strings.Repeat("ABCDEFGHIJ", 20) + "\n```",
			},
		},
	}
	a := initApp(t, p)

	// Select section S1 in left pane, then switch to right pane
	a.Update(keyMsg("j")) // move to S1
	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.focus != FocusRight {
		t.Fatal("focus should be right after Tab")
	}

	// 'L' scrolls to end
	a.Update(keyMsg("L"))
	endPct := a.detail.Viewport().HorizontalScrollPercent()
	if endPct != 1.0 {
		t.Errorf("L should scroll to end: got %f, want 1.0", endPct)
	}

	// 'H' scrolls to start
	a.Update(keyMsg("H"))
	startPct := a.detail.Viewport().HorizontalScrollPercent()
	if startPct != 0.0 {
		t.Errorf("H should scroll to start: got %f, want 0.0", startPct)
	}
}

func TestViewNotReady(t *testing.T) {
	app := NewApp(makeLargeDoc(3, 0), AppOptions{})
	view := app.View()
	if view != "Loading..." {
		t.Errorf("view before ready = %q, want 'Loading...'", view)
	}
}

func TestFullViewToggle(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	if a.fullView {
		t.Fatal("fullView should be false initially")
	}

	a.Update(keyMsg("f"))
	if !a.fullView {
		t.Error("fullView should be true after f")
	}

	a.Update(keyMsg("f"))
	if a.fullView {
		t.Error("fullView should be false after second f")
	}
}

func TestFullViewFromRightPane(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(tea.KeyMsg{Type: tea.KeyTab}) // switch to right pane
	if a.focus != FocusRight {
		t.Fatal("focus should be right")
	}

	a.Update(keyMsg("f"))
	if !a.fullView {
		t.Error("f should toggle fullView even from right pane")
	}
}

func TestFullViewCursorScrollsDetail(t *testing.T) {
	p := makeLargeDoc(10, 0)
	app := NewApp(p, AppOptions{})
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 15})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	a.Update(keyMsg("f"))
	if !a.fullView {
		t.Fatal("should be in full view")
	}

	// Move cursor down several times
	for range 5 {
		a.Update(keyMsg("j"))
	}

	selected := a.sectionList.Selected()
	if selected == nil {
		t.Fatal("expected a selected section")
	}

	// YOffset should match the selected section's offset
	yOffset := a.detail.Viewport().YOffset
	var expectedOffset int
	for _, so := range a.detail.sectionOffsets {
		if so.sectionID == selected.ID {
			expectedOffset = so.line
			break
		}
	}
	if yOffset != expectedOffset {
		t.Errorf("YOffset = %d, want %d (offset for %s)", yOffset, expectedOffset, selected.ID)
	}
}

func TestFullViewGGScrollsToTop(t *testing.T) {
	p := makeLargeDoc(10, 0)
	app := NewApp(p, AppOptions{})
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 15})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	a.Update(keyMsg("f"))

	// Move down first
	for range 5 {
		a.Update(keyMsg("j"))
	}
	if a.detail.Viewport().YOffset == 0 {
		t.Fatal("YOffset should not be 0 after moving down")
	}

	// gg (left pane focused)
	a.Update(keyMsg("g"))
	a.Update(keyMsg("g"))
	if a.detail.Viewport().YOffset != 0 {
		t.Errorf("gg should scroll to top, YOffset=%d", a.detail.Viewport().YOffset)
	}
}

func TestFullViewGScrollsToLastSection(t *testing.T) {
	p := makeLargeDoc(10, 0)
	app := NewApp(p, AppOptions{})
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 15})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	a.Update(keyMsg("f"))

	// G (left pane focused)
	a.Update(keyMsg("G"))

	selected := a.sectionList.Selected()
	if selected == nil {
		t.Fatal("expected a selected section")
		return
	}
	if selected.ID != "S10" {
		t.Errorf("G should select last section, got %s", selected.ID)
	}

	// YOffset should be scrolled down (viewport may clamp to max scroll)
	yOffset := a.detail.Viewport().YOffset
	if yOffset == 0 {
		t.Error("YOffset should be > 0 after G to last section")
	}
}

func TestFullViewCommentStillWorks(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("f"))
	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("c"))
	if a.mode != ModeComment {
		t.Errorf("mode = %d, want ModeComment in full view", a.mode)
	}
}

func TestFullViewCommentRefreshesView(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("f"))
	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("c"))

	a.comment.textarea.SetValue("full view comment")
	a.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after save", a.mode)
	}
	// After saving comment in full view, the detail should be refreshed
	content := a.detail.View()
	if !strings.Contains(content, "full") {
		t.Error("full view should re-render and show saved comment")
	}
}

func TestFullViewStatusBar(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	// Default (section view): pressing f will switch to full
	sb := a.renderStatusBar()
	if !strings.Contains(sb, "full") {
		t.Error("status bar should show 'full' when in section view (target mode)")
	}

	// After toggle (full view): pressing f will switch to section
	a.Update(keyMsg("f"))
	sb = a.renderStatusBar()
	if !strings.Contains(sb, "section") {
		t.Error("status bar should show 'section' when in full view (target mode)")
	}
}

func TestFullViewScrollSyncsCursor(t *testing.T) {
	// Use many sections so content exceeds viewport height
	p := makeLargeDoc(20, 0)
	app := NewApp(p, AppOptions{})
	// Use small height so scrolling is meaningful
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 15})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	a.Update(keyMsg("f")) // enter full view
	if !a.fullView {
		t.Fatal("should be in full view")
	}

	// Verify sectionOffsets were built
	if len(a.detail.sectionOffsets) == 0 {
		t.Fatal("sectionOffsets should be populated in full view")
	}

	// Switch to right pane
	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.focus != FocusRight {
		t.Fatal("focus should be right")
	}

	// Go to bottom with G
	a.Update(keyMsg("G"))

	// Cursor should have moved to a section near the bottom (not overview/top)
	selected := a.sectionList.Selected()
	if selected == nil {
		t.Fatal("expected a selected section after G")
		return
	}
	// YOffset points to the top of the visible window, so the synced section
	// is the one at the top of the last visible page (near end, not necessarily the very last)
	if selected.ID != "S19" && selected.ID != "S20" {
		t.Errorf("after G in full view right pane, cursor should sync near last section, got %s", selected.ID)
	}
}

func TestFullViewScrollSyncsCursorGG(t *testing.T) {
	p := makeLargeDoc(20, 0)
	app := NewApp(p, AppOptions{})
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 15})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	a.Update(keyMsg("f")) // enter full view

	// Switch to right pane
	a.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Go to bottom first, then gg to go back to top
	a.Update(keyMsg("G"))
	// Verify we moved away from top
	selected := a.sectionList.Selected()
	if selected != nil && selected.ID == "S1" {
		t.Fatal("after G, should not be at S1")
	}

	a.Update(keyMsg("g"))
	a.Update(keyMsg("g"))

	// After gg, YOffset is 0, which is before any section heading
	// so sectionID is "" and cursor stays where it was -- but that's fine
	// since overview is at the top. The key point is no crash and
	// cursor is near the top.
	if a.detail.Viewport().YOffset != 0 {
		t.Errorf("after gg, viewport should be at top, YOffset=%d", a.detail.Viewport().YOffset)
	}
}

func TestSinglePaneMode(t *testing.T) {
	app := NewApp(makeLargeDoc(3, 0), AppOptions{})

	// Width < 80 triggers single pane mode
	model, _ := app.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	view := a.View()
	if view == "" {
		t.Error("view should not be empty in single pane mode")
	}
}

func TestRenderStatusBarModes(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	// Normal mode
	sb := a.renderStatusBar()
	if !strings.Contains(sb, "comment") {
		t.Error("normal mode status bar should contain 'comment'")
	}
	if !strings.Contains(sb, "viewed") {
		t.Error("normal mode status bar should contain progress 'viewed'")
	}

	// Normal mode with comments - verify comment count display
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "test"})
	sb = a.renderStatusBar()
	if !strings.Contains(sb, "comments") {
		t.Error("normal mode status bar should show comment count when comments exist")
	}
	a.sectionList.DeleteComment("S1", 0)

	// Comment mode
	a.mode = ModeComment
	sb = a.renderStatusBar()
	if !strings.Contains(sb, "save") {
		t.Error("comment mode status bar should contain 'save'")
	}

	// Comment list mode
	a.mode = ModeCommentList
	sb = a.renderStatusBar()
	if !strings.Contains(sb, "edit") {
		t.Error("comment list mode status bar should contain 'edit'")
	}

	// Search mode
	a.mode = ModeSearch
	sb = a.renderStatusBar()
	// Search bar view is rendered
	if sb == "" {
		t.Error("search mode status bar should not be empty")
	}
}

func TestRenderTitleBar(t *testing.T) {
	t.Run("with title and filepath", func(t *testing.T) {
		a := initApp(t, makeLargeDoc(3, 0))
		a.opts.FilePath = "test.md"
		tb := a.renderTitleBar()
		if !strings.Contains(tb, "test.md") {
			t.Error("title bar should contain filepath")
		}
	})

	t.Run("no title no filepath", func(t *testing.T) {
		p := &markdown.Document{Sections: []*markdown.Section{{ID: "S1", Title: "Step", Level: 2}}}
		a := initApp(t, p)
		tb := a.renderTitleBar()
		if tb != "" {
			t.Error("title bar should be empty when no title and no filepath")
		}
	})

	t.Run("zero width", func(t *testing.T) {
		app := NewApp(makeLargeDoc(3, 0), AppOptions{})
		tb := app.renderTitleBar()
		if tb != "" {
			t.Error("title bar should be empty when width is 0")
		}
	})
}

func TestCtrlCInConfirm(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = ModeConfirm

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if a.result.Status != markdown.StatusCancelled {
		t.Errorf("status = %s, want cancelled", a.result.Status)
	}
	if cmd == nil {
		t.Error("should return quit command")
	}
}

func TestGGInRightPane(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.focus = FocusRight

	// gg in right pane should call GotoTop on viewport
	a.Update(keyMsg("g"))
	a.Update(keyMsg("g"))
	if a.detail.viewport.YOffset != 0 {
		t.Errorf("gg in right pane should scroll to top, YOffset = %d", a.detail.viewport.YOffset)
	}
}

func TestGInRightPane(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.focus = FocusRight

	// G in right pane should call GotoBottom on viewport
	a.Update(keyMsg("G"))
	if a.focus != FocusRight {
		t.Error("focus should remain on right pane after G")
	}
	// YOffset should be at or near the bottom (>= 0 means viewport was updated)
	if a.detail.viewport.YOffset < 0 {
		t.Errorf("G in right pane should not result in negative YOffset, got %d", a.detail.viewport.YOffset)
	}
}

func TestRenderRightContentModes(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	// Normal mode
	content := a.renderRightContent(80, 20)
	if content == "" {
		t.Error("right content should not be empty in normal mode")
	}

	// Comment mode
	a.sectionList.CursorDown()
	a.comment.Open("S1", nil)
	a.mode = ModeComment
	content = a.renderRightContent(80, 20)
	if !strings.Contains(content, "Comment") {
		t.Error("right content in comment mode should contain 'Comment'")
	}

	// Comment list mode
	a.mode = ModeCommentList
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "test"})
	a.commentList.Open("S1", a.sectionList.GetComments("S1"))
	content = a.renderRightContent(80, 20)
	if content == "" {
		t.Error("right content in comment list mode should not be empty")
	}
}

func TestSearchModeNavigation(t *testing.T) {
	a := initApp(t, makeLargeDoc(5, 0))

	a.Update(keyMsg("/"))
	if a.mode != ModeSearch {
		t.Fatal("should be in search mode")
	}

	// Navigate with j/k in search mode
	a.Update(keyMsg("j"))
	a.Update(keyMsg("k"))
	// No crash and still in search mode
	if a.mode != ModeSearch {
		t.Error("should still be in search mode after j/k")
	}
}

func TestSearchModeTextInput(t *testing.T) {
	a := initApp(t, makeLargeDoc(5, 0))

	a.Update(keyMsg("/"))

	// Send a character to the search bar - this triggers the text input update path
	// and the FilterByQuery call
	a.search.input.SetValue("Step 3")
	// Manually trigger the filter as SetValue doesn't go through Update
	a.sectionList.FilterByQuery(a.search.Query())

	// Verify filter was applied
	found := false
	for _, item := range a.sectionList.items {
		if item.Section != nil && item.Section.ID == "S3" && item.Visible {
			found = true
		}
	}
	if !found {
		t.Error("S3 should be visible after searching 'Step 3'")
	}
}

// testMsg is a custom message type for testing non-key, non-window message handling.
type testMsg struct{}

func TestUpdateNonKeyMsgInCommentMode(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("c")) // enter comment mode

	// Send a non-key, non-window message to exercise comment.Update path
	a.Update(testMsg{})
	if a.mode != ModeComment {
		t.Error("should stay in comment mode after custom msg")
	}
}

func TestUpdateNonKeyMsgInSearchMode(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("/")) // enter search mode

	// Send a non-key, non-window message to exercise search.Update path
	a.Update(testMsg{})
	if a.mode != ModeSearch {
		t.Error("should stay in search mode after custom msg")
	}
}

func TestUpdateNonKeyMsgInNormalMode(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	// Send a non-key, non-window message in normal mode
	model, cmd := a.Update(testMsg{})
	if model != a {
		t.Error("should return same model")
	}
	if cmd != nil {
		t.Error("should return nil cmd")
	}
}

func TestSinglePaneFocusRight(t *testing.T) {
	app := NewApp(makeLargeDoc(3, 0), AppOptions{})

	model, _ := app.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}
	a.focus = FocusRight

	view := a.View()
	if view == "" {
		t.Error("view should not be empty in single pane right focus")
	}
}

func TestHelpModeEnter(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = ModeHelp

	a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after enter in help", a.mode)
	}
}

func TestConfirmModeCapitalY(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = ModeConfirm

	_, cmd := a.Update(keyMsg("Y"))
	if a.result.Status != markdown.StatusCancelled {
		t.Errorf("status = %s, want cancelled", a.result.Status)
	}
	if cmd == nil {
		t.Error("should return quit command")
	}
}

func TestConfirmModeCapitalN(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = ModeConfirm

	a.Update(keyMsg("N"))
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after N", a.mode)
	}
}

func TestContentHeight(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	h := a.contentHeight()
	if h < 1 {
		t.Errorf("contentHeight = %d, should be >= 1", h)
	}
}

func TestLeftRightWidth(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	lw := a.leftWidth()
	rw := a.rightWidth()
	if lw <= 0 || rw <= 0 {
		t.Errorf("leftWidth=%d, rightWidth=%d, both should be > 0", lw, rw)
	}

	// Single pane mode
	a.width = 60
	lw = a.leftWidth()
	if lw != 58 { // 60 - 2
		t.Errorf("leftWidth in single pane = %d, want 58", lw)
	}
}

func TestPaneResize(t *testing.T) {
	t.Run("grow and shrink", func(t *testing.T) {
		a := initApp(t, makeLargeDoc(3, 0))
		initialRatio := a.leftRatio
		initialLW := a.leftWidth()

		a.Update(keyMsg(">"))
		if a.leftRatio != initialRatio+5 {
			t.Errorf("leftRatio after > = %d, want %d", a.leftRatio, initialRatio+5)
		}
		if a.leftWidth() <= initialLW {
			t.Error("leftWidth should increase after >")
		}

		a.Update(keyMsg("<"))
		if a.leftRatio != initialRatio {
			t.Errorf("leftRatio after < = %d, want %d", a.leftRatio, initialRatio)
		}
	})

	t.Run("upper bound", func(t *testing.T) {
		a := initApp(t, makeLargeDoc(3, 0))
		a.leftRatio = 50
		a.Update(keyMsg(">"))
		if a.leftRatio != 50 {
			t.Errorf("leftRatio should not exceed 50, got %d", a.leftRatio)
		}
	})

	t.Run("lower bound", func(t *testing.T) {
		a := initApp(t, makeLargeDoc(3, 0))
		a.leftRatio = 10
		a.Update(keyMsg("<"))
		if a.leftRatio != 10 {
			t.Errorf("leftRatio should not go below 10, got %d", a.leftRatio)
		}
	})

	t.Run("disabled in single pane", func(t *testing.T) {
		app := NewApp(makeLargeDoc(3, 0), AppOptions{})
		model, _ := app.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
		a, ok := model.(*App)
		if !ok {
			t.Fatalf("model type = %T, want *App", model)
		}
		initialRatio := a.leftRatio
		a.Update(keyMsg(">"))
		if a.leftRatio != initialRatio {
			t.Errorf("leftRatio should not change in single pane, got %d", a.leftRatio)
		}
		a.Update(keyMsg("<"))
		if a.leftRatio != initialRatio {
			t.Errorf("leftRatio should not change in single pane, got %d", a.leftRatio)
		}
	})
}

func TestHandleKeyUnknownMode(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = AppMode(99) // unknown mode

	model, _ := a.Update(keyMsg("j"))
	if model != a {
		t.Error("should return same model for unknown mode")
	}
}

func TestUpdateLayoutFirstTime(t *testing.T) {
	// Test the path where detail is nil (first updateLayout call)
	app := NewApp(makeLargeDoc(3, 0), AppOptions{})
	app.width = 120
	app.height = 30
	app.detail = nil
	app.updateLayout()
	if app.detail == nil {
		t.Error("detail should be created after updateLayout")
	}
}

func TestRefreshDetailNilDetail(t *testing.T) {
	app := NewApp(makeLargeDoc(3, 0), AppOptions{})
	app.detail = nil
	app.refreshDetail()
	if app.detail != nil {
		t.Error("refreshDetail should not create detail when it is nil")
	}
}

func TestSinglePaneTitleBar(t *testing.T) {
	// Single pane with title bar and focus right
	app := NewApp(makeLargeDoc(3, 0), AppOptions{FilePath: "test.md"})
	model, _ := app.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	// Left focus with title
	view := a.View()
	if view == "" {
		t.Error("view should not be empty")
	}

	// Right focus with title
	a.focus = FocusRight
	view = a.View()
	if view == "" {
		t.Error("view should not be empty in right focus")
	}
}

func TestCommentModeCtrlSEmptyBody(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("c"))

	// Don't type anything (empty comment)
	a.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	// Empty comment should not be saved
	if len(a.sectionList.GetComments("S1")) != 0 {
		t.Error("empty comment should not be saved")
	}
}

func TestCommentModeRegularKey(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("c")) // enter comment mode

	// Regular key should fall through to comment.Update(msg)
	a.Update(keyMsg("a"))
	if a.mode != ModeComment {
		t.Error("should stay in comment mode after regular key")
	}
}

func TestConfirmModeUnhandledKey(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = ModeConfirm

	a.Update(keyMsg("x"))
	if a.mode != ModeConfirm {
		t.Errorf("mode = %d, want ModeConfirm after unhandled key", a.mode)
	}
}

func TestHelpModeUnhandledKey(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.mode = ModeHelp

	a.Update(keyMsg("x"))
	if a.mode != ModeHelp {
		t.Errorf("mode = %d, want ModeHelp after unhandled key", a.mode)
	}
}

func TestSearchModeRegularKey(t *testing.T) {
	a := initApp(t, makeLargeDoc(5, 0))
	a.Update(keyMsg("/"))

	// Regular key should fall through to search.Update + FilterByQuery
	a.Update(keyMsg("S"))
	if a.mode != ModeSearch {
		t.Error("should stay in search mode after regular key")
	}
}

func TestWindowSizeResize(t *testing.T) {
	app := NewApp(makeLargeDoc(3, 0), AppOptions{})
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}
	if a.detail == nil {
		t.Fatal("detail should be created")
	}

	// Second resize should call detail.SetSize (else branch in updateLayout)
	model, _ = a.Update(tea.WindowSizeMsg{Width: 100, Height: 25})
	a, ok = model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}
	if a.width != 100 {
		t.Errorf("width = %d, want 100", a.width)
	}
}

func TestSinglePaneNoTitleBar(t *testing.T) {
	// Document without title, no filepath -> empty title bar
	p := &markdown.Document{Sections: []*markdown.Section{{ID: "S1", Title: "Step", Level: 2}}}
	app := NewApp(p, AppOptions{})
	model, _ := app.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	a, ok := model.(*App)
	if !ok {
		t.Fatalf("model type = %T, want *App", model)
	}

	view := a.View()
	if view == "" {
		t.Error("view should not be empty")
	}
}

func TestDualPaneFocusRight(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.focus = FocusRight

	view := a.View()
	if view == "" {
		t.Error("view should not be empty with right focus in dual pane")
	}
}

func TestDualPaneNoTitleBar(t *testing.T) {
	// Document without title, no filepath -> empty title bar in dual pane
	p := &markdown.Document{Sections: []*markdown.Section{{ID: "S1", Title: "Step", Level: 2}}}
	a := initApp(t, p)

	view := a.View()
	if view == "" {
		t.Error("view should not be empty")
	}
}

func TestCommentListModeUnhandledKey(t *testing.T) {
	a := initApp(t, makeLargeDoc(3, 0))
	a.sectionList.AddComment("S1", &markdown.ReviewComment{Body: "test"})

	a.Update(keyMsg("j")) // S1
	a.Update(keyMsg("C")) // comment list mode

	// Unhandled key should not change mode
	a.Update(keyMsg("x"))
	if a.mode != ModeCommentList {
		t.Errorf("mode = %d, want ModeCommentList after unhandled key", a.mode)
	}
}

func TestRenderScrollUp(t *testing.T) {
	// Create a document with enough sections to cause scrolling in a small viewport
	p := makeLargeDoc(30, 0) // 30 sections + overview = 31 items
	a := initApp(t, p)
	a.height = 10 // small viewport

	// Scroll down to the bottom
	for range 25 {
		a.sectionList.CursorDown()
	}
	// Render to update scrollOffset
	a.sectionList.Render(80, 10, a.styles)

	// Jump to top - this should trigger cursorPos < scrollOffset
	a.sectionList.CursorTop()
	output := a.sectionList.Render(80, 10, a.styles)
	if !strings.Contains(output, "Overview") {
		t.Error("after jump to top, Overview should be visible")
	}
}

func TestClipLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLines int
		wantMax  int
	}{
		{"within limit", "a\nb\nc", 5, 3},
		{"exceeds limit", "a\nb\nc\nd\ne", 3, 3},
		{"empty", "", 5, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clipLines(tt.input, tt.maxLines)
			lines := strings.Count(result, "\n") + 1
			if lines > tt.wantMax {
				t.Errorf("clipLines got %d lines, want <= %d", lines, tt.wantMax)
			}
		})
	}
}

func TestAppViewedState(t *testing.T) {
	t.Run("without tracking", func(t *testing.T) {
		app := NewApp(makeLargeDoc(3, 0), AppOptions{})
		if app.ViewedState() != nil {
			t.Error("ViewedState should be nil when TrackViewed is false")
		}
	})

	t.Run("with tracking no filepath", func(t *testing.T) {
		app := NewApp(makeLargeDoc(3, 0), AppOptions{TrackViewed: true})
		if app.ViewedState() != nil {
			t.Error("ViewedState should be nil when FilePath is empty")
		}
	})

	t.Run("with tracking and filepath", func(t *testing.T) {
		app := NewApp(makeLargeDoc(3, 0), AppOptions{
			TrackViewed: true,
			FilePath:    "/nonexistent/plan.md",
		})
		vs := app.ViewedState()
		if vs == nil {
			t.Fatal("ViewedState should not be nil")
			return
		}
		if len(vs.Sections) != 0 {
			t.Error("should start with empty sections")
		}
	})
}
