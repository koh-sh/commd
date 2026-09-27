package markdown

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/koh-sh/commd/internal/diff"
)

// reloadFile parses source as the new read of the review's file.
func reloadFile(t *testing.T, source, patch string) File {
	t.Helper()
	doc, err := Parse([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	f := File{Path: "doc.md", Doc: doc}
	if patch != "" {
		f.Diff = diff.ParsePatch(patch)
	}
	return f
}

func TestReviewStateReloadComments(t *testing.T) {
	// A new section before First shifts every section ID and line by four.
	inserted := strings.Replace(reviewSource, "## First", "## Zero\n\nzero\n\n## First", 1)
	tests := []struct {
		name      string
		diff      bool
		in        ReviewComment
		source    string
		patch     string
		want      ReviewComment
		wantMoved int
	}{
		{
			name:   "a section comment follows its heading",
			in:     ReviewComment{SectionID: "S2", Action: ActionNote, Body: "b"},
			source: inserted,
			want:   ReviewComment{SectionID: "S3", Action: ActionNote, Body: "b"},
		},
		{
			name:   "an overview comment stays",
			in:     ReviewComment{SectionID: OverviewSectionID, Action: ActionNote, Body: "b"},
			source: inserted,
			want:   ReviewComment{SectionID: OverviewSectionID, Action: ActionNote, Body: "b"},
		},
		{
			name:   "a line comment follows its quote",
			in:     ReviewComment{Action: ActionNote, Body: "b", StartLine: 7, EndLine: 8},
			source: inserted,
			want: ReviewComment{
				SectionID: "S2", Action: ActionNote, Body: "b",
				StartLine: 11, EndLine: 12, Quote: []string{"line six", "line seven"},
			},
		},
		{
			name:   "a repeated quote takes the occurrence nearest the old line",
			in:     ReviewComment{Action: ActionNote, Body: "b", StartLine: 12},
			source: strings.Replace(reviewSource, "line six", "line eleven", 1),
			want:   ReviewComment{SectionID: "S2", Action: ActionNote, Body: "b", StartLine: 12, Quote: []string{"line eleven"}},
		},
		{
			name:      "an edited line becomes a section comment keeping its quote",
			in:        ReviewComment{Action: ActionNote, Body: "b", StartLine: 7},
			source:    strings.Replace(inserted, "line six", "line 6", 1),
			want:      ReviewComment{SectionID: "S2", Action: ActionNote, Body: "b", Quote: []string{"line six"}},
			wantMoved: 1,
		},
		{
			name:      "a comment on a removed heading moves to the overview",
			in:        ReviewComment{SectionID: "S2", Action: ActionNote, Body: "b"},
			source:    strings.Replace(reviewSource, "## Second\n\n", "", 1),
			want:      ReviewComment{SectionID: OverviewSectionID, Action: ActionNote, Body: "b"},
			wantMoved: 1,
		},
		{
			name:      "without an overview it moves to the first section",
			in:        ReviewComment{SectionID: "S2", Action: ActionNote, Body: "b"},
			source:    "# Title\n\n## First\n\nline\n",
			want:      ReviewComment{SectionID: "S1", Action: ActionNote, Body: "b"},
			wantMoved: 1,
		},
		{
			name:   "a removed diff line is found on its side",
			diff:   true,
			in:     ReviewComment{Action: ActionNote, Body: "b", StartLine: 7, Side: diff.SideLeft},
			source: reviewSource,
			patch:  reviewPatch,
			want: ReviewComment{
				SectionID: "S1", Action: ActionNote, Body: "b",
				StartLine: 7, Side: diff.SideLeft, Quote: []string{"old seven"},
			},
		},
		{
			name:   "an added diff line follows its quote",
			diff:   true,
			in:     ReviewComment{Action: ActionNote, Body: "b", StartLine: 13, Side: diff.SideRight},
			source: inserted,
			patch:  strings.Replace(reviewPatch, "@@ -5,8 +5,9 @@", "@@ -5,8 +9,9 @@", 1),
			want: ReviewComment{
				SectionID: "S3", Action: ActionNote, Body: "b",
				StartLine: 17, Side: diff.SideRight, Quote: []string{"added twelve"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReview(t, tt.diff)
			if err := r.SaveComment(tt.in); err != nil {
				t.Fatal(err)
			}
			id := r.Comments()[0].ID
			if moved, _ := r.reload(reloadFile(t, tt.source, tt.patch)); moved != tt.wantMoved {
				t.Errorf("moved = %d, want %d", moved, tt.wantMoved)
			}
			if len(r.Comments()) != 1 {
				t.Fatalf("got %d comments, want 1", len(r.Comments()))
			}
			got := r.Comments()[0]
			if got.ID != id {
				t.Errorf("ID = %q, want %q", got.ID, id)
			}
			assertComment(t, *got, tt.want)
		})
	}
}

func TestReviewStateReloadViewed(t *testing.T) {
	// Zero is new, First moves to S2 unchanged, Second (now S3) is edited.
	source := strings.Replace(reviewSource, "## First", "## Zero\n\nzero\n\n## First", 1) + "more\n"
	tests := []struct {
		name       string
		persist    bool
		wantViewed map[string]bool
	}{
		{name: "session only", wantViewed: map[string]bool{"S1": false, "S2": true, "S3": false}},
		{name: "persisted", persist: true, wantViewed: map[string]bool{"S1": false, "S2": true, "S3": false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReview(t, false)
			if tt.persist {
				r.Viewed = NewViewedState()
			}
			for _, id := range []string{"S1", "S2"} {
				if err := r.SetViewed(id, true); err != nil {
					t.Fatal(err)
				}
			}
			persisted := r.Viewed
			r.reload(reloadFile(t, source, ""))
			for id, want := range tt.wantViewed {
				if got := r.IsViewed(id); got != want {
					t.Errorf("IsViewed(%s) = %v, want %v", id, got, want)
				}
			}
			if r.Viewed != persisted {
				t.Error("the persisted viewed state was replaced")
			}
		})
	}
}

func TestReviewStateReread(t *testing.T) {
	inserted := strings.Replace(reviewSource, "## First", "## Zero\n\nzero\n\n## First", 1)
	tests := []struct {
		name   string
		source string // "" fails the read
		want   ReloadResult
	}{
		{
			name:   "changed content maps the sections that are still there",
			source: inserted,
			want:   ReloadResult{Changed: true, Sections: map[string]string{OverviewSectionID: OverviewSectionID, "S1": "S2", "S2": "S3"}},
		},
		{
			name:   "a removed section is left out",
			source: strings.Replace(reviewSource, "## Second\n\n", "", 1),
			want:   ReloadResult{Changed: true, Sections: map[string]string{OverviewSectionID: OverviewSectionID, "S1": "S1"}},
		},
		{name: "unchanged content is not reloaded", source: reviewSource, want: ReloadResult{}},
		{name: "a failed read changes nothing", want: ReloadResult{Err: errors.New("gone")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReview(t, false)
			doc := r.Doc
			got := r.Reread(func(string) (File, error) {
				if tt.source == "" {
					return File{}, errors.New("gone")
				}
				return reloadFile(t, tt.source, ""), nil
			})
			if got.Changed != tt.want.Changed || (got.Err == nil) != (tt.want.Err == nil) || !maps.Equal(got.Sections, tt.want.Sections) {
				t.Errorf("Reread = %+v, want %+v", got, tt.want)
			}
			if (r.Doc != doc) != tt.want.Changed {
				t.Errorf("document replaced = %v, want %v", r.Doc != doc, tt.want.Changed)
			}
		})
	}
}
