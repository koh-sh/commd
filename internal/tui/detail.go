package tui

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	glamourStyles "charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/koh-sh/commd/internal/markdown"
	"github.com/mattn/go-runewidth"
)

// glamourHorizontalOverhead accounts for glamour's default left/right
// margin (2 each) and padding (2 each) = 8 total.
const glamourHorizontalOverhead = 8

// commentBoxOverhead accounts for the comment box's rounded border (1 each)
// and padding (1 each) = 4 total. Used when computing the total Width passed
// to lipgloss v2's Width(N), which now includes border + padding.
const commentBoxOverhead = 4

type sectionOffset struct {
	line      int
	sectionID string
}

// DetailPane manages the right pane that shows section details.
type DetailPane struct {
	viewport       viewport.Model
	renderer       *glamour.TermRenderer
	theme          string
	sectionOffsets []sectionOffset
}

// customStyle returns a glamour style with red background removed from
// Chroma error tokens. Japanese text can be misidentified as error tokens
// by Chroma, causing distracting red backgrounds.
func customStyle(theme string) ansi.StyleConfig {
	var style ansi.StyleConfig
	if theme == ThemeLight {
		style = glamourStyles.LightStyleConfig
	} else {
		style = glamourStyles.DarkStyleConfig
	}
	if style.CodeBlock.Chroma != nil {
		chroma := *style.CodeBlock.Chroma
		chroma.Error = ansi.StylePrimitive{
			Color: chroma.Error.Color,
		}
		style.CodeBlock.Chroma = &chroma
	}
	return style
}

// NewDetailPane creates a new DetailPane.
func NewDetailPane(width, height int, theme string) *DetailPane {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	// Intentionally ignore error: renderContent falls back to plain text when renderer is nil.
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStyles(customStyle(theme)),
		glamour.WithWordWrap(0),
	)

	return &DetailPane{
		viewport: vp,
		renderer: renderer,
		theme:    theme,
	}
}

// SetSize updates the pane size. It does not re-render current content;
// call ShowSection or ShowOverview after resizing to refresh the viewport.
func (d *DetailPane) SetSize(width, height int) {
	if width == d.viewport.Width() && height == d.viewport.Height() {
		return
	}
	d.viewport.SetWidth(width)
	d.viewport.SetHeight(height)
}

// ShowSection renders and displays a section's content.
func (d *DetailPane) ShowSection(section *markdown.Section, comments []*markdown.ReviewComment) {
	d.sectionOffsets = nil
	var md strings.Builder
	fmt.Fprintf(&md, "## %s: %s\n\n", section.ID, section.Title)
	if section.Body != "" {
		md.WriteString(section.Body + "\n")
	}

	rendered := d.renderMarkdown(md.String())
	d.setViewportContent(d.appendCommentBoxes(rendered, comments))
}

// writeDocHeader writes the document title and preamble as Markdown to the builder.
func writeDocHeader(sb *strings.Builder, doc *markdown.Document) {
	if doc.Title != "" {
		fmt.Fprintf(sb, "# %s\n\n", doc.Title)
	}
	if doc.Preamble != "" {
		sb.WriteString(doc.Preamble + "\n")
	}
}

// ShowOverview renders and displays the document overview (preamble).
func (d *DetailPane) ShowOverview(doc *markdown.Document, comments []*markdown.ReviewComment) {
	d.sectionOffsets = nil
	var content strings.Builder
	writeDocHeader(&content, doc)

	rendered := d.renderMarkdown(content.String())
	d.setViewportContent(d.appendCommentBoxes(rendered, comments))
}

// appendCommentBoxes appends rendered comment boxes to the given content.
func (d *DetailPane) appendCommentBoxes(rendered string, comments []*markdown.ReviewComment) string {
	if len(comments) == 0 {
		return rendered
	}
	var sb strings.Builder
	sb.WriteString(rendered)
	for i, c := range comments {
		sb.WriteString(d.renderCommentBox(c, i, len(comments)))
		sb.WriteString("\n")
	}
	return sb.String()
}

