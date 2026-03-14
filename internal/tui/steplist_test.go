package tui

import (
	"strings"
	"testing"

	"github.com/koh-sh/ccplan/internal/plan"
)

func makePlanWithChildren() *plan.Plan {
	p := &plan.Plan{
		Title:    "Test Plan",
		Preamble: "Overview text",
	}
	s1 := &plan.Step{ID: "S1", Title: "Step 1", Level: 2, Body: "Body 1"}
	s1_1 := &plan.Step{ID: "S1.1", Title: "Sub 1.1", Level: 3, Body: "Body 1.1", Parent: s1}
	s1_2 := &plan.Step{ID: "S1.2", Title: "Sub 1.2", Level: 3, Body: "Body 1.2", Parent: s1}
	s1.Children = []*plan.Step{s1_1, s1_2}
	s2 := &plan.Step{ID: "S2", Title: "Step 2", Level: 2, Body: "Body 2"}
	p.Steps = []*plan.Step{s1, s2}
	return p
}

func makePlanNoPreamble() *plan.Plan {
	p := &plan.Plan{Title: "No Preamble"}
	s1 := &plan.Step{ID: "S1", Title: "Step 1", Level: 2}
	p.Steps = []*plan.Step{s1}
	return p
}

func TestNewStepList(t *testing.T) {
	t.Run("with preamble", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		if !sl.items[0].IsOverview {
			t.Error("first item should be overview when preamble exists")
		}
		// overview + S1 + S1.1 + S1.2 + S2 = 5
		if len(sl.items) != 5 {
			t.Errorf("items count = %d, want 5", len(sl.items))
		}
	})

	t.Run("without preamble", func(t *testing.T) {
		sl := NewStepList(makePlanNoPreamble(), nil)
		if sl.items[0].IsOverview {
			t.Error("first item should not be overview when no preamble")
		}
		if len(sl.items) != 1 {
			t.Errorf("items count = %d, want 1", len(sl.items))
		}
	})
}

func TestCursorUpDown(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

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
	if sl.items[sl.cursor].Step.ID != "S1" {
		t.Errorf("after CursorDown: step = %s, want S1", sl.items[sl.cursor].Step.ID)
	}

	// Move down to S1.1
	sl.CursorDown()
	if sl.items[sl.cursor].Step.ID != "S1.1" {
		t.Errorf("after 2nd CursorDown: step = %s, want S1.1", sl.items[sl.cursor].Step.ID)
	}

	// Move up back to S1
	sl.CursorUp()
	if sl.items[sl.cursor].Step.ID != "S1" {
		t.Errorf("after CursorUp: step = %s, want S1", sl.items[sl.cursor].Step.ID)
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
	sl := NewStepList(makePlanWithChildren(), nil)

	// Collapse S1 to hide children
	sl.CursorDown() // move to S1
	sl.ToggleExpand()

	// S1.1 and S1.2 are now hidden
	// CursorDown from S1 should skip to S2
	sl.CursorDown()
	if sl.items[sl.cursor].Step.ID != "S2" {
		t.Errorf("CursorDown skipping hidden: step = %s, want S2", sl.items[sl.cursor].Step.ID)
	}

	// CursorUp from S2 should skip to S1
	sl.CursorUp()
	if sl.items[sl.cursor].Step.ID != "S1" {
		t.Errorf("CursorUp skipping hidden: step = %s, want S1", sl.items[sl.cursor].Step.ID)
	}
}

func TestCursorTopBottom(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

	sl.CursorBottom()
	if sl.items[sl.cursor].Step.ID != "S2" {
		t.Errorf("CursorBottom: step = %s, want S2", sl.items[sl.cursor].Step.ID)
	}

	sl.CursorTop()
	if !sl.items[sl.cursor].IsOverview {
		t.Error("CursorTop should go to overview")
	}
}

func TestToggleExpand(t *testing.T) {
	t.Run("toggle with children", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
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
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.CursorBottom() // S2 (no children)
		expanded := sl.items[sl.cursor].Expanded
		sl.ToggleExpand() // should be no-op for leaf node
		if sl.items[sl.cursor].Expanded != expanded {
			t.Error("ToggleExpand on leaf should not change Expanded state")
		}
	})

	t.Run("toggle overview", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
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

func TestExpandCollapse(t *testing.T) {
	t.Run("expand already expanded", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.CursorDown() // S1, already expanded
		sl.Expand()     // no-op
		if !sl.items[sl.cursor].Expanded {
			t.Error("should still be expanded")
		}
	})

	t.Run("expand collapsed", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.CursorDown() // S1
		sl.ToggleExpand()
		if sl.items[sl.cursor].Expanded {
			t.Fatal("should be collapsed")
		}
		sl.Expand()
		if !sl.items[sl.cursor].Expanded {
			t.Error("should be expanded after Expand()")
		}
	})

	t.Run("collapse expanded", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.CursorDown() // S1
		sl.Collapse()
		if sl.items[sl.cursor].Expanded {
			t.Error("should be collapsed after Collapse()")
		}
	})

	t.Run("collapse moves to parent", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		// Move to S1.1
		sl.CursorDown() // S1
		sl.CursorDown() // S1.1
		if sl.items[sl.cursor].Step.ID != "S1.1" {
			t.Fatalf("expected S1.1, got %s", sl.items[sl.cursor].Step.ID)
		}
		sl.Collapse() // should move to parent S1
		if sl.items[sl.cursor].Step.ID != "S1" {
			t.Errorf("after Collapse on leaf: step = %s, want S1", sl.items[sl.cursor].Step.ID)
		}
	})

	t.Run("collapse overview is no-op", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		cursorBefore := sl.cursor
		sl.Collapse() // cursor at overview, should be no-op
		if sl.cursor != cursorBefore {
			t.Errorf("Collapse on overview moved cursor from %d to %d", cursorBefore, sl.cursor)
		}
	})

	t.Run("expand no children is no-op", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.CursorBottom() // S2 (no children)
		expanded := sl.items[sl.cursor].Expanded
		sl.Expand() // no-op
		if sl.items[sl.cursor].Expanded != expanded {
			t.Error("Expand on leaf should not change Expanded state")
		}
	})
}

