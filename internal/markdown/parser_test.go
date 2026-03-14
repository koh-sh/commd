package markdown

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yuin/goldmark/ast"
	textm "github.com/yuin/goldmark/text"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to read testdata/%s: %v", name, err)
	}
	return data
}

func TestParseBasic(t *testing.T) {
	source := readTestdata(t, "basic.md")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if doc.Title != "Plan: Authentication System" {
		t.Errorf("Title = %q, want %q", doc.Title, "Plan: Authentication System")
	}

	if doc.Preamble != "This plan implements a basic authentication system." {
		t.Errorf("Preamble = %q, want %q", doc.Preamble, "This plan implements a basic authentication system.")
	}

	if len(doc.Sections) != 3 {
		t.Fatalf("len(Sections) = %d, want 3", len(doc.Sections))
	}

	// Check top-level section IDs
	wantIDs := []string{"S1", "S2", "S3"}
	for i, want := range wantIDs {
		if doc.Sections[i].ID != want {
			t.Errorf("Sections[%d].ID = %q, want %q", i, doc.Sections[i].ID, want)
		}
	}

	// Check S1 children
	s1 := doc.Sections[0]
	if s1.Title != "Step 1: Auth Middleware" {
		t.Errorf("S1.Title = %q, want %q", s1.Title, "Step 1: Auth Middleware")
	}
	if len(s1.Children) != 2 {
		t.Fatalf("len(S1.Children) = %d, want 2", len(s1.Children))
	}
	if s1.Children[0].ID != "S1.1" {
		t.Errorf("S1.Children[0].ID = %q, want %q", s1.Children[0].ID, "S1.1")
	}
	if s1.Children[1].ID != "S1.2" {
		t.Errorf("S1.Children[1].ID = %q, want %q", s1.Children[1].ID, "S1.2")
	}

	// Check parent references
	if s1.Children[0].Parent != s1 {
		t.Error("S1.1.Parent should be S1")
	}

	// Check S2 children
	s2 := doc.Sections[1]
	if len(s2.Children) != 2 {
		t.Fatalf("len(S2.Children) = %d, want 2", len(s2.Children))
	}
	if s2.Children[0].ID != "S2.1" {
		t.Errorf("S2.Children[0].ID = %q, want %q", s2.Children[0].ID, "S2.1")
	}

	// Check S3 has no children
	s3 := doc.Sections[2]
	if len(s3.Children) != 0 {
		t.Errorf("len(S3.Children) = %d, want 0", len(s3.Children))
	}

	// Check AllSections count: 3 top-level + 2 children of S1 + 2 children of S2 = 7
	allSections := doc.AllSections()
	if len(allSections) != 7 {
		t.Errorf("len(AllSections) = %d, want 7", len(allSections))
	}
}

func TestParseNoHeadings(t *testing.T) {
	source := readTestdata(t, "no-headings.md")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if doc.Title != "" {
		t.Errorf("Title = %q, want empty", doc.Title)
	}

	if doc.Preamble == "" {
		t.Error("Preamble should not be empty")
	}

	if len(doc.Sections) != 0 {
		t.Errorf("len(Sections) = %d, want 0", len(doc.Sections))
	}
}

func TestParseDeepNesting(t *testing.T) {
	source := readTestdata(t, "deep-nesting.md")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if doc.Title != "Deep Nesting Plan" {
		t.Errorf("Title = %q, want %q", doc.Title, "Deep Nesting Plan")
	}

	if len(doc.Sections) != 2 {
		t.Fatalf("len(Sections) = %d, want 2", len(doc.Sections))
	}

	// First top-level section should have children
	s1 := doc.Sections[0]
	if s1.ID != "S1" {
		t.Errorf("S1.ID = %q, want %q", s1.ID, "S1")
	}
	if len(s1.Children) != 2 {
		t.Fatalf("len(S1.Children) = %d, want 2", len(s1.Children))
	}

	// S1.1 (Level 3) should have a child at level 4
	s11 := s1.Children[0]
	if s11.ID != "S1.1" {
		t.Errorf("S1.1.ID = %q, want %q", s11.ID, "S1.1")
	}
	if len(s11.Children) != 1 {
		t.Fatalf("len(S1.1.Children) = %d, want 1", len(s11.Children))
	}

	// S1.1.1 (Level 4) should have a child at level 5
	s111 := s11.Children[0]
	if s111.ID != "S1.1.1" {
		t.Errorf("S1.1.1.ID = %q, want %q", s111.ID, "S1.1.1")
	}
	if len(s111.Children) != 1 {
		t.Fatalf("len(S1.1.1.Children) = %d, want 1", len(s111.Children))
	}

	// Check total sections
	allSections := doc.AllSections()
	if len(allSections) != 6 {
		t.Errorf("len(AllSections) = %d, want 6", len(allSections))
	}
}

