package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Palette Colors
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SecondaryColor = lipgloss.Color("#04B575")
	AccentColor    = lipgloss.Color("#FF75B5")
	WarningColor   = lipgloss.Color("#FF9E3B")
	ErrorColor     = lipgloss.Color("#FF5370")
	MutedColor     = lipgloss.Color("#565F89")
	CyanColor      = lipgloss.Color("#7AA2F7")
	BgDark         = lipgloss.Color("#1A1B26")

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

	// SQL Preview Box
	SQLBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Background(lipgloss.Color("#1F2335")).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	SQLTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(AccentColor)

	SQLCodeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7AA2F7"))

	// Help / Footer
	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			MarginTop(1)

	// Chat Messages
	UserTagStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1A1B26")).
			Background(SecondaryColor).
			Padding(0, 1)

	AITagStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(PrimaryColor).
			Padding(0, 1)

	ExecutingTagStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#1A1B26")).
				Background(WarningColor).
				Padding(0, 1)

	SuccessBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(SecondaryColor)

	AIResponseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C0CAF5")).
			PaddingLeft(1)

	ErrorMsgStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ErrorColor).
			PaddingLeft(1)
)
