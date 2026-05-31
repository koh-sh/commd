package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/koh-sh/commd/internal/markdown"
)

func newTestLinePane(lines []string, sections []*markdown.Section) *LinePane {
	styles := stylesForTheme(ThemeDark)
	return NewLinePane(lines, 40, 10, styles, sections)
}

func TestLinePaneCursorMovement(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4", "line5"}

	tests := []struct {
		name   string
		setup  func(*LinePane)
		action func(*LinePane)
		want   int
	}{
		{"down from top", func(lp *LinePane) {}, func(lp *LinePane) { lp.CursorDown() }, 1},
		{"up from middle", func(lp *LinePane) { lp.CursorDown(); lp.CursorDown() }, func(lp *LinePane) { lp.CursorUp() }, 1},
		{"top", func(lp *LinePane) { lp.CursorDown(); lp.CursorDown() }, func(lp *LinePane) { lp.CursorTop() }, 0},
		{"bottom", func(lp *LinePane) {}, func(lp *LinePane) { lp.CursorBottom() }, 4},
		{"down at bottom stays", func(lp *LinePane) { lp.CursorBottom() }, func(lp *LinePane) { lp.CursorDown() }, 4},
		{"up at top stays", func(lp *LinePane) {}, func(lp *LinePane) { lp.CursorUp() }, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(lines, nil)
			tt.setup(lp)
			tt.action(lp)
			if lp.Cursor() != tt.want {
				t.Errorf("cursor = %d, want %d", lp.Cursor(), tt.want)
			}
		})
	}
}

