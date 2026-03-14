package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	comment     *CommentEditor
	commentList *CommentList
	search      *SearchBar
	keymap      KeyMap
	styles      Styles

	mode      AppMode
	focus     Focus
	fullView  bool
	width     int
	height    int
	ready     bool
	leftRatio int // left pane width percentage (default 30, range 10-50)
	opts      AppOptions

	result         AppResult
	pendingG       bool // gg chord: true when first 'g' was pressed
	editCommentIdx int  // index of comment being edited in comment list mode (-1 = new)
}

// AppOptions configures the TUI appearance.
type AppOptions struct {
	Theme       string // "dark" or "light"
	FilePath    string // file path (displayed in title bar)
	TrackViewed bool   // persist viewed state to sidecar file
}

// NewApp creates a new App model.
func NewApp(p *markdown.Document, opts AppOptions) *App {
	var state *markdown.ViewedState
	if opts.TrackViewed && opts.FilePath != "" {
		state = markdown.LoadViewedState(markdown.StatePath(opts.FilePath))
	}
	return &App{
		doc:            p,
		sectionList:    NewSectionList(p, state),
		comment:        NewCommentEditor(),
		commentList:    NewCommentList(),
		search:         NewSearchBar(),
		keymap:         DefaultKeyMap(),
		styles:         stylesForTheme(opts.Theme),
		leftRatio:      30,
		opts:           opts,
		editCommentIdx: -1,
		result: AppResult{
			Status: markdown.StatusCancelled,
		},
	}
}

// Result returns the final result after the TUI exits.
func (a *App) Result() AppResult {
	return a.result
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

	case tea.KeyMsg:
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

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	}
	return a, nil
}

func (a *App) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle 'gg' chord (second g after pending g)
	if a.pendingG {
		a.pendingG = false
		if msg.String() == "g" {
			if a.focus == FocusLeft {
				a.sectionList.CursorTop()
				a.refreshAfterCursorMove()
			} else {
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
		if a.focus == FocusLeft {
			a.sectionList.CursorBottom()
			a.refreshAfterCursorMove()
		} else {
			a.detail.Viewport().GotoBottom()
			a.syncCursorToScroll()
		}
		return a, nil
	}

	switch {
	case key.Matches(msg, a.keymap.Quit):
		if a.sectionList.HasComments() {
			a.mode = ModeConfirm
			return a, nil
		}
		a.result.Status = markdown.StatusCancelled
		return a, tea.Quit

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
		return a.submitReview()

	case key.Matches(msg, a.keymap.PaneGrow):
		a.resizeLeftPane(5)
		return a, nil

	case key.Matches(msg, a.keymap.PaneShrink):
		a.resizeLeftPane(-5)
		return a, nil

	case key.Matches(msg, a.keymap.FullView):
		a.fullView = !a.fullView
		a.refreshDetail()
		return a, nil
	}

	// Horizontal scroll keys apply to the detail pane regardless of focus
	switch {
	case key.Matches(msg, a.keymap.Expand):
		a.detail.Viewport().ScrollRight(4)
		return a, nil
	case key.Matches(msg, a.keymap.Collapse):
		a.detail.Viewport().ScrollLeft(4)
		return a, nil
	case key.Matches(msg, a.keymap.ScrollToStart):
		a.detail.Viewport().SetXOffset(0)
		return a, nil
	case key.Matches(msg, a.keymap.ScrollToEnd):
		a.detail.Viewport().SetXOffset(scrollToEnd)
		return a, nil
	}

	if a.focus == FocusLeft {
		return a.handleLeftPaneKeys(msg)
	}
	return a.handleRightPaneKeys(msg)
}

func (a *App) handleLeftPaneKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if section := a.sectionList.Selected(); section != nil {
			a.editCommentIdx = -1
			cmd := a.comment.Open(section.ID, nil)
			a.mode = ModeComment
			return a, cmd
		}

	case key.Matches(msg, a.keymap.CommentList):
		if section := a.sectionList.Selected(); section != nil {
			comments := a.sectionList.GetComments(section.ID)
			if len(comments) > 0 {
				a.commentList.Open(section.ID, comments)
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

func (a *App) handleRightPaneKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (a *App) handleCommentMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	case msg.Type == tea.KeyCtrlD:
		a.comment.CycleDecoration()
		return a, nil

	case msg.Type == tea.KeyShiftTab:
		a.comment.CycleLabelReverse()
		return a, nil

	case msg.Type == tea.KeyTab:
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

func (a *App) handleCommentListMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
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

func (a *App) handleConfirmMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		a.result.Status = markdown.StatusCancelled
		return a, tea.Quit
	case "n", "N":
		a.mode = ModeNormal
		return a, nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		a.mode = ModeNormal
		return a, nil
	case tea.KeyCtrlC:
		a.result.Status = markdown.StatusCancelled
		return a, tea.Quit
	}
	return a, nil
}

func (a *App) handleHelpMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		a.mode = ModeNormal
		return a, nil
	}
	switch {
	case key.Matches(msg, a.keymap.Help):
		a.mode = ModeNormal
	case msg.String() == "enter", msg.String() == "q":
		a.mode = ModeNormal
	}
	return a, nil
}

