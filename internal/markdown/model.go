package markdown

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/koh-sh/commd/internal/diff"
)

// OverviewSectionID is the virtual section ID used for file-level comments on the overview/preamble.
const OverviewSectionID = "overview"

// Document is the parsed structure of an entire Markdown file.
type Document struct {
	Title       string     // Leading H1 heading text ("" when the document has none)
	Preamble    string     // Text before the first heading
	Sections    []*Section // Top-level sections
	SourceLines []string   // Raw source lines (for line-level commenting)
}

// Section is a single section in a document, corresponding to one heading.
type Section struct {
	ID        string     // Auto-numbered: "S1", "S1.1", "S2", etc.
	Title     string     // Heading text (without the "## " prefix)
	Level     int        // Heading level (2=##, 3=###, ...)
	Body      string     // Markdown text from heading to next heading
	Children  []*Section // Sub-sections (lower-level headings)
	Parent    *Section   // Parent section (nil for top-level)
	StartLine int        // 1-based line number of heading (0 = not set)
	EndLine   int        // 1-based line number of last body line (0 = not set)
}

// AllSections returns a flat list of all sections in depth-first order.
func (d *Document) AllSections() []*Section {
	var result []*Section
	var walk func(sections []*Section)
	walk = func(sections []*Section) {
		for _, s := range sections {
			result = append(result, s)
			walk(s.Children)
		}
	}
	walk(d.Sections)
	return result
}

