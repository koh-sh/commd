package markdown

import "fmt"

// Document is the parsed structure of an entire Markdown file.
type Document struct {
	Title    string     // H1 heading text (or filename if no H1)
	Preamble string     // Text before the first heading
	Sections []*Section // Top-level sections
}

// Section is a single section in a document, corresponding to one heading.
type Section struct {
	ID       string     // Auto-numbered: "S1", "S1.1", "S2", etc.
	Title    string     // Heading text (without the "## " prefix)
	Level    int        // Heading level (2=##, 3=###, ...)
	Body     string     // Markdown text from heading to next heading
	Children []*Section // Sub-sections (lower-level headings)
	Parent   *Section   // Parent section (nil for top-level)
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

// ReviewComment is a review comment on a single section.
type ReviewComment struct {
	SectionID  string     // Target section ID
	Action     ActionType // Comment action type
	Decoration Decoration // Comment decoration (e.g. non-blocking, blocking)
	Body       string     // Comment body text
}

// FormatLabel returns the formatted label string for display.
// With decoration: "action (decoration)", without: "action".
func (c *ReviewComment) FormatLabel() string {
	return FormatActionLabel(c.Action, c.Decoration)
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

// Status is the exit status of a TUI review session.
type Status string

const (
	StatusSubmitted Status = "submitted"
	StatusApproved  Status = "approved"
	StatusCancelled Status = "cancelled"
)
