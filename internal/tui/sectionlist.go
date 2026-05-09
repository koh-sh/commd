package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/koh-sh/commd/internal/markdown"
	"github.com/mattn/go-runewidth"
)

// SectionListItem is a flattened section for display in the section list.
type SectionListItem struct {
	Section    *markdown.Section
	Depth      int
	Expanded   bool
	Visible    bool
	IsOverview bool // true for the overview (preamble) entry
}

// SectionList manages the left pane section tree.
type SectionList struct {
	items        []SectionListItem
	cursor       int
	scrollOffset int
	comments     map[string][]*markdown.ReviewComment // sectionID -> comments
	viewed       map[string]bool                      // sectionID -> viewed flag
	viewedState  *markdown.ViewedState
	doc          *markdown.Document
}

// NewSectionList creates a new SectionList from a parsed document.
func NewSectionList(doc *markdown.Document, state *markdown.ViewedState) *SectionList {
	sl := &SectionList{
		comments:    make(map[string][]*markdown.ReviewComment),
		viewed:      make(map[string]bool),
		viewedState: state,
		doc:         doc,
	}

	// Add overview entry if there's a preamble
	if doc.Preamble != "" {
		sl.items = append(sl.items, SectionListItem{
			Visible:    true,
			IsOverview: true,
		})
	}

	// Flatten the section tree
	var flatten func(sections []*markdown.Section, depth int)
	flatten = func(sections []*markdown.Section, depth int) {
		for _, s := range sections {
			sl.items = append(sl.items, SectionListItem{
				Section:  s,
				Depth:    depth,
				Expanded: true,
				Visible:  true,
			})
			flatten(s.Children, depth+1)
		}
	}
	flatten(doc.Sections, 0)

	// Restore viewed flags from persisted state
	if state != nil {
		for i, item := range sl.items {
			if item.Section != nil && state.IsSectionViewed(item.Section) {
				sl.viewed[sl.items[i].Section.ID] = true
			}
		}
	}

	return sl
}

// CursorUp moves the cursor up to the previous visible item.
func (sl *SectionList) CursorUp() {
	for i := sl.cursor - 1; i >= 0; i-- {
		if sl.items[i].Visible {
			sl.cursor = i
			return
		}
	}
}

// CursorDown moves the cursor down to the next visible item.
func (sl *SectionList) CursorDown() {
	for i := sl.cursor + 1; i < len(sl.items); i++ {
		if sl.items[i].Visible {
			sl.cursor = i
			return
		}
	}
}

// CursorTop moves the cursor to the first visible item.
func (sl *SectionList) CursorTop() {
	for i := 0; i < len(sl.items); i++ {
		if sl.items[i].Visible {
			sl.cursor = i
			return
		}
	}
}

// CursorBottom moves the cursor to the last visible item.
func (sl *SectionList) CursorBottom() {
	for i := len(sl.items) - 1; i >= 0; i-- {
		if sl.items[i].Visible {
			sl.cursor = i
			return
		}
	}
}

// CursorHalfPageDown moves the cursor down by half the visible page height.
func (sl *SectionList) CursorHalfPageDown(pageHeight int) {
	sl.cursorMoveNDown(max(pageHeight/2, 1))
}

// CursorHalfPageUp moves the cursor up by half the visible page height.
func (sl *SectionList) CursorHalfPageUp(pageHeight int) {
	sl.cursorMoveNUp(max(pageHeight/2, 1))
}

// CursorPageDown moves the cursor down by the visible page height.
func (sl *SectionList) CursorPageDown(pageHeight int) {
	sl.cursorMoveNDown(max(pageHeight, 1))
}

// CursorPageUp moves the cursor up by the visible page height.
func (sl *SectionList) CursorPageUp(pageHeight int) {
	sl.cursorMoveNUp(max(pageHeight, 1))
}

// cursorMoveNDown moves the cursor down by n visible items.
func (sl *SectionList) cursorMoveNDown(n int) {
	for range n {
		next := -1
		for i := sl.cursor + 1; i < len(sl.items); i++ {
			if sl.items[i].Visible {
				next = i
				break
			}
		}
		if next < 0 {
			break
		}
		sl.cursor = next
	}
}

// cursorMoveNUp moves the cursor up by n visible items.
func (sl *SectionList) cursorMoveNUp(n int) {
	for range n {
		prev := -1
		for i := sl.cursor - 1; i >= 0; i-- {
			if sl.items[i].Visible {
				prev = i
				break
			}
		}
		if prev < 0 {
			break
		}
		sl.cursor = prev
	}
}

// ToggleExpand toggles the expand/collapse state of the current section.
func (sl *SectionList) ToggleExpand() {
	if sl.cursor >= len(sl.items) {
		return
	}
	item := &sl.items[sl.cursor]
	if item.IsOverview || item.Section == nil || len(item.Section.Children) == 0 {
		return
	}
	item.Expanded = !item.Expanded
	sl.updateVisibility()
}

