package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/koh-sh/ccplan/internal/plan"
)

func TestNewDetailPane(t *testing.T) {
	dp := NewDetailPane(80, 24, "dark")
	if dp == nil {
		t.Fatal("NewDetailPane returned nil")
		return
	}
	if dp.viewport.Width != 80 {
		t.Errorf("width = %d, want 80", dp.viewport.Width)
	}
	if dp.viewport.Height != 24 {
		t.Errorf("height = %d, want 24", dp.viewport.Height)
	}
}

func TestDetailPaneShowStep(t *testing.T) {
	dp := NewDetailPane(80, 40, "dark")
	step := &plan.Step{ID: "S1", Title: "Test Step", Body: "Unique test body here"}

	dp.ShowStep(step, nil)
	content := dp.View()
	if !strings.Contains(content, "S1") {
		t.Error("view should contain step ID")
	}
	// glamour wraps text and inserts ANSI codes; check individual words
	if !strings.Contains(content, "Unique") {
		t.Errorf("view should contain step body word, got:\n%s", content)
	}
}

func TestDetailPaneShowStepWithComments(t *testing.T) {
	dp := NewDetailPane(80, 40, "dark")
	step := &plan.Step{ID: "S1", Title: "Test Step", Body: "Body"}
	comments := []*plan.ReviewComment{
		{StepID: "S1", Action: plan.ActionSuggestion, Body: "Review text"},
	}

	dp.ShowStep(step, comments)
	content := dp.View()
	if !strings.Contains(content, "Review Comment") {
		t.Error("view should contain 'Review Comment' box header")
	}
	if !strings.Contains(content, "Review text") {
		t.Error("view should contain comment body")
	}
}

func TestDetailPaneShowStepWithMultipleComments(t *testing.T) {
	dp := NewDetailPane(80, 40, "dark")
	step := &plan.Step{ID: "S1", Title: "Test Step", Body: "Body"}
	comments := []*plan.ReviewComment{
		{StepID: "S1", Action: plan.ActionSuggestion, Body: "First"},
		{StepID: "S1", Action: plan.ActionIssue, Body: "Second"},
	}

	dp.ShowStep(step, comments)
	content := dp.View()
	if !strings.Contains(content, "#1") {
		t.Error("view should contain '#1' for numbered comments")
	}
	if !strings.Contains(content, "#2") {
		t.Error("view should contain '#2' for numbered comments")
	}
}

func TestDetailPaneShowOverview(t *testing.T) {
	dp := NewDetailPane(80, 40, "dark")
	p := &plan.Plan{Title: "My Plan", Preamble: "Unique overview text"}

	dp.ShowOverview(p)
	content := dp.View()
	if !strings.Contains(content, "Unique") {
		t.Errorf("view should contain preamble word, got:\n%s", content)
	}
}

func TestDetailPaneSetSize(t *testing.T) {
	dp := NewDetailPane(80, 24, "dark")

	// Same size should be no-op
	dp.SetSize(80, 24)
	if dp.viewport.Width != 80 || dp.viewport.Height != 24 {
		t.Error("same size should not change")
	}

	// Different size
	dp.SetSize(100, 30)
	if dp.viewport.Width != 100 {
		t.Errorf("width = %d, want 100", dp.viewport.Width)
	}
	if dp.viewport.Height != 30 {
		t.Errorf("height = %d, want 30", dp.viewport.Height)
	}
}

func TestDetailPaneHorizontalScroll(t *testing.T) {
	const width = 40
	dp := NewDetailPane(width, 40, "dark")

	longCode := "```\n" + strings.Repeat("x", 100) + "\n```"
	step := &plan.Step{ID: "S1", Title: "Test", Body: longCode}

	dp.ShowStep(step, nil)

	// Verify scroll starts at 0%
	if pct := dp.Viewport().HorizontalScrollPercent(); pct != 0 {
		t.Errorf("initial HorizontalScrollPercent = %f, want 0", pct)
	}

	// Scroll right and verify position changed
	dp.Viewport().ScrollRight(4)
	if pct := dp.Viewport().HorizontalScrollPercent(); pct == 0 {
		t.Error("after ScrollRight(4) HorizontalScrollPercent should be > 0")
	}

	// ShowStep resets scroll position
	dp.Viewport().ScrollRight(10)
	dp.ShowStep(step, nil)
	if pct := dp.Viewport().HorizontalScrollPercent(); pct != 0 {
		t.Errorf("after ShowStep HorizontalScrollPercent = %f, want 0", pct)
	}
}

