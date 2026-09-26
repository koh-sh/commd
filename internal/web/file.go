package web

import (
	"errors"
	"fmt"
	"strings"

	"github.com/koh-sh/commd/internal/diff"
	"github.com/koh-sh/commd/internal/markdown"
)

// File is one document reviewed in the browser.
type File struct {
	Path   string
	Doc    *markdown.Document
	Diff   *diff.Info            // nil: review the full source instead of a diff
	Viewed *markdown.ViewedState // nil: viewed marks live only for the session
}

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

// displayLine is one line of the source (or diff) view.
type displayLine struct {
	Text      string
	Line      int    // file line number: new file, or old file for removed lines
	Side      string // diff.SideRight / diff.SideLeft; "" outside diff mode
	Type      diff.LineType
	SectionID string // section the line belongs to
}

// fileState is the review state of the file under review.
type fileState struct {
	File
	lines    []displayLine
	sections map[string]bool // valid section-level comment targets
	comments []*comment      // in creation order
	viewed   map[string]bool // section ID -> viewed
	rendered []sectionJSON   // sections with HTML, built on first use (the source never changes)
}

func newFileState(f File) *fileState {
	fs := &fileState{
		File:     f,
		sections: make(map[string]bool),
		viewed:   make(map[string]bool),
	}
	if f.Diff != nil {
		fs.lines = diffLines(f.Doc, f.Diff)
	} else {
		fs.lines = sourceLines(f.Doc)
	}
	// Like the TUI, the overview is an entry only when there is a preamble.
	if f.Doc.Preamble != "" {
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
func sourceLines(doc *markdown.Document) []displayLine {
	lines := make([]displayLine, len(doc.SourceLines))
	for i, text := range doc.SourceLines {
		lines[i] = displayLine{Text: text, Line: i + 1, SectionID: doc.SectionIDAtLine(i + 1)}
	}
	return lines
}

// diffLines returns the diff lines, each under a section. Added and context
// lines belong to the section of their new-file line. Removed lines no longer
// exist in the new file, so they follow the nearest preceding line that does
// (or the next one at the start of the diff).
func diffLines(doc *markdown.Document, info *diff.Info) []displayLine {
	lines := make([]displayLine, len(info.Lines))
	current := ""
	for i, dl := range info.Lines {
		line := displayLine{Text: dl.Content, Type: dl.Type}
		if dl.Type == diff.Removed {
			line.Line, line.Side = dl.OldLine, diff.SideLeft
		} else {
			line.Line, line.Side = dl.NewLine, diff.SideRight
			current = doc.SectionIDAtLine(dl.NewLine)
		}
		line.SectionID = current
		lines[i] = line
	}
	// Leading removed lines have no preceding new-file line.
	next := markdown.OverviewSectionID
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i].SectionID == "" {
			lines[i].SectionID = next
		} else if lines[i].Side == diff.SideRight {
			next = lines[i].SectionID
		}
	}
	return lines
}

// addComment validates and stores a new comment.
func (f *fileState) addComment(in commentInput) (*comment, error) {
	c := &comment{ID: newID()}
	if err := setLabel(&c.ReviewComment, in); err != nil {
		return nil, err
	}
	if in.StartLine > 0 {
		if err := f.setLineTarget(&c.ReviewComment, in); err != nil {
			return nil, err
		}
	} else {
		if !f.sections[in.SectionID] {
			return nil, fmt.Errorf("section %q: %w", in.SectionID, errNotFound)
		}
		c.SectionID = in.SectionID
	}
	f.comments = append(f.comments, c)
	return c, nil
}

// updateComment changes the label, decoration, and body of a comment. The
// target lines never change.
func (f *fileState) updateComment(id string, in commentInput) (*comment, error) {
	i := f.commentIndex(id)
	if i < 0 {
		return nil, fmt.Errorf("comment %q: %w", id, errNotFound)
	}
	updated := *f.comments[i]
	if err := setLabel(&updated.ReviewComment, in); err != nil {
		return nil, err
	}
	*f.comments[i] = updated
	return &updated, nil
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

// buildReview returns the comments in document order (overview first, then
// sections depth-first), keeping creation order within a section, like the
// TUI.
func (f *fileState) buildReview() *markdown.ReviewResult {
	result := &markdown.ReviewResult{}
	order := []string{markdown.OverviewSectionID}
	for _, sec := range f.Doc.AllSections() {
		order = append(order, sec.ID)
	}
	for _, id := range order {
		for _, c := range f.comments {
			if c.SectionID == id {
				result.Comments = append(result.Comments, c.ReviewComment)
			}
		}
	}
	return result
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
	var quote []string
	for i, l := range f.lines {
		if l.Side != side || l.Line < in.StartLine || l.Line > end {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
		quote = append(quote, l.Text)
	}
	if first < 0 || f.lines[first].Line != in.StartLine || f.lines[last].Line != end {
		return fmt.Errorf("lines %s are not in the view", markdown.FormatLineRef(in.StartLine, end))
	}
	c.SectionID = f.lines[first].SectionID
	c.StartLine = in.StartLine
	if end > in.StartLine {
		c.EndLine = end
	}
	c.Side = side
	c.Quote = quote
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
