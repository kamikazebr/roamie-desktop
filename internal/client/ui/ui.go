// Package ui provides TUI components for the Roamie CLI using Bubble Tea
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Styles for the UI components
var (
	// Colors
	primaryColor   = lipgloss.Color("#00D4AA") // Roamie teal
	secondaryColor = lipgloss.Color("#888888")
	warningColor   = lipgloss.Color("#FFAA00")
	errorColor     = lipgloss.Color("#FF5555")
	successColor   = lipgloss.Color("#00FF00")

	// Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	UnselectedStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	CursorStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(successColor)
)