func TestWrapProse(t *testing.T) {
	tests := []struct {
		name  string
		md    string
		width int
		want  string
	}{
		{"empty string", "", 40, ""},
		{"zero width returns as-is", "some long text here", 0, "some long text here"},
		{"negative width returns as-is", "text", -1, "text"},
		{"short line no wrap", "hello world", 40, "hello world"},
		{
			"long English prose wrapped with hard breaks",
			"aaa bbb ccc ddd",
			7,
			"aaa bbb  \nccc ddd",
		},
		{
			"long CJK wrapped with hard breaks",
			"あいうえおかきくけこ",
			10,
			"あいうえお  \nかきくけこ",
		},
		{
			"backtick code block preserved",
			"```\n" + strings.Repeat("x", 100) + "\n```",
			20,
			"```\n" + strings.Repeat("x", 100) + "\n```",
		},
		{
			"tilde code block preserved",
			"~~~\n" + strings.Repeat("あ", 50) + "\n~~~",
			10,
			"~~~\n" + strings.Repeat("あ", 50) + "\n~~~",
		},
		{
			"prose before and after code block",
			"aaa bbb ccc ddd\n```\ncode\n```\neee fff ggg hhh",
			7,
			"aaa bbb  \nccc ddd\n```\ncode\n```\neee fff  \nggg hhh",
		},
		{
			"mixed CJK and English",
			"hello あいう world",
			14,
			"hello あいう  \nworld",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapProse(tt.md, tt.width)
			if got != tt.want {
				t.Errorf("wrapProse(%q, %d):\n got: %q\nwant: %q", tt.md, tt.width, got, tt.want)
			}
		})
	}
}