func TestLinePaneSelectedRange(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}

	tests := []struct {
		name      string
		setup     func(*LinePane)
		wantStart int
		wantEnd   int
	}{
		{
			name:      "single line no selection",
			setup:     func(lp *LinePane) {},
			wantStart: 1,
			wantEnd:   0,
		},
		{
			name: "visual select forward",
			setup: func(lp *LinePane) {
				lp.CursorDown() // cursor=1
				lp.StartVisualSelect()
				lp.CursorDown() // cursor=2
				lp.CursorDown() // cursor=3
			},
			wantStart: 2,
			wantEnd:   4,
		},
		{
			name: "visual select backward",
			setup: func(lp *LinePane) {
				lp.CursorBottom() // cursor=4
				lp.CursorUp()     // cursor=3
				lp.StartVisualSelect()
				lp.CursorUp() // cursor=2
				lp.CursorUp() // cursor=1
			},
			wantStart: 2,
			wantEnd:   4,
		},
		{
			name: "after cancel",
			setup: func(lp *LinePane) {
				lp.CursorDown() // cursor=1
				lp.StartVisualSelect()
				lp.CursorDown() // cursor=2
				lp.CancelVisualSelect()
			},
			wantStart: 3,
			wantEnd:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(lines, nil)
			tt.setup(lp)
			start, end := lp.SelectedRange()
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("SelectedRange() = (%d, %d), want (%d, %d)", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestLinePaneSectionIDAtLine(t *testing.T) {
	sections := []*markdown.Section{
		{ID: "S1", StartLine: 5, EndLine: 10},
		{ID: "S2", StartLine: 12, EndLine: 20},
	}
	lp := newTestLinePane(make([]string, 25), sections)

	tests := []struct {
		line int
		want string
	}{
		{1, markdown.OverviewSectionID},
		{4, markdown.OverviewSectionID},
		{5, "S1"},
		{10, "S1"},
		{11, "S1"},
		{12, "S2"},
		{20, "S2"},
		{25, "S2"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("line_%d", tt.line), func(t *testing.T) {
			got := lp.SectionIDAtLine(tt.line)
			if got != tt.want {
				t.Errorf("SectionIDAtLine(%d) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestLinePaneSectionIDAtCursor(t *testing.T) {
	sections := []*markdown.Section{
		{ID: "S1", StartLine: 5, EndLine: 10},
		{ID: "S2", StartLine: 12, EndLine: 20},
	}

	tests := []struct {
		name        string
		diffLineMap []int // nil = non-diff mode
		cursor      int
		want        string
	}{
		{name: "non-diff uses cursor+1 as file line", diffLineMap: nil, cursor: 4, want: "S1"},
		{name: "non-diff before any section", diffLineMap: nil, cursor: 0, want: markdown.OverviewSectionID},
		// In diff mode the cursor is a display index; the file line comes from
		// diffLineMap. Display index 3 maps to file line 12 (S2), proving the
		// display index is no longer used directly as a file line.
		{name: "diff maps display index to file line", diffLineMap: []int{1, 5, 0, 12}, cursor: 3, want: "S2"},
		{name: "diff maps to first section", diffLineMap: []int{1, 5, 0, 12}, cursor: 1, want: "S1"},
		{name: "diff non-commentable line returns empty", diffLineMap: []int{1, 5, 0, 12}, cursor: 2, want: ""},
		{name: "diff cursor past map returns empty", diffLineMap: []int{1, 5}, cursor: 9, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(make([]string, 25), sections)
			lp.diffLineMap = tt.diffLineMap
			lp.cursor = tt.cursor

			got := lp.SectionIDAtCursor()
			if got != tt.want {
				t.Errorf("SectionIDAtCursor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLinePaneScrollToLine(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line"
	}
	lp := newTestLinePane(lines, nil)
	lp.SetSize(40, 10)

	lp.ScrollToLine(50)
	if lp.Cursor() != 49 { // 0-based
		t.Errorf("cursor = %d, want 49", lp.Cursor())
	}
	// Cursor should be visible
	if lp.scrollOffset > 49 || lp.scrollOffset+lp.height <= 49 {
		t.Errorf("cursor not visible: scrollOffset=%d, height=%d, cursor=%d",
			lp.scrollOffset, lp.height, lp.Cursor())
	}
}

func TestLinePaneViewContainsLineNumbers(t *testing.T) {
	lines := []string{"hello", "world", "test"}
	lp := newTestLinePane(lines, nil)
	lp.SetSize(40, 10)

	view := lp.View()
	if len(view) == 0 {
		t.Fatal("View() returned empty string")
	}
	if !strings.Contains(view, "│") {
		t.Error("View() should contain gutter separator │")
	}
	if !strings.Contains(view, "hello") {
		t.Error("View() should contain source line content 'hello'")
	}
}

func TestLinePaneEmptyLines(t *testing.T) {
	lp := newTestLinePane([]string{}, nil)
	lp.SetSize(40, 10)

	view := lp.View()
	if view != "" {
		t.Errorf("View() on empty should return empty string, got %q", view)
	}

	start, end := lp.SelectedRange()
	if start != 1 || end != 0 {
		t.Errorf("SelectedRange() on empty = (%d, %d), want (1, 0)", start, end)
	}
}

func TestLinePaneIsVisualSelect(t *testing.T) {
	lp := newTestLinePane([]string{"a", "b"}, nil)

	if lp.IsVisualSelect() {
		t.Error("should not be in visual select initially")
	}

	lp.StartVisualSelect()
	if !lp.IsVisualSelect() {
		t.Error("should be in visual select after StartVisualSelect")
	}

	lp.CancelVisualSelect()
	if lp.IsVisualSelect() {
		t.Error("should not be in visual select after cancel")
	}
}

func TestLinePaneViewRange(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	lp := newTestLinePane(lines, nil)

	// Set view range to lines 3-5 (1-based)
	lp.SetViewRange(3, 5)
	if lp.rangeStart() != 2 || lp.rangeEnd() != 5 {
		t.Errorf("range = [%d,%d), want [2,5)", lp.rangeStart(), lp.rangeEnd())
	}
	// Cursor should be clamped into range
	if lp.cursor < 2 || lp.cursor > 4 {
		t.Errorf("cursor = %d, should be in [2,4]", lp.cursor)
	}

	// CursorTop/Bottom should respect range
	lp.CursorTop()
	if lp.cursor != 2 {
		t.Errorf("CursorTop = %d, want 2", lp.cursor)
	}
	lp.CursorBottom()
	if lp.cursor != 4 {
		t.Errorf("CursorBottom = %d, want 4", lp.cursor)
	}

	// CursorUp at range top should not move
	lp.CursorTop()
	lp.CursorUp()
	if lp.cursor != 2 {
		t.Errorf("CursorUp at top = %d, want 2", lp.cursor)
	}

	// CursorDown at range bottom should not move
	lp.CursorBottom()
	lp.CursorDown()
	if lp.cursor != 4 {
		t.Errorf("CursorDown at bottom = %d, want 4", lp.cursor)
	}

	// AtRangeTop/Bottom
	lp.CursorTop()
	if !lp.AtRangeTop() {
		t.Error("should be AtRangeTop")
	}
	if lp.AtRangeBottom() {
		t.Error("should not be AtRangeBottom")
	}
	lp.CursorBottom()
	if lp.AtRangeTop() {
		t.Error("should not be AtRangeTop")
	}
	if !lp.AtRangeBottom() {
		t.Error("should be AtRangeBottom")
	}

	// ClearViewRange restores full range
	lp.ClearViewRange()
	if lp.rangeStart() != 0 || lp.rangeEnd() != 8 {
		t.Errorf("after clear: range = [%d,%d), want [0,8)", lp.rangeStart(), lp.rangeEnd())
	}
}

func TestLinePaneViewRangeRendering(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4", "line5"}
	lp := newTestLinePane(lines, nil)
	lp.SetSize(40, 10)

	// Full view shows all lines
	view := lp.View()
	if len(view) == 0 {
		t.Fatal("full view should not be empty")
	}

	// Section view: only lines 2-3
	lp.SetViewRange(2, 3)
	view = lp.View()
	if len(view) == 0 {
		t.Fatal("section view should not be empty")
	}
	if strings.Contains(view, "line1") {
		t.Error("section view should not contain line1 (out of range)")
	}
	if !strings.Contains(view, "line2") {
		t.Error("section view should contain line2")
	}
	if strings.Contains(view, "line4") {
		t.Error("section view should not contain line4 (out of range)")
	}
}

func TestLinePanePageScroll(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "line"
	}
	lp := newTestLinePane(lines, nil)
	lp.SetSize(40, 10)

	lp.HalfPageDown()
	if lp.Cursor() != 5 {
		t.Errorf("after HalfPageDown: cursor = %d, want 5", lp.Cursor())
	}

	lp.PageDown()
	if lp.Cursor() != 15 {
		t.Errorf("after PageDown: cursor = %d, want 15", lp.Cursor())
	}

	lp.HalfPageUp()
	if lp.Cursor() != 10 {
		t.Errorf("after HalfPageUp: cursor = %d, want 10", lp.Cursor())
	}

	lp.PageUp()
	if lp.Cursor() != 0 {
		t.Errorf("after PageUp: cursor = %d, want 0", lp.Cursor())
	}
}

func TestLinePaneDiffMode(t *testing.T) {
	lines := []string{" context", "-removed", "+added", " more"}
	diffLineMap := []int{1, 1, 2, 3}
	diffSideMap := []string{"RIGHT", "LEFT", "RIGHT", "RIGHT"}

	tests := []struct {
		name      string
		cursor    int
		wantCan   bool
		wantStart int
		wantEnd   int
	}{
		{"context line commentable", 0, true, 1, 0},
		{"removed line commentable", 1, true, 1, 0},
		{"added line commentable", 2, true, 2, 0},
		{"second context commentable", 3, true, 3, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(lines, nil)
			lp.diffLineMap = diffLineMap
			lp.diffSideMap = diffSideMap
			lp.cursor = tt.cursor
			lp.selectAnchor = -1
			if got := lp.CanComment(); got != tt.wantCan {
				t.Errorf("CanComment() = %v, want %v", got, tt.wantCan)
			}
			start, end := lp.SelectedRange()
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("SelectedRange() = (%d, %d), want (%d, %d)", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestLinePaneDiffModeSetViewRange(t *testing.T) {
	lines := []string{" ctx1", "+add", "-del", " ctx2", " ctx3", "+add2"}
	lp := newTestLinePane(lines, nil)
	lp.diffLineMap = []int{1, 2, 0, 5, 6, 10}
	lp.diffSideMap = []string{"RIGHT", "RIGHT", "LEFT", "RIGHT", "RIGHT", "RIGHT"}

	// Set view range to file lines 5-6 (should show diff indices 3-4)
	lp.SetViewRange(5, 6)
	if lp.rangeStart() != 3 {
		t.Errorf("rangeStart() = %d, want 3", lp.rangeStart())
	}
	if lp.rangeEnd() != 5 {
		t.Errorf("rangeEnd() = %d, want 5", lp.rangeEnd())
	}

	// Set range that has no diff lines — should clear view range
	lp.SetViewRange(20, 30)

	// Clear view range
	lp.ClearViewRange()
	if lp.rangeStart() != 0 || lp.rangeEnd() != len(lines) {
		t.Errorf("after clear: start=%d end=%d, want 0-%d", lp.rangeStart(), lp.rangeEnd(), len(lines))
	}
}

func TestLinePaneDiffEmptyRange(t *testing.T) {
	lines := []string{" ctx"}
	lp := newTestLinePane(lines, nil)
	lp.diffLineMap = []int{10}
	lp.diffSideMap = []string{"RIGHT"}
	lp.SetSize(40, 5)

	// Set range that has no diff lines
	lp.SetViewRange(1, 5)
	if !lp.emptyRange {
		t.Error("expected emptyRange to be true")
	}

	view := lp.View()
	if !strings.Contains(view, "No changes") {
		t.Errorf("expected 'No changes' message, got %q", view)
	}

	// Clear should reset
	lp.ClearViewRange()
	if lp.emptyRange {
		t.Error("expected emptyRange to be false after clear")
	}
}

func TestLinePaneCursorSide(t *testing.T) {
	lp := newTestLinePane([]string{"a", "b"}, nil)
	lp.diffSideMap = []string{"RIGHT", "LEFT"}

	lp.cursor = 0
	if got := lp.CursorSide(); got != "RIGHT" {
		t.Errorf("CursorSide() = %q, want RIGHT", got)
	}
	lp.cursor = 1
	if got := lp.CursorSide(); got != "LEFT" {
		t.Errorf("CursorSide() = %q, want LEFT", got)
	}

	// Non-diff mode
	lp2 := newTestLinePane([]string{"a"}, nil)
	if got := lp2.CursorSide(); got != "" {
		t.Errorf("CursorSide() non-diff = %q, want empty", got)
	}
}

func TestLinePaneDiffStyleForLine(t *testing.T) {
	lines := []string{"  context", "+ added", "- removed", "+ also added"}
	lp := newTestLinePane(lines, nil)
	lp.diffTypeMap = []byte{' ', '+', '-', '+'}

	tests := []struct {
		name     string
		idx      int
		wantNil  bool
		wantKind string // "added", "removed", or ""
	}{
		{"context line has no style", 0, true, ""},
		{"added line has added style", 1, false, "added"},
		{"removed line has removed style", 2, false, "removed"},
		{"another added line", 3, false, "added"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lp.diffStyleForLine(tt.idx)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil style, got non-nil")
				}
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil style for %s", tt.wantKind)
			}
			// Verify it's the correct style by comparing foreground color
			switch tt.wantKind {
			case "added":
				if got.GetForeground() != lp.styles.DiffAdded.GetForeground() {
					t.Errorf("expected DiffAdded foreground")
				}
			case "removed":
				if got.GetForeground() != lp.styles.DiffRemoved.GetForeground() {
					t.Errorf("expected DiffRemoved foreground")
				}
			}
		})
	}

	// Non-diff mode returns nil
	lp2 := newTestLinePane([]string{"a"}, nil)
	if got := lp2.diffStyleForLine(0); got != nil {
		t.Error("non-diff mode should return nil")
	}

	// Out of bounds returns nil
	if got := lp.diffStyleForLine(99); got != nil {
		t.Error("out of bounds should return nil")
	}
}

func TestLinePaneDiffModeVisualSelect(t *testing.T) {
	lines := []string{" ctx", "+add1", "-del", "+add2", " ctx2"}
	lp := newTestLinePane(lines, nil)
	lp.diffLineMap = []int{1, 2, 0, 3, 4}
	lp.diffSideMap = []string{"RIGHT", "RIGHT", "LEFT", "RIGHT", "RIGHT"}

	// Visual select from line 0 to 3 (includes removed line in middle)
	lp.cursor = 0
	lp.StartVisualSelect()
	lp.cursor = 3
	start, end := lp.SelectedRange()
	if start != 1 || end != 3 {
		t.Errorf("visual select range = (%d, %d), want (1, 3)", start, end)
	}
}

// TestLinePaneDiffModeVisualSelectSingleSide verifies that a visual selection
// spanning both removed (LEFT/old) and added (RIGHT/new) lines is restricted to
// the cursor's side, so it never yields a mixed-side range (which GitHub
// rejects with HTTP 422).
func TestLinePaneDiffModeVisualSelectSingleSide(t *testing.T) {
	// Display: 0:ctx 1:-del(old10) 2:-del(old11) 3:+add(new20) 4:+add(new21)
	lines := []string{" ctx", "-del1", "-del2", "+add1", "+add2"}
	diffLineMap := []int{5, 10, 11, 20, 21}
	diffSideMap := []string{"RIGHT", "LEFT", "LEFT", "RIGHT", "RIGHT"}

	tests := []struct {
		name      string
		anchor    int
		cursor    int
		wantStart int
		wantEnd   int
	}{
		{
			// Cursor ends on an added (RIGHT) line: keep only new-file lines.
			name: "cursor on RIGHT keeps added lines", anchor: 1, cursor: 4,
			wantStart: 20, wantEnd: 21,
		},
		{
			// Cursor ends on a removed (LEFT) line: keep only old-file lines.
			name: "cursor on LEFT keeps removed lines", anchor: 4, cursor: 1,
			wantStart: 10, wantEnd: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(lines, nil)
			lp.diffLineMap = diffLineMap
			lp.diffSideMap = diffSideMap
			lp.cursor = tt.anchor
			lp.StartVisualSelect()
			lp.cursor = tt.cursor
			start, end := lp.SelectedRange()
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("SelectedRange() = (%d, %d), want (%d, %d)", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestLinePaneOverviewComments(t *testing.T) {
	tests := []struct {
		name         string
		comment      *markdown.ReviewComment
		wantRendered bool
	}{
		{
			name: "overview comment shown at top",
			comment: &markdown.ReviewComment{
				SectionID: markdown.OverviewSectionID,
				Action:    markdown.ActionIssue,
				Body:      "RAWOVERVIEW",
			},
			wantRendered: true,
		},
		{
			// A section-level comment that is not the Overview is not tied to a
			// line either, but it belongs to a real section, so it must NOT be
			// hoisted to the top of the raw view.
			name: "non-overview section comment not shown at top",
			comment: &markdown.ReviewComment{
				SectionID: "S1",
				Action:    markdown.ActionIssue,
				Body:      "SECTIONLEVEL",
			},
			wantRendered: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane([]string{"line one", "line two", "line three"}, nil)
			lp.SetComments([]*markdown.ReviewComment{tt.comment})

			plain := ansiRe.ReplaceAllString(lp.View(), "")
			got := strings.Contains(plain, tt.comment.Body)
			if got != tt.wantRendered {
				t.Errorf("raw view contains %q = %v, want %v\nview:\n%s", tt.comment.Body, got, tt.wantRendered, plain)
			}
		})
	}
}

func TestLinePaneOverviewCommentHiddenWhenScrolled(t *testing.T) {
	// Enough lines that the top scrolls out of view.
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	lp := newTestLinePane(lines, nil)
	lp.SetComments([]*markdown.ReviewComment{
		{SectionID: markdown.OverviewSectionID, Action: markdown.ActionIssue, Body: "TOPONLY"},
	})

	// Scroll past the top: the overview box anchors to the start of the range,
	// so it should no longer be visible.
	lp.ScrollToLine(25)
	plain := ansiRe.ReplaceAllString(lp.View(), "")
	if strings.Contains(plain, "TOPONLY") {
		t.Errorf("overview comment should not render once scrolled past the top, got:\n%s", plain)
	}
}
