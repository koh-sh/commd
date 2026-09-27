package markdown

import (
	"cmp"
	"errors"
	"slices"
	"testing"

	"github.com/koh-sh/commd/internal/diff"
)

const reviewSource = "# Title\n\nIntro.\n\n## First\n\nline six\nline seven\n\n## Second\n\nline eleven\n"

// reviewPatch changes line 7 of reviewSource and adds a line to the Second
// section; the removed line is numbered 7 in the old file.
const reviewPatch = "@@ -5,8 +5,9 @@\n ## First\n \n-old seven\n+line six\n line seven\n \n ## Second\n \n line eleven\n+added twelve\n"

func newTestReview(t *testing.T, withDiff bool) *ReviewState {
	t.Helper()
	doc, err := Parse([]byte(reviewSource))
	if err != nil {
		t.Fatal(err)
	}
	f := File{Path: "doc.md", Doc: doc}
	if withDiff {
		f.Diff = diff.ParsePatch(reviewPatch)
	}
	return NewReviewState(f)
}

func assertComment(t *testing.T, got, want ReviewComment) {
	t.Helper()
	if got.SectionID != want.SectionID || got.Action != want.Action || got.Decoration != want.Decoration ||
		got.Body != want.Body || got.StartLine != want.StartLine || got.EndLine != want.EndLine ||
		got.Side != want.Side || !slices.Equal(got.Quote, want.Quote) {
		t.Errorf("comment = %+v, want %+v", got, want)
	}
}

func TestReviewStateAddComment(t *testing.T) {
	tests := []struct {
		name        string
		diff        bool
		in          ReviewComment
		wantErr     error // matched with errors.Is when set
		wantErrText bool  // any other error
		wantNone    bool  // nothing is stored
		want        ReviewComment
	}{
		{
			name: "section comment",
			in:   ReviewComment{SectionID: "S2", Action: ActionIssue, Decoration: DecorationBlocking, Body: "  fix  "},
			want: ReviewComment{SectionID: "S2", Action: ActionIssue, Decoration: DecorationBlocking, Body: "fix"},
		},
		{
			name: "overview comment",
			in:   ReviewComment{SectionID: OverviewSectionID, Action: ActionNote, Body: "b"},
			want: ReviewComment{SectionID: OverviewSectionID, Action: ActionNote, Body: "b"},
		},
		{
			name: "a quote sent with a section comment is dropped",
			in:   ReviewComment{SectionID: "S1", Action: ActionNote, Body: "b", Quote: []string{"x"}},
			want: ReviewComment{SectionID: "S1", Action: ActionNote, Body: "b"},
		},
		{name: "blank body is dropped", in: ReviewComment{SectionID: "S1", Action: ActionNote, Body: " \n"}, wantNone: true},
		{name: "unknown section", in: ReviewComment{SectionID: "S9", Action: ActionNote, Body: "b"}, wantErr: ErrNotFound},
		{name: "unknown label", in: ReviewComment{SectionID: "S1", Action: "rant", Body: "b"}, wantErrText: true},
		{name: "unknown decoration", in: ReviewComment{SectionID: "S1", Action: ActionNote, Decoration: "loud", Body: "b"}, wantErrText: true},
		{
			name: "line range takes the section and quote from the source",
			in:   ReviewComment{SectionID: "S2", Action: ActionQuestion, Body: "b", StartLine: 7, EndLine: 8},
			want: ReviewComment{
				SectionID: "S1", Action: ActionQuestion, Body: "b",
				StartLine: 7, EndLine: 8, Quote: []string{"line six", "line seven"},
			},
		},
		{
			name: "single line has no end line",
			in:   ReviewComment{Action: ActionQuestion, Body: "b", StartLine: 12, EndLine: 12},
			want: ReviewComment{SectionID: "S2", Action: ActionQuestion, Body: "b", StartLine: 12, Quote: []string{"line eleven"}},
		},
		{
			// Lines before the first heading are commentable.
			name: "title line",
			in:   ReviewComment{Action: ActionNote, Body: "b", StartLine: 1},
			want: ReviewComment{SectionID: OverviewSectionID, Action: ActionNote, Body: "b", StartLine: 1, Quote: []string{"# Title"}},
		},
		{name: "line past the end", in: ReviewComment{Action: ActionQuestion, Body: "b", StartLine: 99}, wantErrText: true},
		{
			name: "removed diff line",
			diff: true,
			in:   ReviewComment{Action: ActionQuestion, Body: "b", StartLine: 7, Side: diff.SideLeft},
			want: ReviewComment{
				SectionID: "S1", Action: ActionQuestion, Body: "b",
				StartLine: 7, Side: diff.SideLeft, Quote: []string{"old seven"},
			},
		},
		{
			name: "added diff line",
			diff: true,
			in:   ReviewComment{Action: ActionQuestion, Body: "b", StartLine: 13, Side: diff.SideRight},
			want: ReviewComment{
				SectionID: "S2", Action: ActionQuestion, Body: "b",
				StartLine: 13, Side: diff.SideRight, Quote: []string{"added twelve"},
			},
		},
		{name: "diff line without side", diff: true, in: ReviewComment{Action: ActionQuestion, Body: "b", StartLine: 7}, wantErrText: true},
		{name: "line outside the diff", diff: true, in: ReviewComment{Action: ActionQuestion, Body: "b", StartLine: 1, Side: diff.SideRight}, wantErrText: true},
		{
			name:        "range ending outside the diff",
			diff:        true,
			in:          ReviewComment{Action: ActionQuestion, Body: "b", StartLine: 12, EndLine: 14, Side: diff.SideRight},
			wantErrText: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReview(t, tt.diff)
			err := r.SaveComment(tt.in)
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			case tt.wantErrText:
				if err == nil {
					t.Fatal("expected an error")
				}
			case err != nil:
				t.Fatal(err)
			}
			if err != nil || tt.wantNone {
				if n := len(r.Comments()); n != 0 {
					t.Errorf("stored %d comments, want none", n)
				}
				return
			}
			c := r.Comments()[0]
			if c.ID == "" {
				t.Error("comment has no ID")
			}
			assertComment(t, *c, tt.want)
		})
	}
}

