package web

import (
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

func testFile(t *testing.T, withDiff bool) markdown.File {
	t.Helper()
	f := markdown.File{Path: "doc.md", Doc: mustParse(t, testSource)}
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
			f := newFileState(markdown.File{Path: "a.md", Doc: mustParse(t, tt.src)}, nil)
			var got []string
			for _, s := range f.rendered {
				got = append(got, s.ID)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("sections = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileLines(t *testing.T) {
	tests := []struct {
		name string
		diff bool
		want []lineJSON // the lines of the Second section
	}{
		{
			name: "source",
			want: []lineJSON{
				{Text: "## Second", Line: 10, Section: "S2"},
				{Text: "", Line: 11, Section: "S2"},
				{Text: "line eleven", Line: 12, Section: "S2"},
			},
		},
		{
			name: "diff",
			diff: true,
			want: []lineJSON{
				{Text: "## Second", Line: 10, Side: diff.SideRight, Type: " ", Section: "S2"},
				{Text: "", Line: 11, Side: diff.SideRight, Type: " ", Section: "S2"},
				{Text: "line eleven", Line: 12, Side: diff.SideRight, Type: " ", Section: "S2"},
				{Text: "added twelve", Line: 13, Side: diff.SideRight, Type: "+", Section: "S2"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFileState(testFile(t, tt.diff), nil)
			var got []lineJSON
			for _, l := range f.lines {
				if l.Section == "S2" {
					got = append(got, l)
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("lines = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFileComments(t *testing.T) {
	tests := []struct {
		name       string
		update     *markdown.ReviewComment // applied to the created comment
		wantBodies []string
		wantID     bool // the stored comment has a server-assigned ID
	}{
		{name: "the page cannot choose the ID of a new comment", wantBodies: []string{"b"}, wantID: true},
		{name: "update", update: &markdown.ReviewComment{Action: markdown.ActionIssue, Body: "changed"}, wantBodies: []string{"changed"}, wantID: true},
		{name: "emptying a comment deletes it, as in the TUI", update: &markdown.ReviewComment{Action: markdown.ActionIssue, Body: " "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFileState(testFile(t, false), nil)
			in := markdown.ReviewComment{ID: "chosen", SectionID: "S1", Action: markdown.ActionNote, Body: "b"}
			if err := f.addComment(in); err != nil {
				t.Fatal(err)
			}
			if tt.update != nil {
				if err := f.updateComment(f.Comments()[0].ID, *tt.update); err != nil {
					t.Fatal(err)
				}
			}
			var bodies []string
			for _, c := range f.Comments() {
				bodies = append(bodies, c.Body)
				if tt.wantID && (c.ID == "" || c.ID == "chosen") {
					t.Errorf("ID = %q, want one assigned by the server", c.ID)
				}
			}
			if !slices.Equal(bodies, tt.wantBodies) {
				t.Errorf("comments = %q, want %q", bodies, tt.wantBodies)
			}
		})
	}
}
