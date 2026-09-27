package markdown

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/koh-sh/commd/internal/diff"
)

// ErrNotFound marks lookups of unknown comments or sections.
var ErrNotFound = errors.New("not found")

// ReviewState is the review of one file in progress: its comments and viewed
// marks. The TUI and the browser both keep their review here, so the rules
// for saving comments and marking sections are the same in both.
type ReviewState struct {
	File
	comments []*ReviewComment // in creation order
	viewed   map[string]bool  // section ID -> viewed
	lastID   int
}

// NewReviewState starts the review of f, restoring the viewed marks from
// f.Viewed. Marking sections updates f.Viewed in place, for the caller to
// persist.
func NewReviewState(f File) *ReviewState {
	r := &ReviewState{File: f, viewed: make(map[string]bool)}
	if f.Viewed != nil {
		for _, s := range f.Doc.AllSections() {
			if f.Viewed.IsSectionViewed(s) {
				r.viewed[s.ID] = true
			}
		}
	}
	return r
}

// Comments returns every comment in creation order.
func (r *ReviewState) Comments() []*ReviewComment {
	return r.comments
}

// SectionComments returns the comments on a section in creation order.
func (r *ReviewState) SectionComments(sectionID string) []*ReviewComment {
	var out []*ReviewComment
	for _, c := range r.comments {
		if c.SectionID == sectionID {
			out = append(out, c)
		}
	}
	return out
}

// SaveComment stores c: a new comment when c.ID is empty, otherwise a change
// to the label, decoration and body of the comment with that ID (its target
// never changes). An empty body drops a new comment and deletes an existing
// one. A new comment is validated and gets its ID, and a line comment also
// its section and quoted source text.
func (r *ReviewState) SaveComment(c ReviewComment) error {
	c.Body = strings.TrimSpace(c.Body)
	if c.ID != "" {
		existing := r.find(c.ID)
		if existing == nil {
			return fmt.Errorf("comment %q: %w", c.ID, ErrNotFound)
		}
		if c.Body == "" {
			return r.DeleteComment(c.ID)
		}
		if err := validateLabel(c); err != nil {
			return err
		}
		existing.Action, existing.Decoration, existing.Body = c.Action, c.Decoration, c.Body
		return nil
	}
	if c.Body == "" {
		return nil
	}
	if err := validateLabel(c); err != nil {
		return err
	}
	if err := r.setTarget(&c); err != nil {
		return err
	}
	r.lastID++
	c.ID = "c" + strconv.Itoa(r.lastID)
	r.comments = append(r.comments, &c)
	return nil
}

// DeleteComment removes the comment with the given ID.
func (r *ReviewState) DeleteComment(id string) error {
	i := slices.IndexFunc(r.comments, func(c *ReviewComment) bool { return c.ID == id })
	if i < 0 {
		return fmt.Errorf("comment %q: %w", id, ErrNotFound)
	}
	r.comments = slices.Delete(r.comments, i, i+1)
	return nil
}

