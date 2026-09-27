package web

import (
	"github.com/koh-sh/commd/internal/diff"
	"github.com/koh-sh/commd/internal/markdown"
)

// The JSON types below are the contract with the page (static/*.js).

// stateJSON is the whole session state the page renders from. The page
// holds no review state of its own: every change goes through the API and
// the page re-renders from the response.
type stateJSON struct {
	Phase string    `json:"phase"` // phasePick, phaseReview or phaseDone
	Pick  []string  `json:"pick,omitempty"`
	File  *fileJSON `json:"file,omitempty"`
	// Seq identifies the file under review; requests echo it back.
	Seq int `json:"seq"`
	// MultiFile is set when several files are reviewed: dialogs say
	// finish/skip this file.
	MultiFile bool `json:"multiFile"`
	// Theme is the initial color theme (--theme).
	Theme        string                `json:"theme"`
	Labels       []markdown.ActionType `json:"labels"`
	Decorations  []markdown.Decoration `json:"decorations"`
	DefaultLabel markdown.ActionType   `json:"defaultLabel"`
	// OverviewID is the section ID of the overview, which is not a heading.
	OverviewID string `json:"overviewId"`
	// Reload reports a reload, set only in the reply to one.
	Reload *reloadJSON `json:"reload,omitempty"`
}

// reloadJSON is the outcome of rereading the file under review.
type reloadJSON struct {
	Message string `json:"message"` // worded as in the TUI
	Changed bool   `json:"changed"` // the page now shows the new content
	Failed  bool   `json:"failed"`  // the file could not be read
	// Sections maps the section IDs before the reload to the new ones, so R
	// stays on the selected section (see markdown.ReloadResult).
	Sections map[string]string `json:"sections,omitempty"`
}

type fileJSON struct {
	Path     string        `json:"path"`
	Title    string        `json:"title"`
	Diff     bool          `json:"diff"`
	Sections []sectionJSON `json:"sections"`
	Lines    []lineJSON    `json:"lines"`
	Comments []commentJSON `json:"comments"`
	Viewed   []string      `json:"viewed"`
	// The questions of the submit and quit dialogs, worded as in the TUI.
	ConfirmSubmit string `json:"confirmSubmit"`
	ConfirmQuit   string `json:"confirmQuit"`
}

type sectionJSON struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Depth int    `json:"depth"`
	HTML  string `json:"html"` // rendered Markdown; raw HTML in the source is escaped
}

type lineJSON struct {
	Text    string `json:"text"`
	Line    int    `json:"line"`
	Side    string `json:"side,omitempty"`
	Type    string `json:"type,omitempty"` // "+", "-", " " in diff mode
	Section string `json:"section"`        // section the line belongs to
}

// commentJSON is a comment with its display strings, formatted here so the
// page words them as the TUI and the review output do.
type commentJSON struct {
	*markdown.ReviewComment
	Label     string `json:"label"`               // "action (decoration)"
	Ref       string `json:"ref,omitempty"`       // "L10-L15"
	OutputRef string `json:"outputRef,omitempty"` // Ref as the review output shows it
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

// state returns the session state for the browser.
func (s *session) state() stateJSON {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := stateJSON{
		Phase:        s.phase,
		Seq:          s.seq,
		MultiFile:    s.multiFile,
		Theme:        s.theme,
		Labels:       markdown.ActionLabels,
		Decorations:  markdown.DecorationLabels,
		DefaultLabel: markdown.DefaultAction,
		OverviewID:   markdown.OverviewSectionID,
	}
	switch s.phase {
	case phasePick:
		out.Pick = s.review.Pick
	case phaseReview:
		f := s.current.toJSON(s.multiFile)
		out.File = &f
	}
	return out
}

func (f *fileState) toJSON(multiFile bool) fileJSON {
	out := fileJSON{
		Path:          f.Path,
		Title:         f.Doc.Title,
		Diff:          f.Diff != nil,
		Sections:      f.rendered,
		Lines:         f.lines,
		Comments:      make([]commentJSON, len(f.Comments())),
		Viewed:        []string{},
		ConfirmSubmit: f.ConfirmSubmitMessage(multiFile),
		ConfirmQuit:   f.ConfirmQuitMessage(multiFile),
	}
	for i, c := range f.Comments() {
		out.Comments[i] = commentJSON{ReviewComment: c, Label: c.FormatLabel(), Ref: c.FormatLineRef(), OutputRef: c.DisplayLineRef()}
	}
	for _, sec := range out.Sections {
		if f.IsViewed(sec.ID) {
			out.Viewed = append(out.Viewed, sec.ID)
		}
	}
	return out
}
