package web

import (
	"bytes"
	"fmt"
	"net/url"
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
	// Start and End are the source lines the section view of the raw source
	// shows, as in the TUI (the overview spans the lines before the first
	// heading). Diff lines are shown by their section instead.
	Start int    `json:"start"`
	End   int    `json:"end"`
	HTML  string `json:"html"` // rendered Markdown; raw HTML in the source is escaped
	Text  string `json:"text"` // section body for search ("" for the overview, which the TUI matches by name only)
}

type lineJSON struct {
	Text    string `json:"text"`
	Line    int    `json:"line"`
	Side    string `json:"side,omitempty"`
	Type    string `json:"type,omitempty"` // "+", "-", " " in diff mode
	Section string `json:"section"`
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

// assetURL returns the imageURL function for a file under review: relative
// image paths are served from the file's directory by /assets/ (see
// serveAsset), authenticated by the token in the query since images cannot
// send headers. Other destinations (URLs, absolute paths) are kept.
func assetURL(seq int, token string) func(string) string {
	return func(dest string) string {
		u, err := url.Parse(dest)
		if err != nil || u.Scheme != "" || u.Host != "" || u.Path == "" || strings.HasPrefix(u.Path, "/") {
			return dest
		}
		return fmt.Sprintf("/assets/%d/%s?t=%s", seq, u.EscapedPath(), url.QueryEscape(token))
	}
}

// state returns the session state for the browser.
func (s *session) state(theme, token string) stateJSON {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := stateJSON{
		Phase:        s.phase,
		Seq:          s.seq,
		Multi:        s.multi,
		Theme:        theme,
		Labels:       make([]string, len(markdown.ActionLabels)),
		Decorations:  make([]string, len(markdown.DecorationLabels)),
		DefaultLabel: string(markdown.DefaultAction),
	}
	for i, a := range markdown.ActionLabels {
		out.Labels[i] = string(a)
	}
	for i, d := range markdown.DecorationLabels {
		out.Decorations[i] = string(d)
	}
	switch s.phase {
	case phasePick:
		out.Pick = s.review.Pick
	case phaseReview:
		f := s.current.toJSON(assetURL(s.seq, token))
		out.File = &f
	}
	return out
}

func (f *fileState) toJSON(imageURL func(string) string) fileJSON {
	out := fileJSON{
		Path:     f.Path,
		Title:    f.Doc.Title,
		Diff:     f.Diff != nil,
		Sections: f.renderedSections(imageURL),
		Lines:    make([]lineJSON, len(f.lines)),
		Comments: f.commentsJSON(),
		Viewed:   []string{},
	}
	for i, l := range f.lines {
		out.Lines[i] = lineJSON{Text: l.Text, Line: l.Line, Side: l.Side, Section: l.SectionID}
		if l.Type != 0 {
			out.Lines[i].Type = string(l.Type)
		}
	}
	for _, sec := range out.Sections {
		if f.viewed[sec.ID] {
			out.Viewed = append(out.Viewed, sec.ID)
		}
	}
	return out
}

// sectionsJSON returns the sections in display order, the overview first.
// Each section is rendered from its own source lines, so headings keep their
// inline formatting and setext style.
func (f *fileState) sectionsJSON(imageURL func(string) string) []sectionJSON {
	out := []sectionJSON{} // never null in JSON: the page reads .length
	doc := f.Doc
	all := doc.AllSections()
	if f.sections[markdown.OverviewSectionID] {
		end := len(doc.SourceLines)
		if len(all) > 0 {
			end = all[0].StartLine - 1
		}
		md := sourceRange(doc, 1, end)
		out = append(out, sectionJSON{
			ID:    markdown.OverviewSectionID,
			Title: "Overview",
			Start: 1,
			End:   end,
			HTML:  renderHTML(md, imageURL),
		})
	}
	var walk func(sections []*markdown.Section, depth int)
	walk = func(sections []*markdown.Section, depth int) {
		for _, sec := range sections {
			md := sourceRange(doc, sec.StartLine, sec.EndLine)
			out = append(out, sectionJSON{
				ID:    sec.ID,
				Title: sec.Title,
				Depth: depth,
				Start: sec.StartLine,
				End:   sec.EndLine,
				HTML:  renderHTML(md, imageURL),
				Text:  sec.Body, // what the TUI search matches, with the ID and title
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

// renderedSections returns sectionsJSON, rendering it only once per file.
func (f *fileState) renderedSections(imageURL func(string) string) []sectionJSON {
	if f.rendered == nil {
		f.rendered = f.sectionsJSON(imageURL)
	}
	return f.rendered
}