func TestReviewStateUpdateComment(t *testing.T) {
	original := ReviewComment{SectionID: "S1", Action: ActionQuestion, Body: "q", StartLine: 7, Quote: []string{"line six"}}
	tests := []struct {
		name        string
		id          string // "" targets the comment created for the test
		in          ReviewComment
		wantErr     error // matched with errors.Is when set
		wantErrText bool  // any other error
		wantGone    bool
		want        ReviewComment
	}{
		{
			// The target lines never change on update.
			name: "label and body change, target stays",
			in:   ReviewComment{Action: ActionIssue, Decoration: DecorationIfMinor, Body: "changed", StartLine: 1},
			want: ReviewComment{
				SectionID: "S1", Action: ActionIssue, Decoration: DecorationIfMinor,
				Body: "changed", StartLine: 7, Quote: []string{"line six"},
			},
		},
		{name: "emptied body deletes the comment", in: ReviewComment{Action: ActionNote, Body: "  "}, wantGone: true},
		{name: "unknown id", id: "nope", in: ReviewComment{Action: ActionNote, Body: "b"}, wantErr: ErrNotFound, want: original},
		{name: "unknown label changes nothing", in: ReviewComment{Action: "rant", Body: "b"}, wantErrText: true, want: original},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReview(t, false)
			if err := r.SaveComment(ReviewComment{Action: ActionQuestion, Body: "q", StartLine: 7}); err != nil {
				t.Fatal(err)
			}
			c := r.Comments()[0]
			tt.in.ID = cmp.Or(tt.id, c.ID)
			err := r.SaveComment(tt.in)
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			case tt.wantErrText:
				if err == nil {
					t.Fatal("expected an error")
				}
			case err != nil:
				t.Fatal(err)
			}
			if tt.wantGone {
				if n := len(r.Comments()); n != 0 {
					t.Errorf("comments = %d, want 0", n)
				}
				return
			}
			assertComment(t, *c, tt.want)
		})
	}
}

func TestReviewStateDeleteComment(t *testing.T) {
	tests := []struct {
		name         string
		deleteTwice  bool
		id           string // "" targets the comment created for the test
		wantErr      error
		wantComments int
	}{
		{name: "delete", wantComments: 0},
		{name: "delete again", deleteTwice: true, wantErr: ErrNotFound, wantComments: 0},
		{name: "unknown id", id: "nope", wantErr: ErrNotFound, wantComments: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReview(t, false)
			if err := r.SaveComment(ReviewComment{SectionID: "S1", Action: ActionNote, Body: "b"}); err != nil {
				t.Fatal(err)
			}
			id := cmp.Or(tt.id, r.Comments()[0].ID)
			if tt.deleteTwice {
				if err := r.DeleteComment(id); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.DeleteComment(id); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if got := len(r.Comments()); got != tt.wantComments {
				t.Errorf("comments = %d, want %d", got, tt.wantComments)
			}
		})
	}
}

