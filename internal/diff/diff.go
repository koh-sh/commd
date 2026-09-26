// Package diff parses unified diff patches into line-level data used by the
// diff review view. It is independent of where the patch came from (GitHub
// API or a local git working tree).
package diff

import (
	"fmt"
	"strconv"
	"strings"
)

// LineType represents the type of a diff line.
type LineType byte

const (
	Context LineType = ' '
	Added   LineType = '+'
	Removed LineType = '-'
)

// Side constants identify which side of a diff a line belongs to. They match
// the values GitHub's PR review API expects.
const (
	SideRight = "RIGHT" // new file (added and context lines)
	SideLeft  = "LEFT"  // old file (removed lines)
)

// Line represents a single line in a unified diff.
type Line struct {
	Type    LineType
	Content string // line content without the +/-/space prefix
	NewLine int    // 1-based line number in the new file (0 for removed lines)
	OldLine int    // 1-based line number in the old file (0 for added lines)
}

// Position returns the file line number and side a comment on the line
// targets: removed lines live on the LEFT side and are numbered by the old
// file, the others on the RIGHT side numbered by the new file.
func (l Line) Position() (line int, side string) {
	if l.Type == Removed {
		return l.OldLine, SideLeft
	}
	return l.NewLine, SideRight
}

// Info contains parsed diff data for a single file.
type Info struct {
	Lines []Line
}

// ParsePatch parses a unified diff patch string into Info. The patch must
// start at the first hunk header (see StripHeader for git output). Returns
// nil for an empty patch.
func ParsePatch(patch string) *Info {
	if patch == "" {
		return nil
	}

	info := &Info{}

	lines := strings.Split(patch, "\n")
	var newLine, oldLine int

	for _, line := range lines {
		// CRLF files diff with "\r\n" line endings; keep the "\r" out of
		// Content so it never reaches the display or the quoted output.
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "@@") {
			newLine, oldLine = parseHunkHeader(line)
			continue
		}

		if len(line) == 0 {
			continue
		}

		prefix := line[0]
		content := ""
		if len(line) > 1 {
			content = line[1:]
		}

		switch prefix {
		case '+':
			info.Lines = append(info.Lines, Line{
				Type:    Added,
				Content: content,
				NewLine: newLine,
			})
			newLine++
		case '-':
			info.Lines = append(info.Lines, Line{
				Type:    Removed,
				Content: content,
				OldLine: oldLine,
			})
			oldLine++
		case ' ':
			info.Lines = append(info.Lines, Line{
				Type:    Context,
				Content: content,
				NewLine: newLine,
				OldLine: oldLine,
			})
			newLine++
			oldLine++
		case '\\':
			// "\ No newline at end of file" — skip
		}
	}

	return info
}

// StripHeader removes everything before the first hunk header ("@@"). Local
// `git diff` output starts with "diff --git", "index", "---", and "+++" lines
// that ParsePatch would otherwise misread as removed/added content. GitHub's
// API patches already start at the first hunk, so they pass through unchanged.
func StripHeader(patch string) string {
	idx := strings.Index(patch, "@@")
	if idx < 0 {
		return ""
	}
	// Only treat "@@" as a hunk header when it starts a line.
	if idx > 0 && patch[idx-1] != '\n' {
		nl := strings.Index(patch[idx:], "\n@@")
		if nl < 0 {
			return ""
		}
		idx += nl + 1
	}
	return patch[idx:]
}

// AddedFilePatch builds a patch that marks every line of content as added.
// Used for untracked files, which have no git diff but should still be
// reviewable in the diff view.
func AddedFilePatch(content []byte) string {
	text := strings.TrimSuffix(string(content), "\n")
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	fmt.Fprintf(&sb, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, l := range lines {
		sb.WriteString("+" + l + "\n")
	}
	return sb.String()
}

// FormatDiffLines returns display strings for the diff lines with +/-/space prefix.
// A space separator is inserted between the prefix and content for readability.
func (d *Info) FormatDiffLines() []string {
	result := make([]string, len(d.Lines))
	for i, dl := range d.Lines {
		result[i] = fmt.Sprintf("%c %s", dl.Type, dl.Content)
	}
	return result
}

// LineSideMap builds line number, side, and type maps from the parsed diff.
// lineMap maps display index → file line number, sideMap → "RIGHT"/"LEFT",
// typeMap → LineType byte ('+', '-', ' ').
func (d *Info) LineSideMap() (lineMap []int, sideMap []string, typeMap []byte) {
	lineMap = make([]int, len(d.Lines))
	sideMap = make([]string, len(d.Lines))
	typeMap = make([]byte, len(d.Lines))
	for i, dl := range d.Lines {
		typeMap[i] = byte(dl.Type)
		lineMap[i], sideMap[i] = dl.Position()
	}
	return
}

// parseHunkHeader parses "@@ -old,count +new,count @@" and returns (newStart, oldStart).
func parseHunkHeader(line string) (int, int) {
	newStart := 1
	oldStart := 1

	// Parse -X part
	if _, after, ok := strings.Cut(line, "-"); ok {
		oldStart = parseNumberAt(after)
	}
	// Parse +N part
	if _, after, ok := strings.Cut(line, "+"); ok {
		newStart = parseNumberAt(after)
	}

	return newStart, oldStart
}

func parseNumberAt(s string) int {
	end := len(s)
	for i, c := range s {
		if c < '0' || c > '9' {
			end = i
			break
		}
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 1
	}
	return n
}
