// Package ui holds lipgloss styles shared by the app's views.
package ui

import "github.com/charmbracelet/lipgloss"

var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	Dim = lipgloss.NewStyle().
		Foreground(lipgloss.Color("243"))

	Highlight = lipgloss.NewStyle().
			Background(lipgloss.Color("63")).
			Foreground(lipgloss.Color("230"))

	Hint = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	Sender = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	Time = lipgloss.NewStyle().
		Foreground(lipgloss.Color("243"))

	// OutMsg styles the message body. Incoming plain, outgoing italic tinted.
	InMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	// OutMsg styles the message body. Incoming plain, outgoing italic tinted.
	OutMsg = lipgloss.NewStyle().
		Foreground(lipgloss.Color("120")).
		Italic(true)

	Err = lipgloss.NewStyle().
		Foreground(lipgloss.Color("203")).
		Bold(true)
)