// updateVisibility updates the Visible field for all items based on parent expansion state.
func (sl *SectionList) updateVisibility() {
	collapsed := make(map[*markdown.Section]bool)
	for _, item := range sl.items {
		if item.Section != nil && !item.Expanded {
			collapsed[item.Section] = true
		}
	}

	for i := range sl.items {
		if sl.items[i].IsOverview {
			sl.items[i].Visible = true
			continue
		}
		if sl.items[i].Section == nil {
			continue
		}

		visible := true
		parent := sl.items[i].Section.Parent
		for parent != nil {
			if collapsed[parent] {
				visible = false
				break
			}
			parent = parent.Parent
		}
		sl.items[i].Visible = visible
	}
}

// Selected returns the currently selected section (or nil for overview).
func (sl *SectionList) Selected() *markdown.Section {
	if sl.cursor >= len(sl.items) {
		return nil
	}
	return sl.items[sl.cursor].Section
}

// IsOverviewSelected returns true if the overview entry is selected.
func (sl *SectionList) IsOverviewSelected() bool {
	if sl.cursor >= len(sl.items) {
		return false
	}
	return sl.items[sl.cursor].IsOverview
}

// AddComment appends a comment for a section.
func (sl *SectionList) AddComment(sectionID string, comment *markdown.ReviewComment) {
	if comment == nil || comment.Body == "" {
		return
	}
	sl.comments[sectionID] = append(sl.comments[sectionID], comment)
}

// UpdateComment replaces a comment at the given index for a section.
func (sl *SectionList) UpdateComment(sectionID string, index int, comment *markdown.ReviewComment) {
	comments := sl.comments[sectionID]
	if index < 0 || index >= len(comments) {
		return
	}
	if comment == nil || comment.Body == "" {
		sl.DeleteComment(sectionID, index)
		return
	}
	sl.comments[sectionID][index] = comment
}

// DeleteComment removes a comment at the given index for a section.
func (sl *SectionList) DeleteComment(sectionID string, index int) {
	comments := sl.comments[sectionID]
	if index < 0 || index >= len(comments) {
		return
	}
	sl.comments[sectionID] = append(comments[:index], comments[index+1:]...)
	if len(sl.comments[sectionID]) == 0 {
		delete(sl.comments, sectionID)
	}
}

// ToggleViewed toggles the viewed flag for a section.
func (sl *SectionList) ToggleViewed(sectionID string) {
	sl.viewed[sectionID] = !sl.viewed[sectionID]

	// Sync with persisted state
	if sl.viewedState != nil {
		if section := sl.doc.FindSection(sectionID); section != nil {
			if sl.viewed[sectionID] {
				sl.viewedState.MarkViewed(section)
			} else {
				sl.viewedState.UnmarkViewed(section)
			}
		}
	}
}

// ViewedState returns the underlying ViewedState for persistence.
func (sl *SectionList) ViewedState() *markdown.ViewedState {
	return sl.viewedState
}

// IsViewed returns whether a section is marked as viewed.
func (sl *SectionList) IsViewed(sectionID string) bool {
	return sl.viewed[sectionID]
}

// GetComments returns all comments for a section.
func (sl *SectionList) GetComments(sectionID string) []*markdown.ReviewComment {
	return sl.comments[sectionID]
}

// HasComments returns true if there are any comments.
func (sl *SectionList) HasComments() bool {
	for _, comments := range sl.comments {
		if len(comments) > 0 {
			return true
		}
	}
	return false
}

// BuildReviewResult creates a ReviewResult from all comments.
func (sl *SectionList) BuildReviewResult() *markdown.ReviewResult {
	result := &markdown.ReviewResult{}

	// Include overview comments first
	for _, c := range sl.comments[markdown.OverviewSectionID] {
		result.Comments = append(result.Comments, *c)
	}

	// Walk sections in order to maintain consistent ordering
	allSections := sl.doc.AllSections()
	for _, s := range allSections {
		for _, c := range sl.comments[s.ID] {
			result.Comments = append(result.Comments, *c)
		}
	}

	return result
}

// Render renders the section list for display within the given height.
func (sl *SectionList) Render(width, height int, styles Styles) string {
	// Build list of visible item indices
	var visibleIndices []int
	for i, item := range sl.items {
		if item.Visible {
			visibleIndices = append(visibleIndices, i)
		}
	}

	// Find cursor position in visible list
	cursorPos := 0
	for vi, idx := range visibleIndices {
		if idx == sl.cursor {
			cursorPos = vi
			break
		}
	}

	// Calculate available lines for items
	itemLines := max(height, 1)

	// Adjust scroll offset to keep cursor visible
	if cursorPos < sl.scrollOffset {
		sl.scrollOffset = cursorPos
	}
	if cursorPos >= sl.scrollOffset+itemLines {
		sl.scrollOffset = cursorPos - itemLines + 1
	}
	if sl.scrollOffset < 0 {
		sl.scrollOffset = 0
	}

	var sb strings.Builder

	// Only render items in the visible window
	end := min(sl.scrollOffset+itemLines, len(visibleIndices))

	for vi := sl.scrollOffset; vi < end; vi++ {
		i := visibleIndices[vi]
		item := sl.items[i]

		var line string
		if item.IsOverview {
			badge := sl.renderBadge(markdown.OverviewSectionID, styles)
			line = "  Overview" + badge
		} else if item.Section != nil {
			indent := strings.Repeat("  ", item.Depth)
			prefix := " "
			if len(item.Section.Children) > 0 {
				if item.Expanded {
					prefix = "▼"
				} else {
					prefix = "▶"
				}
			}

			badge := sl.renderBadge(item.Section.ID, styles)
			sectionText := fmt.Sprintf("%s%s %s %s", indent, prefix, item.Section.ID, item.Section.Title)
			line = truncate(sectionText, width-4-lipgloss.Width(badge)) + badge
		}

		if i == sl.cursor {
			line = styles.SelectedSection.Render("> " + line)
		} else {
			line = styles.NormalSection.Render("  " + line)
		}

		sb.WriteString(line + "\n")
	}

	return sb.String()
}

