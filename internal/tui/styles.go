package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Palette Colors
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SecondaryColor = lipgloss.Color("#04B575")
	AccentColor    = lipgloss.Color("#E03177")
	WarningColor   = lipgloss.Color("#D97706")
	ErrorColor     = lipgloss.Color("#E11D48")
	MutedColor     = lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94A3B8"}
	CyanColor      = lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#7AA2F7"}
	BgBox          = lipgloss.AdaptiveColor{Light: "#F1F5F9", Dark: "#1F2335"}

	// Header Styles
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(PrimaryColor).
			Padding(0, 1)

	BadgeReadOnly = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(SecondaryColor).
			Padding(0, 1)

	BadgeReadWrite = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(WarningColor).
			Padding(0, 1)

	BadgeAutoExec = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(PrimaryColor).
			Padding(0, 1)

	BadgeManualApprove = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#C0CAF5"}).
				Background(lipgloss.AdaptiveColor{Light: "#E2E8F0", Dark: "#3B4261"}).
				Padding(0, 1)

	// SQL Preview Box
	SQLBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	SQLTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#9D174D", Dark: "#FF75B5"})

	SQLCodeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0369A1", Dark: "#7AA2F7"})

	// Help / Footer
	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			MarginTop(1)

	// Chat Messages
	UserTagStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(SecondaryColor).
			Padding(0, 1)

	AITagStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(PrimaryColor).
			Padding(0, 1)

	ExecutingTagStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(WarningColor).
				Padding(0, 1)

	SuccessBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(SecondaryColor)

	AIResponseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#E2E8F0"}).
			PaddingLeft(1)

	ErrorMsgStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ErrorColor).
			PaddingLeft(1)
)
