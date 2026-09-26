package web

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/koh-sh/commd/internal/diff"
	"github.com/koh-sh/commd/internal/markdown"
)

// commentInput is what the browser sends to create or update a comment.
type commentInput struct {
	SectionID  string `json:"sectionId"`
	Action     string `json:"action"`
	Decoration string `json:"decoration"`
	Body       string `json:"body"`
	StartLine  int    `json:"startLine"`
	EndLine    int    `json:"endLine"`
	Side       string `json:"side"`
}

// comment is a stored review comment. Comments are addressed by ID so the
// browser never depends on list positions.
type comment struct {
	ID string
	markdown.ReviewComment
}

// fileState is the review state of the file under review.
type fileState struct {
	markdown.File
	lines    []lineJSON      // the source (or diff) view
	sections map[string]bool // valid section-level comment targets
	comments []*comment      // in creation order
	viewed   map[string]bool // section ID -> viewed
	rendered []sectionJSON   // sections with HTML (the source never changes)
}

// newFileState prepares f for review, rendering its sections once with image
// destinations passed through imageURL (see renderHTML).
func newFileState(f markdown.File, imageURL func(string) string) *fileState {
	fs := &fileState{
		File:     f,
		sections: make(map[string]bool),
		viewed:   make(map[string]bool),
		rendered: renderSections(f.Doc, imageURL),
	}
	if f.Diff != nil {
		fs.lines = diffLines(f.Doc, f.Diff)
	} else {
		fs.lines = sourceLines(f.Doc)
	}
	if f.Doc.HasOverview() {
		fs.sections[markdown.OverviewSectionID] = true
	}
	for _, sec := range f.Doc.AllSections() {
		fs.sections[sec.ID] = true
		if f.Viewed != nil && f.Viewed.IsSectionViewed(sec) {
			fs.viewed[sec.ID] = true
		}
	}
	return fs
}

// sourceLines returns the full source, each line under its section.
func sourceLines(doc *markdown.Document) []lineJSON {
	sections := doc.LineSections()
	lines := make([]lineJSON, len(doc.SourceLines))
	for i, text := range doc.SourceLines {
		lines[i] = lineJSON{Text: text, Line: i + 1, Section: sections[i]}
	}
	return lines
}

// diffLines returns the diff lines, each under its section (see
// markdown.Document.DiffLineSections). Removed lines are numbered by the old
// file.
func diffLines(doc *markdown.Document, info *diff.Info) []lineJSON {
	sections := doc.DiffLineSections(info)
	lines := make([]lineJSON, len(info.Lines))
	for i, dl := range info.Lines {
		line := lineJSON{Text: dl.Content, Type: string(dl.Type), Section: sections[i]}
		line.Line, line.Side = dl.Position()
		lines[i] = line
	}
	return lines
}

// addComment validates and stores a new comment.
func (f *fileState) addComment(in commentInput) error {
	c := &comment{ID: newID()}
	if err := setLabel(&c.ReviewComment, in); err != nil {
		return err
	}
	if in.StartLine > 0 {
		if err := f.setLineTarget(&c.ReviewComment, in); err != nil {
			return err
		}
	} else {
		if !f.sections[in.SectionID] {
			return fmt.Errorf("section %q: %w", in.SectionID, errNotFound)
		}
		c.SectionID = in.SectionID
	}
	f.comments = append(f.comments, c)
	return nil
}

// updateComment changes the label, decoration, and body of a comment. The
// target lines never change. setLabel changes nothing when it fails.
func (f *fileState) updateComment(id string, in commentInput) error {
	i := f.commentIndex(id)
	if i < 0 {
		return fmt.Errorf("comment %q: %w", id, errNotFound)
	}
	return setLabel(&f.comments[i].ReviewComment, in)
}

func (f *fileState) deleteComment(id string) error {
	i := f.commentIndex(id)
	if i < 0 {
		return fmt.Errorf("comment %q: %w", id, errNotFound)
	}
	f.comments = append(f.comments[:i], f.comments[i+1:]...)
	return nil
}

// setViewed marks a section as viewed or not, syncing the persisted state.
// Like the TUI, only real sections (not the overview) can be marked.
func (f *fileState) setViewed(sectionID string, viewed bool) error {
	sec := f.Doc.FindSection(sectionID)
	if sec == nil {
		return fmt.Errorf("section %q: %w", sectionID, errNotFound)
	}
	f.viewed[sectionID] = viewed
	if f.Viewed != nil {
		if viewed {
			f.Viewed.MarkViewed(sec)
		} else {
			f.Viewed.UnmarkViewed(sec)
		}
	}
	return nil
}

// buildReview returns the comments in document order, keeping creation order
// within a section (see markdown.NewReviewResult).
func (f *fileState) buildReview() *markdown.ReviewResult {
	comments := make([]markdown.ReviewComment, len(f.comments))
	for i, c := range f.comments {
		comments[i] = c.ReviewComment
	}
	return markdown.NewReviewResult(f.Doc, comments)
}

// search returns the IDs of the sections a search for query shows (see
// markdown.Document.SearchSections).
func (f *fileState) search(query string) []string {
	ids := slices.Sorted(maps.Keys(f.Doc.SearchSections(query)))
	if ids == nil {
		return []string{} // never null in JSON
	}
	return ids
}

func (f *fileState) commentIndex(id string) int {
	for i, c := range f.comments {
		if c.ID == id {
			return i
		}
	}
	return -1
}

// setLineTarget validates a line range and fills the section, side, and the
// quoted source text of a line-level comment. The range must start and end
// on lines shown in the view (on one diff side in diff mode).
func (f *fileState) setLineTarget(c *markdown.ReviewComment, in commentInput) error {
	end := max(in.EndLine, in.StartLine)
	side := ""
	if f.Diff != nil {
		side = in.Side
		if side != diff.SideRight && side != diff.SideLeft {
			return fmt.Errorf("side must be %s or %s in diff mode", diff.SideRight, diff.SideLeft)
		}
	}
	first, last := -1, -1
	for i, l := range f.lines {
		if l.Side != side || l.Line < in.StartLine || l.Line > end {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 0 || f.lines[first].Line != in.StartLine || f.lines[last].Line != end {
		return fmt.Errorf("lines %s are not in the view", markdown.FormatLineRef(in.StartLine, end))
	}
	c.SectionID = f.lines[first].Section
	c.StartLine = in.StartLine
	if end > in.StartLine {
		c.EndLine = end
	}
	c.Side = side
	c.Quote = f.Doc.Quote(f.Diff, c.StartLine, c.EndLine, side)
	return nil
}

// setLabel validates and applies the label, decoration, and body.
func setLabel(c *markdown.ReviewComment, in commentInput) error {
	action, ok := markdown.ParseAction(in.Action)
	if !ok {
		return fmt.Errorf("unknown label %q", in.Action)
	}
	deco, ok := markdown.ParseDecoration(in.Decoration)
	if !ok {
		return fmt.Errorf("unknown decoration %q", in.Decoration)
	}
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return errors.New("comment body is empty")
	}
	c.Action, c.Decoration, c.Body = action, deco, body
	return nil
}
