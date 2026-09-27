package tui

import (
	"strings"
	"testing"

	"github.com/koh-sh/commd/internal/markdown"
)

func makeDocWithChildren() *markdown.Document {
	p := &markdown.Document{
		Title:    "Test Plan",
		Preamble: "Overview text",
	}
	s1 := &markdown.Section{ID: "S1", Title: "Step 1", Level: 2, Body: "Body 1"}
	s1_1 := &markdown.Section{ID: "S1.1", Title: "Sub 1.1", Level: 3, Body: "Body 1.1", Parent: s1}
	s1_2 := &markdown.Section{ID: "S1.2", Title: "Sub 1.2", Level: 3, Body: "Body 1.2", Parent: s1}
	s1.Children = []*markdown.Section{s1_1, s1_2}
	s2 := &markdown.Section{ID: "S2", Title: "Step 2", Level: 2, Body: "Body 2"}
	p.Sections = []*markdown.Section{s1, s2}
	return p
}

func makeDocNoPreamble() *markdown.Document {
	p := &markdown.Document{Title: "No Preamble"}
	s1 := &markdown.Section{ID: "S1", Title: "Step 1", Level: 2}
	p.Sections = []*markdown.Section{s1}
	return p
}

func newTestSectionList(doc *markdown.Document) *SectionList {
	return NewSectionList(markdown.NewReviewState(markdown.File{Doc: doc}))
}

func TestRenderBadge(t *testing.T) {
	tests := []struct {
		name     string
		comments int
		viewed   bool
		want     []string
	}{
		{name: "no badge"},
		{name: "single comment", comments: 1, want: []string{"[*]"}},
		{name: "multiple comments", comments: 2, want: []string{"[*2]"}},
		{name: "viewed", viewed: true, want: []string{"[✓]"}},
		{name: "comments and viewed", comments: 1, viewed: true, want: []string{"[*]", "[✓]"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl := newTestSectionList(makeDocWithChildren())
			for range tt.comments {
				if err := sl.review.SaveComment(markdown.ReviewComment{SectionID: "S1", Action: markdown.ActionNote, Body: "b"}); err != nil {
					t.Fatal(err)
				}
			}
			if tt.viewed {
				if err := sl.review.SetViewed("S1", true); err != nil {
					t.Fatal(err)
				}
			}
			badge := sl.renderBadge("S1", defaultStyles())
			if len(tt.want) == 0 && badge != "" {
				t.Errorf("badge = %q, want none", badge)
			}
			for _, w := range tt.want {
				if !strings.Contains(badge, w) {
					t.Errorf("badge = %q, want it to contain %q", badge, w)
				}
			}
		})
	}
}

func TestNewSectionList(t *testing.T) {
	t.Run("with preamble", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		if !sl.items[0].IsOverview {
			t.Error("first item should be overview when preamble exists")
		}
		// overview + S1 + S1.1 + S1.2 + S2 = 5
		if len(sl.items) != 5 {
			t.Errorf("items count = %d, want 5", len(sl.items))
		}
	})

	t.Run("without preamble", func(t *testing.T) {
		sl := newTestSectionList(makeDocNoPreamble())
		if sl.items[0].IsOverview {
			t.Error("first item should not be overview when no preamble")
		}
		if len(sl.items) != 1 {
			t.Errorf("items count = %d, want 1", len(sl.items))
		}
	})
}

