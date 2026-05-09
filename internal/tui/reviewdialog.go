package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ReviewAction represents the user's chosen action in the review dialog.
type ReviewAction int

const (
	ReviewActionExit    ReviewAction = iota // do nothing
	ReviewActionApprove                     // approve the PR
	ReviewActionComment                     // submit as comment
)

// ReviewDialogResult holds the outcome of the review dialog.
type ReviewDialogResult struct {
	Action ReviewAction
	Body   string // optional PR-level comment
}

// reviewDialogMode tracks the dialog's internal state.
type reviewDialogMode int

const (
	dialogModeSelect reviewDialogMode = iota
	dialogModeBody
)

// ReviewDialog is a Bubble Tea model for the post-review action selection.
type ReviewDialog struct {
	// Input
	fileSummary []string // "file.md: N comment(s)" lines
	hasComments bool

	// State
	mode     reviewDialogMode
	cursor   int
	options  []string
	actions  []ReviewAction
	textarea textarea.Model
	result   ReviewDialogResult
	quitting bool
	width    int
	height   int
}

// NewReviewDialog creates a review dialog.
// fileSummary contains "file.md: N comment(s)" lines for display.
func NewReviewDialog(fileSummary []string, hasComments bool) *ReviewDialog {
	ta := textarea.New()
	ta.Placeholder = "PR comment (optional)..."
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.SetVirtualCursor(true)

	d := &ReviewDialog{
		fileSummary: fileSummary,
		hasComments: hasComments,
		textarea:    ta,
	}

	if hasComments {
		d.options = []string{"Comment", "Approve", "Exit"}
		d.actions = []ReviewAction{ReviewActionComment, ReviewActionApprove, ReviewActionExit}
	} else {
		d.options = []string{"Approve", "Exit"}
		d.actions = []ReviewAction{ReviewActionApprove, ReviewActionExit}
	}

	return d
}

// Result returns the dialog result.
func (d *ReviewDialog) Result() ReviewDialogResult {
	return d.result
}

// Init implements tea.Model.
func (d *ReviewDialog) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (d *ReviewDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		d.textarea.SetWidth(min(msg.Width-8, 50))
		return d, nil
	case tea.KeyPressMsg:
		if d.mode == dialogModeBody {
			return d.updateBody(msg)
		}
		return d.updateSelect(msg)
	}
	return d, nil
}

func (d *ReviewDialog) updateSelect(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"))):
		d.result.Action = ReviewActionExit
		d.quitting = true
		return d, tea.Quit
	case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
		if d.cursor < len(d.options)-1 {
			d.cursor++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
		if d.cursor > 0 {
			d.cursor--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		action := d.actions[d.cursor]
		if action == ReviewActionExit {
			d.result.Action = ReviewActionExit
			d.quitting = true
			return d, tea.Quit
		}
		// Approve or Comment: move to body input
		d.result.Action = action
		d.mode = dialogModeBody
		return d, d.textarea.Focus()
	}
	return d, nil
}

func (d *ReviewDialog) updateBody(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		d.mode = dialogModeSelect
		d.textarea.Blur()
		return d, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+s"))):
		d.result.Body = strings.TrimSpace(d.textarea.Value())
		d.quitting = true
		return d, tea.Quit
	}

	var cmd tea.Cmd
	d.textarea, cmd = d.textarea.Update(msg)
	return d, cmd
}

// View implements tea.Model.
func (d *ReviewDialog) View() tea.View {
	v := tea.NewView(d.renderView())
	v.AltScreen = true
	return v
}

// renderView returns the rendered string content for the current state.
func (d *ReviewDialog) renderView() string {
	if d.quitting {
		return ""
	}

	dialogWidth := min(d.width-4, 50)

	var content strings.Builder

	// Summary
	for _, line := range d.fileSummary {
		content.WriteString(line + "\n")
	}
	content.WriteString("\n")

	if d.mode == dialogModeSelect {
		for i, opt := range d.options {
			cursor := "  "
			if i == d.cursor {
				cursor = "▸ "
			}
			line := cursor + opt
			if i == d.cursor {
				line = lipgloss.NewStyle().
					Foreground(lipgloss.Color("14")).
					Bold(true).
					Render(line)
			}
			content.WriteString(line + "\n")
		}
		content.WriteString("\n")
		content.WriteString(
			lipgloss.NewStyle().Foreground(lipgloss.Color("8")).
				Render("↑/↓") + " navigate  " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("8")).
					Render("enter") + " select  " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("8")).
					Render("q") + " cancel",
		)
	} else {
		action := "Comment"
		if d.result.Action == ReviewActionApprove {
			action = "Approve"
		}
		fmt.Fprintf(&content, "Action: %s\n\n", lipgloss.NewStyle().Bold(true).Render(action))
		content.WriteString(d.textarea.View())
		content.WriteString("\n\n")
		content.WriteString(
			lipgloss.NewStyle().Foreground(lipgloss.Color("8")).
				Render("ctrl+s") + " submit  " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("8")).
					Render("esc") + " back",
		)
	}

	// Wrap in a styled border dialog (matching renderConfirm style)
	dialog := lipgloss.NewStyle().
		Width(dialogWidth).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("170")).
		Render(content.String())

	// Center on screen
	return lipgloss.Place(d.width, d.height,
		lipgloss.Center, lipgloss.Center,
		dialog)
}
