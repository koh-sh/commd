package markdown

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Loader reads a file for review. The TUI and the browser call it when a
// file's turn comes and again when the reviewer reloads it.
type Loader func(path string) (File, error)

// SkipNotice is how the front ends report a file that could not be loaded
// and is skipped.
func SkipNotice(path string, err error) string {
	return fmt.Sprintf("Skipping %s: %v", path, err)
}

// ReloadResult is the outcome of ReviewState.Reread.
type ReloadResult struct {
	Err     error // the file could not be read; the review is unchanged
	Changed bool  // the review now shows the new content
	Moved   int   // comments that could not be placed and were moved (see reload)
	// Sections maps the section IDs before the reload to those of the same
	// sections after it, so a front end can stay on the selected one.
	Sections map[string]string
}

// Message is how the front ends report the reload.
func (res ReloadResult) Message() string {
	switch {
	case res.Err != nil:
		return fmt.Sprintf("Cannot reload: %v", res.Err)
	case !res.Changed:
		return "File unchanged"
	case res.Moved > 0:
		return fmt.Sprintf("Reloaded; %d comments no longer match the file and were moved to a section", res.Moved)
	default:
		return "Reloaded"
	}
}

// Reread reads the file under review again with load and, when its content
// changed, switches the review to it (see reload). A file that cannot be
// read leaves the review as it was.
func (r *ReviewState) Reread(load Loader) ReloadResult {
	f, err := load(r.Path)
	if err != nil {
		return ReloadResult{Err: err}
	}
	if sameContent(r.File, f) {
		return ReloadResult{}
	}
	moved, sections := r.reload(f)
	return ReloadResult{Changed: true, Moved: moved, Sections: sections}
}

// sameContent reports whether a new read of a file shows what the review
// already shows: the same source and, in a diff, the same diff lines.
func sameContent(a, b File) bool {
	return slices.Equal(a.Doc.SourceLines, b.Doc.SourceLines) &&
		(a.Diff == nil) == (b.Diff == nil) &&
		(a.Diff == nil || slices.Equal(a.Diff.Lines, b.Diff.Lines))
}

// reload switches the review to f, a fresh read of the same file, keeping
// what was reviewed so far. Section IDs are positional and line numbers
// shift with edits, so the comments are placed anew:
//   - a section comment follows its heading, found by the titles from the
//     top level down;
//   - a line comment follows its quoted text, taking the occurrence nearest
//     its old position.
//
// A comment that cannot be placed becomes a section comment on its section,
// or on the first one listed when that heading is gone too; a line comment
// keeps its quote so its target stays readable. reload returns how many
// comments were moved that way.
//
// Viewed marks stay on sections whose title and body are unchanged. The
// persisted viewed state (File.Viewed) belongs to the review, not to f, so
// it is kept.
func (r *ReviewState) reload(f File) (moved int, sections map[string]string) {
	oldDoc := r.Doc
	oldKeys := sectionKeys(oldDoc)
	viewedHashes := make(map[string]bool)
	for _, s := range oldDoc.AllSections() {
		if r.viewed[s.ID] {
			viewedHashes[contentHash(s)] = true
		}
	}

	f.Viewed = r.Viewed
	next := NewReviewState(f)
	for _, s := range f.Doc.AllSections() {
		if viewedHashes[contentHash(s)] {
			next.viewed[s.ID] = true
		}
	}
	r.File, r.viewed = next.File, next.viewed

	sections = sectionMap(oldKeys, f.Doc)
	fallback := r.firstListedSection()
	for _, c := range r.comments {
		section := sections[c.SectionID]
		if c.StartLine > 0 && r.relocate(c) {
			continue
		}
		if c.StartLine > 0 || section == "" {
			moved++
		}
		c.StartLine, c.EndLine, c.Side = 0, 0, ""
		c.SectionID = cmp.Or(section, fallback)
	}
	return moved, sections
}

// sectionMap maps the section IDs of oldKeys (see sectionKeys) to the IDs of
// the same sections in d, leaving out those that are gone. The overview maps
// to itself while d has one.
func sectionMap(oldKeys map[string]string, d *Document) map[string]string {
	newIDs := make(map[string]string)
	for id, key := range sectionKeys(d) {
		newIDs[key] = id
	}
	out := make(map[string]string)
	for oldID, key := range oldKeys {
		if id, ok := newIDs[key]; ok {
			out[oldID] = id
		}
	}
	if d.HasOverview() {
		out[OverviewSectionID] = OverviewSectionID
	}
	return out
}

// sectionKeys returns a key per section ID that identifies the section by
// content rather than position: its heading titles from the top level down,
// plus how many earlier sections have the same titles.
func sectionKeys(d *Document) map[string]string {
	keys := make(map[string]string)
	seen := make(map[string]int)
	for _, s := range d.AllSections() {
		var titles []string
		for p := s; p != nil; p = p.Parent {
			titles = append(titles, p.Title)
		}
		slices.Reverse(titles)
		path := strings.Join(titles, "\x00")
		keys[s.ID] = path + "\x00#" + strconv.Itoa(seen[path])
		seen[path]++
	}
	return keys
}

// firstListedSection returns the first entry of the section list: the
// overview when there is one, else the first section. A document with
// neither has only the overview to hold comments.
func (r *ReviewState) firstListedSection() string {
	if !r.Doc.HasOverview() && len(r.Doc.Sections) > 0 {
		return r.Doc.Sections[0].ID
	}
	return OverviewSectionID
}

// numberedLine is a line of the review with its line number.
type numberedLine struct {
	line int
	text string
}

// relocate moves the line comment c to where its quote now is, choosing the
// occurrence nearest its old start line. It reports whether the quote was
// found.
func (r *ReviewState) relocate(c *ReviewComment) bool {
	if len(c.Quote) == 0 {
		return false
	}
	lines := r.sideLines(c.Side)
	best := -1
	for i := 0; i+len(c.Quote) <= len(lines); i++ {
		if !quoteAt(lines[i:], c.Quote) {
			continue
		}
		if best < 0 || distance(lines[i].line, c.StartLine) < distance(lines[best].line, c.StartLine) {
			best = i
		}
	}
	if best < 0 {
		return false
	}
	placed := *c
	placed.StartLine = lines[best].line
	placed.EndLine = lines[best+len(c.Quote)-1].line
	if err := r.setTarget(&placed); err != nil {
		return false
	}
	*c = placed
	return true
}

// sideLines returns the lines a line comment on side can target, in order:
// the source lines, or the diff lines on side.
func (r *ReviewState) sideLines(side string) []numberedLine {
	var out []numberedLine
	if r.Diff == nil {
		for i, text := range r.Doc.SourceLines {
			out = append(out, numberedLine{line: i + 1, text: text})
		}
		return out
	}
	for _, dl := range r.Diff.Lines {
		if line, lineSide := dl.Position(); lineSide == side {
			out = append(out, numberedLine{line: line, text: dl.Content})
		}
	}
	return out
}

// quoteAt reports whether lines start with the quoted text.
func quoteAt(lines []numberedLine, quote []string) bool {
	for i, q := range quote {
		if lines[i].text != q {
			return false
		}
	}
	return true
}

func distance(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