func TestCursorUpDown(t *testing.T) {
	sl := newTestSectionList(makeDocWithChildren())

	// Initial cursor at 0 (overview)
	if sl.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", sl.cursor)
	}

	// CursorUp at top should stay
	sl.CursorUp()
	if sl.cursor != 0 {
		t.Errorf("CursorUp at top: cursor = %d, want 0", sl.cursor)
	}

	// Move down
	sl.CursorDown()
	if sl.items[sl.cursor].Section.ID != "S1" {
		t.Errorf("after CursorDown: section = %s, want S1", sl.items[sl.cursor].Section.ID)
	}

	// Move down to S1.1
	sl.CursorDown()
	if sl.items[sl.cursor].Section.ID != "S1.1" {
		t.Errorf("after 2nd CursorDown: section = %s, want S1.1", sl.items[sl.cursor].Section.ID)
	}

	// Move up back to S1
	sl.CursorUp()
	if sl.items[sl.cursor].Section.ID != "S1" {
		t.Errorf("after CursorUp: section = %s, want S1", sl.items[sl.cursor].Section.ID)
	}

	// CursorDown at bottom should stay
	sl.CursorBottom()
	bottom := sl.cursor
	sl.CursorDown()
	if sl.cursor != bottom {
		t.Errorf("CursorDown at bottom: cursor = %d, want %d", sl.cursor, bottom)
	}
}

func TestCursorUpDownSkipsHidden(t *testing.T) {
	sl := newTestSectionList(makeDocWithChildren())

	// Collapse S1 to hide children
	sl.CursorDown() // move to S1
	sl.ToggleExpand()

	// S1.1 and S1.2 are now hidden
	// CursorDown from S1 should skip to S2
	sl.CursorDown()
	if sl.items[sl.cursor].Section.ID != "S2" {
		t.Errorf("CursorDown skipping hidden: section = %s, want S2", sl.items[sl.cursor].Section.ID)
	}

	// CursorUp from S2 should skip to S1
	sl.CursorUp()
	if sl.items[sl.cursor].Section.ID != "S1" {
		t.Errorf("CursorUp skipping hidden: section = %s, want S1", sl.items[sl.cursor].Section.ID)
	}
}

func TestCursorTopBottom(t *testing.T) {
	sl := newTestSectionList(makeDocWithChildren())

	sl.CursorBottom()
	if sl.items[sl.cursor].Section.ID != "S2" {
		t.Errorf("CursorBottom: section = %s, want S2", sl.items[sl.cursor].Section.ID)
	}

	sl.CursorTop()
	if !sl.items[sl.cursor].IsOverview {
		t.Error("CursorTop should go to overview")
	}
}

func TestToggleExpand(t *testing.T) {
	t.Run("toggle with children", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		sl.CursorDown() // S1

		if !sl.items[sl.cursor].Expanded {
			t.Fatal("S1 should start expanded")
		}

		sl.ToggleExpand()
		if sl.items[sl.cursor].Expanded {
			t.Error("S1 should be collapsed after toggle")
		}
		// Children should be hidden
		if sl.items[2].Visible { // S1.1
			t.Error("S1.1 should be hidden after collapse")
		}

		sl.ToggleExpand()
		if !sl.items[sl.cursor].Expanded {
			t.Error("S1 should be expanded after second toggle")
		}
		if !sl.items[2].Visible { // S1.1
			t.Error("S1.1 should be visible after expand")
		}
	})

	t.Run("toggle without children", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		sl.CursorBottom() // S2 (no children)
		expanded := sl.items[sl.cursor].Expanded
		sl.ToggleExpand() // should be no-op for leaf node
		if sl.items[sl.cursor].Expanded != expanded {
			t.Error("ToggleExpand on leaf should not change Expanded state")
		}
	})

	t.Run("toggle overview", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		// cursor at overview
		if !sl.IsOverviewSelected() {
			t.Fatal("cursor should be on overview")
		}
		sl.ToggleExpand() // should be no-op
		if !sl.IsOverviewSelected() {
			t.Error("cursor should remain on overview after ToggleExpand")
		}
	})
}

