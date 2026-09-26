package web

import (
	"cmp"
	"errors"
	"slices"
	"testing"

	"github.com/koh-sh/commd/internal/diff"
	"github.com/koh-sh/commd/internal/markdown"
)

const testSource = "# Title\n\nIntro.\n\n## First\n\nline six\nline seven\n\n## Second\n\nline eleven\n"

// testPatch changes line 7 of testSource and adds a line to the Second
// section; the removed line is numbered 7 in the old file.
const testPatch = "@@ -5,8 +5,9 @@\n ## First\n \n-old seven\n+line six\n line seven\n \n ## Second\n \n line eleven\n+added twelve\n"

func mustParse(t *testing.T, src string) *markdown.Document {
	t.Helper()
	doc, err := markdown.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func testFile(t *testing.T, withDiff bool) File {
	t.Helper()
	f := File{Path: "doc.md", Doc: mustParse(t, testSource)}
	if withDiff {
		f.Diff = diff.ParsePatch(testPatch)
	}
	return f
}

func TestFileSections(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string // section-level comment targets
	}{
		{name: "preamble gives an overview", src: "Intro.\n\n## A\n", want: []string{"overview", "S1"}},
		{name: "title alone gives no overview, like the TUI", src: "# Title\n\n## A\n", want: []string{"S1"}},
		{name: "headings only", src: "## A\n\n## B\n", want: []string{"S1", "S2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFileState(File{Path: "a.md", Doc: mustParse(t, tt.src)})
			var got []string
			for _, s := range f.sectionsJSON(nil) {
				got = append(got, s.ID)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("sections = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileAddComment(t *testing.T) {
	tests := []struct {
		name        string
		diff        bool
		in          commentInput
		wantErr     error // matched with errors.Is when set
		wantErrText bool  // any other error
		want        markdown.ReviewComment
	}{
		{
			name: "section comment",
			in:   commentInput{SectionID: "S2", Action: "issue", Decoration: "blocking", Body: "  fix  "},
			want: markdown.ReviewComment{SectionID: "S2", Action: markdown.ActionIssue, Decoration: markdown.DecorationBlocking, Body: "fix"},
		},
		{
			name: "overview comment",
			in:   commentInput{SectionID: "overview", Action: "note", Body: "b"},
			want: markdown.ReviewComment{SectionID: "overview", Action: markdown.ActionNote, Body: "b"},
		},
		{name: "unknown section", in: commentInput{SectionID: "S9", Action: "note", Body: "b"}, wantErr: errNotFound},
		{name: "unknown label", in: commentInput{SectionID: "S1", Action: "rant", Body: "b"}, wantErrText: true},
		{name: "unknown decoration", in: commentInput{SectionID: "S1", Action: "note", Decoration: "loud", Body: "b"}, wantErrText: true},
		{name: "blank body", in: commentInput{SectionID: "S1", Action: "note", Body: " \n"}, wantErrText: true},
		{
			name: "line range takes the section and quote from the source",
			in:   commentInput{Action: "question", Body: "b", StartLine: 7, EndLine: 8},
			want: markdown.ReviewComment{
				SectionID: "S1", Action: markdown.ActionQuestion, Body: "b",
				StartLine: 7, EndLine: 8, Quote: []string{"line six", "line seven"},
			},
		},
		{
			name: "single line has no end line",
			in:   commentInput{Action: "question", Body: "b", StartLine: 12, EndLine: 12},
			want: markdown.ReviewComment{SectionID: "S2", Action: markdown.ActionQuestion, Body: "b", StartLine: 12, Quote: []string{"line eleven"}},
		},
		{
			// Lines before the first heading are commentable, as in the TUI.
			name: "title line",
			in:   commentInput{Action: "note", Body: "b", StartLine: 1},
			want: markdown.ReviewComment{SectionID: "overview", Action: markdown.ActionNote, Body: "b", StartLine: 1, Quote: []string{"# Title"}},
		},
		{name: "line past the end", in: commentInput{Action: "question", Body: "b", StartLine: 99}, wantErrText: true},
		{
			name: "removed diff line",
			diff: true,
			in:   commentInput{Action: "question", Body: "b", StartLine: 7, Side: diff.SideLeft},
			want: markdown.ReviewComment{
				SectionID: "S1", Action: markdown.ActionQuestion, Body: "b",
				StartLine: 7, Side: diff.SideLeft, Quote: []string{"old seven"},
			},
		},
		{
			name: "added diff line",
			diff: true,
			in:   commentInput{Action: "question", Body: "b", StartLine: 13, Side: diff.SideRight},
			want: markdown.ReviewComment{
				SectionID: "S2", Action: markdown.ActionQuestion, Body: "b",
				StartLine: 13, Side: diff.SideRight, Quote: []string{"added twelve"},
			},
		},
		{name: "diff line without side", diff: true, in: commentInput{Action: "question", Body: "b", StartLine: 7}, wantErrText: true},
		{name: "line outside the diff", diff: true, in: commentInput{Action: "question", Body: "b", StartLine: 1, Side: diff.SideRight}, wantErrText: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFileState(testFile(t, tt.diff))
			c, err := f.addComment(tt.in)
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			case tt.wantErrText:
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			case err != nil:
				t.Fatal(err)
			}
			if c.ID == "" {
				t.Error("comment has no ID")
			}
			assertComment(t, c.ReviewComment, tt.want)
		})
	}
}

func assertComment(t *testing.T, got, want markdown.ReviewComment) {
	t.Helper()
	if got.SectionID != want.SectionID || got.Action != want.Action || got.Decoration != want.Decoration ||
		got.Body != want.Body || got.StartLine != want.StartLine || got.EndLine != want.EndLine ||
		got.Side != want.Side || !slices.Equal(got.Quote, want.Quote) {
		t.Errorf("comment = %+v, want %+v", got, want)
	}
}

func TestFileUpdateComment(t *testing.T) {
	tests := []struct {
		name    string
		id      string // "" targets the comment created for the test
		in      commentInput
		wantErr error
		want    markdown.ReviewComment
	}{
		{
			// The target lines never change on update.
			name: "label and body change, target stays",
			in:   commentInput{Action: "issue", Decoration: "if-minor", Body: "changed", StartLine: 1},
			want: markdown.ReviewComment{
				SectionID: "S1", Action: markdown.ActionIssue, Decoration: markdown.DecorationIfMinor,
				Body: "changed", StartLine: 7, Quote: []string{"line six"},
			},
		},
		{name: "unknown id", id: "nope", in: commentInput{Action: "note", Body: "b"}, wantErr: errNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFileState(testFile(t, false))
			c, err := f.addComment(commentInput{Action: "question", Body: "q", StartLine: 7})
			if err != nil {
				t.Fatal(err)
			}
			updated, err := f.updateComment(cmp.Or(tt.id, c.ID), tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				assertComment(t, updated.ReviewComment, tt.want)
			}
		})
	}
}

func TestFileDeleteComment(t *testing.T) {
	tests := []struct {
		name         string
		deleteTwice  bool
		id           string // "" targets the comment created for the test
		wantErr      error
		wantComments int
	}{
		{name: "delete", wantComments: 0},
		{name: "delete again", deleteTwice: true, wantErr: errNotFound, wantComments: 0},
		{name: "unknown id", id: "nope", wantErr: errNotFound, wantComments: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFileState(testFile(t, false))
			c, err := f.addComment(commentInput{SectionID: "S1", Action: "note", Body: "b"})
			if err != nil {
				t.Fatal(err)
			}
			id := cmp.Or(tt.id, c.ID)
			if tt.deleteTwice {
				if err := f.deleteComment(id); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.deleteComment(id); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if got := len(f.comments); got != tt.wantComments {
				t.Errorf("comments = %d, want %d", got, tt.wantComments)
			}
		})
	}
}

func TestFileSetViewed(t *testing.T) {
	tests := []struct {
		name          string
		initiallySeen bool // S1 is persisted as viewed before the review starts
		section       string
		viewed        bool
		wantErr       error
		wantStored    bool // state reports S1 as viewed afterwards
	}{
		{name: "mark", section: "S1", viewed: true, wantStored: true},
		{name: "unmark", initiallySeen: true, section: "S1", viewed: false, wantStored: false},
		{name: "overview cannot be marked, like the TUI", section: "overview", viewed: true, wantErr: errNotFound},
		{name: "unknown section", section: "S9", viewed: true, wantErr: errNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := mustParse(t, testSource)
			state := markdown.NewViewedState()
			if tt.initiallySeen {
				state.MarkViewed(doc.FindSection("S1"))
			}
			f := newFileState(File{Path: "doc.md", Doc: doc, Viewed: state})
			if got := f.viewed["S1"]; got != tt.initiallySeen {
				t.Fatalf("restored S1 viewed = %v, want %v", got, tt.initiallySeen)
			}
			if err := f.setViewed(tt.section, tt.viewed); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got := state.IsSectionViewed(doc.FindSection("S1")); got != tt.wantStored {
				t.Errorf("persisted S1 viewed = %v, want %v", got, tt.wantStored)
			}
		})
	}
}

func TestFileBuildReview(t *testing.T) {
	f := newFileState(testFile(t, false))
	for _, in := range []commentInput{
		{SectionID: "S2", Action: "note", Body: "s2"},
		{Action: "note", Body: "s1 line", StartLine: 7},
		{SectionID: "overview", Action: "note", Body: "overview"},
		{SectionID: "S1", Action: "note", Body: "s1 section"},
	} {
		if _, err := f.addComment(in); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, c := range f.buildReview().Comments {
		got = append(got, c.Body)
	}
	// Overview first, then sections in document order, creation order within.
	want := []string{"overview", "s1 line", "s1 section", "s2"}
	if !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}
