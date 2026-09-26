package markdown

import (
	"cmp"
	"maps"
	"slices"
	"testing"

	"github.com/koh-sh/commd/internal/diff"
)

func TestReviewCommentFormatLabel(t *testing.T) {
	tests := []struct {
		name    string
		comment ReviewComment
		want    string
	}{
		{
			name:    "no decoration",
			comment: ReviewComment{Action: ActionSuggestion},
			want:    "suggestion",
		},
		{
			name:    "zero value decoration",
			comment: ReviewComment{Action: ActionIssue, Decoration: DecorationNone},
			want:    "issue",
		},
		{
			name:    "non-blocking",
			comment: ReviewComment{Action: ActionSuggestion, Decoration: DecorationNonBlocking},
			want:    "suggestion (non-blocking)",
		},
		{
			name:    "blocking",
			comment: ReviewComment{Action: ActionIssue, Decoration: DecorationBlocking},
			want:    "issue (blocking)",
		},
		{
			name:    "if-minor",
			comment: ReviewComment{Action: ActionNitpick, Decoration: DecorationIfMinor},
			want:    "nitpick (if-minor)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.comment.FormatLabel()
			if got != tt.want {
				t.Errorf("FormatLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReviewCommentFormatLineRef(t *testing.T) {
	tests := []struct {
		name    string
		comment ReviewComment
		want    string
	}{
		{
			name:    "section-level (no line info)",
			comment: ReviewComment{StartLine: 0, EndLine: 0},
			want:    "",
		},
		{
			name:    "single line",
			comment: ReviewComment{StartLine: 10, EndLine: 0},
			want:    "L10",
		},
		{
			name:    "single line (EndLine equals StartLine)",
			comment: ReviewComment{StartLine: 5, EndLine: 5},
			want:    "L5",
		},
		{
			name:    "line range",
			comment: ReviewComment{StartLine: 10, EndLine: 15},
			want:    "L10-L15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.comment.FormatLineRef()
			if got != tt.want {
				t.Errorf("FormatLineRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

// sectionsDoc has the sections S1 (lines 5-8), its child S1.1 (9-12) and S2
// (13-15), after a preamble (lines 1-4).
const sectionsDoc = `# Title

Preamble text

## Alpha

alpha body

### Beta

beta body

## Gamma

gamma body
`

func TestDocumentLineSections(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "preamble and nested sections",
			source: sectionsDoc,
			want: []string{
				OverviewSectionID, OverviewSectionID, OverviewSectionID, OverviewSectionID,
				"S1", "S1", "S1", "S1",
				"S1.1", "S1.1", "S1.1", "S1.1",
				"S2", "S2", "S2",
			},
		},
		{
			name:   "no headings",
			source: "just text\nmore text\n",
			want:   []string{OverviewSectionID, OverviewSectionID},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse([]byte(tt.source))
			if err != nil {
				t.Fatal(err)
			}
			if got := doc.LineSections(); !slices.Equal(got, tt.want) {
				t.Errorf("LineSections() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDocumentDiffLineSections(t *testing.T) {
	doc, err := Parse([]byte(sectionsDoc))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		lines []diff.Line
		want  []string
	}{
		{
			name: "added and context lines use their new-file line",
			lines: []diff.Line{
				{Type: diff.Context, NewLine: 7, OldLine: 7},
				{Type: diff.Added, NewLine: 13},
			},
			want: []string{"S1", "S2"},
		},
		{
			// The old line number (14) would fall in S2 of the new file.
			name: "removed lines follow the preceding line",
			lines: []diff.Line{
				{Type: diff.Context, NewLine: 11, OldLine: 13},
				{Type: diff.Removed, OldLine: 14},
				{Type: diff.Added, NewLine: 13},
			},
			want: []string{"S1.1", "S1.1", "S2"},
		},
		{
			name: "leading removed lines take the next line's section",
			lines: []diff.Line{
				{Type: diff.Removed, OldLine: 1},
				{Type: diff.Added, NewLine: 9},
			},
			want: []string{"S1.1", "S1.1"},
		},
		{
			name:  "only removed lines belong to the overview",
			lines: []diff.Line{{Type: diff.Removed, OldLine: 3}},
			want:  []string{OverviewSectionID},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := doc.DiffLineSections(&diff.Info{Lines: tt.lines})
			if !slices.Equal(got, tt.want) {
				t.Errorf("DiffLineSections() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDocumentQuote(t *testing.T) {
	doc := &Document{SourceLines: []string{"one", "two", "three"}}
	info := &diff.Info{Lines: []diff.Line{
		{Type: diff.Context, Content: "ctx", NewLine: 1, OldLine: 1},
		{Type: diff.Removed, Content: "old a", OldLine: 2},
		{Type: diff.Removed, Content: "old b", OldLine: 3},
		{Type: diff.Added, Content: "new a", NewLine: 2},
		{Type: diff.Context, Content: "tail", NewLine: 3, OldLine: 4},
	}}
	tests := []struct {
		name  string
		info  *diff.Info
		start int
		end   int
		side  string
		want  []string
	}{
		{name: "source single line", start: 2, want: []string{"two"}},
		{name: "source range", start: 1, end: 3, want: []string{"one", "two", "three"}},
		{name: "source end clamped to file", start: 3, end: 9, want: []string{"three"}},
		{name: "source start past end", start: 5, want: nil},
		{name: "zero start", start: 0, want: nil},
		{name: "diff right side", info: info, start: 2, end: 3, side: diff.SideRight, want: []string{"new a", "tail"}},
		{name: "diff left side has the removed lines", info: info, start: 2, end: 3, side: diff.SideLeft, want: []string{"old a", "old b"}},
		{name: "diff no lines on side", info: info, start: 1, side: diff.SideLeft, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := doc.Quote(tt.info, tt.start, tt.end, tt.side)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Quote(%d, %d, %q) = %v, want %v", tt.start, tt.end, tt.side, got, tt.want)
			}
		})
	}
}

func TestDocumentSearchSections(t *testing.T) {
	tests := []struct {
		name  string
		src   string // sectionsDoc when empty
		query string
		want  []string
	}{
		{name: "match shows its descendants", query: "alpha", want: []string{"S1", "S1.1"}},
		{name: "match shows its ancestors", query: "BETA body", want: []string{"S1", "S1.1"}},
		{name: "match by ID", query: "s2", want: []string{"S2"}},
		{name: "overview by name only", query: "over", want: []string{OverviewSectionID}},
		{name: "no overview without a preamble", src: "# Title\n\n## A\n", query: "over", want: nil},
		{name: "preamble text does not match the overview", query: "preamble", want: nil},
		{name: "no match", query: "nonexistent", want: nil},
		{name: "empty query", query: "", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse([]byte(cmp.Or(tt.src, sectionsDoc)))
			if err != nil {
				t.Fatal(err)
			}
			got := slices.Sorted(maps.Keys(doc.SearchSections(tt.query)))
			if !slices.Equal(got, tt.want) {
				t.Errorf("SearchSections(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestParseActionAndDecoration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		parse    func(string) (string, bool)
		want     string
		wantOkay bool
	}{
		{name: "action", input: "issue", parse: parseActionString, want: "issue", wantOkay: true},
		{name: "unknown action", input: "rant", parse: parseActionString},
		{name: "empty action", input: "", parse: parseActionString},
		{name: "decoration", input: "blocking", parse: parseDecorationString, want: "blocking", wantOkay: true},
		{name: "no decoration", input: "", parse: parseDecorationString, want: "", wantOkay: true},
		{name: "unknown decoration", input: "loud", parse: parseDecorationString},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.parse(tt.input)
			if got != tt.want || ok != tt.wantOkay {
				t.Errorf("parse(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.wantOkay)
			}
		})
	}
}

func parseActionString(s string) (string, bool) {
	a, ok := ParseAction(s)
	return string(a), ok
}

func parseDecorationString(s string) (string, bool) {
	d, ok := ParseDecoration(s)
	return string(d), ok
}

func TestNewReviewResult(t *testing.T) {
	doc, err := Parse([]byte(sectionsDoc))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		comments []ReviewComment
		want     []string // bodies in result order
	}{
		{
			name: "document order, creation order within a section",
			comments: []ReviewComment{
				{SectionID: "S2", Body: "s2"},
				{SectionID: "S1.1", Body: "child"},
				{SectionID: "S1", Body: "s1 first"},
				{SectionID: OverviewSectionID, Body: "overview"},
				{SectionID: "S1", Body: "s1 second"},
			},
			want: []string{"overview", "s1 first", "s1 second", "child", "s2"},
		},
		{
			name:     "unknown sections are dropped",
			comments: []ReviewComment{{SectionID: "S9", Body: "gone"}, {SectionID: "S1", Body: "kept"}},
			want:     []string{"kept"},
		},
		{name: "no comments", comments: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, c := range NewReviewResult(doc, tt.comments).Comments {
				got = append(got, c.Body)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("comments = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReviewResultStatus(t *testing.T) {
	tests := []struct {
		name   string
		result ReviewResult
		want   Status
	}{
		{name: "no comments", result: ReviewResult{}, want: StatusApproved},
		{name: "with comments", result: ReviewResult{Comments: []ReviewComment{{Body: "b"}}}, want: StatusSubmitted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Status(); got != tt.want {
				t.Errorf("Status() = %s, want %s", got, tt.want)
			}
		})
	}
}
