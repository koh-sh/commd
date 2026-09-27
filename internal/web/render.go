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
