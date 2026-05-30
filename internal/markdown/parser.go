package markdown

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Parse parses a Markdown source into a Document structure.
// It uses goldmark to build an AST and walks headings to create sections.
func Parse(source []byte) (*Document, error) {
	md := goldmark.New()
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	document := &Document{
		SourceLines: strings.Split(string(source), "\n"),
	}

	type headingInfo struct {
		level int
		title string
		start int // byte offset of the heading line start (including # markers)
		end   int // byte offset past the heading line end (including newline)
	}

	var headings []headingInfo

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}

		title := extractHeadingText(heading, source)
		start := findHeadingStart(heading, source)
		end := findHeadingEnd(heading, source)

		headings = append(headings, headingInfo{
			level: heading.Level,
			title: title,
			start: start,
			end:   end,
		})

		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking AST: %w", err)
	}

	if len(headings) == 0 {
		document.Preamble = strings.TrimSpace(string(source))
		return document, nil
	}

	// Extract preamble (text before first heading)
	if headings[0].start > 0 {
		document.Preamble = strings.TrimSpace(string(source[:headings[0].start]))
	}

	var flatSections []flatSection

	for i, h := range headings {
		bodyStart := h.end
		var bodyEnd int
		if i+1 < len(headings) {
			bodyEnd = headings[i+1].start
		} else {
			bodyEnd = len(source)
		}
		body := ""
		if bodyStart < bodyEnd {
			body = strings.TrimSpace(string(source[bodyStart:bodyEnd]))
		}

		// Promote only a document-leading H1 to the Title. A non-leading H1
		// (the first heading is not H1, or an H1 appears later) stays a
		// level-1 section so it remains visible instead of being folded into
		// the preamble.
		if i == 0 && h.level == 1 {
			document.Title = h.title
			if body != "" {
				if document.Preamble != "" {
					document.Preamble = document.Preamble + "\n\n" + body
				} else {
					document.Preamble = body
				}
			}
			continue
		}

		headingLine := byteOffsetToLine(source, h.start)
		endLine := headingLine
		if bodyStart < bodyEnd {
			endLine = lastNonEmptyLine(source, bodyStart, bodyEnd, headingLine)
		}

		flatSections = append(flatSections, flatSection{
			level:     h.level,
			title:     h.title,
			body:      body,
			startLine: headingLine,
			endLine:   endLine,
		})
	}

	document.Sections = buildHierarchy(flatSections)

	return document, nil
}

// findHeadingStart finds the byte offset where the heading line starts in source.
// goldmark Lines() positions may point past the # markers for ATX headings,
// so we always search backwards to find the true line start.
func findHeadingStart(heading *ast.Heading, source []byte) int {
	pos := -1

	// Try Lines() first
	if heading.Lines().Len() > 0 {
		pos = heading.Lines().At(0).Start
	}

	// Fall back to child text nodes
	if pos < 0 {
		pos = findFirstTextPos(heading)
	}

	if pos < 0 {
		return 0
	}

	// Search backwards to find the actual line start (before # markers)
	for pos > 0 && source[pos-1] != '\n' {
		pos--
	}
	return pos
}

// findHeadingEnd finds the byte offset past the heading line end (including newline).
func findHeadingEnd(heading *ast.Heading, source []byte) int {
	pos := -1

	// Try Lines() first
	if heading.Lines().Len() > 0 {
		last := heading.Lines().At(heading.Lines().Len() - 1)
		pos = last.Stop
	}

	// Fall back to child text nodes
	if pos < 0 {
		pos = findLastTextPos(heading)
	}

	if pos < 0 {
		return 0
	}

	// Advance to end of line and include newline
	for pos < len(source) && source[pos] != '\n' {
		pos++
	}
	if pos < len(source) {
		pos++ // include the newline
	}
	return pos
}

// findFirstTextPos returns the Start position of the first Text segment in the node.
func findFirstTextPos(n ast.Node) int {
	pos := -1
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := child.(*ast.Text); ok {
			pos = t.Segment.Start
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return pos
}

// findLastTextPos returns the Stop position of the last Text segment in the node.
func findLastTextPos(n ast.Node) int {
	pos := -1
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := child.(*ast.Text); ok {
			pos = t.Segment.Stop
		}
		return ast.WalkContinue, nil
	})
	return pos
}

// buildHierarchy converts a flat list of sections into a tree based on heading levels.
func buildHierarchy(flatSections []flatSection) []*Section {
	if len(flatSections) == 0 {
		return nil
	}

	var topLevel []*Section
	var parentStack []*Section

	for _, fs := range flatSections {
		section := &Section{
			Level:     fs.level,
			Title:     fs.title,
			Body:      fs.body,
			StartLine: fs.startLine,
			EndLine:   fs.endLine,
		}

		// Pop from stack until we find a parent with a lower level
		for len(parentStack) > 0 && parentStack[len(parentStack)-1].Level >= fs.level {
			parentStack = parentStack[:len(parentStack)-1]
		}

		if len(parentStack) > 0 {
			parent := parentStack[len(parentStack)-1]
			section.Parent = parent
			parent.Children = append(parent.Children, section)
		} else {
			topLevel = append(topLevel, section)
		}

		parentStack = append(parentStack, section)
	}

	assignIDs(topLevel, "")
	return topLevel
}

// assignIDs assigns hierarchical IDs to sections (S1, S1.1, S1.2, S2, ...).
func assignIDs(sections []*Section, prefix string) {
	for i, s := range sections {
		if prefix == "" {
			s.ID = fmt.Sprintf("S%d", i+1)
		} else {
			s.ID = fmt.Sprintf("%s.%d", prefix, i+1)
		}
		assignIDs(s.Children, s.ID)
	}
}

// extractHeadingText extracts the plain text content of a heading node.
func extractHeadingText(heading *ast.Heading, source []byte) string {
	var sb strings.Builder
	for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
		extractNodeText(child, source, &sb)
	}
	return sb.String()
}

// extractNodeText recursively extracts text from an AST node.
func extractNodeText(n ast.Node, source []byte, sb *strings.Builder) {
	if t, ok := n.(*ast.Text); ok {
		sb.Write(t.Segment.Value(source))
		if t.HardLineBreak() || t.SoftLineBreak() {
			sb.WriteByte(' ')
		}
	}
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		extractNodeText(child, source, sb)
	}
}

type flatSection struct {
	level     int
	title     string
	body      string
	startLine int // 1-based line number of heading
	endLine   int // 1-based line number of last body line
}

// byteOffsetToLine returns the 1-based line number for a byte offset.
func byteOffsetToLine(source []byte, offset int) int {
	line := 1
	for i := range min(offset, len(source)) {
		if source[i] == '\n' {
			line++
		}
	}
	return line
}

// lastNonEmptyLine returns the 1-based line number of the last non-empty line
// in the range [startOffset, endOffset) of source. Returns startLine if no
// non-empty line is found.
func lastNonEmptyLine(source []byte, startOffset, endOffset int, startLine int) int {
	if startOffset < 0 {
		startOffset = 0
	}
	if endOffset > len(source) {
		endOffset = len(source)
	}
	if startOffset >= endOffset {
		return startLine
	}
	text := string(source[startOffset:endOffset])
	lines := strings.Split(text, "\n")
	last := startLine
	lineNum := byteOffsetToLine(source, startOffset)
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			last = lineNum
		}
		lineNum++
	}
	return last
}