func (a *App) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Confirm search, stay at current cursor position
		a.search.Close()
		a.mode = ModeNormal
		a.refreshDetail()
		return a, nil

	case tea.KeyEsc:
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
	if !a.fullView || a.detail == nil {
		return
	}
	sectionID := a.detail.SectionIDAtOffset(a.detail.Viewport().YOffset)
	if sectionID == "" {
		return
	}
	a.sectionList.SelectBySectionID(sectionID)
}

// refreshAfterCursorMove updates the detail pane after cursor movement.
// In full view, scrolls to the selected section; otherwise, refreshes the detail content.
func (a *App) refreshAfterCursorMove() {
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
	if a.detail == nil {
		return
	}

	if a.fullView {
		a.detail.ShowAll(a.doc, a.sectionList.GetComments)
		return
	}

	if a.sectionList.IsOverviewSelected() {
		a.detail.ShowOverview(a.doc)
		return
	}

	if section := a.sectionList.Selected(); section != nil {
		comments := a.sectionList.GetComments(section.ID)
		a.detail.ShowSection(section, comments)
	}
}

func (a *App) renderTitleBar() string {
	if a.width == 0 {
		return ""
	}

	innerWidth := a.width - 2
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
	return a.styles.InactiveBorder.
		Width(innerWidth).
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

func (a *App) contentHeightWith(tbHeight int) int {
	return max(a.height-tbHeight-3, 1)
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
		return a.width - 2
	}
	return a.width * a.leftRatio / 100
}

func (a *App) rightWidth() int {
	if a.width < 80 {
		return a.width - 2
	}
	return a.width - a.leftWidth() - 4
}

func (a *App) updateLayout() {
	ch := a.contentHeight()
	rw := a.rightWidth()

	if a.detail == nil {
		a.detail = NewDetailPane(rw, ch, a.opts.Theme)
	} else {
		a.detail.SetSize(rw, ch)
	}

	a.comment.SetWidth(rw - 2)
}

// View implements tea.Model.
func (a *App) View() string {
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
	lw := a.leftWidth()
	rw := a.rightWidth()
	singlePane := a.width < 80

	// Left pane: clip content BEFORE applying border
	leftContent := clipLines(a.sectionList.Render(lw, ch, a.styles), ch)
	leftBorder := a.styles.InactiveBorder
	if a.focus == FocusLeft {
		leftBorder = a.styles.ActiveBorder
	}
	leftPane := leftBorder.
		Width(lw).
		Height(ch).
		Render(leftContent)

	if singlePane {
		var pane string
		if a.focus == FocusRight {
			rightContent := clipLines(a.renderRightContent(rw, ch), ch)
			rightBorder := a.styles.ActiveBorder
			pane = rightBorder.Width(rw).Height(ch).Render(rightContent)
		} else {
			pane = leftPane
		}
		if titleBar != "" {
			return titleBar + "\n" + pane + "\n" + a.renderStatusBar()
		}
		return pane + "\n" + a.renderStatusBar()
	}

	// Right pane: clip content BEFORE applying border
	rightContent := clipLines(a.renderRightContent(rw, ch), ch)
	rightBorder := a.styles.InactiveBorder
	if a.focus == FocusRight {
		rightBorder = a.styles.ActiveBorder
	}
	rightPane := rightBorder.
		Width(rw).
		Height(ch).
		Render(rightContent)

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	if titleBar != "" {
		return titleBar + "\n" + content + "\n" + a.renderStatusBar()
	}
	return content + "\n" + a.renderStatusBar()
}

func (a *App) renderRightContent(width, height int) string {
	switch a.mode {
	case ModeComment:
		commentHeight := 7
		detailHeight := max(height-commentHeight-2, 1)

		a.detail.SetSize(width, detailHeight)
		detailView := a.detail.View()

		separator := a.styles.CommentBorder.Width(width - 2).Render("Comment [" + a.comment.FormatLabel() + "]")
		commentView := a.comment.View()

		return detailView + "\n" + separator + "\n" + commentView

	case ModeCommentList:
		return a.commentList.Render(width, height, a.styles)
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

	if a.mode == ModeSearch {
		return a.search.View()
	}

	viewMode := "full"
	if a.fullView {
		viewMode = "section"
	}

	progress := fmt.Sprintf("[%d/%d viewed]", a.sectionList.ViewedCount(), a.sectionList.TotalSectionCount())
	if commentCount := a.sectionList.TotalCommentCount(); commentCount > 0 {
		progress += fmt.Sprintf(" [%d comments]", commentCount)
	}

	return a.styles.StatusBar.Render(
		a.statusEntry("enter", "toggle") + "  " +
			a.statusEntry("f", viewMode) + "  " +
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
	dialog := lipgloss.NewStyle().
		Width(40).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("170")).
		Render(
			"You have review comments.\n\nQuit without submitting?\n\n" +
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
	help := fmt.Sprintf(`%s

  Navigation:
    j/k, Up/Down   Move cursor up/down
    gg              Go to top
    G               Go to bottom
    Enter           Toggle expand/collapse
    f               Toggle full/section view
    h/l, Left/Right Scroll detail pane left/right
    H/L             Scroll detail to start/end
    >/<             Resize left pane
    Tab             Switch between left/right pane

  Review:
    c               Add comment on selected section
    C               Manage comments (edit/delete)
    v               Toggle viewed mark
    /               Search sections
    s               Submit review

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
`, a.styles.Title.Render("commd - Help"))

	return clipLines(help, a.height)
}
