package main

import "github.com/charmbracelet/lipgloss"

// Palette mirrors web/src/styles.css's dark theme (--accent, --green,
// --red, --yellow, --text-dim, --text-faint) so the TUI reads as the same
// product as the dashboard, not a visually unrelated tool that happens to
// share a backend.
var (
	colAccent = lipgloss.Color("#4f8cff")
	colGreen  = lipgloss.Color("#3ecf8e")
	colRed    = lipgloss.Color("#f0556b")
	colYellow = lipgloss.Color("#e0a640")
	colDim    = lipgloss.Color("#8b93a7")
	colFaint  = lipgloss.Color("#5b6377")

	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(colAccent).Padding(0, 1)
	styleHelp  = lipgloss.NewStyle().Foreground(colFaint)
	styleErr   = lipgloss.NewStyle().Foreground(colRed).Bold(true)
	styleDim   = lipgloss.NewStyle().Foreground(colDim)
	styleField = lipgloss.NewStyle().Foreground(colDim).Width(14)

	styleStatusBar = lipgloss.NewStyle().Padding(0, 1)
)

// statusColors mirrors StatusPill.tsx's STATUS_COLORS exactly, so a status
// string reads the same color here as it does in the dashboard.
var statusColors = map[string]lipgloss.Color{
	"running":     colGreen,
	"success":     colGreen,
	"healthy":     colGreen,
	"failed":      colRed,
	"unhealthy":   colRed,
	"error":       colRed,
	"building":    colYellow,
	"queued":      colYellow,
	"timeout":     colYellow,
	"rolled_back": colYellow,
	"pending":     colFaint,
	"stopped":     colFaint,
	"unknown":     colFaint,
}

func statusColor(status string) lipgloss.Color {
	if c, ok := statusColors[status]; ok {
		return c
	}
	return colFaint
}