func TestFilterByQuery(t *testing.T) {
	t.Run("partial match", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		sl.FilterByQuery("Sub")
		// S1.1 and S1.2 match, S1 is ancestor
		for _, item := range sl.items {
			if item.Section != nil && item.Section.ID == "S2" && item.Visible {
				t.Error("S2 should be hidden")
			}
			if item.Section != nil && item.Section.ID == "S1" && !item.Visible {
				t.Error("S1 (ancestor of match) should be visible")
			}
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		sl.FilterByQuery("step 2")
		for _, item := range sl.items {
			if item.Section != nil && item.Section.ID == "S2" && !item.Visible {
				t.Error("S2 should match case-insensitive")
			}
		}
	})

	t.Run("shows descendants", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		sl.FilterByQuery("Step 1")
		// S1 matches, children should be visible
		for _, item := range sl.items {
			if item.Section != nil && item.Section.ID == "S1.1" && !item.Visible {
				t.Error("S1.1 (descendant of match) should be visible")
			}
		}
	})

	t.Run("overview match", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		sl.FilterByQuery("over")
		if !sl.items[0].Visible {
			t.Error("overview should match 'over'")
		}
	})

	t.Run("empty query clears filter", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		sl.FilterByQuery("nonexistent")
		sl.FilterByQuery("")
		for _, item := range sl.items {
			if !item.Visible {
				t.Error("all items should be visible after empty query")
			}
		}
	})

	t.Run("body match", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		sl.FilterByQuery("Body 2")
		for _, item := range sl.items {
			if item.Section != nil && item.Section.ID == "S2" && !item.Visible {
				t.Error("S2 should match via body text")
			}
			if item.Section != nil && item.Section.ID == "S1" && item.Visible {
				t.Error("S1 should be hidden when only S2 body matches")
			}
			if item.Section != nil && item.Section.ID == "S1.1" && item.Visible {
				t.Error("S1.1 should be hidden when only S2 body matches")
			}
			if item.Section != nil && item.Section.ID == "S1.2" && item.Visible {
				t.Error("S1.2 should be hidden when only S2 body matches")
			}
		}
	})

	t.Run("cursor moves to visible on hidden", func(t *testing.T) {
		sl := newTestSectionList(makeDocWithChildren())
		sl.CursorBottom() // S2
		sl.FilterByQuery("Sub")
		// S2 is hidden, cursor should move to a visible item
		if sl.cursor < len(sl.items) && !sl.items[sl.cursor].Visible {
			t.Error("cursor should be on a visible item after filter")
		}
	})
}

func TestClearFilter(t *testing.T) {
	sl := newTestSectionList(makeDocWithChildren())
	sl.FilterByQuery("nonexistent")
	sl.ClearFilter()
	for _, item := range sl.items {
		if !item.Visible {
			t.Error("all items should be visible after ClearFilter")
		}
	}
}

func TestSelectedAndIsOverviewSelected(t *testing.T) {
	sl := newTestSectionList(makeDocWithChildren())

	// At overview
	if !sl.IsOverviewSelected() {
		t.Error("overview should be selected initially")
	}
	if sl.Selected() != nil {
		t.Error("Selected() should be nil for overview")
	}

	sl.CursorDown()
	if sl.IsOverviewSelected() {
		t.Error("should not be overview after CursorDown")
	}
	if sl.Selected() == nil || sl.Selected().ID != "S1" {
		t.Error("Selected should be S1")
	}
}

func TestRender(t *testing.T) {
	sl := newTestSectionList(makeDocWithChildren())
	styles := defaultStyles()
	output := sl.Render(80, 20, styles)

	if !strings.Contains(output, "Overview") {
		t.Error("render should contain 'Overview'")
	}
	if !strings.Contains(output, "S1") {
		t.Error("render should contain 'S1'")
	}
	// Cursor marker
	if !strings.Contains(output, ">") {
		t.Error("render should contain cursor marker '>'")
	}
}

func TestTruncateShortString(t *testing.T) {
	result := truncate("hi", 10)
	if result != "hi" {
		t.Errorf("truncate short string = %q, want %q", result, "hi")
	}
}

func TestTruncateMaxWidthThree(t *testing.T) {
	result := truncate("hello world", 3)
	if len(result) > 3 {
		t.Errorf("truncate with maxWidth=3: got %q, too long", result)
	}
}

