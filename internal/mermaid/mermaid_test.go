package mermaid

import (
	"strings"
	"testing"
)

func TestRenderBlocks_Flowchart(t *testing.T) {
	input := "```mermaid\ngraph LR\n    A[Client] --> B[Server]\n```"
	got := RenderBlocks(input)

	// Should be converted to a plain code block (no "mermaid" language tag).
	if strings.Contains(got, "```mermaid") {
		t.Error("mermaid fence should be replaced with plain fence")
	}
	// Rendered ASCII art should contain the node labels.
	if !strings.Contains(got, "Client") || !strings.Contains(got, "Server") {
		t.Errorf("expected node labels in output, got:\n%s", got)
	}
	// Should be wrapped in plain code fences.
	if !strings.HasPrefix(got, "```\n") {
		t.Error("output should start with plain code fence")
	}
	if !strings.HasSuffix(got, "\n```") {
		t.Errorf("output should end with plain code fence, got:\n%s", got)
	}
}

func TestRenderBlocks_NonMermaidBlock(t *testing.T) {
	input := "```go\nfmt.Println(\"hello\")\n```"
	got := RenderBlocks(input)
	if got != input {
		t.Errorf("non-mermaid block should be unchanged\n got: %q\nwant: %q", got, input)
	}
}

func TestRenderBlocks_UnsupportedDiagramFallback(t *testing.T) {
	input := "```mermaid\ngantt\n    title A Gantt Chart\n    dateFormat YYYY-MM-DD\n    section Section\n    A task :a1, 2024-01-01, 30d\n```"
	got := RenderBlocks(input)

	// Unsupported diagram type should fall back to a plain code block with original source.
	if !strings.Contains(got, "gantt") {
		t.Error("fallback should preserve original source")
	}
}

func TestRenderBlocks_SurroundingTextPreserved(t *testing.T) {
	input := "Before text\n\n```mermaid\ngraph LR\n    A --> B\n```\n\nAfter text"
	got := RenderBlocks(input)

	if !strings.HasPrefix(got, "Before text") {
		t.Error("text before mermaid block should be preserved")
	}
	if !strings.HasSuffix(got, "After text") {
		t.Error("text after mermaid block should be preserved")
	}
}

func TestRenderBlocks_UnclosedBlock(t *testing.T) {
	input := "```mermaid\ngraph LR\n    A --> B"
	got := RenderBlocks(input)

	// Unclosed block should be emitted unchanged.
	if !strings.HasPrefix(got, "```mermaid") {
		t.Errorf("unclosed block should preserve original opening, got:\n%s", got)
	}
	if !strings.Contains(got, "A --> B") {
		t.Error("unclosed block should preserve content")
	}
}

func TestTryRenderMermaid_InvalidInput(t *testing.T) {
	source := "this is not valid mermaid"
	got := tryRenderMermaid(source)
	if got != source {
		t.Errorf("invalid input should return original source\n got: %q\nwant: %q", got, source)
	}
}