func TestSoftWrapLine(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
		want  []string
	}{
		{"empty line", "   ", 40, []string{"   "}},
		{"single word fits", "hello", 40, []string{"hello"}},
		{"wraps at boundary", "aaa bbb ccc", 7, []string{"aaa bbb  ", "ccc"}},
		{
			"preserves leading indent",
			"  - item one two three",
			14,
			[]string{"  - item one  ", "  two three"},
		},
		{
			"indent width >= width uses effectiveWidth 1",
			"          word",
			5,
			[]string{"          word"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := softWrapLine(tt.line, tt.width)
			if len(got) != len(tt.want) {
				t.Fatalf("softWrapLine(%q, %d) returned %d lines, want %d:\n got: %q\nwant: %q",
					tt.line, tt.width, len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestHardWrapCJK(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
		want  []string
	}{
		{"short line no wrap", "あいう", 10, []string{"あいう"}},
		{"basic wrap", "あいうえおかきくけこ", 10, []string{"あいうえお  ", "かきくけこ"}},
		{
			"preserves leading indent",
			"  あいうえおかきくけこ",
			14,
			[]string{"  あいうえおか  ", "  きくけこ"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hardWrapCJK(tt.line, tt.width)
			if len(got) != len(tt.want) {
				t.Fatalf("hardWrapCJK(%q, %d) returned %d lines, want %d:\n got: %q\nwant: %q",
					tt.line, tt.width, len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDetailPaneShowAll(t *testing.T) {
	dp := NewDetailPane(80, 80, "dark")
	p := &plan.Plan{
		Title:    "My Plan",
		Preamble: "Unique preamble content",
		Steps: []*plan.Step{
			{ID: "S1", Title: "First Step", Level: 2, Body: "Alpha body text"},
			{ID: "S2", Title: "Second Step", Level: 2, Body: "Bravo body text"},
		},
	}
	getComments := func(string) []*plan.ReviewComment { return nil }

	dp.ShowAll(p, getComments)
	content := dp.View()
	if !strings.Contains(content, "Unique") {
		t.Errorf("view should contain preamble word, got:\n%s", content)
	}
	if !strings.Contains(content, "S1") {
		t.Error("view should contain step S1")
	}
	if !strings.Contains(content, "Alpha") {
		t.Error("view should contain first step body")
	}
	if !strings.Contains(content, "S2") {
		t.Error("view should contain step S2")
	}
	if !strings.Contains(content, "Bravo") {
		t.Error("view should contain second step body")
	}
}

func TestDetailPaneShowAllWithComments(t *testing.T) {
	dp := NewDetailPane(80, 80, "dark")
	p := &plan.Plan{
		Title: "Plan",
		Steps: []*plan.Step{
			{ID: "S1", Title: "Step One", Level: 2, Body: "Body"},
		},
	}
	getComments := func(stepID string) []*plan.ReviewComment {
		if stepID == "S1" {
			return []*plan.ReviewComment{
				{StepID: "S1", Action: plan.ActionSuggestion, Body: "Review feedback"},
			}
		}
		return nil
	}

	dp.ShowAll(p, getComments)
	content := dp.View()
	if !strings.Contains(content, "Review") {
		t.Error("view should contain review comment")
	}
}

func TestDetailPaneShowAllNoPreamble(t *testing.T) {
	dp := NewDetailPane(80, 80, "dark")
	p := &plan.Plan{
		Title: "NoPreamblePlan",
		Steps: []*plan.Step{
			{ID: "S1", Title: "Only Step", Level: 2, Body: "Unique nopreamble body"},
		},
	}
	getComments := func(string) []*plan.ReviewComment { return nil }

	dp.ShowAll(p, getComments)
	content := dp.View()
	if !strings.Contains(content, "Unique") {
		t.Errorf("view should contain step body, got:\n%s", content)
	}
	if !strings.Contains(content, "S1") {
		t.Error("view should contain step S1")
	}
}

func TestBuildSectionOffsets(t *testing.T) {
	dp := NewDetailPane(80, 80, "dark")
	p := &plan.Plan{
		Title:    "Test Plan",
		Preamble: "Preamble text",
		Steps: []*plan.Step{
			{ID: "S1", Title: "First", Level: 2, Body: "Body 1"},
			{ID: "S2", Title: "Second", Level: 2, Body: "Body 2"},
		},
	}
	getComments := func(string) []*plan.ReviewComment { return nil }

	dp.ShowAll(p, getComments)

	if len(dp.sectionOffsets) != 2 {
		t.Fatalf("sectionOffsets count = %d, want 2", len(dp.sectionOffsets))
	}
	if dp.sectionOffsets[0].stepID != "S1" {
		t.Errorf("first offset stepID = %s, want S1", dp.sectionOffsets[0].stepID)
	}
	if dp.sectionOffsets[1].stepID != "S2" {
		t.Errorf("second offset stepID = %s, want S2", dp.sectionOffsets[1].stepID)
	}
	if dp.sectionOffsets[0].line >= dp.sectionOffsets[1].line {
		t.Errorf("S1 line (%d) should be before S2 line (%d)", dp.sectionOffsets[0].line, dp.sectionOffsets[1].line)
	}
}

func TestBuildSectionOffsetsWithChildren(t *testing.T) {
	dp := NewDetailPane(80, 80, "dark")
	s1 := &plan.Step{ID: "S1", Title: "Parent", Level: 2, Body: "Body"}
	s1_1 := &plan.Step{ID: "S1.1", Title: "Child", Level: 3, Body: "Child body", Parent: s1}
	s1.Children = []*plan.Step{s1_1}
	p := &plan.Plan{
		Title: "Plan",
		Steps: []*plan.Step{s1},
	}
	getComments := func(string) []*plan.ReviewComment { return nil }

	dp.ShowAll(p, getComments)

	if len(dp.sectionOffsets) != 2 {
		t.Fatalf("sectionOffsets count = %d, want 2", len(dp.sectionOffsets))
	}
	if dp.sectionOffsets[0].stepID != "S1" {
		t.Errorf("first offset stepID = %s, want S1", dp.sectionOffsets[0].stepID)
	}
	if dp.sectionOffsets[1].stepID != "S1.1" {
		t.Errorf("second offset stepID = %s, want S1.1", dp.sectionOffsets[1].stepID)
	}
}

func TestStepIDAtOffset(t *testing.T) {
	dp := NewDetailPane(80, 80, "dark")
	dp.sectionOffsets = []sectionOffset{
		{line: 5, stepID: "S1"},
		{line: 20, stepID: "S2"},
		{line: 40, stepID: "S3"},
	}

	tests := []struct {
		name    string
		yOffset int
		want    string
	}{
		{"before any section", 0, ""},
		{"at first section", 5, "S1"},
		{"between S1 and S2", 15, "S1"},
		{"at S2", 20, "S2"},
		{"between S2 and S3", 30, "S2"},
		{"at S3", 40, "S3"},
		{"past S3", 100, "S3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dp.StepIDAtOffset(tt.yOffset)
			if got != tt.want {
				t.Errorf("StepIDAtOffset(%d) = %q, want %q", tt.yOffset, got, tt.want)
			}
		})
	}
}

func TestShowStepClearsSectionOffsets(t *testing.T) {
	dp := NewDetailPane(80, 40, "dark")
	dp.sectionOffsets = []sectionOffset{{line: 0, stepID: "S1"}}

	step := &plan.Step{ID: "S1", Title: "Test", Body: "Body"}
	dp.ShowStep(step, nil)

	if dp.sectionOffsets != nil {
		t.Error("ShowStep should clear sectionOffsets")
	}
}

func TestShowOverviewClearsSectionOffsets(t *testing.T) {
	dp := NewDetailPane(80, 40, "dark")
	dp.sectionOffsets = []sectionOffset{{line: 0, stepID: "S1"}}

	p := &plan.Plan{Title: "Plan", Preamble: "Text"}
	dp.ShowOverview(p)

	if dp.sectionOffsets != nil {
		t.Error("ShowOverview should clear sectionOffsets")
	}
}

func TestCustomStyle(t *testing.T) {
	dark := customStyle("dark")
	light := customStyle("light")

	// Both should not have BackgroundColor on Error token
	if dark.CodeBlock.Chroma != nil && dark.CodeBlock.Chroma.Error.BackgroundColor != nil {
		t.Error("dark style should not have error background color")
	}
	if light.CodeBlock.Chroma != nil && light.CodeBlock.Chroma.Error.BackgroundColor != nil {
		t.Error("light style should not have error background color")
	}
}

func TestRenderCommentBox(t *testing.T) {
	dp := NewDetailPane(80, 40, "dark")
	comment := &plan.ReviewComment{
		StepID: "S1",
		Action: plan.ActionSuggestion,
		Body:   "Test comment body",
	}

	box := dp.renderCommentBox(comment, 0, 1)
	if !strings.Contains(box, "Review Comment") {
		t.Error("box should contain 'Review Comment' header")
	}
	if !strings.Contains(box, "suggestion") {
		t.Error("box should contain action label")
	}
	if !strings.Contains(box, "Test comment body") {
		t.Error("box should contain comment body")
	}
	// Box should contain border characters
	if !strings.Contains(box, "╭") || !strings.Contains(box, "╰") {
		t.Error("box should contain rounded border characters")
	}
}

func TestRenderCommentBoxNumbered(t *testing.T) {
	dp := NewDetailPane(80, 40, "dark")
	comment := &plan.ReviewComment{
		StepID: "S1",
		Action: plan.ActionIssue,
		Body:   "Numbered comment",
	}

	box := dp.renderCommentBox(comment, 1, 3)
	if !strings.Contains(box, "#2") {
		t.Error("box should contain '#2' for numbered comment")
	}
}

func TestRenderCommentBoxEmptyBody(t *testing.T) {
	dp := NewDetailPane(80, 40, "dark")
	comment := &plan.ReviewComment{
		StepID: "S1",
		Action: plan.ActionSuggestion,
	}

	box := dp.renderCommentBox(comment, 0, 1)
	if !strings.Contains(box, "Review Comment") {
		t.Error("box should contain header even with empty body")
	}
}

func TestRenderCommentBoxLightTheme(t *testing.T) {
	dp := NewDetailPane(80, 40, "light")
	if dp.commentBorderColor() != "33" {
		t.Errorf("light theme border color = %s, want 33", dp.commentBorderColor())
	}

	dpDark := NewDetailPane(80, 40, "dark")
	if dpDark.commentBorderColor() != "62" {
		t.Errorf("dark theme border color = %s, want 62", dpDark.commentBorderColor())
	}
}

func TestScrollToStepID(t *testing.T) {
	dp := NewDetailPane(80, 10, "dark")
	// Set enough content so viewport allows scrolling
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	dp.viewport.SetContent(strings.Join(lines, "\n"))
	dp.sectionOffsets = []sectionOffset{
		{line: 5, stepID: "S1"},
		{line: 20, stepID: "S2"},
		{line: 40, stepID: "S3"},
	}

	t.Run("known stepID", func(t *testing.T) {
		dp.ScrollToStepID("S2")
		if dp.viewport.YOffset != 20 {
			t.Errorf("YOffset = %d, want 20", dp.viewport.YOffset)
		}
	})

	t.Run("unknown stepID", func(t *testing.T) {
		dp.viewport.SetYOffset(10)
		dp.ScrollToStepID("S99")
		if dp.viewport.YOffset != 10 {
			t.Errorf("YOffset should not change for unknown stepID, got %d", dp.viewport.YOffset)
		}
	})

	t.Run("empty stepID scrolls to top", func(t *testing.T) {
		dp.viewport.SetYOffset(10)
		dp.ScrollToStepID("")
		if dp.viewport.YOffset != 0 {
			t.Errorf("YOffset = %d, want 0 for empty stepID", dp.viewport.YOffset)
		}
	})
}

func TestRenderCommentBoxWithDecoration(t *testing.T) {
	dp := NewDetailPane(80, 40, "dark")
	comment := &plan.ReviewComment{
		StepID:     "S1",
		Action:     plan.ActionSuggestion,
		Decoration: plan.DecorationNonBlocking,
		Body:       "Decorated comment",
	}

	box := dp.renderCommentBox(comment, 0, 1)
	if !strings.Contains(box, "suggestion (non-blocking)") {
		t.Error("box should contain formatted label with decoration")
	}
	if !strings.Contains(box, "Decorated comment") {
		t.Error("box should contain comment body")
	}
}

func TestInsertCommentBoxes(t *testing.T) {
	dp := NewDetailPane(80, 80, "dark")
	p := &plan.Plan{
		Title: "Plan",
		Steps: []*plan.Step{
			{ID: "S1", Title: "Step One", Level: 2, Body: "Body one"},
			{ID: "S2", Title: "Step Two", Level: 2, Body: "Body two"},
		},
	}
	getComments := func(stepID string) []*plan.ReviewComment {
		if stepID == "S1" {
			return []*plan.ReviewComment{
				{StepID: "S1", Action: plan.ActionSuggestion, Body: "Comment on S1"},
			}
		}
		return nil
	}

	dp.ShowAll(p, getComments)
	content := dp.viewport.View()

	// Should contain comment box content
	if !strings.Contains(content, "Comment on S1") {
		t.Error("ShowAll should contain comment box content")
	}
	if !strings.Contains(content, "Review Comment") {
		t.Error("ShowAll should contain comment box header")
	}
}