// ShowAll renders the entire document in a single view.
func (d *DetailPane) ShowAll(doc *markdown.Document, getComments func(string) []*markdown.ReviewComment) {
	var md strings.Builder
	writeDocHeader(&md, doc)

	var sectionOrder []string
	var walkBuild func([]*markdown.Section)
	walkBuild = func(sections []*markdown.Section) {
		for _, section := range sections {
			md.WriteString("\n")
			heading := strings.Repeat("#", section.Level)
			fmt.Fprintf(&md, "%s %s: %s\n\n", heading, section.ID, section.Title)
			if section.Body != "" {
				md.WriteString(section.Body + "\n")
			}
			sectionOrder = append(sectionOrder, section.ID)
			walkBuild(section.Children)
		}
	}
	walkBuild(doc.Sections)

	rendered := d.renderMarkdown(md.String())
	d.buildSectionOffsets(rendered)

	if d.hasAnyComments(sectionOrder, getComments) {
		rendered = d.insertCommentBoxes(rendered, sectionOrder, getComments)
		d.buildSectionOffsets(rendered)
	}
	d.setViewportContent(rendered)
}

// renderMarkdown renders Markdown to a styled string without setting viewport content.
func (d *DetailPane) renderMarkdown(md string) string {
	wrapWidth := d.viewport.Width() - glamourHorizontalOverhead
	md = renderMermaidBlocks(md)
	md = wrapProse(md, wrapWidth)
	if d.renderer != nil {
		if r, err := d.renderer.Render(md); err == nil {
			return r
		}
	}
	return md
}

// setViewportContent sets the viewport content and resets scroll position.
func (d *DetailPane) setViewportContent(content string) {
	d.viewport.SetContent(content)
	d.viewport.SetXOffset(0)
	d.viewport.GotoTop()
}

// wrapProse wraps prose lines in Markdown to the given width using Markdown
// hard breaks (two trailing spaces + newline). Code blocks (fenced with
// ``` or ~~~) are preserved as-is. glamour is configured with WordWrap(0) so
// it won't re-join these hard-broken lines or wrap code blocks.
func wrapProse(md string, width int) string {
	if width <= 0 {
		return md
	}
	lines := strings.Split(md, "\n")
	var result []string
	var fenceMarker string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if fenceMarker == "" &&
			(strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			fenceMarker = trimmed[:3]
			result = append(result, line)
			continue
		}
		if fenceMarker != "" && strings.HasPrefix(trimmed, fenceMarker) {
			fenceMarker = ""
			result = append(result, line)
			continue
		}
		if fenceMarker != "" || runewidth.StringWidth(line) <= width {
			result = append(result, line)
			continue
		}
		wrapped := softWrapLine(line, width)
		needsHardWrap := false
		for _, w := range wrapped {
			// 末尾の "  " (hard break marker) を除いた幅で判定
			if runewidth.StringWidth(strings.TrimRight(w, " ")) > width {
				needsHardWrap = true
				break
			}
		}
		if needsHardWrap {
			result = append(result, hardWrapCJK(line, width)...)
		} else {
			result = append(result, wrapped...)
		}
	}
	return strings.Join(result, "\n")
}

// softWrapLine breaks a long line at word boundaries, appending two trailing
// spaces to each continuation line so Markdown renders them as hard breaks.
// Preserves leading whitespace indent.
func softWrapLine(line string, width int) []string {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]

	words := strings.Fields(trimmed)
	if len(words) == 0 {
		return []string{line}
	}

	indentWidth := runewidth.StringWidth(indent)
	effectiveWidth := width - indentWidth
	if effectiveWidth <= 0 {
		effectiveWidth = 1
	}

	var lines []string
	current := words[0]
	currentWidth := runewidth.StringWidth(current)
	for _, word := range words[1:] {
		ww := runewidth.StringWidth(word)
		if currentWidth+1+ww > effectiveWidth {
			lines = append(lines, indent+current+"  ")
			current = word
			currentWidth = ww
		} else {
			current += " " + word
			currentWidth += 1 + ww
		}
	}
	lines = append(lines, indent+current)
	return lines
}

