package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/koh-sh/ccplan/internal/plan"
)

// AppMode represents the current application mode.
type AppMode int

const (
	ModeNormal      AppMode = iota // Step list navigation
	ModeComment                    // Comment input
	ModeCommentList                // Comment list management
	ModeConfirm                    // Confirmation dialog
	ModeHelp                       // Help overlay
	ModeSearch                     // Step search
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
	Review *plan.ReviewResult
	Status plan.Status
}

// App is the main Bubble Tea model for the TUI.
type App struct {
	plan        *plan.Plan
	stepList    *StepList
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
	FilePath    string // plan file path (displayed in title bar)
	TrackViewed bool   // persist viewed state to sidecar file
}

// NewApp creates a new App model.
func NewApp(p *plan.Plan, opts AppOptions) *App {
	var state *plan.ViewedState
	if opts.TrackViewed && opts.FilePath != "" {
		state = plan.LoadViewedState(plan.StatePath(opts.FilePath))
	}
	return &App{
		plan:           p,
		stepList:       NewStepList(p, state),
		comment:        NewCommentEditor(),
		commentList:    NewCommentList(),
		search:         NewSearchBar(),
		keymap:         DefaultKeyMap(),
		styles:         stylesForTheme(opts.Theme),
		leftRatio:      30,
		opts:           opts,
		editCommentIdx: -1,
		result: AppResult{
			Status: plan.StatusCancelled,
		},
	}
}

// Result returns the final result after the TUI exits.
func (a *App) Result() AppResult {
	return a.result
}

// ViewedState returns the current viewed state for persistence.
func (a *App) ViewedState() *plan.ViewedState {
	return a.stepList.ViewedState()
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
				a.stepList.CursorTop()
				a.refreshAfterCursorMove()
			} else {
				a.detail.Viewport().GotoTop()
				a.syncCursorToScroll()
			}
			return a, nil
		}
		// Not 'g' after pending g — fall through to normal handling
	}

	// Check for 'g' (start of gg chord) and 'G' (go to bottom)
	switch msg.String() {
	case "g":
		a.pendingG = true
		return a, nil
	case "G":
		if a.focus == FocusLeft {
			a.stepList.CursorBottom()
			a.refreshAfterCursorMove()
		} else {
			a.detail.Viewport().GotoBottom()
			a.syncCursorToScroll()
		}
		return a, nil
	}

	switch {
	case key.Matches(msg, a.keymap.Quit):
		if a.stepList.HasComments() {
			a.mode = ModeConfirm
			return a, nil
		}
		a.result.Status = plan.StatusCancelled
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
		a.stepList.CursorUp()
		a.refreshAfterCursorMove()

	case key.Matches(msg, a.keymap.Down):
		a.stepList.CursorDown()
		a.refreshAfterCursorMove()

	case key.Matches(msg, a.keymap.Toggle):
		a.stepList.ToggleExpand()

	case key.Matches(msg, a.keymap.Comment):
		if step := a.stepList.Selected(); step != nil {
			a.editCommentIdx = -1
			a.comment.Open(step.ID, nil)
			a.mode = ModeComment
			return a, a.comment.textarea.Focus()
		}

	case key.Matches(msg, a.keymap.CommentList):
		if step := a.stepList.Selected(); step != nil {
			comments := a.stepList.GetComments(step.ID)
			if len(comments) > 0 {
				a.commentList.Open(step.ID, comments)
				a.mode = ModeCommentList
			}
		}

	case key.Matches(msg, a.keymap.Viewed):
		if step := a.stepList.Selected(); step != nil {
			a.stepList.ToggleViewed(step.ID)
		}

	case key.Matches(msg, a.keymap.Search):
		a.search.Open()
		a.mode = ModeSearch
		return a, a.search.input.Focus()
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
				a.stepList.UpdateComment(a.comment.StepID(), a.editCommentIdx, result)
			} else {
				a.stepList.AddComment(a.comment.StepID(), result)
			}
		}
		a.returnFromComment()
		a.refreshDetail()
		return a, nil

	case key.Matches(msg, a.keymap.Cancel):
		a.returnFromComment()
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
		comments := a.stepList.GetComments(a.comment.StepID())
		a.commentList.Open(a.comment.StepID(), comments)
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
		stepID := a.commentList.StepID()
		idx := a.commentList.Cursor()
		comments := a.stepList.GetComments(stepID)
		if idx >= 0 && idx < len(comments) {
			a.editCommentIdx = idx
			a.comment.Open(stepID, comments[idx])
			a.mode = ModeComment
			return a, a.comment.textarea.Focus()
		}
	case key.Matches(msg, a.keymap.Delete):
		// Delete selected comment
		stepID := a.commentList.StepID()
		idx := a.commentList.Cursor()
		a.stepList.DeleteComment(stepID, idx)
		comments := a.stepList.GetComments(stepID)
		if len(comments) == 0 {
			a.commentList.Close()
			a.mode = ModeNormal
		} else {
			a.commentList.Open(stepID, comments)
		}
		a.refreshDetail()
		return a, nil
	}

	return a, nil
}

