package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/koh-sh/commd/internal/markdown"
)

// AppMode represents the current application mode.
type AppMode int

const (
	ModeNormal      AppMode = iota // Section list navigation
	ModeComment                    // Comment input
	ModeCommentList                // Comment list management
	ModeConfirm                    // Confirmation dialog
	ModeHelp                       // Help overlay
	ModeSearch                     // Section search
	ModeLineSelect                 // Visual line selection in raw view
)

// confirmKind identifies the action pending confirmation.
type confirmKind int

const (
	confirmQuit   confirmKind = iota // quit without submitting
	confirmSubmit                    // submit the review
)

// Focus represents which pane has focus.
type Focus int

const (
	FocusLeft Focus = iota
	FocusRight
)

// scrollToEnd is a value large enough to be clamped to the maximum horizontal offset.
const scrollToEnd = 1 << 30

// AppResult is the result returned when the TUI exits.
type AppResult struct {
	Review *markdown.ReviewResult
	Status markdown.Status
}

// App is the main Bubble Tea model for the TUI.
type App struct {
	doc         *markdown.Document
	sectionList *SectionList
	detail      *DetailPane
	linePane    *LinePane
	comment     *CommentEditor
	commentList *CommentList
	search      *SearchBar
	keymap      KeyMap
	styles      Styles

	mode      AppMode
	focus     Focus
	fullView  bool
	rawView   bool // true = raw source + line numbers, false = glamour rendering
	width     int
	height    int
	ready     bool
	leftRatio int // left pane width percentage (default 30, range 10-50)
	opts      AppOptions

	result         AppResult
	confirmAction  confirmKind // what the confirm dialog is for
	pendingG       bool        // gg chord: true when first 'g' was pressed
	editCommentIdx int         // index of comment being edited in comment list mode (-1 = new)
}

// DiffData holds parsed diff information for PR mode display.
type DiffData struct {
	DisplayLines []string // formatted diff lines (with +/-/space prefix)
	LineMap      []int    // maps display index → file line number for commenting
	SideMap      []string // maps display index → "RIGHT" or "LEFT"
	TypeMap      []byte   // maps display index → diff line type ('+', '-', ' ')
}

// AppOptions configures the TUI appearance.
type AppOptions struct {
	Theme       string    // "dark" or "light"
	FilePath    string    // file path (displayed in title bar)
	TrackViewed bool      // persist viewed state to sidecar file
	PRMode      bool      // PR review mode: changes dialog text and enables diff view
	Diff        *DiffData // when set, raw view shows diff instead of full source
}

// NewApp creates a new App model.
func NewApp(doc *markdown.Document, opts AppOptions) *App {
	var state *markdown.ViewedState
	if opts.TrackViewed && opts.FilePath != "" {
		state = markdown.LoadViewedState(markdown.StatePath(opts.FilePath))
	}
	styles := stylesForTheme(opts.Theme)
	a := &App{
		doc:            doc,
		sectionList:    NewSectionList(doc, state),
		comment:        NewCommentEditor(),
		commentList:    NewCommentList(),
		search:         NewSearchBar(),
		keymap:         DefaultKeyMap(),
		styles:         styles,
		leftRatio:      30,
		opts:           opts,
		editCommentIdx: -1,
		result: AppResult{
			Status: markdown.StatusCancelled,
		},
	}
	if opts.Diff != nil {
		// PR mode: use diff lines, start in raw view with section filtering
		a.linePane = NewLinePane(opts.Diff.DisplayLines, 0, 0, styles, doc.AllSections())
		a.linePane.diffLineMap = opts.Diff.LineMap
		a.linePane.diffSideMap = opts.Diff.SideMap
		a.linePane.diffTypeMap = opts.Diff.TypeMap
		// Recalculate gutter width from max file line number
		maxLine := 1
		for _, l := range opts.Diff.LineMap {
			if l > maxLine {
				maxLine = l
			}
		}
		a.linePane.gutterWidth = len(fmt.Sprintf("%d", maxLine)) + 1
		a.rawView = true
	} else if len(doc.SourceLines) > 0 {
		a.linePane = NewLinePane(doc.SourceLines, 0, 0, styles, doc.AllSections())
	}
	return a
}

// Result returns the final result after the TUI exits.
func (a *App) Result() AppResult {
	return a.result
}

// isRawMode returns true when raw source view is active.
func (a *App) isRawMode() bool {
	return a.rawView && a.linePane != nil
}

