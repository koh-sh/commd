package tui

import (
	"strings"

	"github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
	"github.com/AlexanderGrooff/mermaid-ascii/pkg/render"
)

// renderMermaidBlocks scans Markdown for ```mermaid fenced code blocks and
// replaces each with its ASCII art rendering. On conversion failure the
// original block is kept unchanged.
func renderMermaidBlocks(md string) string {
	lines := strings.Split(md, "\n")
	var result []string
	var buf []string
	inMermaid := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inMermaid {
			if trimmed == "```mermaid" {
				inMermaid = true
				buf = nil
				continue
			}
			result = append(result, line)
			continue
		}

		// Inside a mermaid block.
		if trimmed == "```" {
			source := strings.Join(buf, "\n")
			rendered := tryRenderMermaid(source)
			result = append(result, "```")
			result = append(result, rendered)
			result = append(result, "```")
			inMermaid = false
			buf = nil
			continue
		}
		buf = append(buf, line)
	}

	// Unclosed mermaid block: emit original lines unchanged.
	if inMermaid {
		result = append(result, "```mermaid")
		result = append(result, buf...)
	}

	return strings.Join(result, "\n")
}

// tryRenderMermaid attempts to convert mermaid source to ASCII art.
// On error or panic (e.g. unsupported diagram type, malformed input that
// trips the third-party renderer) it returns the original source.
func tryRenderMermaid(source string) (result string) {
	defer func() {
		if r := recover(); r != nil {
			result = source
		}
	}()
	cfg := diagram.DefaultConfig()
	rendered, err := render.RenderDiagram(source, cfg)
	if err != nil {
		return source
	}
	return strings.TrimRight(rendered, "\n")
}