func (r *ReviewState) find(id string) *ReviewComment {
	for _, c := range r.comments {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func validateLabel(c ReviewComment) error {
	if !slices.Contains(ActionLabels, c.Action) {
		return fmt.Errorf("unknown label %q", c.Action)
	}
	if !slices.Contains(DecorationLabels, c.Decoration) {
		return fmt.Errorf("unknown decoration %q", c.Decoration)
	}
	return nil
}

// setTarget validates the target of a new comment: a section (or the
// overview), or a line range. A line range must start and end on lines shown
// in the review (on one side in a diff); its section is that of its first
// line, and it quotes the lines, since line numbers alone go stale once the
// file is edited and removed diff lines exist nowhere else.
func (r *ReviewState) setTarget(c *ReviewComment) error {
	c.Quote = nil
	if c.StartLine <= 0 {
		c.StartLine, c.EndLine, c.Side = 0, 0, ""
		if r.hasSection(c.SectionID) {
			return nil
		}
		return fmt.Errorf("section %q: %w", c.SectionID, ErrNotFound)
	}
	end := max(c.EndLine, c.StartLine)
	section, err := r.lineSection(c.StartLine, end, c.Side)
	if err != nil {
		return err
	}
	c.SectionID = section
	if r.Diff == nil {
		c.Side = ""
	}
	c.EndLine = 0
	if end > c.StartLine {
		c.EndLine = end
	}
	c.Quote = r.Doc.Quote(r.Diff, c.StartLine, c.EndLine, c.Side)
	return nil
}

func (r *ReviewState) hasSection(id string) bool {
	if id == OverviewSectionID {
		return r.Doc.HasOverview()
	}
	return r.Doc.FindSection(id) != nil
}

// lineSection returns the section of line start after checking that
// start..end are lines of the review: of the source, or of the diff on side.
func (r *ReviewState) lineSection(start, end int, side string) (string, error) {
	notShown := fmt.Errorf("lines %s are not in the view", FormatLineRef(start, end))
	if r.Diff == nil {
		if end > len(r.Doc.SourceLines) {
			return "", notShown
		}
		return r.Doc.LineSections()[start-1], nil
	}
	if side != diff.SideRight && side != diff.SideLeft {
		return "", fmt.Errorf("side must be %s or %s in diff mode", diff.SideRight, diff.SideLeft)
	}
	first, last := -1, -1
	for i, dl := range r.Diff.Lines {
		line, lineSide := dl.Position()
		if lineSide != side || line < start || line > end {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 0 {
		return "", notShown
	}
	if line, _ := r.Diff.Lines[first].Position(); line != start {
		return "", notShown
	}
	if line, _ := r.Diff.Lines[last].Position(); line != end {
		return "", notShown
	}
	return r.Doc.DiffLineSections(r.Diff)[first], nil
}

// SetViewed marks a section as viewed or not, updating File.Viewed when it
// is set. Only real sections can be marked, not the overview.
func (r *ReviewState) SetViewed(sectionID string, viewed bool) error {
	s := r.Doc.FindSection(sectionID)
	if s == nil {
		return fmt.Errorf("section %q: %w", sectionID, ErrNotFound)
	}
	r.viewed[sectionID] = viewed
	if r.Viewed != nil {
		if viewed {
			r.Viewed.MarkViewed(s)
		} else {
			r.Viewed.UnmarkViewed(s)
		}
	}
	return nil
}

// IsViewed reports whether a section is marked as viewed.
func (r *ReviewState) IsViewed(sectionID string) bool {
	return r.viewed[sectionID]
}

// ViewedCount returns the number of sections marked as viewed.
func (r *ReviewState) ViewedCount() int {
	n := 0
	for _, v := range r.viewed {
		if v {
			n++
		}
	}
	return n
}

// Result returns the comments in document order, which the output follows:
// the overview first, then the sections depth-first, keeping creation order
// within a section.
func (r *ReviewState) Result() *ReviewResult {
	rank := map[string]int{OverviewSectionID: 0}
	for i, s := range r.Doc.AllSections() {
		rank[s.ID] = i + 1
	}
	comments := make([]ReviewComment, len(r.comments))
	for i, c := range r.comments {
		comments[i] = *c
	}
	slices.SortStableFunc(comments, func(a, b ReviewComment) int {
		return cmp.Compare(rank[a.SectionID], rank[b.SectionID])
	})
	return &ReviewResult{Comments: comments}
}

// ConfirmSubmitMessage is the question the submit dialog asks.
func (r *ReviewState) ConfirmSubmitMessage(multiFile bool) string {
	if multiFile {
		return fmt.Sprintf("Finish reviewing this file? (%d comments)", len(r.comments))
	}
	return fmt.Sprintf("Submit review? (%d comments)", len(r.comments))
}

// ConfirmQuitMessage is the question the quit dialog asks.
func (r *ReviewState) ConfirmQuitMessage(multiFile bool) string {
	switch {
	case multiFile:
		return "Skip this file?"
	case len(r.comments) > 0:
		return "You have review comments.\n\nQuit without submitting?"
	default:
		return "Quit review?"
	}
}