// ViewedState returns the current viewed state for persistence.
func (a *App) ViewedState() *markdown.ViewedState {
	return a.sectionList.ViewedState()
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateLayout()
		a.ready = true
		a.refreshDetail()
		return a, nil

	case tea.KeyPressMsg:
		return a.handleKey(msg)
	}

	if a.mode == ModeComment {
		cmd := a.comment.Update(msg)
		return a, cmd
	}

	if a.mode == ModeSearch {
		cmd := a.search.Update(msg)
		return a, cmd
	}

	return a, nil
}

func (a *App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch a.mode {
	case ModeNormal:
		return a.handleNormalMode(msg)
	case ModeComment:
		return a.handleCommentMode(msg)
	case ModeCommentList:
		return a.handleCommentListMode(msg)
	case ModeConfirm:
		return a.handleConfirmMode(msg)
	case ModeHelp:
		return a.handleHelpMode(msg)
	case ModeSearch:
		return a.handleSearchMode(msg)
	case ModeLineSelect:
		return a.handleLineSelectMode(msg)
	}
	return a, nil
}

func (a *App) handleNormalMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Handle 'gg' chord (second g after pending g)
	if a.pendingG {
		a.pendingG = false
		if msg.String() == "g" {
			switch {
			case a.focus == FocusLeft:
				a.sectionList.CursorTop()
				a.refreshAfterCursorMove()
			case a.isRawMode():
				a.linePane.CursorTop()
				a.syncSectionFromLineCursor()
			default:
				a.detail.Viewport().GotoTop()
				a.syncCursorToScroll()
			}
			return a, nil
		}
		// Not 'g' after pending g -- fall through to normal handling
	}

	// Check for 'g' (start of gg chord) and 'G' (go to bottom)
	switch msg.String() {
	case "g":
		a.pendingG = true
		return a, nil
	case "G":
		switch {
		case a.focus == FocusLeft:
			a.sectionList.CursorBottom()
			a.refreshAfterCursorMove()
		case a.isRawMode():
			a.linePane.CursorBottom()
			a.syncSectionFromLineCursor()
		default:
			a.detail.Viewport().GotoBottom()
			a.syncCursorToScroll()
		}
		return a, nil
	}

	switch {
	case key.Matches(msg, a.keymap.Quit):
		if msg.String() == "ctrl+c" {
			a.result.Status = markdown.StatusCancelled
			return a, tea.Quit
		}
		a.confirmAction = confirmQuit
		a.mode = ModeConfirm
		return a, nil

	case key.Matches(msg, a.keymap.Help):
		a.mode = ModeHelp
		return a, nil

	case key.Matches(msg, a.keymap.Tab):
		if a.focus == FocusLeft {
			a.focus = FocusRight
		} else {
			a.focus = FocusLeft
		}
		return a, nil

	case key.Matches(msg, a.keymap.Submit):
		a.confirmAction = confirmSubmit
		a.mode = ModeConfirm
		return a, nil

	case key.Matches(msg, a.keymap.PaneGrow):
		a.resizeLeftPane(5)
		return a, nil

	case key.Matches(msg, a.keymap.PaneShrink):
		a.resizeLeftPane(-5)
		return a, nil

	case key.Matches(msg, a.keymap.FullView):
		a.fullView = !a.fullView
		if a.isRawMode() {
			a.updateLinePaneViewRange()
		}
		a.refreshDetail()
		return a, nil

	case key.Matches(msg, a.keymap.RawView):
		if a.linePane != nil {
			a.rawView = !a.rawView
			if a.rawView {
				// Switch to raw view
				a.focus = FocusRight
				a.updateLinePaneViewRange()
				a.linePane.CursorTop()
				a.refreshLinePane()
			} else {
				// Switch back to glamour view
				a.refreshDetail()
			}
			return a, nil
		}
	}

	// Horizontal scroll keys apply to the detail pane regardless of focus
	switch {
	case key.Matches(msg, a.keymap.ScrollRight):
		a.detail.Viewport().ScrollRight(4)
		return a, nil
	case key.Matches(msg, a.keymap.ScrollLeft):
		a.detail.Viewport().ScrollLeft(4)
		return a, nil
	case key.Matches(msg, a.keymap.ScrollToStart):
		a.detail.Viewport().SetXOffset(0)
		return a, nil
	case key.Matches(msg, a.keymap.ScrollToEnd):
		a.detail.Viewport().SetXOffset(scrollToEnd)
		return a, nil
	}

	// Page scroll keys dispatch based on focus
	switch {
	case key.Matches(msg, a.keymap.HalfPageDown):
		switch {
		case a.focus == FocusLeft:
			a.sectionList.CursorHalfPageDown(a.contentHeight())
			a.refreshAfterCursorMove()
		case a.isRawMode():
			a.linePane.HalfPageDown()
			a.syncSectionFromLineCursor()
		default:
			a.detail.Viewport().HalfPageDown()
			a.syncCursorToScroll()
		}
		return a, nil
	case key.Matches(msg, a.keymap.HalfPageUp):
		switch {
		case a.focus == FocusLeft:
			a.sectionList.CursorHalfPageUp(a.contentHeight())
			a.refreshAfterCursorMove()
		case a.isRawMode():
			a.linePane.HalfPageUp()
			a.syncSectionFromLineCursor()
		default:
			a.detail.Viewport().HalfPageUp()
			a.syncCursorToScroll()
		}
		return a, nil
	case key.Matches(msg, a.keymap.PageDown):
		switch {
		case a.focus == FocusLeft:
			a.sectionList.CursorPageDown(a.contentHeight())
			a.refreshAfterCursorMove()
		case a.isRawMode():
			a.linePane.PageDown()
			a.syncSectionFromLineCursor()
		default:
			a.detail.Viewport().PageDown()
			a.syncCursorToScroll()
		}
		return a, nil
	case key.Matches(msg, a.keymap.PageUp):
		switch {
		case a.focus == FocusLeft:
			a.sectionList.CursorPageUp(a.contentHeight())
			a.refreshAfterCursorMove()
		case a.isRawMode():
			a.linePane.PageUp()
			a.syncSectionFromLineCursor()
		default:
			a.detail.Viewport().PageUp()
			a.syncCursorToScroll()
		}
		return a, nil
	}

	if a.focus == FocusLeft {
		return a.handleLeftPaneKeys(msg)
	}
	return a.handleRightPaneKeys(msg)
}

