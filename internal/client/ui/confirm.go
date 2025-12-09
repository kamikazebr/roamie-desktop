package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ConfirmModel is the Bubble Tea model for confirmation prompts
type ConfirmModel struct {
	title       string
	description string
	cursor      int // 0 = Yes, 1 = No
	confirmed   bool
	quitting    bool
	aborted     bool
	yesLabel    string
	noLabel     string
}

// ConfirmOption configures the confirm dialog
type ConfirmOption func(*ConfirmModel)

// WithDescription adds a description to the confirm dialog
func WithDescription(desc string) ConfirmOption {
	return func(m *ConfirmModel) {
		m.description = desc
	}
}

// WithDefaultNo sets the default selection to No
func WithDefaultNo() ConfirmOption {
	return func(m *ConfirmModel) {
		m.cursor = 1
	}
}

// WithLabels sets custom labels for Yes/No
func WithLabels(yes, no string) ConfirmOption {
	return func(m *ConfirmModel) {
		m.yesLabel = yes
		m.noLabel = no
	}
}

// NewConfirm creates a new confirm dialog
func NewConfirm(title string, opts ...ConfirmOption) ConfirmModel {
	m := ConfirmModel{
		title:    title,
		cursor:   0, // Default to Yes
		yesLabel: "Sim",
		noLabel:  "Não",
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// Init implements tea.Model
func (m ConfirmModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.aborted = true
			m.quitting = true
			return m, tea.Quit

		case "left", "h", "right", "l", "tab":
			// Toggle between Yes and No
			m.cursor = 1 - m.cursor

		case "y", "Y":
			m.cursor = 0
			m.confirmed = true
			m.quitting = true
			return m, tea.Quit

		case "n", "N":
			m.cursor = 1
			m.confirmed = false
			m.quitting = true
			return m, tea.Quit

		case "enter", " ":
			m.confirmed = m.cursor == 0
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View implements tea.Model
func (m ConfirmModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render(m.title))
	b.WriteString("\n")

	// Description
	if m.description != "" {
		b.WriteString(HelpStyle.Render(m.description))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Options
	yesStyle := UnselectedStyle
	noStyle := UnselectedStyle
	yesCursor := "  "
	noCursor := "  "

	if m.cursor == 0 {
		yesStyle = SelectedStyle
		yesCursor = CursorStyle.Render("▸ ")
	} else {
		noStyle = SelectedStyle
		noCursor = CursorStyle.Render("▸ ")
	}

	b.WriteString(yesCursor)
	b.WriteString(yesStyle.Render(m.yesLabel))
	b.WriteString("\n")
	b.WriteString(noCursor)
	b.WriteString(noStyle.Render(m.noLabel))
	b.WriteString("\n")

	// Help
	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("←/→ navegar • Enter confirmar • y/n atalho • Esc cancelar"))

	return b.String()
}

// Confirmed returns true if the user selected Yes
func (m ConfirmModel) Confirmed() bool {
	return m.confirmed && !m.aborted
}

// Aborted returns true if the user cancelled
func (m ConfirmModel) Aborted() bool {
	return m.aborted
}

// Confirm runs a confirmation prompt and returns true if confirmed
// Returns false if the user cancels or selects No
func Confirm(title string, opts ...ConfirmOption) (bool, error) {
	m := NewConfirm(title, opts...)
	p := tea.NewProgram(m)

	result, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("failed to run confirm: %w", err)
	}

	finalModel := result.(ConfirmModel)
	return finalModel.Confirmed(), nil
}

// ConfirmWithDefault runs a confirmation prompt with a default value
func ConfirmWithDefault(title string, defaultYes bool, opts ...ConfirmOption) (bool, error) {
	if !defaultYes {
		opts = append(opts, WithDefaultNo())
	}
	return Confirm(title, opts...)
}