func TestAddComment(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

	// Normal add
	c := &plan.ReviewComment{StepID: "S1", Action: plan.ActionSuggestion, Body: "test"}
	sl.AddComment("S1", c)
	if len(sl.comments["S1"]) != 1 {
		t.Errorf("comments count = %d, want 1", len(sl.comments["S1"]))
	}

	// Add nil
	sl.AddComment("S1", nil)
	if len(sl.comments["S1"]) != 1 {
		t.Error("nil comment should not be added")
	}

	// Add empty body
	sl.AddComment("S1", &plan.ReviewComment{Body: ""})
	if len(sl.comments["S1"]) != 1 {
		t.Error("empty body comment should not be added")
	}

	// Add second
	c2 := &plan.ReviewComment{StepID: "S1", Action: plan.ActionIssue, Body: "issue"}
	sl.AddComment("S1", c2)
	if len(sl.comments["S1"]) != 2 {
		t.Errorf("comments count = %d, want 2", len(sl.comments["S1"]))
	}
}

func TestUpdateComment(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)
	c := &plan.ReviewComment{StepID: "S1", Action: plan.ActionSuggestion, Body: "original"}
	sl.AddComment("S1", c)

	// Normal update
	updated := &plan.ReviewComment{StepID: "S1", Action: plan.ActionIssue, Body: "updated"}
	sl.UpdateComment("S1", 0, updated)
	if sl.comments["S1"][0].Body != "updated" {
		t.Errorf("body = %s, want updated", sl.comments["S1"][0].Body)
	}

	// Update with empty body -> deletes
	sl.UpdateComment("S1", 0, &plan.ReviewComment{Body: ""})
	if len(sl.comments["S1"]) != 0 {
		t.Error("update with empty body should delete")
	}

	// Update out of range should be no-op
	sl.UpdateComment("S1", 5, updated)
	if len(sl.comments["S1"]) != 0 {
		t.Error("out-of-range update should not add comments")
	}
}