func (a *App) handleLeftPaneKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keymap.Up):
		a.sectionList.CursorUp()
		a.refreshAfterCursorMove()

	case key.Matches(msg, a.keymap.Down):
		a.sectionList.CursorDown()
		a.refreshAfterCursorMove()

	case key.Matches(msg, a.keymap.Toggle):
		a.sectionList.ToggleExpand()

	case key.Matches(msg, a.keymap.Comment):
		if a.rawView {
			// In raw view, section-level comments are disabled from left pane
			return a, nil
		}
		sectionID := a.selectedSectionID()
		if sectionID != "" {
			a.editCommentIdx = -1
			cmd := a.comment.Open(sectionID, nil)
			a.mode = ModeComment
			return a, cmd
		}

	case key.Matches(msg, a.keymap.CommentList):
		sectionID := a.selectedSectionID()
		if sectionID != "" {
			comments := a.sectionList.GetComments(sectionID)
			if len(comments) > 0 {
				a.commentList.Open(sectionID, comments)
				a.mode = ModeCommentList
			}
		}

	case key.Matches(msg, a.keymap.Viewed):
		if section := a.sectionList.Selected(); section != nil {
			a.sectionList.ToggleViewed(section.ID)
		}

	case key.Matches(msg, a.keymap.Search):
		cmd := a.search.Open()
		a.mode = ModeSearch
		return a, cmd
	}

	return a, nil
}

func (a *App) handleRightPaneKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.isRawMode() {
		return a.handleLinePaneKeys(msg)
	}
	switch {
	case key.Matches(msg, a.keymap.Up):
		a.detail.Viewport().ScrollUp(1)
		a.syncCursorToScroll()
	case key.Matches(msg, a.keymap.Down):
		a.detail.Viewport().ScrollDown(1)
		a.syncCursorToScroll()
	}
	return a, nil
}

func (a *App) handleLinePaneKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keymap.Up):
		if !a.fullView && a.linePane.AtRangeTop() {
			prevID := a.selectedSectionID()
			a.sectionList.CursorUp()
			if a.selectedSectionID() != prevID {
				a.updateLinePaneViewRange()
				a.linePane.CursorBottom()
			}
		} else {
			a.linePane.CursorUp()
			a.syncSectionFromLineCursor()
		}
	case key.Matches(msg, a.keymap.Down):
		if !a.fullView && a.linePane.AtRangeBottom() {
			prevID := a.selectedSectionID()
			a.sectionList.CursorDown()
			if a.selectedSectionID() != prevID {
				a.updateLinePaneViewRange()
				a.linePane.CursorTop()
			}
		} else {
			a.linePane.CursorDown()
			a.syncSectionFromLineCursor()
		}
	case key.Matches(msg, a.keymap.Comment):
		if !a.linePane.CanComment() {
			return a, nil
		}
		startLine, endLine := a.linePane.SelectedRange()
		sectionID := a.linePane.SectionIDAtLine(startLine)
		a.editCommentIdx = -1
		cmd := a.comment.OpenWithLines(sectionID, nil, startLine, endLine, a.linePane.CursorSide())
		a.mode = ModeComment
		return a, cmd
	case key.Matches(msg, a.keymap.VisualSelect):
		a.linePane.StartVisualSelect()
		a.mode = ModeLineSelect
	case key.Matches(msg, a.keymap.CommentList):
		sectionID := a.linePane.SectionIDAtLine(a.linePane.Cursor() + 1)
		comments := a.sectionList.GetComments(sectionID)
		if len(comments) > 0 {
			a.commentList.Open(sectionID, comments)
			a.mode = ModeCommentList
		}
	}
	return a, nil
}