// FindSection returns the section with the given ID, or nil if not found.
func (d *Document) FindSection(id string) *Section {
	for _, s := range d.AllSections() {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// LineSections returns the ID of the section each source line belongs to
// (index i is line i+1): the last section whose heading starts at or before
// the line, or OverviewSectionID before the first heading.
func (d *Document) LineSections() []string {
	out := make([]string, len(d.SourceLines))
	sections := d.AllSections() // document order: ascending StartLine
	current := OverviewSectionID
	next := 0
	for i := range out {
		for next < len(sections) && sections[next].StartLine <= i+1 {
			if sections[next].StartLine > 0 {
				current = sections[next].ID
			}
			next++
		}
		out[i] = current
	}
	return out
}

// DiffLineSections returns the ID of the section each diff line belongs to.
// Added and context lines belong to the section of their new-file line.
// Removed lines no longer exist in the new file, so they follow the nearest
// preceding line that does (or the next one at the start of the diff). The
// lines of a section are therefore contiguous in the diff.
func (d *Document) DiffLineSections(info *diff.Info) []string {
	bySource := d.LineSections()
	out := make([]string, len(info.Lines))
	current := ""
	for i, dl := range info.Lines {
		if dl.Type != diff.Removed && len(bySource) > 0 {
			current = bySource[min(max(dl.NewLine, 1), len(bySource))-1]
		}
		out[i] = current
	}
	// Leading removed lines have no preceding new-file line.
	next := OverviewSectionID
	for i := len(out) - 1; i >= 0; i-- {
		if out[i] == "" {
			out[i] = next
		} else {
			next = out[i]
		}
	}
	return out
}

// SearchSections returns the IDs of the sections a search for query shows:
// those whose ID, title or body contain it (ignoring case), with their
// ancestors and descendants. The overview matches by name only. An empty
// query matches nothing.
func (d *Document) SearchSections(query string) map[string]bool {
	shown := make(map[string]bool)
	query = strings.ToLower(query)
	if query == "" {
		return shown
	}
	if strings.Contains("overview", query) { //nolint:gocritic // intentional: match when query is a substring of "overview"
		shown[OverviewSectionID] = true
	}
	var showDescendants func(sections []*Section)
	showDescendants = func(sections []*Section) {
		for _, s := range sections {
			shown[s.ID] = true
			showDescendants(s.Children)
		}
	}
	for _, s := range d.AllSections() {
		if !strings.Contains(strings.ToLower(s.ID+" "+s.Title+" "+s.Body), query) {
			continue
		}
		shown[s.ID] = true
		for p := s.Parent; p != nil; p = p.Parent {
			shown[p.ID] = true
		}
		showDescendants(s.Children)
	}
	return shown
}

// ReviewComment is a review comment on a single section.
type ReviewComment struct {
	SectionID  string     // Target section ID
	Action     ActionType // Comment action type
	Decoration Decoration // Comment decoration (e.g. non-blocking, blocking)
	Body       string     // Comment body text
	StartLine  int        // 1-based start line (0 = section-level comment)
	EndLine    int        // 1-based end line (0 = single line if StartLine > 0)
	Side       string     // "RIGHT" or "LEFT" (for diff comments)
	Quote      []string   // source text of the commented lines (line-level only)
}

// IsRemoved reports whether the comment targets removed (old-side) diff lines.
func (c *ReviewComment) IsRemoved() bool {
	return c.Side == diff.SideLeft
}

// ParseAction returns the ActionType for a label string.
func ParseAction(s string) (ActionType, bool) {
	for _, a := range ActionLabels {
		if string(a) == s {
			return a, true
		}
	}
	return "", false
}

// ParseDecoration returns the Decoration for a label string; "" is
// DecorationNone.
func ParseDecoration(s string) (Decoration, bool) {
	for _, d := range DecorationLabels {
		if string(d) == s {
			return d, true
		}
	}
	return "", false
}

// FileReview pairs a reviewed file with its parsed document and comments.
type FileReview struct {
	Path   string
	Doc    *Document
	Review *ReviewResult
}

// FormatLabel returns the formatted label string for display.
// With decoration: "action (decoration)", without: "action".
func (c *ReviewComment) FormatLabel() string {
	return FormatActionLabel(c.Action, c.Decoration)
}

// FormatLineRef returns a line reference string for display.
// Returns "L10" for single line, "L10-L15" for range, or "" for section-level.
func (c *ReviewComment) FormatLineRef() string {
	return FormatLineRef(c.StartLine, c.EndLine)
}

// FormatLineRef formats a line reference from start and end line numbers.
// Returns "L10" for single line, "L10-L15" for range, or "" if startLine is 0.
func FormatLineRef(startLine, endLine int) string {
	if startLine == 0 {
		return ""
	}
	if endLine == 0 || endLine == startLine {
		return fmt.Sprintf("L%d", startLine)
	}
	return fmt.Sprintf("L%d-L%d", startLine, endLine)
}

// FormatActionLabel formats an action and decoration pair for display.
func FormatActionLabel(action ActionType, deco Decoration) string {
	if deco == DecorationNone {
		return string(action)
	}
	return fmt.Sprintf("%s (%s)", action, deco)
}

// ActionType is the type of review action, based on Conventional Comments labels.
type ActionType string

const (
	ActionSuggestion ActionType = "suggestion"
	ActionIssue      ActionType = "issue"
	ActionQuestion   ActionType = "question"
	ActionNitpick    ActionType = "nitpick"
	ActionTodo       ActionType = "todo"
	ActionThought    ActionType = "thought"
	ActionNote       ActionType = "note"
	ActionPraise     ActionType = "praise"
	ActionChore      ActionType = "chore"
)

// ActionLabels is the ordered list of action labels for Tab cycling.
var ActionLabels = []ActionType{
	ActionSuggestion,
	ActionIssue,
	ActionQuestion,
	ActionNitpick,
	ActionTodo,
	ActionThought,
	ActionNote,
	ActionPraise,
	ActionChore,
}

// DefaultAction is the default action type for new comments.
const DefaultAction = ActionQuestion

// Decoration is the decoration modifier for a Conventional Comment.
type Decoration string

const (
	DecorationNone        Decoration = ""
	DecorationNonBlocking Decoration = "non-blocking"
	DecorationBlocking    Decoration = "blocking"
	DecorationIfMinor     Decoration = "if-minor"
)

// DecorationLabels is the ordered list of decoration labels for cycling.
var DecorationLabels = []Decoration{
	DecorationNone,
	DecorationNonBlocking,
	DecorationBlocking,
	DecorationIfMinor,
}

// ReviewResult holds the entire review output.
type ReviewResult struct {
	Comments []ReviewComment
}

// NewReviewResult returns the comments in document order, which the output
// follows: the overview first, then the sections depth-first, keeping the
// given order within a section. Comments on unknown sections are dropped.
func NewReviewResult(doc *Document, comments []ReviewComment) *ReviewResult {
	rank := map[string]int{OverviewSectionID: 0}
	for i, s := range doc.AllSections() {
		rank[s.ID] = i + 1
	}
	var ordered []ReviewComment
	for _, c := range comments {
		if _, ok := rank[c.SectionID]; ok {
			ordered = append(ordered, c)
		}
	}
	slices.SortStableFunc(ordered, func(a, b ReviewComment) int {
		return cmp.Compare(rank[a.SectionID], rank[b.SectionID])
	})
	return &ReviewResult{Comments: ordered}
}

// Status returns the status of a submitted review: approved when it has no
// comments.
func (r *ReviewResult) Status() Status {
	if len(r.Comments) == 0 {
		return StatusApproved
	}
	return StatusSubmitted
}

// Status is the exit status of a review session (TUI or browser).
type Status string

const (
	StatusSubmitted Status = "submitted"
	StatusApproved  Status = "approved"
	StatusCancelled Status = "cancelled"
)
