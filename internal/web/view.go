package web

import (
	"bytes"
	"strings"

	"github.com/koh-sh/commd/internal/markdown"
	"github.com/koh-sh/commd/internal/mermaid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
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
	Seq   int  `json:"seq"`
	Multi bool `json:"multi"`
	// Theme is the initial color theme (--theme).
	Theme        string   `json:"theme"`
	Labels       []string `json:"labels"`
	Decorations  []string `json:"decorations"`
	DefaultLabel string   `json:"defaultLabel"`
	// OverviewID is the section ID of the overview, which is not a heading.
	OverviewID string `json:"overviewId"`
}

type fileJSON struct {
	Path     string        `json:"path"`
	Title    string        `json:"title"`
	Diff     bool          `json:"diff"`
	Sections []sectionJSON `json:"sections"`
	Lines    []lineJSON    `json:"lines"`
	Comments []commentJSON `json:"comments"`
	Viewed   []string      `json:"viewed"`
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

type commentJSON struct {
	ID         string   `json:"id"`
	SectionID  string   `json:"sectionId"`
	Action     string   `json:"action"`
	Decoration string   `json:"decoration"`
	Body       string   `json:"body"`
	StartLine  int      `json:"startLine,omitempty"`
	EndLine    int      `json:"endLine,omitempty"`
	Side       string   `json:"side,omitempty"`
	Quote      []string `json:"quote,omitempty"`
}

// renderer converts Markdown to HTML. goldmark escapes raw HTML and drops
// dangerous link URLs unless html.WithUnsafe is set, so the output is safe
// to insert into the page.
var renderer = goldmark.New(goldmark.WithExtensions(extension.GFM))

// renderHTML renders Markdown, passing every image destination through
// imageURL (nil keeps them as they are).
func renderHTML(md string, imageURL func(dest string) string) string {
	src := []byte(mermaid.RenderBlocks(md))
	doc := renderer.Parser().Parse(text.NewReader(src))
	if imageURL != nil {
		_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if img, ok := n.(*ast.Image); ok && entering {
				img.Destination = []byte(imageURL(string(img.Destination)))
			}
			return ast.WalkContinue, nil
		})
	}
	var buf bytes.Buffer
	if err := renderer.Renderer().Render(&buf, src, doc); err != nil {
		// goldmark only fails on writer errors, which bytes.Buffer never returns.
		return ""
	}
	return buf.String()
}

// labelsJSON and decorationsJSON are the comment labels the editor cycles
// through, in the TUI's order.
var (
	labelsJSON      = labelStrings(markdown.ActionLabels)
	decorationsJSON = labelStrings(markdown.DecorationLabels)
)

func labelStrings[T ~string](labels []T) []string {
	out := make([]string, len(labels))
	for i, l := range labels {
		out[i] = string(l)
	}
	return out
}

// state returns the session state for the browser.
func (s *session) state() stateJSON {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := stateJSON{
		Phase:        s.phase,
		Seq:          s.seq,
		Multi:        s.multi,
		Theme:        s.theme,
		Labels:       labelsJSON,
		Decorations:  decorationsJSON,
		DefaultLabel: string(markdown.DefaultAction),
		OverviewID:   markdown.OverviewSectionID,
	}
	switch s.phase {
	case phasePick:
		out.Pick = s.review.Pick
	case phaseReview:
		f := s.current.toJSON()
		out.File = &f
	}
	return out
}

func (f *fileState) toJSON() fileJSON {
	out := fileJSON{
		Path:     f.Path,
		Title:    f.Doc.Title,
		Diff:     f.Diff != nil,
		Sections: f.rendered,
		Lines:    f.lines,
		Comments: f.commentsJSON(),
		Viewed:   []string{},
	}
	for _, sec := range out.Sections {
		if f.viewed[sec.ID] {
			out.Viewed = append(out.Viewed, sec.ID)
		}
	}
	return out
}

// renderSections returns the sections in display order, the overview first.
// Each section is rendered from its own source lines, so headings keep their
// inline formatting and setext style.
func renderSections(doc *markdown.Document, imageURL func(string) string) []sectionJSON {
	out := []sectionJSON{} // never null in JSON: the page reads .length
	all := doc.AllSections()
	if doc.HasOverview() {
		// The overview is the preamble: the lines before the first heading.
		end := len(doc.SourceLines)
		if len(all) > 0 {
			end = all[0].StartLine - 1
		}
		out = append(out, sectionJSON{
			ID:    markdown.OverviewSectionID,
			Title: markdown.OverviewTitle,
			HTML:  renderHTML(sourceRange(doc, 1, end), imageURL),
		})
	}
	var walk func(sections []*markdown.Section, depth int)
	walk = func(sections []*markdown.Section, depth int) {
		for _, sec := range sections {
			out = append(out, sectionJSON{
				ID:    sec.ID,
				Title: sec.Title,
				Depth: depth,
				HTML:  renderHTML(sourceRange(doc, sec.StartLine, sec.EndLine), imageURL),
			})
			walk(sec.Children, depth+1)
		}
	}
	walk(doc.Sections, 0)
	return out
}

// sourceRange joins the 1-based source lines start..end, clamped to the file.
func sourceRange(doc *markdown.Document, start, end int) string {
	start = max(start, 1)
	end = min(end, len(doc.SourceLines))
	if start > end {
		return ""
	}
	return strings.Join(doc.SourceLines[start-1:end], "\n")
}

func (f *fileState) commentsJSON() []commentJSON {
	out := make([]commentJSON, len(f.comments))
	for i, c := range f.comments {
		out[i] = c.toJSON()
	}
	return out
}

func (c *comment) toJSON() commentJSON {
	return commentJSON{
		ID:         c.ID,
		SectionID:  c.SectionID,
		Action:     string(c.Action),
		Decoration: string(c.Decoration),
		Body:       c.Body,
		StartLine:  c.StartLine,
		EndLine:    c.EndLine,
		Side:       c.Side,
		Quote:      c.Quote,
	}
}