func (a *App) handleLineSelectMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keymap.Up):
		a.linePane.CursorUp()
		a.syncSectionFromLineCursor()
	case key.Matches(msg, a.keymap.Down):
		a.linePane.CursorDown()
		a.syncSectionFromLineCursor()
	case key.Matches(msg, a.keymap.Comment):
		startLine, endLine := a.linePane.SelectedRange()
		if startLine == 0 {
			return a, nil // no valid diff line selected
		}
		sectionID := a.linePane.SectionIDAtLine(startLine)
		a.linePane.CancelVisualSelect()
		a.editCommentIdx = -1
		cmd := a.comment.OpenWithLines(sectionID, nil, startLine, endLine, a.linePane.CursorSide())
		a.mode = ModeComment
		return a, cmd
	case key.Matches(msg, a.keymap.Cancel):
		a.linePane.CancelVisualSelect()
		a.mode = ModeNormal
	}
	return a, nil
}

// syncSectionFromLineCursor updates the left pane cursor to match the section
// containing the current line cursor position.
func (a *App) syncSectionFromLineCursor() {
	if a.linePane == nil {
		return
	}
	sectionID := a.linePane.SectionIDAtLine(a.linePane.Cursor() + 1)
	if sectionID != "" {
		a.sectionList.SelectBySectionID(sectionID)
	}
}

func (a *App) handleCommentMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keymap.Save):
		result := a.comment.Result()
		if result != nil {
			if a.editCommentIdx >= 0 {
				a.sectionList.UpdateComment(a.comment.SectionID(), a.editCommentIdx, result)
			} else {
				a.sectionList.AddComment(a.comment.SectionID(), result)
			}
		}
		a.returnFromComment()
		a.refreshDetail()
		return a, nil

	case key.Matches(msg, a.keymap.Cancel):
		a.returnFromComment()
		return a, nil

	case key.Matches(msg, a.keymap.CommentCycleDeco):
		a.comment.CycleDecoration()
		return a, nil

	case key.Matches(msg, a.keymap.CommentLabelPrev):
		a.comment.CycleLabelReverse()
		return a, nil

	case key.Matches(msg, a.keymap.CommentLabelNext):
		a.comment.CycleLabel()
		return a, nil
	}

	cmd := a.comment.Update(msg)
	return a, cmd
}

// returnFromComment closes the comment editor and returns to the appropriate mode.
func (a *App) returnFromComment() {
	a.comment.Close()
	if a.editCommentIdx >= 0 {
		comments := a.sectionList.GetComments(a.comment.SectionID())
		a.commentList.Open(a.comment.SectionID(), comments)
		a.mode = ModeCommentList
	} else {
		a.mode = ModeNormal
	}
	a.editCommentIdx = -1
}

func (a *App) handleCommentListMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, a.keymap.Cancel) {
		a.commentList.Close()
		a.mode = ModeNormal
		a.refreshDetail()
		return a, nil
	}

	switch {
	case key.Matches(msg, a.keymap.Up):
		a.commentList.CursorUp()
		return a, nil
	case key.Matches(msg, a.keymap.Down):
		a.commentList.CursorDown()
		return a, nil
	}

	switch {
	case key.Matches(msg, a.keymap.Edit):
		// Edit selected comment
		sectionID := a.commentList.SectionID()
		idx := a.commentList.Cursor()
		comments := a.sectionList.GetComments(sectionID)
		if idx >= 0 && idx < len(comments) {
			a.editCommentIdx = idx
			cmd := a.comment.Open(sectionID, comments[idx])
			a.mode = ModeComment
			return a, cmd
		}
	case key.Matches(msg, a.keymap.Delete):
		// Delete selected comment
		sectionID := a.commentList.SectionID()
		idx := a.commentList.Cursor()
		a.sectionList.DeleteComment(sectionID, idx)
		comments := a.sectionList.GetComments(sectionID)
		if len(comments) == 0 {
			a.commentList.Close()
			a.mode = ModeNormal
		} else {
			a.commentList.Open(sectionID, comments)
		}
		a.refreshDetail()
		return a, nil
	}

	return a, nil
}

func (a *App) handleConfirmMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		return a.executeConfirm()
	case "n", "N", "esc":
		a.mode = ModeNormal
		return a, nil
	case "ctrl+c":
		a.result.Status = markdown.StatusCancelled
		return a, tea.Quit
	}
	return a, nil
}