func TestReviewStateSetViewed(t *testing.T) {
	tests := []struct {
		name          string
		initiallySeen bool // S1 is persisted as viewed before the review starts
		persist       bool // the file has a ViewedState
		section       string
		viewed        bool
		wantErr       error
		wantViewed    bool // S1 is viewed afterwards
	}{
		{name: "mark", persist: true, section: "S1", viewed: true, wantViewed: true},
		{name: "unmark", persist: true, initiallySeen: true, section: "S1", viewed: false},
		{name: "mark without persistence", section: "S1", viewed: true, wantViewed: true},
		{name: "overview cannot be marked", persist: true, section: OverviewSectionID, viewed: true, wantErr: ErrNotFound},
		{name: "unknown section", persist: true, section: "S9", viewed: true, wantErr: ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse([]byte(reviewSource))
			if err != nil {
				t.Fatal(err)
			}
			f := File{Path: "doc.md", Doc: doc}
			if tt.persist {
				f.Viewed = NewViewedState()
				if tt.initiallySeen {
					f.Viewed.MarkViewed(doc.FindSection("S1"))
				}
			}
			r := NewReviewState(f)
			if got := r.IsViewed("S1"); got != tt.initiallySeen {
				t.Fatalf("restored S1 viewed = %v, want %v", got, tt.initiallySeen)
			}
			if err := r.SetViewed(tt.section, tt.viewed); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got := r.IsViewed("S1"); got != tt.wantViewed {
				t.Errorf("S1 viewed = %v, want %v", got, tt.wantViewed)
			}
			if tt.persist {
				if got := f.Viewed.IsSectionViewed(doc.FindSection("S1")); got != tt.wantViewed {
					t.Errorf("persisted S1 viewed = %v, want %v", got, tt.wantViewed)
				}
			}
			if want := map[bool]int{true: 1, false: 0}[tt.wantViewed]; r.ViewedCount() != want {
				t.Errorf("ViewedCount() = %d, want %d", r.ViewedCount(), want)
			}
		})
	}
}

func TestReviewStateResult(t *testing.T) {
	tests := []struct {
		name string
		in   []ReviewComment
		want []string // bodies in result order
	}{
		{
			name: "overview first, then document order, creation order within a section",
			in: []ReviewComment{
				{SectionID: "S2", Action: ActionNote, Body: "s2"},
				{Action: ActionNote, Body: "s1 line", StartLine: 7},
				{SectionID: OverviewSectionID, Action: ActionNote, Body: "overview"},
				{SectionID: "S1", Action: ActionNote, Body: "s1 section"},
			},
			want: []string{"overview", "s1 line", "s1 section", "s2"},
		},
		{name: "no comments", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReview(t, false)
			for _, c := range tt.in {
				if err := r.SaveComment(c); err != nil {
					t.Fatal(err)
				}
			}
			var got []string
			for _, c := range r.Result().Comments {
				got = append(got, c.Body)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("order = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReviewStateConfirmMessages(t *testing.T) {
	tests := []struct {
		name       string
		comments   int
		multiFile  bool
		wantSubmit string
		wantQuit   string
	}{
		{name: "no comments", wantSubmit: "Submit review? (0 comments)", wantQuit: "Quit review?"},
		{name: "with comments", comments: 2, wantSubmit: "Submit review? (2 comments)", wantQuit: "You have review comments.\n\nQuit without submitting?"},
		{name: "multi-file", comments: 1, multiFile: true, wantSubmit: "Finish reviewing this file? (1 comments)", wantQuit: "Skip this file?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReview(t, false)
			for range tt.comments {
				if err := r.SaveComment(ReviewComment{SectionID: "S1", Action: ActionNote, Body: "b"}); err != nil {
					t.Fatal(err)
				}
			}
			if got := r.ConfirmSubmitMessage(tt.multiFile); got != tt.wantSubmit {
				t.Errorf("ConfirmSubmitMessage() = %q, want %q", got, tt.wantSubmit)
			}
			if got := r.ConfirmQuitMessage(tt.multiFile); got != tt.wantQuit {
				t.Errorf("ConfirmQuitMessage() = %q, want %q", got, tt.wantQuit)
			}
		})
	}
}