func TestParseCodeBlockHash(t *testing.T) {
	source := readTestdata(t, "code-block-hash.md")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if doc.Title != "Plan with Code Blocks" {
		t.Errorf("Title = %q, want %q", doc.Title, "Plan with Code Blocks")
	}

	// Should only have 2 sections (code block # should not be parsed as headings)
	if len(doc.Sections) != 2 {
		t.Fatalf("len(Sections) = %d, want 2", len(doc.Sections))
	}

	if doc.Sections[0].Title != "Step 1: Configuration" {
		t.Errorf("Sections[0].Title = %q, want %q", doc.Sections[0].Title, "Step 1: Configuration")
	}
	if doc.Sections[1].Title != "Step 2: Implementation" {
		t.Errorf("Sections[1].Title = %q, want %q", doc.Sections[1].Title, "Step 2: Implementation")
	}

	// Body of section 2 should contain the code block
	if doc.Sections[1].Body == "" {
		t.Error("Sections[1].Body should not be empty")
	}
}

func TestParseSingleSection(t *testing.T) {
	source := readTestdata(t, "single-section.md")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if doc.Title != "Simple Plan" {
		t.Errorf("Title = %q, want %q", doc.Title, "Simple Plan")
	}

	if len(doc.Sections) != 1 {
		t.Fatalf("len(Sections) = %d, want 1", len(doc.Sections))
	}

	if doc.Sections[0].ID != "S1" {
		t.Errorf("Sections[0].ID = %q, want %q", doc.Sections[0].ID, "S1")
	}
	if doc.Sections[0].Title != "The Only Step" {
		t.Errorf("Sections[0].Title = %q, want %q", doc.Sections[0].Title, "The Only Step")
	}
}

func TestParseEmptySource(t *testing.T) {
	doc, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if doc.Title != "" {
		t.Errorf("Title = %q, want empty", doc.Title)
	}
	if len(doc.Sections) != 0 {
		t.Errorf("len(Sections) = %d, want 0", len(doc.Sections))
	}
}

func TestParseMultipleH1(t *testing.T) {
	source := []byte(`# First Title

Intro text.

# Second Title

This should be a step.

## Sub Step

Content here.
`)
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if doc.Title != "First Title" {
		t.Errorf("Title = %q, want %q", doc.Title, "First Title")
	}

	// Second H1 is top-level, H2 is its child
	if len(doc.Sections) != 1 {
		t.Fatalf("len(Sections) = %d, want 1", len(doc.Sections))
	}

	if doc.Sections[0].Title != "Second Title" {
		t.Errorf("Sections[0].Title = %q, want %q", doc.Sections[0].Title, "Second Title")
	}

	if len(doc.Sections[0].Children) != 1 {
		t.Fatalf("len(Sections[0].Children) = %d, want 1", len(doc.Sections[0].Children))
	}
	if doc.Sections[0].Children[0].Title != "Sub Step" {
		t.Errorf("Sections[0].Children[0].Title = %q, want %q", doc.Sections[0].Children[0].Title, "Sub Step")
	}
}