// executeConfirm performs the action pending in confirmAction.
func (a *App) executeConfirm() (tea.Model, tea.Cmd) {
	switch a.confirmAction {
	case confirmSubmit:
		return a.submitReview()
	case confirmQuit:
		a.result.Status = markdown.StatusCancelled
		return a, tea.Quit
	}
	return a, nil
}

func (a *App) handleHelpMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keymap.Cancel),
		key.Matches(msg, a.keymap.Help),
		key.Matches(msg, a.keymap.Toggle),
		msg.String() == "q":
		a.mode = ModeNormal
	}
	return a, nil
}

func (a *App) handleSearchMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keymap.Toggle):
		// Confirm search, stay at current cursor position
		a.search.Close()
		a.mode = ModeNormal
		a.refreshDetail()
		return a, nil

	case key.Matches(msg, a.keymap.Cancel):
		// Cancel search, restore full list
		a.search.Close()
		a.sectionList.ClearFilter()
		a.mode = ModeNormal
		a.refreshDetail()
		return a, nil
	}

	// Handle navigation within search results
	switch {
	case key.Matches(msg, a.keymap.Up):
		a.sectionList.CursorUp()
		a.refreshAfterCursorMove()
		return a, nil
	case key.Matches(msg, a.keymap.Down):
		a.sectionList.CursorDown()
		a.refreshAfterCursorMove()
		return a, nil
	}

	// Update search input
	cmd := a.search.Update(msg)
	// Apply filter with updated query
	a.sectionList.FilterByQuery(a.search.Query())
	a.refreshDetail()
	return a, cmd
}

func (a *App) submitReview() (tea.Model, tea.Cmd) {
	review := a.sectionList.BuildReviewResult()

	if len(review.Comments) == 0 {
		a.result.Status = markdown.StatusApproved
	} else {
		a.result.Status = markdown.StatusSubmitted
	}
	a.result.Review = review

	return a, tea.Quit
}

func (a *App) syncCursorToScroll() {
	if a.isRawMode() {
		a.syncSectionFromLineCursor()
		return
	}
	if !a.fullView || a.detail == nil {
		return
	}
	sectionID := a.detail.SectionIDAtOffset(a.detail.Viewport().YOffset())
	if sectionID == "" {
		return
	}
	a.sectionList.SelectBySectionID(sectionID)
}

// refreshAfterCursorMove updates the detail pane after cursor movement.
// In full view, scrolls to the selected section; otherwise, refreshes the detail content.
func (a *App) refreshAfterCursorMove() {
	if a.isRawMode() {
		if a.fullView {
			// Full view: scroll to section
			if section := a.sectionList.Selected(); section != nil && section.StartLine > 0 {
				a.linePane.ScrollToLine(section.StartLine)
			}
		} else {
			// Section view: update view range and reset to top
			a.updateLinePaneViewRange()
			a.linePane.CursorTop()
		}
		return
	}
	if a.fullView {
		a.scrollDetailToSelected()
	} else {
		a.refreshDetail()
	}
}

func (a *App) scrollDetailToSelected() {
	if a.detail == nil {
		return
	}
	if a.sectionList.IsOverviewSelected() {
		a.detail.ScrollToSectionID("")
		return
	}
	if section := a.sectionList.Selected(); section != nil {
		a.detail.ScrollToSectionID(section.ID)
	}
}

func (a *App) refreshDetail() {
	if a.isRawMode() {
		a.refreshLinePane()
		return
	}

	if a.detail == nil {
		return
	}

	if a.fullView {
		a.detail.ShowAll(a.doc, a.sectionList.GetComments)
		return
	}

	if a.sectionList.IsOverviewSelected() {
		comments := a.sectionList.GetComments(markdown.OverviewSectionID)
		a.detail.ShowOverview(a.doc, comments)
		return
	}

	if section := a.sectionList.Selected(); section != nil {
		comments := a.sectionList.GetComments(section.ID)
		a.detail.ShowSection(section, comments)
	}
}

func (a *App) refreshLinePane() {
	if a.linePane == nil {
		return
	}
	a.updateLinePaneViewRange()
	// Collect all comments (both section-level and line-level) for inline display
	var allComments []*markdown.ReviewComment
	overviewComments := a.sectionList.GetComments(markdown.OverviewSectionID)
	allComments = append(allComments, overviewComments...)
	for _, s := range a.doc.AllSections() {
		allComments = append(allComments, a.sectionList.GetComments(s.ID)...)
	}
	a.linePane.SetComments(allComments)
}