func TestRenderCollapsedSection(t *testing.T) {
	sl := newTestSectionList(makeDocWithChildren())
	styles := defaultStyles()

	// Collapse S1 to get ▶ prefix rendered
	sl.CursorDown() // move to S1
	sl.ToggleExpand()

	output := sl.Render(80, 20, styles)

	// S1 should still appear (collapsed)
	if !strings.Contains(output, "S1") {
		t.Error("collapsed S1 should still be in render output")
	}
	// Children should not appear
	if strings.Contains(output, "S1.1") {
		t.Error("collapsed child S1.1 should not be in render output")
	}
	// ▶ prefix should appear
	if !strings.Contains(output, "▶") {
		t.Error("collapsed section should show ▶ prefix")
	}
}

func TestSelectedOutOfBounds(t *testing.T) {
	sl := newTestSectionList(&markdown.Document{})
	// Empty document, no items - cursor is already out of range
	sl.cursor = 999
	if sl.Selected() != nil {
		t.Error("Selected() should return nil for out of bounds cursor")
	}
	if sl.IsOverviewSelected() {
		t.Error("IsOverviewSelected() should return false for out of bounds cursor")
	}
}

func TestToggleExpandOutOfBounds(t *testing.T) {
	sl := newTestSectionList(&markdown.Document{})
	sl.cursor = 999

	sl.ToggleExpand()
	if sl.cursor != 999 {
		t.Error("ToggleExpand out of bounds should not move cursor")
	}
}

func TestTotalSectionCount(t *testing.T) {
	sl := newTestSectionList(makeDocWithChildren())
	// S1, S1.1, S1.2, S2 = 4 sections (overview excluded)
	if got := sl.TotalSectionCount(); got != 4 {
		t.Errorf("TotalSectionCount = %d, want 4", got)
	}

	sl2 := newTestSectionList(makeDocNoPreamble())
	if got := sl2.TotalSectionCount(); got != 1 {
		t.Errorf("TotalSectionCount (no preamble) = %d, want 1", got)
	}
}

func TestSelectBySectionID(t *testing.T) {
	sl := newTestSectionList(makeDocWithChildren())

	// Move to S2
	sl.SelectBySectionID("S2")
	if sl.Selected() == nil || sl.Selected().ID != "S2" {
		t.Errorf("cursor should be on S2, got %v", sl.Selected())
	}

	// Move to S1.1
	sl.SelectBySectionID("S1.1")
	if sl.Selected() == nil || sl.Selected().ID != "S1.1" {
		t.Errorf("cursor should be on S1.1, got %v", sl.Selected())
	}

	// Non-existent ID should not move cursor
	sl.SelectBySectionID("S99")
	if sl.Selected() == nil || sl.Selected().ID != "S1.1" {
		t.Errorf("cursor should remain on S1.1 for non-existent ID, got %v", sl.Selected())
	}

	// Hidden item should not be selected
	sl.CursorDown() // move away from S1.1
	sl.SelectBySectionID("S1")
	sl.ToggleExpand() // collapse S1, hiding S1.1 and S1.2
	sl.SelectBySectionID("S1.1")
	if sl.Selected() != nil && sl.Selected().ID == "S1.1" {
		t.Error("hidden S1.1 should not be selected")
	}
}

