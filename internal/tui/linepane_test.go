package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/koh-sh/commd/internal/markdown"
	"github.com/mattn/go-runewidth"
)

func newTestLinePane(lines []string, lineSections []string) *LinePane {
	styles := stylesForTheme(ThemeDark)
	return NewLinePane(lines, 40, 10, styles, lineSections)
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

func TestLinePaneSectionIDAtCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		want   string
	}{
		{name: "line before any section", cursor: 0, want: markdown.OverviewSectionID},
		{name: "line in a section", cursor: 2, want: "S1"},
		{name: "cursor past the lines", cursor: 9, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(make([]string, 4), []string{markdown.OverviewSectionID, "S1", "S1", "S2"})
			lp.cursor = tt.cursor
			if got := lp.SectionIDAtCursor(); got != tt.want {
				t.Errorf("SectionIDAtCursor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLinePaneSelectedSectionID(t *testing.T) {
	// A removed line (LEFT) of S1 followed by added lines (RIGHT) of S1 and S2.
	sides := []string{"RIGHT", "LEFT", "RIGHT", "RIGHT"}
	sections := []string{"S1", "S1", "S1", "S2"}
	tests := []struct {
		name   string
		anchor int // -1 = no selection
		cursor int
		want   string
	}{
		{name: "cursor line without selection", anchor: -1, cursor: 3, want: "S2"},
		{name: "first selected line", anchor: 0, cursor: 3, want: "S1"},
		{name: "first line on the cursor's side", anchor: 1, cursor: 3, want: "S1"},
		{name: "selection upwards", anchor: 3, cursor: 2, want: "S1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(make([]string, 4), sections)
			lp.diffLineMap = []int{1, 2, 2, 5}
			lp.diffSideMap = sides
			lp.selectAnchor = tt.anchor
			lp.cursor = tt.cursor
			if got := lp.SelectedSectionID(); got != tt.want {
				t.Errorf("SelectedSectionID() = %q, want %q", got, tt.want)
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
	lp := newTestLinePane(lines, []string{"S1", "S1", "S2", "S2", "S2", "S3", "S3", "S3"})

	// Show S2: lines 3-5 (1-based)
	lp.SetViewSection("S2")
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
	lp := newTestLinePane(lines, []string{"S1", "S2", "S2", "S3", "S3"})
	lp.SetSize(40, 10)

	// Full view shows all lines
	view := lp.View()
	if len(view) == 0 {
		t.Fatal("full view should not be empty")
	}

	// Section view: only lines 2-3
	lp.SetViewSection("S2")
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

func TestLinePaneSetViewSection(t *testing.T) {
	// Diff lines: a removed line follows the preceding line's section (S1).
	sections := []string{markdown.OverviewSectionID, "S1", "S1", "S1", "S2", "S2"}
	tests := []struct {
		name      string
		section   string
		wantStart int
		wantEnd   int
		wantEmpty bool
	}{
		{name: "section with a removed line", section: "S1", wantStart: 1, wantEnd: 4},
		{name: "last section", section: "S2", wantStart: 4, wantEnd: 6},
		{name: "section without lines", section: "S3", wantEmpty: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(make([]string, len(sections)), sections)
			lp.SetViewSection(tt.section)
			if lp.emptyRange != tt.wantEmpty {
				t.Fatalf("emptyRange = %v, want %v", lp.emptyRange, tt.wantEmpty)
			}
			if !tt.wantEmpty && (lp.rangeStart() != tt.wantStart || lp.rangeEnd() != tt.wantEnd) {
				t.Errorf("range = [%d,%d), want [%d,%d)", lp.rangeStart(), lp.rangeEnd(), tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestLinePaneDiffEmptyRange(t *testing.T) {
	lines := []string{" ctx"}
	lp := newTestLinePane(lines, []string{"S2"})
	lp.diffLineMap = []int{10}
	lp.diffSideMap = []string{"RIGHT"}
	lp.SetSize(40, 5)

	// A section with no diff lines
	lp.SetViewSection("S1")
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
		{
			// Selection spans both sides but only one RIGHT line survives the
			// side filter: the range collapses to a single line (endLine 0).
			name: "single matched line after side filter collapses to single line", anchor: 1, cursor: 3,
			wantStart: 20, wantEnd: 0,
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

// TestSetSizeKeepsCursorVisible verifies that resizing the pane re-clamps the
// scroll offset so the cursor never ends up outside the visible rows.
func TestSetSizeKeepsCursorVisible(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i+1)
	}

	tests := []struct {
		name   string
		move   func(lp *LinePane)
		height int
	}{
		{"shrink with cursor at bottom", func(lp *LinePane) { lp.CursorBottom() }, 4},
		{"shrink with cursor below new height", func(lp *LinePane) {
			for range 7 {
				lp.CursorDown()
			}
		}, 3},
		{"grow does not scroll past the end", func(lp *LinePane) { lp.CursorBottom() }, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(lines, nil) // height 10
			tt.move(lp)

			lp.SetSize(40, tt.height)

			if lp.cursor < lp.scrollOffset || lp.cursor >= lp.scrollOffset+tt.height {
				t.Errorf("cursor %d not within visible rows [%d, %d)", lp.cursor, lp.scrollOffset, lp.scrollOffset+tt.height)
			}
			if maxOffset := max(len(lines)-tt.height, 0); lp.scrollOffset > maxOffset {
				t.Errorf("scrollOffset = %d, want <= %d", lp.scrollOffset, maxOffset)
			}
		})
	}
}

func TestLinePaneEmptyRangeBlocksCommenting(t *testing.T) {
	lp := newTestLinePane([]string{"+a", "+b"}, []string{"S1", "S1"})
	lp.diffLineMap = []int{3, 4}
	lp.diffSideMap = []string{"RIGHT", "RIGHT"}
	lp.SetSize(40, 5)
	lp.SetViewSection("S2") // no diff lines in this section

	if lp.CanComment() {
		t.Error("CanComment() = true in an empty range, want false")
	}
	if start, end := lp.SelectedRange(); start != 0 || end != 0 {
		t.Errorf("SelectedRange() = (%d, %d) in an empty range, want (0, 0)", start, end)
	}
	lp.StartVisualSelect()
	if lp.IsVisualSelect() {
		t.Error("StartVisualSelect() started a selection in an empty range")
	}
}

func TestLinePaneScrollToLineDiffMode(t *testing.T) {
	// Hunk starting at new line 100: context, added, removed, context.
	lines := []string{"  a", "+ b", "- z", "  c"}
	tests := []struct {
		name       string
		line       int
		wantCursor int
	}{
		{"first hunk line", 100, 0},
		{"added line", 101, 1},
		{"context after removed line", 102, 3},
		{"line before the hunk snaps to first hunk line", 1, 0},
		{"line past the hunk snaps to last display line", 500, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(lines, nil)
			lp.diffLineMap = []int{100, 101, 100, 102}
			lp.diffSideMap = []string{"RIGHT", "RIGHT", "LEFT", "RIGHT"}
			lp.SetSize(40, 10)

			lp.ScrollToLine(tt.line)
			if lp.Cursor() != tt.wantCursor {
				t.Errorf("cursor = %d, want %d", lp.Cursor(), tt.wantCursor)
			}
		})
	}
}

func TestLinePaneCommentBoxKeyedBySide(t *testing.T) {
	// Old line 1 was replaced by new line 1, so both sides carry line 1.
	lines := []string{"- old", "+ new", "  ctx"}
	comment := &markdown.ReviewComment{SectionID: "S1", StartLine: 1, Side: "LEFT", Body: "on removed", Action: markdown.ActionNote}

	tests := []struct {
		name      string
		side      string
		wantAfter string // the line the box must directly follow
	}{
		{"LEFT comment follows the removed line", "LEFT", "- old"},
		{"RIGHT comment follows the added line", "RIGHT", "+ new"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(lines, nil)
			lp.diffLineMap = []int{1, 1, 2}
			lp.diffSideMap = []string{"LEFT", "RIGHT", "RIGHT"}
			lp.diffTypeMap = []byte{'-', '+', ' '}
			lp.SetSize(60, 12)
			c := *comment
			c.Side = tt.side
			lp.SetComments([]*markdown.ReviewComment{&c})

			view := ansiRe.ReplaceAllString(lp.View(), "")
			if n := strings.Count(view, "on removed"); n != 1 {
				t.Fatalf("comment box rendered %d times, want 1:\n%s", n, view)
			}
			rows := strings.Split(view, "\n")
			for i, row := range rows {
				if strings.Contains(row, "Review Comment") {
					if !strings.Contains(rows[i-2], tt.wantAfter) { // i-1 is the box top border
						t.Errorf("box follows %q, want %q", rows[i-2], tt.wantAfter)
					}
					return
				}
			}
			t.Fatal("comment box not rendered")
		})
	}
}

func TestLinePaneCursorVisibleWithCommentBoxes(t *testing.T) {
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%02d", i+1)
	}
	comments := []*markdown.ReviewComment{
		{SectionID: "S1", StartLine: 1, Body: "first", Action: markdown.ActionNote},
		{SectionID: "S1", StartLine: 6, Body: "sixth", Action: markdown.ActionNote},
	}
	tests := []struct {
		name   string
		move   func(lp *LinePane)
		cursor int
	}{
		{"cursor below a box near the top", func(lp *LinePane) {
			for range 3 {
				lp.CursorDown()
			}
		}, 3},
		{"cursor at the bottom", func(lp *LinePane) { lp.CursorBottom() }, 11},
		{"jump then shrink", func(lp *LinePane) { lp.ScrollToLine(7); lp.SetSize(60, 4) }, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newTestLinePane(lines, nil)
			lp.SetSize(60, 6)
			lp.SetComments(comments)
			tt.move(lp)

			if lp.cursor != tt.cursor {
				t.Fatalf("cursor = %d, want %d", lp.cursor, tt.cursor)
			}
			view := ansiRe.ReplaceAllString(lp.View(), "")
			if !strings.Contains(view, lines[lp.cursor]) {
				t.Errorf("cursor line %q not rendered (scrollOffset=%d):\n%s", lines[lp.cursor], lp.scrollOffset, view)
			}
		})
	}
}

func TestFitToWidth(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"pads short text", "ab", 4, "ab  "},
		{"truncates long text", "abcdef", 4, "abcd"},
		{"leading tab expands to the first tab stop", "\tfoo", 10, "    foo   "},
		{"tab inside text expands to the next tab stop", "ab\tc\td", 12, "ab  c   d   "},
		{"tab after wide characters is truncated at the width", "日本\tx", 8, "日本    "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fitToWidth(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("fitToWidth(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
			if w := runewidth.StringWidth(got); w != tt.width {
				t.Errorf("width = %d, want %d", w, tt.width)
			}
		})
	}
}
