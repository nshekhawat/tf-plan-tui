package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/hashicorp/tf-plan-tui/internal/parser"
)

var (
	// Color palette
	ColorCreate  = lipgloss.AdaptiveColor{Light: "#22c55e", Dark: "#4ade80"}
	ColorUpdate  = lipgloss.AdaptiveColor{Light: "#eab308", Dark: "#facc15"}
	ColorDelete  = lipgloss.AdaptiveColor{Light: "#ef4444", Dark: "#f87171"}
	ColorReplace = lipgloss.AdaptiveColor{Light: "#f97316", Dark: "#fb923c"}
	ColorRead    = lipgloss.AdaptiveColor{Light: "#6366f1", Dark: "#818cf8"}
	ColorNoOp    = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"}

	ColorBorder       = lipgloss.AdaptiveColor{Light: "#d1d5db", Dark: "#374151"}
	ColorActiveBorder = lipgloss.AdaptiveColor{Light: "#6366f1", Dark: "#818cf8"}
	ColorHeader       = lipgloss.AdaptiveColor{Light: "#1f2937", Dark: "#f9fafb"}
	ColorMuted        = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#6b7280"}
	ColorDiffAdd      = lipgloss.AdaptiveColor{Light: "#166534", Dark: "#86efac"}
	ColorDiffRemove   = lipgloss.AdaptiveColor{Light: "#991b1b", Dark: "#fca5a5"}

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			Foreground(ColorHeader)

	ActivePaneBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorActiveBorder)

	InactivePaneBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder)

	SelectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.AdaptiveColor{Light: "#e0e7ff", Dark: "#1e1b4b"}).
				Bold(true)

	DiffAddStyle    = lipgloss.NewStyle().Foreground(ColorDiffAdd)
	DiffRemoveStyle = lipgloss.NewStyle().Foreground(ColorDiffRemove)

	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1)

	FilterActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Reverse(true).
				Padding(0, 1)

	FilterInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1)
)

// ActionColor returns the color for a given action.
func ActionColor(action parser.Action) lipgloss.AdaptiveColor {
	switch action {
	case parser.ActionCreate:
		return ColorCreate
	case parser.ActionUpdate:
		return ColorUpdate
	case parser.ActionDelete:
		return ColorDelete
	case parser.ActionReplace:
		return ColorReplace
	case parser.ActionRead:
		return ColorRead
	default:
		return ColorNoOp
	}
}

// ActionStyle returns a styled string for the given action icon.
func ActionStyle(action parser.Action) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActionColor(action)).Bold(true)
}