func TestDeleteComment(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)
	sl.AddComment("S1", &plan.ReviewComment{Body: "a"})
	sl.AddComment("S1", &plan.ReviewComment{Body: "b"})

	// Delete first
	sl.DeleteComment("S1", 0)
	if len(sl.comments["S1"]) != 1 {
		t.Errorf("comments count = %d, want 1", len(sl.comments["S1"]))
	}
	if sl.comments["S1"][0].Body != "b" {
		t.Errorf("remaining comment = %s, want b", sl.comments["S1"][0].Body)
	}

	// Delete last -> map entry removed
	sl.DeleteComment("S1", 0)
	if _, exists := sl.comments["S1"]; exists {
		t.Error("map entry should be removed when no comments remain")
	}

	// Delete out of range should be no-op
	sl.DeleteComment("S1", 0)
	if _, exists := sl.comments["S1"]; exists {
		t.Error("out-of-range delete should not create map entry")
	}
}

func TestToggleViewed(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

	if sl.IsViewed("S1") {
		t.Error("S1 should not be viewed initially")
	}

	sl.ToggleViewed("S1")
	if !sl.IsViewed("S1") {
		t.Error("S1 should be viewed after toggle")
	}

	sl.ToggleViewed("S1")
	if sl.IsViewed("S1") {
		t.Error("S1 should not be viewed after second toggle")
	}
}

func TestHasComments(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

	if sl.HasComments() {
		t.Error("should have no comments initially")
	}

	sl.AddComment("S1", &plan.ReviewComment{Body: "test"})
	if !sl.HasComments() {
		t.Error("should have comments after adding")
	}

	sl.DeleteComment("S1", 0)
	if sl.HasComments() {
		t.Error("should have no comments after deleting all")
	}
}

func TestBuildReviewResult(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)
	sl.AddComment("S2", &plan.ReviewComment{StepID: "S2", Body: "s2 comment"})
	sl.AddComment("S1", &plan.ReviewComment{StepID: "S1", Body: "s1 comment"})
	sl.AddComment("S1", &plan.ReviewComment{StepID: "S1", Body: "s1 second"})

	result := sl.BuildReviewResult()
	if len(result.Comments) != 3 {
		t.Fatalf("comments count = %d, want 3", len(result.Comments))
	}
	// Order should follow step order (S1, S1, S2)
	if result.Comments[0].StepID != "S1" {
		t.Errorf("first comment stepID = %s, want S1", result.Comments[0].StepID)
	}
	if result.Comments[1].StepID != "S1" {
		t.Errorf("second comment stepID = %s, want S1", result.Comments[1].StepID)
	}
	if result.Comments[2].StepID != "S2" {
		t.Errorf("third comment stepID = %s, want S2", result.Comments[2].StepID)
	}
}

func TestFilterByQuery(t *testing.T) {
	t.Run("partial match", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.FilterByQuery("Sub")
		// S1.1 and S1.2 match, S1 is ancestor
		for _, item := range sl.items {
			if item.Step != nil && item.Step.ID == "S2" && item.Visible {
				t.Error("S2 should be hidden")
			}
			if item.Step != nil && item.Step.ID == "S1" && !item.Visible {
				t.Error("S1 (ancestor of match) should be visible")
			}
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.FilterByQuery("step 2")
		for _, item := range sl.items {
			if item.Step != nil && item.Step.ID == "S2" && !item.Visible {
				t.Error("S2 should match case-insensitive")
			}
		}
	})

	t.Run("shows descendants", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.FilterByQuery("Step 1")
		// S1 matches, children should be visible
		for _, item := range sl.items {
			if item.Step != nil && item.Step.ID == "S1.1" && !item.Visible {
				t.Error("S1.1 (descendant of match) should be visible")
			}
		}
	})

	t.Run("overview match", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.FilterByQuery("over")
		if !sl.items[0].Visible {
			t.Error("overview should match 'over'")
		}
	})

	t.Run("empty query clears filter", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.FilterByQuery("nonexistent")
		sl.FilterByQuery("")
		for _, item := range sl.items {
			if !item.Visible {
				t.Error("all items should be visible after empty query")
			}
		}
	})

	t.Run("body match", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.FilterByQuery("Body 2")
		for _, item := range sl.items {
			if item.Step != nil && item.Step.ID == "S2" && !item.Visible {
				t.Error("S2 should match via body text")
			}
			if item.Step != nil && item.Step.ID == "S1" && item.Visible {
				t.Error("S1 should be hidden when only S2 body matches")
			}
			if item.Step != nil && item.Step.ID == "S1.1" && item.Visible {
				t.Error("S1.1 should be hidden when only S2 body matches")
			}
			if item.Step != nil && item.Step.ID == "S1.2" && item.Visible {
				t.Error("S1.2 should be hidden when only S2 body matches")
			}
		}
	})

	t.Run("cursor moves to visible on hidden", func(t *testing.T) {
		sl := NewStepList(makePlanWithChildren(), nil)
		sl.CursorBottom() // S2
		sl.FilterByQuery("Sub")
		// S2 is hidden, cursor should move to a visible item
		if sl.cursor < len(sl.items) && !sl.items[sl.cursor].Visible {
			t.Error("cursor should be on a visible item after filter")
		}
	})
}