// hardWrapCJK breaks a long line at character boundaries, appending two
// trailing spaces to each continuation line so Markdown renders them as
// hard breaks (<br>). Preserves leading whitespace indent.
func hardWrapCJK(line string, width int) []string {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	indentWidth := runewidth.StringWidth(indent)
	effectiveWidth := width - indentWidth
	if effectiveWidth <= 0 {
		effectiveWidth = 1
	}

	var lines []string
	var current strings.Builder
	currentWidth := 0

	for _, r := range trimmed {
		rw := runewidth.RuneWidth(r)
		if currentWidth+rw > effectiveWidth && currentWidth > 0 {
			lines = append(lines, indent+current.String()+"  ")
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += rw
	}
	if current.Len() > 0 {
		lines = append(lines, indent+current.String())
	}
	return lines
}

// View returns the viewport view.
func (d *DetailPane) View() string {
	return d.viewport.View()
}

// Viewport returns a pointer to the viewport for event handling.
func (d *DetailPane) Viewport() *viewport.Model {
	return &d.viewport
}

var (
	ansiRe           = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	sectionHeadingRe = regexp.MustCompile(`^(?:#{1,6}\s+)?(S\d+(?:\.\d+)*):\s`)
)

func (d *DetailPane) buildSectionOffsets(rendered string) {
	d.sectionOffsets = nil
	for i, line := range strings.Split(rendered, "\n") {
		stripped := strings.TrimSpace(ansiRe.ReplaceAllString(line, ""))
		if m := sectionHeadingRe.FindStringSubmatch(stripped); m != nil {
			d.sectionOffsets = append(d.sectionOffsets, sectionOffset{line: i, sectionID: m[1]})
		}
	}
}

// SectionIDAtOffset returns the section ID visible at the given vertical offset.
func (d *DetailPane) SectionIDAtOffset(yOffset int) string {
	result := ""
	for _, so := range d.sectionOffsets {
		if so.line <= yOffset {
			result = so.sectionID
		} else {
			break
		}
	}
	return result
}

// ScrollToSectionID scrolls the viewport to the given section's offset.
func (d *DetailPane) ScrollToSectionID(sectionID string) {
	if sectionID == "" {
		d.viewport.GotoTop()
		return
	}
	for _, so := range d.sectionOffsets {
		if so.sectionID == sectionID {
			d.viewport.SetYOffset(so.line)
			return
		}
	}
}

func (d *DetailPane) commentBorderColor() string {
	if d.theme == ThemeLight {
		return "33"
	}
	return "62"
}

func (d *DetailPane) renderCommentBox(comment *markdown.ReviewComment, index, total int) string {
	var header string
	if total == 1 {
		header = fmt.Sprintf("Review Comment [%s]", comment.FormatLabel())
	} else {
		header = fmt.Sprintf("Review Comment #%d [%s]", index+1, comment.FormatLabel())
	}

	content := header
	if comment.Body != "" {
		content += "\n\n" + comment.Body
	}

	// Match the inline-rendered glamour body width (viewport - glamour overhead),
	// plus this box's own border + padding so the visible content area aligns.
	boxWidth := d.viewport.Width() - glamourHorizontalOverhead + commentBoxOverhead
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(d.commentBorderColor())).
		Width(boxWidth).
		Padding(0, 1)

	return style.Render(content)
}

func (d *DetailPane) hasAnyComments(sectionOrder []string, getComments func(string) []*markdown.ReviewComment) bool {
	for _, id := range sectionOrder {
		if len(getComments(id)) > 0 {
			return true
		}
	}
	return false
}

func (d *DetailPane) insertCommentBoxes(rendered string, sectionOrder []string, getComments func(string) []*markdown.ReviewComment) string {
	lines := strings.Split(rendered, "\n")

	endLines := make(map[string]int)
	for i, so := range d.sectionOffsets {
		if i+1 < len(d.sectionOffsets) {
			endLines[so.sectionID] = d.sectionOffsets[i+1].line
		} else {
			endLines[so.sectionID] = len(lines)
		}
	}

	for i := len(sectionOrder) - 1; i >= 0; i-- {
		sectionID := sectionOrder[i]
		comments := getComments(sectionID)
		if len(comments) == 0 {
			continue
		}
		insertAt := endLines[sectionID]
		var boxes []string
		for ci, c := range comments {
			boxStr := d.renderCommentBox(c, ci, len(comments))
			boxes = append(boxes, strings.Split(boxStr, "\n")...)
		}
		boxes = append(boxes, "")
		newLines := make([]string, 0, len(lines)+len(boxes))
		newLines = append(newLines, lines[:insertAt]...)
		newLines = append(newLines, boxes...)
		newLines = append(newLines, lines[insertAt:]...)
		lines = newLines
	}

	return strings.Join(lines, "\n")
}