func TestParseLevelSkip(t *testing.T) {
	source := []byte(`# Plan

### Level 3 without Level 2

Content.

## Level 2

More content.
`)
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Both should be top-level since H3 has no H2 parent
	if len(doc.Sections) != 2 {
		t.Fatalf("len(Sections) = %d, want 2", len(doc.Sections))
	}

	if doc.Sections[0].ID != "S1" {
		t.Errorf("Sections[0].ID = %q, want %q", doc.Sections[0].ID, "S1")
	}
	if doc.Sections[1].ID != "S2" {
		t.Errorf("Sections[1].ID = %q, want %q", doc.Sections[1].ID, "S2")
	}
}

func TestParseHeadingWithInlineCode(t *testing.T) {
	source := []byte("# Plan\n\n## Step with `code` in title\n\nBody text.\n")
	p, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(p.Sections) != 1 {
		t.Fatalf("sections count = %d, want 1", len(p.Sections))
	}
	// extractNodeText should recurse into inline code
	if p.Sections[0].Title != "Step with code in title" {
		t.Errorf("Title = %q, want %q", p.Sections[0].Title, "Step with code in title")
	}
}

func TestParseHeadingWithEmphasis(t *testing.T) {
	source := []byte("# Plan\n\n## Step with **bold** text\n\nBody.\n")
	p, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(p.Sections) != 1 {
		t.Fatalf("sections count = %d, want 1", len(p.Sections))
	}
	if p.Sections[0].Title != "Step with bold text" {
		t.Errorf("Title = %q, want %q", p.Sections[0].Title, "Step with bold text")
	}
}

func TestParsePreamble(t *testing.T) {
	source := []byte("Some text before any heading.\n\n## First Step\n\nStep body.\n")
	p, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if p.Preamble != "Some text before any heading." {
		t.Errorf("Preamble = %q", p.Preamble)
	}
}

func TestParsePreambleWithH1Body(t *testing.T) {
	// When there's preamble before H1 AND body text after H1, they should be concatenated
	source := []byte("Pre-heading text.\n\n# Title\n\nBody after title.\n")
	p, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if p.Title != "Title" {
		t.Errorf("Title = %q, want %q", p.Title, "Title")
	}
	want := "Pre-heading text.\n\nBody after title."
	if p.Preamble != want {
		t.Errorf("Preamble = %q, want %q", p.Preamble, want)
	}
}

func TestParseOnlyH1(t *testing.T) {
	// Only H1 heading, no sections -> buildHierarchy([]) returns nil
	source := []byte("# Title Only\n\nSome description.\n")
	p, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if p.Title != "Title Only" {
		t.Errorf("Title = %q, want %q", p.Title, "Title Only")
	}
	if len(p.Sections) != 0 {
		t.Errorf("len(Sections) = %d, want 0", len(p.Sections))
	}
}

func TestParseSetextHeadings(t *testing.T) {
	// Setext-style headings (=== for H1, --- for H2) may trigger different
	// goldmark AST behavior for heading line positions
	source := []byte("Title\n=====\n\nSome preamble.\n\nStep One\n--------\n\nStep body.\n")
	p, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if p.Title != "Title" {
		t.Errorf("Title = %q, want %q", p.Title, "Title")
	}
	if len(p.Sections) != 1 {
		t.Fatalf("len(Sections) = %d, want 1", len(p.Sections))
	}
	if p.Sections[0].Title != "Step One" {
		t.Errorf("Sections[0].Title = %q, want %q", p.Sections[0].Title, "Step One")
	}
	// Body may include setext underline due to heading end position
	if p.Sections[0].Body == "" {
		t.Error("Sections[0].Body should not be empty")
	}
}

func TestFindFirstTextPos(t *testing.T) {
	t.Run("with text child", func(t *testing.T) {
		heading := ast.NewHeading(2)
		textNode := ast.NewTextSegment(textm.NewSegment(5, 10))
		heading.AppendChild(heading, textNode)

		pos := findFirstTextPos(heading)
		if pos != 5 {
			t.Errorf("findFirstTextPos = %d, want 5", pos)
		}
	})

	t.Run("no text child", func(t *testing.T) {
		heading := ast.NewHeading(2)
		pos := findFirstTextPos(heading)
		if pos != -1 {
			t.Errorf("findFirstTextPos with no text = %d, want -1", pos)
		}
	})
}

