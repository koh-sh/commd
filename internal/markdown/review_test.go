package markdown

import (
	"strings"
	"testing"
)

func TestFormatReview(t *testing.T) {
	tests := []struct {
		name         string
		doc          *Document
		result       *ReviewResult
		filePath     string
		wantEmpty    bool
		wantContains []string
		wantCounts   map[string]int
	}{
		{
			name: "basic two sections",
			doc: &Document{
				Title: "Test Plan",
				Sections: []*Section{
					{ID: "S1", Title: "First Step", Level: 2},
					{ID: "S2", Title: "Second Step", Level: 2},
				},
			},
			result: &ReviewResult{
				Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionSuggestion, Body: "Change the algorithm."},
					{SectionID: "S2", Action: ActionSuggestion, Body: "Not needed."},
				},
			},
			filePath: "/path/to/plan.md",
			wantContains: []string{
				"## S1: First Step\n",
				"[suggestion] Change the algorithm.",
				"## S2: Second Step\n",
				"/path/to/plan.md",
			},
		},
		{
			name:      "empty comments returns empty string",
			doc:       &Document{Title: "Test"},
			result:    &ReviewResult{},
			wantEmpty: true,
		},
		{
			name: "single section with body",
			doc: &Document{
				Sections: []*Section{
					{ID: "S1", Title: "Step One", Level: 2},
				},
			},
			result: &ReviewResult{
				Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionSuggestion, Body: "Looks good but needs refactoring."},
				},
			},
			filePath: "test.md",
			wantContains: []string{
				"[suggestion] Looks good but needs refactoring.",
			},
		},
		{
			name: "grouped comments under same section",
			doc: &Document{
				Sections: []*Section{
					{ID: "S1", Title: "JWT verification", Level: 2},
					{ID: "S3", Title: "Add tests", Level: 2},
				},
			},
			result: &ReviewResult{
				Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionSuggestion, Body: "Switch to HS256."},
					{SectionID: "S1", Action: ActionIssue, Body: "Not needed."},
					{SectionID: "S3", Action: ActionQuestion, Body: "Coverage target?"},
				},
			},
			filePath: "/path/to/plan.md",
			wantContains: []string{
				"[suggestion] Switch to HS256.",
				"[issue] Not needed.",
				"## S3: Add tests\n",
				"[question] Coverage target?",
			},
			wantCounts: map[string]int{
				"## S1: JWT verification": 1,
			},
		},
		{
			name: "comment with decoration",
			doc: &Document{
				Sections: []*Section{
					{ID: "S1", Title: "Step One", Level: 2},
				},
			},
			result: &ReviewResult{
				Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionSuggestion, Decoration: DecorationNonBlocking, Body: "Use a cache."},
				},
			},
			filePath: "plan.md",
			wantContains: []string{
				"[suggestion (non-blocking)] Use a cache.",
			},
		},
		{
			name: "comment without decoration (zero value)",
			doc: &Document{
				Sections: []*Section{
					{ID: "S1", Title: "Step One", Level: 2},
				},
			},
			result: &ReviewResult{
				Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionSuggestion, Body: "plain comment"},
				},
			},
			filePath: "plan.md",
			wantContains: []string{
				"[suggestion] plain comment",
			},
		},
		{
			name: "overview comment uses Overview heading",
			doc: &Document{
				Title: "Test Plan",
				Sections: []*Section{
					{ID: "S1", Title: "First Step", Level: 2},
				},
			},
			result: &ReviewResult{
				Comments: []ReviewComment{
					{SectionID: OverviewSectionID, Action: ActionNote, Body: "Overall looks good."},
					{SectionID: "S1", Action: ActionSuggestion, Body: "Change the algorithm."},
				},
			},
			filePath: "plan.md",
			wantContains: []string{
				"## Overview\n",
				"[note] Overall looks good.",
				"## S1: First Step\n",
				"[suggestion] Change the algorithm.",
			},
		},
		{
			name: "line-level comment single line",
			doc: &Document{
				Sections: []*Section{
					{ID: "S1", Title: "Step One", Level: 2},
				},
			},
			result: &ReviewResult{
				Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionSuggestion, Body: "Fix this line.", StartLine: 10},
				},
			},
			filePath: "plan.md",
			wantContains: []string{
				"---\n",
				"`L10` [suggestion] Fix this line.",
			},
		},
		{
			name: "line-level comment range",
			doc: &Document{
				Sections: []*Section{
					{ID: "S1", Title: "Step One", Level: 2},
				},
			},
			result: &ReviewResult{
				Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionIssue, Decoration: DecorationBlocking, Body: "Refactor this block.", StartLine: 10, EndLine: 15},
				},
			},
			filePath: "plan.md",
			wantContains: []string{
				"`L10-L15` [issue (blocking)] Refactor this block.",
			},
		},
		{
			name: "mixed section and line comments",
			doc: &Document{
				Sections: []*Section{
					{ID: "S1", Title: "Step One", Level: 2},
				},
			},
			result: &ReviewResult{
				Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionNote, Body: "Section comment."},
					{SectionID: "S1", Action: ActionSuggestion, Body: "Line comment.", StartLine: 5},
				},
			},
			filePath: "plan.md",
			wantContains: []string{
				"## S1: Step One\n",
				"[note] Section comment.",
				"---\n",
				"`L5` [suggestion] Line comment.",
			},
		},
		{
			name: "empty filePath uses fallback text",
			doc: &Document{
				Sections: []*Section{
					{ID: "S1", Title: "Step One", Level: 2},
				},
			},
			result: &ReviewResult{
				Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionSuggestion, Body: "comment"},
				},
			},
			filePath: "",
			wantContains: []string{
				"the file",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := FormatReview(tt.result, tt.doc, tt.filePath)

			if tt.wantEmpty {
				if output != "" {
					t.Errorf("expected empty output, got: %q", output)
				}
				return
			}

			for _, s := range tt.wantContains {
				if !strings.Contains(output, s) {
					t.Errorf("output missing %q, got:\n%s", s, output)
				}
			}

			for s, wantN := range tt.wantCounts {
				if gotN := strings.Count(output, s); gotN != wantN {
					t.Errorf("expected %q to appear %d time(s), got %d in:\n%s", s, wantN, gotN, output)
				}
			}
		})
	}
}