// updateLinePaneViewRange sets the linePane view range based on fullView and selected section.
func (a *App) updateLinePaneViewRange() {
	if a.linePane == nil {
		return
	}
	if a.fullView {
		a.linePane.ClearViewRange()
		return
	}
	// Section view: show only the selected section's lines
	if a.sectionList.IsOverviewSelected() {
		// Overview: show from line 1 to the start of the first section
		sections := a.doc.AllSections()
		if len(sections) > 0 && sections[0].StartLine > 1 {
			a.linePane.SetViewRange(1, sections[0].StartLine-1)
		} else {
			a.linePane.SetViewRange(1, a.linePane.LineCount())
		}
		return
	}
	if section := a.sectionList.Selected(); section != nil {
		endLine := section.EndLine
		if endLine <= 0 {
			endLine = a.linePane.LineCount()
		}
		a.linePane.SetViewRange(section.StartLine, endLine)
		return
	}
	a.linePane.ClearViewRange()
}

// selectedSectionID returns the section ID of the currently selected item.
// Returns OverviewSectionID for overview, the section's ID for sections, or "" if nothing is selected.
func (a *App) selectedSectionID() string {
	if a.sectionList.IsOverviewSelected() {
		return markdown.OverviewSectionID
	}
	if section := a.sectionList.Selected(); section != nil {
		return section.ID
	}
	return ""
}

func (a *App) renderTitleBar() string {
	if a.width == 0 {
		return ""
	}

	var parts []string
	if a.doc.Title != "" {
		parts = append(parts, a.doc.Title)
	}
	if a.opts.FilePath != "" {
		parts = append(parts, "("+a.opts.FilePath+")")
	}

	if len(parts) == 0 {
		return ""
	}

	content := a.styles.Title.Render(strings.Join(parts, " "))
	// In lipgloss v2, Width(N) sets the total width including borders,
	// so passing a.width here makes the title bar exactly a.width cells wide.
	return a.styles.InactiveBorder.
		Width(a.width).
		Render(content)
}

func (a *App) titleBarHeight() int {
	tb := a.renderTitleBar()
	if tb == "" {
		return 0
	}
	return lipgloss.Height(tb)
}

func (a *App) contentHeight() int {
	return a.contentHeightWith(a.titleBarHeight())
}

// contentHeightWith returns the pane height (including border) for the given title bar height.
// Layout: tbHeight + \n + pane + \n + statusBar(1) = a.height
func (a *App) contentHeightWith(tbHeight int) int {
	return max(a.height-tbHeight-3, 4)
}

func (a *App) resizeLeftPane(delta int) {
	newRatio := a.leftRatio + delta
	if a.width < 80 || newRatio < 10 || newRatio > 50 {
		return
	}
	a.leftRatio = newRatio
	a.updateLayout()
	a.refreshDetail()
}

func (a *App) leftWidth() int {
	if a.width < 80 {
		return a.width
	}
	return a.width * a.leftRatio / 100
}

// rightWidth returns the right pane's total width (including its borders).
// In lipgloss v2, Width(N) sets the total width including borders, so the two
// panes' widths sum to a.width with no extra subtraction needed.
func (a *App) rightWidth() int {
	if a.width < 80 {
		return a.width
	}
	return a.width - a.leftWidth()
}

func (a *App) updateLayout() {
	ch := a.contentHeight()
	innerH := paneInnerSize(ch)
	rw := a.rightWidth()
	innerW := paneInnerSize(rw)

	if a.detail == nil {
		a.detail = NewDetailPane(innerW, innerH, a.opts.Theme)
	} else {
		a.detail.SetSize(innerW, innerH)
	}

	if a.linePane != nil {
		a.linePane.SetSize(innerW, innerH)
	}

	a.comment.SetWidth(innerW)
}

// View implements tea.Model.
func (a *App) View() tea.View {
	return altScreenView(a.renderApp())
}