func TestClearFilter(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)
	sl.FilterByQuery("nonexistent")
	sl.ClearFilter()
	for _, item := range sl.items {
		if !item.Visible {
			t.Error("all items should be visible after ClearFilter")
		}
	}
}

func TestSelectedAndIsOverviewSelected(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

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
	sl := NewStepList(makePlanWithChildren(), nil)
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

func TestRenderBadge(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)
	styles := defaultStyles()

	// No badge
	badge := sl.renderBadge("S1", styles)
	if badge != "" {
		t.Errorf("empty badge expected, got %q", badge)
	}

	// Single comment
	sl.AddComment("S1", &plan.ReviewComment{Body: "test"})
	badge = sl.renderBadge("S1", styles)
	if !strings.Contains(badge, "[*]") {
		t.Error("badge should contain [*] for single comment")
	}

	// Multiple comments
	sl.AddComment("S1", &plan.ReviewComment{Body: "test2"})
	badge = sl.renderBadge("S1", styles)
	if !strings.Contains(badge, "[*2]") {
		t.Error("badge should contain [*2] for 2 comments")
	}

	// Viewed
	sl.ToggleViewed("S1")
	badge = sl.renderBadge("S1", styles)
	if !strings.Contains(badge, "[✓]") {
		t.Error("badge should contain [✓] for viewed")
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

func TestRenderCollapsedStep(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)
	styles := defaultStyles()

	// Collapse S1 to get ▶ prefix rendered
	sl.CursorDown() // move to S1
	sl.Collapse()

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
		t.Error("collapsed step should show ▶ prefix")
	}
}

func TestSelectedOutOfBounds(t *testing.T) {
	sl := NewStepList(&plan.Plan{}, nil)
	// Empty plan, no items - cursor is already out of range
	sl.cursor = 999
	if sl.Selected() != nil {
		t.Error("Selected() should return nil for out of bounds cursor")
	}
	if sl.IsOverviewSelected() {
		t.Error("IsOverviewSelected() should return false for out of bounds cursor")
	}
}

func TestToggleExpandOutOfBounds(t *testing.T) {
	sl := NewStepList(&plan.Plan{}, nil)
	sl.cursor = 999

	sl.ToggleExpand()
	if sl.cursor != 999 {
		t.Error("ToggleExpand out of bounds should not move cursor")
	}

	sl.Expand()
	if sl.cursor != 999 {
		t.Error("Expand out of bounds should not move cursor")
	}

	sl.Collapse()
	if sl.cursor != 999 {
		t.Error("Collapse out of bounds should not move cursor")
	}
}

func TestGetComments(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

	// No comments
	comments := sl.GetComments("S1")
	if len(comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(comments))
	}

	// With comments
	sl.AddComment("S1", &plan.ReviewComment{Body: "test"})
	comments = sl.GetComments("S1")
	if len(comments) != 1 {
		t.Errorf("expected 1 comment, got %d", len(comments))
	}
}

func TestTotalStepCount(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)
	// S1, S1.1, S1.2, S2 = 4 steps (overview excluded)
	if got := sl.TotalStepCount(); got != 4 {
		t.Errorf("TotalStepCount = %d, want 4", got)
	}

	sl2 := NewStepList(makePlanNoPreamble(), nil)
	if got := sl2.TotalStepCount(); got != 1 {
		t.Errorf("TotalStepCount (no preamble) = %d, want 1", got)
	}
}

func TestViewedCount(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

	if got := sl.ViewedCount(); got != 0 {
		t.Errorf("ViewedCount initial = %d, want 0", got)
	}

	sl.ToggleViewed("S1")
	sl.ToggleViewed("S2")
	if got := sl.ViewedCount(); got != 2 {
		t.Errorf("ViewedCount after marking 2 = %d, want 2", got)
	}

	sl.ToggleViewed("S1") // unmark
	if got := sl.ViewedCount(); got != 1 {
		t.Errorf("ViewedCount after unmarking 1 = %d, want 1", got)
	}
}

