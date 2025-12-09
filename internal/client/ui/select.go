package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// SelectOption represents an option in the select menu
type SelectOption struct {
	Label       string
	Description string
	Value       string
}

// SelectModel is the Bubble Tea model for selection
type SelectModel struct {
	title    string
	options  []SelectOption
	cursor   int
	selected int
	quitting bool
	aborted  bool
}

// NewSelect creates a new select menu
func NewSelect(title string, options []SelectOption) SelectModel {
	return SelectModel{
		title:    title,
		options:  options,
		cursor:   0,
		selected: -1,
	}
}

// Init implements tea.Model
func (m SelectModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m SelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.aborted = true
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}

		case "enter", " ":
			m.selected = m.cursor
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View implements tea.Model
func (m SelectModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	if m.title != "" {
		b.WriteString(TitleStyle.Render(m.title))
		b.WriteString("\n\n")
	}

	// Options
	for i, opt := range m.options {
		cursor := "  "
		style := UnselectedStyle

		if i == m.cursor {
			cursor = CursorStyle.Render("▸ ")
			style = SelectedStyle
		}

		b.WriteString(cursor)
		b.WriteString(style.Render(opt.Label))

		if opt.Description != "" {
			b.WriteString("\n    ")
			b.WriteString(HelpStyle.Render(opt.Description))
		}
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("↑/↓ navegar • Enter selecionar • Esc cancelar"))

	return b.String()
}

// Selected returns the selected index, or -1 if aborted
func (m SelectModel) Selected() int {
	if m.aborted {
		return -1
	}
	return m.selected
}

// SelectedOption returns the selected option, or nil if aborted
func (m SelectModel) SelectedOption() *SelectOption {
	if m.aborted || m.selected < 0 || m.selected >= len(m.options) {
		return nil
	}
	return &m.options[m.selected]
}

// Aborted returns true if the user cancelled
func (m SelectModel) Aborted() bool {
	return m.aborted
}

// Select runs a selection prompt and returns the selected index
// Returns -1 if the user cancels
func Select(title string, options []SelectOption) (int, error) {
	m := NewSelect(title, options)
	p := tea.NewProgram(m)

	result, err := p.Run()
	if err != nil {
		return -1, fmt.Errorf("failed to run select: %w", err)
	}

	finalModel := result.(SelectModel)
	return finalModel.Selected(), nil
}

// SelectWithDefault runs a selection prompt with a default option highlighted
func SelectWithDefault(title string, options []SelectOption, defaultIndex int) (int, error) {
	m := NewSelect(title, options)
	if defaultIndex >= 0 && defaultIndex < len(options) {
		m.cursor = defaultIndex
	}
	p := tea.NewProgram(m)

	result, err := p.Run()
	if err != nil {
		return -1, fmt.Errorf("failed to run select: %w", err)
	}

	finalModel := result.(SelectModel)
	return finalModel.Selected(), nil
}