func TestFindLastTextPos(t *testing.T) {
	t.Run("with text child", func(t *testing.T) {
		heading := ast.NewHeading(2)
		textNode := ast.NewTextSegment(textm.NewSegment(5, 10))
		heading.AppendChild(heading, textNode)

		pos := findLastTextPos(heading)
		if pos != 10 {
			t.Errorf("findLastTextPos = %d, want 10", pos)
		}
	})

	t.Run("multiple text children", func(t *testing.T) {
		heading := ast.NewHeading(2)
		text1 := ast.NewTextSegment(textm.NewSegment(5, 10))
		text2 := ast.NewTextSegment(textm.NewSegment(12, 20))
		heading.AppendChild(heading, text1)
		heading.AppendChild(heading, text2)

		pos := findLastTextPos(heading)
		if pos != 20 {
			t.Errorf("findLastTextPos = %d, want 20", pos)
		}
	})

	t.Run("no text child", func(t *testing.T) {
		heading := ast.NewHeading(2)
		pos := findLastTextPos(heading)
		if pos != -1 {
			t.Errorf("findLastTextPos with no text = %d, want -1", pos)
		}
	})
}

func TestFindHeadingStartNoLines(t *testing.T) {
	t.Run("no lines no text", func(t *testing.T) {
		heading := ast.NewHeading(2)
		source := []byte("## Test\n")
		pos := findHeadingStart(heading, source)
		if pos != 0 {
			t.Errorf("findHeadingStart = %d, want 0", pos)
		}
	})

	t.Run("no lines with text child", func(t *testing.T) {
		heading := ast.NewHeading(2)
		textNode := ast.NewTextSegment(textm.NewSegment(3, 7))
		heading.AppendChild(heading, textNode)

		source := []byte("## Test\nMore text\n")
		pos := findHeadingStart(heading, source)
		// Should search backwards from pos 3 to find line start (pos 0)
		if pos != 0 {
			t.Errorf("findHeadingStart = %d, want 0", pos)
		}
	})
}

func TestFindHeadingEndNoLines(t *testing.T) {
	t.Run("no lines no text", func(t *testing.T) {
		heading := ast.NewHeading(2)
		source := []byte("## Test\n")
		pos := findHeadingEnd(heading, source)
		if pos != 0 {
			t.Errorf("findHeadingEnd = %d, want 0", pos)
		}
	})

	t.Run("no lines with text child", func(t *testing.T) {
		heading := ast.NewHeading(2)
		textNode := ast.NewTextSegment(textm.NewSegment(3, 7))
		heading.AppendChild(heading, textNode)

		source := []byte("## Test\nMore text\n")
		pos := findHeadingEnd(heading, source)
		// pos 7 -> advance to newline at pos 7 ('\n') -> pos 8
		if pos != 8 {
			t.Errorf("findHeadingEnd = %d, want 8", pos)
		}
	})

	t.Run("no lines text at end without newline", func(t *testing.T) {
		heading := ast.NewHeading(2)
		textNode := ast.NewTextSegment(textm.NewSegment(3, 7))
		heading.AppendChild(heading, textNode)

		source := []byte("## Test")
		pos := findHeadingEnd(heading, source)
		// pos 7 = len(source), no newline to advance past
		if pos != 7 {
			t.Errorf("findHeadingEnd = %d, want 7", pos)
		}
	})
}

func TestFindSection(t *testing.T) {
	source := readTestdata(t, "basic.md")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	section := doc.FindSection("S1.2")
	if section == nil {
		t.Fatal("FindSection(S1.2) returned nil")
		return
	}
	if section.Title != "1.2 Middleware Registration" {
		t.Errorf("FindSection(S1.2).Title = %q, want %q", section.Title, "1.2 Middleware Registration")
	}

	missing := doc.FindSection("S99")
	if missing != nil {
		t.Error("FindSection(S99) should return nil")
	}
}