func TestTotalCommentCount(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

	if got := sl.TotalCommentCount(); got != 0 {
		t.Errorf("TotalCommentCount initial = %d, want 0", got)
	}

	sl.AddComment("S1", &plan.ReviewComment{Body: "a"})
	sl.AddComment("S1", &plan.ReviewComment{Body: "b"})
	sl.AddComment("S2", &plan.ReviewComment{Body: "c"})
	if got := sl.TotalCommentCount(); got != 3 {
		t.Errorf("TotalCommentCount = %d, want 3", got)
	}
}

func TestViewedStateRestoration(t *testing.T) {
	p := makePlanWithChildren()
	state := plan.NewViewedState()
	// Mark S1 as viewed with its current content
	for _, s := range p.AllSteps() {
		if s.ID == "S1" {
			state.MarkViewed(s)
		}
	}

	sl := NewStepList(p, state)

	if !sl.IsViewed("S1") {
		t.Error("S1 should be restored as viewed")
	}
	if sl.IsViewed("S2") {
		t.Error("S2 should not be viewed")
	}
}

func TestViewedStateStaleHash(t *testing.T) {
	p := makePlanWithChildren()
	state := plan.NewViewedState()

	// Mark S1 as viewed
	s1 := p.FindStep("S1")
	if s1 == nil {
		t.Fatal("S1 not found in plan")
		return
	}
	state.MarkViewed(s1)

	// Change S1's body before creating StepList
	s1.Body = "changed body content"

	sl := NewStepList(p, state)

	if sl.IsViewed("S1") {
		t.Error("S1 should not be viewed after content change (stale hash)")
	}
}

func TestToggleViewedSyncsState(t *testing.T) {
	p := makePlanWithChildren()
	state := plan.NewViewedState()
	sl := NewStepList(p, state)

	s1 := p.FindStep("S1")
	if s1 == nil {
		t.Fatal("S1 not found in plan")
	}

	// Toggle on
	sl.ToggleViewed("S1")
	if !state.IsStepViewed(s1) {
		t.Error("ViewedState should be updated after ToggleViewed on")
	}

	// Toggle off
	sl.ToggleViewed("S1")
	if state.IsStepViewed(s1) {
		t.Error("ViewedState should be updated after ToggleViewed off")
	}
}

func TestViewedStateGetter(t *testing.T) {
	state := plan.NewViewedState()
	sl := NewStepList(makePlanWithChildren(), state)

	if sl.ViewedState() != state {
		t.Error("ViewedState() should return the same state pointer")
	}
}

func TestSelectByStepID(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

	// Move to S2
	sl.SelectByStepID("S2")
	if sl.Selected() == nil || sl.Selected().ID != "S2" {
		t.Errorf("cursor should be on S2, got %v", sl.Selected())
	}

	// Move to S1.1
	sl.SelectByStepID("S1.1")
	if sl.Selected() == nil || sl.Selected().ID != "S1.1" {
		t.Errorf("cursor should be on S1.1, got %v", sl.Selected())
	}

	// Non-existent ID should not move cursor
	sl.SelectByStepID("S99")
	if sl.Selected() == nil || sl.Selected().ID != "S1.1" {
		t.Errorf("cursor should remain on S1.1 for non-existent ID, got %v", sl.Selected())
	}

	// Hidden item should not be selected
	sl.CursorDown() // move away from S1.1
	sl.SelectByStepID("S1")
	sl.ToggleExpand() // collapse S1, hiding S1.1 and S1.2
	sl.SelectByStepID("S1.1")
	if sl.Selected() != nil && sl.Selected().ID == "S1.1" {
		t.Error("hidden S1.1 should not be selected")
	}
}

func TestViewedStateNil(t *testing.T) {
	sl := NewStepList(makePlanWithChildren(), nil)

	// ToggleViewed should not panic with nil state
	sl.ToggleViewed("S1")
	if !sl.IsViewed("S1") {
		t.Error("S1 should be viewed after toggle even with nil state")
	}

	if sl.ViewedState() != nil {
		t.Error("ViewedState() should return nil when no state provided")
	}
}