func TestFormatReviewLineComments(t *testing.T) {
	doc := &Document{
		Sections: []*Section{{ID: "S1", Title: "Step", Level: 2, StartLine: 1, EndLine: 10}},
	}
	tests := []struct {
		name         string
		comment      ReviewComment
		wantContains []string
		wantMissing  []string
	}{
		{
			name:         "single line with quote",
			comment:      ReviewComment{SectionID: "S1", Action: ActionIssue, Body: "Wrong.", StartLine: 5, Quote: []string{"the line"}},
			wantContains: []string{"`L5` [issue] Wrong.\n> the line\n"},
			wantMissing:  []string{"(removed)"},
		},
		{
			name:         "removed diff line is marked",
			comment:      ReviewComment{SectionID: "S1", Action: ActionQuestion, Body: "Why drop?", StartLine: 3, EndLine: 4, Side: "LEFT", Quote: []string{"old a", "old b"}},
			wantContains: []string{"`L3-L4 (removed)` [question] Why drop?\n> old a\n> old b\n"},
		},
		{
			name:         "long quote is elided after five lines",
			comment:      ReviewComment{SectionID: "S1", Action: ActionNote, Body: "Range.", StartLine: 1, EndLine: 7, Quote: []string{"1", "2", "3", "4", "5", "6", "7"}},
			wantContains: []string{"> 1\n> 2\n> 3\n> 4\n> 5\n> ...\n"},
			wantMissing:  []string{"> 6"},
		},
		{
			name:         "section comment keeps the quote of lines edited away",
			comment:      ReviewComment{SectionID: "S1", Action: ActionNote, Body: "Gone.", Quote: []string{"old line"}},
			wantContains: []string{"## S1: Step\n[note] Gone.\n> old line\n"},
		},
		{
			name:         "no quote omits blockquote",
			comment:      ReviewComment{SectionID: "S1", Action: ActionNote, Body: "Bare.", StartLine: 2},
			wantContains: []string{"`L2` [note] Bare.\n"},
			wantMissing:  []string{">"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatReview(&ReviewResult{Comments: []ReviewComment{tt.comment}}, doc, "f.md")
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q:\n%s", want, got)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(got, missing) {
					t.Errorf("output should not contain %q:\n%s", missing, got)
				}
			}
		})
	}
}

func TestFormatReviews(t *testing.T) {
	docA := &Document{Sections: []*Section{{ID: "S1", Title: "Alpha", Level: 2}}}
	docB := &Document{Sections: []*Section{{ID: "S1", Title: "Beta", Level: 2}}}
	tests := []struct {
		name         string
		files        []FileResult
		wantEmpty    bool
		wantContains []string
		wantMissing  []string
	}{
		{
			name: "two files with comments",
			files: []FileResult{
				{Path: "docs/a.md", Doc: docA, Review: &ReviewResult{Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionSuggestion, Body: "Tighten."},
				}}},
				{Path: "docs/b.md", Doc: docB, Review: &ReviewResult{Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionIssue, Body: "Broken.", StartLine: 4, Quote: []string{"x"}},
				}}},
			},
			wantContains: []string{
				"# Review\n\nPlease review and address the following comments.\n",
				"\n## docs/a.md\n\n### S1: Alpha\n[suggestion] Tighten.\n",
				"\n## docs/b.md\n\n---\n\n`L4` [issue] Broken.\n> x\n",
			},
		},
		{
			name: "files without comments are omitted",
			files: []FileResult{
				{Path: "docs/a.md", Doc: docA, Review: &ReviewResult{}},
				{Path: "docs/b.md", Doc: docB, Review: &ReviewResult{Comments: []ReviewComment{
					{SectionID: "S1", Action: ActionNote, Body: "Only this."},
				}}},
			},
			wantContains: []string{"## docs/b.md"},
			wantMissing:  []string{"docs/a.md"},
		},
		{
			name: "nil and empty reviews yield empty output",
			files: []FileResult{
				{Path: "docs/a.md", Doc: docA, Review: nil},
				{Path: "docs/b.md", Doc: docB, Review: &ReviewResult{}},
			},
			wantEmpty: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatReviews(tt.files)
			if tt.wantEmpty {
				if got != "" {
					t.Fatalf("expected empty output, got %q", got)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q:\n%s", want, got)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(got, missing) {
					t.Errorf("output should not contain %q:\n%s", missing, got)
				}
			}
		})
	}
}