// renderApp returns the rendered string content for the current state.
func (a *App) renderApp() string {
	if !a.ready {
		return "Loading..."
	}

	// Full-screen overlay modes
	switch a.mode {
	case ModeHelp:
		return a.renderHelp()
	case ModeConfirm:
		return a.renderConfirm()
	}

	// Title bar (full width, above panes) -- computed once
	titleBar := a.renderTitleBar()
	tbHeight := 0
	if titleBar != "" {
		tbHeight = lipgloss.Height(titleBar)
	}
	ch := a.contentHeightWith(tbHeight)
	innerH := paneInnerSize(ch)
	lw := a.leftWidth()
	rw := a.rightWidth()
	leftInnerW := paneInnerSize(lw)
	rightInnerW := paneInnerSize(rw)
	singlePane := a.width < 80

	// Left pane
	leftContent := clipLines(a.sectionList.Render(leftInnerW, innerH, a.styles), innerH)
	leftBorder := a.styles.InactiveBorder
	if a.focus == FocusLeft {
		leftBorder = a.styles.ActiveBorder
	}
	leftPane := leftBorder.
		Width(lw).
		Height(ch).
		MaxHeight(ch).
		Render(leftContent)

	if singlePane {
		var pane string
		if a.focus == FocusRight {
			rightContent := clipLines(a.renderRightContent(rightInnerW, innerH), innerH)
			rightBorder := a.styles.ActiveBorder
			pane = rightBorder.Width(rw).Height(ch).MaxHeight(ch).Render(rightContent)
		} else {
			pane = leftPane
		}
		if titleBar != "" {
			return titleBar + "\n" + pane + "\n" + a.renderStatusBar()
		}
		return pane + "\n" + a.renderStatusBar()
	}

	// Right pane
	rightContent := clipLines(a.renderRightContent(rightInnerW, innerH), innerH)
	rightBorder := a.styles.InactiveBorder
	if a.focus == FocusRight {
		rightBorder = a.styles.ActiveBorder
	}
	rightPane := rightBorder.
		Width(rw).
		Height(ch).
		MaxHeight(ch).
		Render(rightContent)

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	var result string
	if titleBar != "" {
		result = titleBar + "\n" + content + "\n" + a.renderStatusBar()
	} else {
		result = content + "\n" + a.renderStatusBar()
	}

	return result
}

func (a *App) renderRightContent(width, height int) string {
	switch a.mode {
	case ModeComment:
		// Show line ref in comment header when editing a line-level comment
		commentLabel := "Comment [" + a.comment.FormatLabel() + "]"
		if ref := a.comment.FormatLineRef(); ref != "" {
			commentLabel += " (" + ref + ")"
		}
		separator := a.styles.CommentBorder.Width(width).Render(commentLabel)
		commentView := a.comment.View()

		// \n between parts does not add extra lines in lipgloss.Height
		sepHeight := lipgloss.Height(separator)
		commentViewHeight := lipgloss.Height(commentView)
		detailHeight := max(height-sepHeight-commentViewHeight, 1)

		var detailView string
		if a.isRawMode() {
			a.linePane.SetSize(width, detailHeight)
			detailView = a.linePane.View()
		} else {
			a.detail.SetSize(width, detailHeight)
			detailView = a.detail.View()
		}
		return clipLines(detailView, detailHeight) + "\n" + separator + "\n" + commentView

	case ModeCommentList:
		return a.commentList.Render(width, height, a.styles)

	case ModeLineSelect:
		if a.linePane != nil {
			a.linePane.SetSize(width, height)
			return a.linePane.View()
		}
		return ""
	}

	if a.isRawMode() {
		a.linePane.SetSize(width, height)
		return a.linePane.View()
	}

	return a.detail.View()
}

func (a *App) statusEntry(key, label string) string {
	return a.styles.StatusKey.Render(key) + " " + label
}