func (a *App) handleConfirmMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		a.result.Status = plan.StatusCancelled
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
		a.result.Status = plan.StatusCancelled
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
		a.stepList.ClearFilter()
		a.mode = ModeNormal
		a.refreshDetail()
		return a, nil
	}

	// Handle navigation within search results
	switch {
	case key.Matches(msg, a.keymap.Up):
		a.stepList.CursorUp()
		a.refreshAfterCursorMove()
		return a, nil
	case key.Matches(msg, a.keymap.Down):
		a.stepList.CursorDown()
		a.refreshAfterCursorMove()
		return a, nil
	}

	// Update search input
	cmd := a.search.Update(msg)
	// Apply filter with updated query
	a.stepList.FilterByQuery(a.search.Query())
	a.refreshDetail()
	return a, cmd
}

func (a *App) submitReview() (tea.Model, tea.Cmd) {
	review := a.stepList.BuildReviewResult()

	if len(review.Comments) == 0 {
		a.result.Status = plan.StatusApproved
	} else {
		a.result.Status = plan.StatusSubmitted
	}
	a.result.Review = review

	return a, tea.Quit
}

func (a *App) syncCursorToScroll() {
	if !a.fullView || a.detail == nil {
		return
	}
	stepID := a.detail.StepIDAtOffset(a.detail.Viewport().YOffset)
	if stepID == "" {
		return
	}
	a.stepList.SelectByStepID(stepID)
}

// refreshAfterCursorMove updates the detail pane after cursor movement.
// In full view, scrolls to the selected step; otherwise, refreshes the detail content.
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
	if a.stepList.IsOverviewSelected() {
		a.detail.ScrollToStepID("")
		return
	}
	if step := a.stepList.Selected(); step != nil {
		a.detail.ScrollToStepID(step.ID)
	}
}

func (a *App) refreshDetail() {
	if a.detail == nil {
		return
	}

	if a.fullView {
		a.detail.ShowAll(a.plan, a.stepList.GetComments)
		return
	}

	if a.stepList.IsOverviewSelected() {
		a.detail.ShowOverview(a.plan)
		return
	}

	if step := a.stepList.Selected(); step != nil {
		comments := a.stepList.GetComments(step.ID)
		a.detail.ShowStep(step, comments)
	}
}

func (a *App) renderTitleBar() string {
	if a.width == 0 {
		return ""
	}

	innerWidth := a.width - 2
	var parts []string
	if a.plan.Title != "" {
		parts = append(parts, a.plan.Title)
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
	h := max(a.height-a.titleBarHeight()-3, 1)
	return h
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

	ch := a.contentHeight()
	lw := a.leftWidth()
	rw := a.rightWidth()
	singlePane := a.width < 80

	// Left pane: clip content BEFORE applying border
	leftContent := clipLines(a.stepList.Render(lw, ch, a.styles), ch)
	leftBorder := a.styles.InactiveBorder
	if a.focus == FocusLeft {
		leftBorder = a.styles.ActiveBorder
	}
	leftPane := leftBorder.
		Width(lw).
		Height(ch).
		Render(leftContent)

	// Title bar (full width, above panes)
	titleBar := a.renderTitleBar()

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

		separator := a.styles.CommentBorder.Width(width - 2).Render("Comment [" + string(a.comment.Label()) + "]")
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
				a.styles.Title.Render(string(a.comment.Label())) + "  " +
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

	progress := fmt.Sprintf("[%d/%d viewed]", a.stepList.ViewedCount(), a.stepList.TotalStepCount())
	if commentCount := a.stepList.TotalCommentCount(); commentCount > 0 {
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
    c               Add comment on selected step
    C               Manage comments (edit/delete)
    v               Toggle viewed mark
    /               Search steps
    s               Submit review

  Comment Editor:
    Tab             Cycle label (forward)
    Shift+Tab       Cycle label (reverse)
    Ctrl+S          Save comment
    Esc             Cancel editing

  Other:
    ?               Toggle this help
    q, Ctrl+C       Quit

  Press Esc or ? or q to close this help.
`, a.styles.Title.Render("ccplan review - Help"))

	return clipLines(help, a.height)
}