// renderBadge renders the badge for a section (comment indicator, viewed mark).
func (sl *SectionList) renderBadge(sectionID string, styles Styles) string {
	commentCount := len(sl.comments[sectionID])
	isViewed := sl.viewed[sectionID]

	var badge string
	if commentCount == 1 {
		badge += styles.SectionBadge.Render(" [*]")
	} else if commentCount > 1 {
		badge += styles.SectionBadge.Render(fmt.Sprintf(" [*%d]", commentCount))
	}
	if isViewed {
		badge += styles.ViewedBadge.Render(" [✓]")
	}
	return badge
}

// TotalSectionCount returns the number of sections (excluding overview).
func (sl *SectionList) TotalSectionCount() int {
	count := 0
	for _, item := range sl.items {
		if !item.IsOverview && item.Section != nil {
			count++
		}
	}
	return count
}

// ViewedCount returns the number of viewed sections.
func (sl *SectionList) ViewedCount() int {
	count := 0
	for _, viewed := range sl.viewed {
		if viewed {
			count++
		}
	}
	return count
}

// TotalCommentCount returns the total number of comments across all sections.
func (sl *SectionList) TotalCommentCount() int {
	count := 0
	for _, comments := range sl.comments {
		count += len(comments)
	}
	return count
}

// FilterByQuery filters the section list to show only sections matching the query.
// Matching is case-insensitive against section ID, Title, and Body.
// If a child matches, its ancestors are shown. If a parent matches, its children are shown.
func (sl *SectionList) FilterByQuery(query string) {
	if query == "" {
		sl.ClearFilter()
		return
	}

	query = strings.ToLower(query)

	// First pass: mark direct matches
	matched := make(map[int]bool)
	for i, item := range sl.items {
		if item.IsOverview {
			if strings.Contains("overview", query) { //nolint:gocritic // intentional: match when query is a substring of "overview"
				matched[i] = true
			}
			continue
		}
		if item.Section == nil {
			continue
		}
		text := strings.ToLower(item.Section.ID + " " + item.Section.Title + " " + item.Section.Body)
		if strings.Contains(text, query) {
			matched[i] = true
		}
	}

	// Second pass: if a section matches, show its ancestors
	ancestorVisible := make(map[*markdown.Section]bool)
	for i, item := range sl.items {
		if !matched[i] || item.Section == nil {
			continue
		}
		parent := item.Section.Parent
		for parent != nil {
			ancestorVisible[parent] = true
			parent = parent.Parent
		}
	}

	// Third pass: if a section matches, show its descendants
	descendantVisible := make(map[*markdown.Section]bool)
	for i, item := range sl.items {
		if !matched[i] || item.Section == nil {
			continue
		}
		var markDescendants func(sections []*markdown.Section)
		markDescendants = func(sections []*markdown.Section) {
			for _, s := range sections {
				descendantVisible[s] = true
				markDescendants(s.Children)
			}
		}
		markDescendants(item.Section.Children)
	}

	// Apply visibility
	for i := range sl.items {
		item := &sl.items[i]
		if item.IsOverview {
			item.Visible = matched[i]
			continue
		}
		if item.Section == nil {
			item.Visible = false
			continue
		}
		item.Visible = matched[i] || ancestorVisible[item.Section] || descendantVisible[item.Section]
	}

	// Move cursor to first visible item if current is hidden
	if sl.cursor < len(sl.items) && !sl.items[sl.cursor].Visible {
		sl.CursorTop()
	}
}

// SelectBySectionID moves the cursor to the item with the given section ID.
func (sl *SectionList) SelectBySectionID(sectionID string) {
	for i, item := range sl.items {
		if item.Section != nil && item.Section.ID == sectionID && item.Visible {
			sl.cursor = i
			return
		}
	}
}

// ClearFilter resets visibility to respect only expansion state.
func (sl *SectionList) ClearFilter() {
	sl.updateVisibility()
}

// truncate truncates a string to fit within max display-width cells, with ellipsis.
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return runewidth.Truncate(s, maxWidth, "")
	}
	return runewidth.Truncate(s, maxWidth, "...")
}