func TestCursorPageScroll(t *testing.T) {
	// makeDocWithChildren: overview(0), S1(1), S1.1(2), S1.2(3), S2(4) — 5 visible items
	tests := []struct {
		name       string
		startID    string // "" means overview
		method     string
		pageHeight int
		wantID     string // "" means overview
	}{
		// half page down
		{name: "half page down from top", startID: "", method: "HalfPageDown", pageHeight: 4, wantID: "S1.1"},
		{name: "half page down near bottom", startID: "S1.2", method: "HalfPageDown", pageHeight: 4, wantID: "S2"},
		{name: "half page down at bottom", startID: "S2", method: "HalfPageDown", pageHeight: 4, wantID: "S2"},
		// half page up
		{name: "half page up from bottom", startID: "S2", method: "HalfPageUp", pageHeight: 4, wantID: "S1.1"},
		{name: "half page up near top", startID: "S1", method: "HalfPageUp", pageHeight: 4, wantID: ""},
		{name: "half page up at top", startID: "", method: "HalfPageUp", pageHeight: 4, wantID: ""},
		// full page down
		{name: "full page down from top", startID: "", method: "PageDown", pageHeight: 4, wantID: "S2"},
		{name: "full page down at bottom", startID: "S2", method: "PageDown", pageHeight: 4, wantID: "S2"},
		// full page up
		{name: "full page up from bottom", startID: "S2", method: "PageUp", pageHeight: 4, wantID: ""},
		{name: "full page up at top", startID: "", method: "PageUp", pageHeight: 4, wantID: ""},
		// height=1
		{name: "half page down height 1", startID: "", method: "HalfPageDown", pageHeight: 1, wantID: "S1"},
		{name: "full page down height 1", startID: "", method: "PageDown", pageHeight: 1, wantID: "S1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl := newTestSectionList(makeDocWithChildren())
			// Move cursor to startID
			if tt.startID != "" {
				sl.SelectBySectionID(tt.startID)
			}

			switch tt.method {
			case "HalfPageDown":
				sl.CursorHalfPageDown(tt.pageHeight)
			case "HalfPageUp":
				sl.CursorHalfPageUp(tt.pageHeight)
			case "PageDown":
				sl.CursorPageDown(tt.pageHeight)
			case "PageUp":
				sl.CursorPageUp(tt.pageHeight)
			}

			gotID := ""
			if sel := sl.Selected(); sel != nil {
				gotID = sel.ID
			}
			if gotID != tt.wantID {
				t.Errorf("cursor at %q, want %q", gotID, tt.wantID)
			}
		})
	}
}

func TestCursorPageScrollSkipsHidden(t *testing.T) {
	sl := newTestSectionList(makeDocWithChildren())
	// Collapse S1 so S1.1 and S1.2 are hidden
	// Visible: overview(0), S1(1), S2(4)
	sl.SelectBySectionID("S1")
	sl.ToggleExpand()
	sl.CursorTop() // back to overview

	sl.CursorHalfPageDown(4) // move 2 visible items: overview -> S1 -> S2
	gotID := ""
	if sel := sl.Selected(); sel != nil {
		gotID = sel.ID
	}
	if gotID != "S2" {
		t.Errorf("half page down with collapsed children: cursor at %q, want S2", gotID)
	}
}

func TestSelectBySectionIDFallbacks(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(sl *SectionList)
		sectionID    string
		wantOverview bool
		wantID       string
	}{
		{
			name:         "overview ID selects the overview entry",
			setup:        func(sl *SectionList) { sl.SelectBySectionID("S2") },
			sectionID:    markdown.OverviewSectionID,
			wantOverview: true,
		},
		{
			name: "hidden child selects its visible parent",
			setup: func(sl *SectionList) {
				sl.SelectBySectionID("S2")
				sl.SelectBySectionID("S1")
				sl.ToggleExpand() // collapse S1
				sl.SelectBySectionID("S2")
			},
			sectionID: "S1.1",
			wantID:    "S1",
		},
		{
			name:      "unknown ID leaves the cursor alone",
			setup:     func(sl *SectionList) { sl.SelectBySectionID("S2") },
			sectionID: "S99",
			wantID:    "S2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := makeDocWithChildren()
			doc.Preamble = "intro"
			sl := newTestSectionList(doc)
			tt.setup(sl)

			sl.SelectBySectionID(tt.sectionID)

			if sl.IsOverviewSelected() != tt.wantOverview {
				t.Errorf("IsOverviewSelected() = %v, want %v", sl.IsOverviewSelected(), tt.wantOverview)
			}
			if tt.wantID != "" && (sl.Selected() == nil || sl.Selected().ID != tt.wantID) {
				t.Errorf("Selected() = %v, want %s", sl.Selected(), tt.wantID)
			}
		})
	}
}
