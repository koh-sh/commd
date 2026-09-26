package markdown

import "testing"

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

func TestDocumentSectionIDAtLine(t *testing.T) {
	doc, err := Parse(readTestdata(t, "basic.md"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		line int
		want string
	}{
		{name: "title line", line: 1, want: OverviewSectionID},
		{name: "preamble", line: 3, want: OverviewSectionID},
		{name: "section heading", line: 5, want: "S1"},
		{name: "section body before child", line: 8, want: "S1"},
		{name: "child heading", line: 9, want: "S1.1"},
		{name: "past last line", line: 999, want: "S3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doc.SectionIDAtLine(tt.line); got != tt.want {
				t.Errorf("SectionIDAtLine(%d) = %q, want %q", tt.line, got, tt.want)
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