func (a *App) renderStatusBar() string {
	if a.mode == ModeComment {
		return a.styles.StatusBar.Render(
			a.statusEntry("tab/S-tab", "label:") + " " +
				a.styles.Title.Render(a.comment.FormatLabel()) + "  " +
				a.statusEntry("ctrl+d", "deco") + "  " +
				a.statusEntry("ctrl+s", "save") + "  " +
				a.statusEntry("esc", "cancel"),
		)
	}

	if a.mode == ModeCommentList {
		return a.styles.StatusBar.Render(
			a.statusEntry("j/k", "navigate") + "  " +
				a.statusEntry("e", "edit") + "  " +
				a.statusEntry("d", "delete") + "  " +
				a.statusEntry("esc", "back"),
		)
	}

	if a.mode == ModeLineSelect {
		lineInfo := ""
		if a.linePane != nil {
			startLine, endLine := a.linePane.SelectedRange()
			lineInfo = markdown.FormatLineRef(startLine, endLine)
		}
		return a.styles.StatusBar.Render(
			a.styles.Title.Render("VISUAL") + "  " +
				a.statusEntry("j/k", "extend") + "  " +
				a.statusEntry("c", "comment") + "  " +
				a.statusEntry("esc", "cancel") + "  " +
				lineInfo,
		)
	}

	if a.mode == ModeSearch {
		return a.search.View()
	}

	if a.isRawMode() {
		lineInfo := fmt.Sprintf("L%d/%d", a.linePane.Cursor()+1, a.linePane.LineCount())
		progress := ""
		if commentCount := a.sectionList.TotalCommentCount(); commentCount > 0 {
			progress = fmt.Sprintf(" [%d comments]", commentCount)
		}
		// Label shows the mode that f will switch TO (not the current mode)
		viewMode := "full"
		if a.fullView {
			viewMode = "section"
		}

		return a.styles.StatusBar.Render(
			a.statusEntry("r", "render") + "  " +
				a.statusEntry("f", viewMode) + "  " +
				a.statusEntry("c", "comment") + "  " +
				a.statusEntry("V", "select") + "  " +
				a.statusEntry("C", "comments") + "  " +
				a.statusEntry("s", "submit") + "  " +
				a.statusEntry("tab", "switch") + "  " +
				a.statusEntry("?", "help") + "  " +
				a.statusEntry("q", "quit") + "  " +
				lineInfo + progress,
		)
	}

	// Label shows the mode that f will switch TO (not the current mode)
	viewMode := "full"
	if a.fullView {
		viewMode = "section"
	}

	progress := fmt.Sprintf("[%d/%d viewed]", a.sectionList.ViewedCount(), a.sectionList.TotalSectionCount())
	if commentCount := a.sectionList.TotalCommentCount(); commentCount > 0 {
		progress += fmt.Sprintf(" [%d comments]", commentCount)
	}

	rawToggle := ""
	if a.linePane != nil {
		rawToggle = a.statusEntry("r", "raw") + "  "
	}

	return a.styles.StatusBar.Render(
		a.statusEntry("enter", "toggle") + "  " +
			a.statusEntry("f", viewMode) + "  " +
			rawToggle +
			a.statusEntry("c", "comment") + "  " +
			a.statusEntry("C", "comments") + "  " +
			a.statusEntry("v", "viewed") + "  " +
			a.statusEntry("/", "search") + "  " +
			a.statusEntry("s", "submit") + "  " +
			a.statusEntry("tab", "switch") + "  " +
			a.statusEntry("?", "help") + "  " +
			a.statusEntry("q", "quit") + "  " +
			progress,
	)
}

// renderConfirm renders a full-screen confirmation dialog.
func (a *App) renderConfirm() string {
	var message string
	switch a.confirmAction {
	case confirmSubmit:
		if a.opts.PRMode {
			message = fmt.Sprintf("Finish reviewing this file? (%d comments)", a.sectionList.TotalCommentCount())
		} else {
			message = fmt.Sprintf("Submit review? (%d comments)", a.sectionList.TotalCommentCount())
		}
	case confirmQuit:
		switch {
		case a.opts.PRMode:
			message = "Skip this file?"
		case a.sectionList.HasComments():
			message = "You have review comments.\n\nQuit without submitting?"
		default:
			message = "Quit review?"
		}
	}

	dialog := lipgloss.NewStyle().
		Width(40).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("170")).
		Render(
			message + "\n\n" +
				a.styles.StatusKey.Render("y") + " yes   " +
				a.styles.StatusKey.Render("n") + " no   " +
				a.styles.StatusKey.Render("esc") + " cancel",
		)

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, dialog)
}

// clipLines truncates a string to at most maxLines lines.
func clipLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	pos := 0
	for range maxLines {
		idx := strings.IndexByte(s[pos:], '\n')
		if idx < 0 {
			return s
		}
		pos += idx + 1
	}
	return s[:pos-1]
}

func (a *App) renderHelp() string {
	rawViewHelp := ""
	if a.linePane != nil {
		rawViewHelp = `
  Raw Source View (r to toggle):
    j/k             Move line cursor
    c               Add line comment at cursor
    V               Start visual line selection
    V + j/k + c     Comment on selected range
    Esc             Cancel visual selection
    C               Manage comments for section at cursor
`
	}
	help := fmt.Sprintf(`%s

  Navigation:
    j/k, Up/Down   Move cursor up/down
    gg              Go to top
    G               Go to bottom
    Enter           Toggle expand/collapse
    f               Toggle full/section view
    r               Toggle raw source/rendered view
    h/l, Left/Right Scroll detail pane left/right
    H/L             Scroll detail to start/end
    Ctrl+D/Ctrl+U   Half page down/up
    Ctrl+F/Ctrl+B   Full page down/up
    >/<             Resize left pane
    Tab             Switch between left/right pane

  Review:
    c               Add comment on selected section
    C               Manage comments (edit/delete)
    v               Toggle viewed mark
    /               Search sections
    s               Submit review
%s
  Comment Editor:
    Tab             Cycle label (forward)
    Shift+Tab       Cycle label (reverse)
    Ctrl+D          Cycle decoration
    Ctrl+S          Save comment
    Esc             Cancel editing

  Other:
    ?               Toggle this help
    q, Ctrl+C       Quit

  Press Esc or ? or q to close this help.
`, a.styles.Title.Render("commd - Help"), rawViewHelp)

	return clipLines(help, a.height)
}
