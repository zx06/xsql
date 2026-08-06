package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Palette Colors (Adaptive Catppuccin / Tokyo Night Theme)
	PrimaryColor   = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#A78BFA"}
	SecondaryColor = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
	AccentColor    = lipgloss.AdaptiveColor{Light: "#BE185D", Dark: "#F472B6"}
	WarningColor   = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"}
	ErrorColor     = lipgloss.AdaptiveColor{Light: "#E11D48", Dark: "#F87171"}
	InfoColor      = lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#38BDF8"}
	MutedColor     = lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"}
	TextNormal     = lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#F8FAFC"}
	HeaderBg       = lipgloss.AdaptiveColor{Light: "#E2E8F0", Dark: "#1E293B"}

	// Header Container & Badges
	HeaderBarStyle = lipgloss.NewStyle().
			Background(HeaderBg).
			Padding(0, 1)

	HeaderTitleBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(PrimaryColor).
				Padding(0, 1)

	HeaderProfileBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(InfoColor).
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
			Background(AccentColor).
			Padding(0, 1)

	BadgeManualApprove = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#E2E8F0"}).
				Background(lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#475569"}).
				Padding(0, 1)

	// Keybinding Badges
	KeyBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#F8FAFC"}).
			Background(lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#334155"}).
			Padding(0, 1)

	KeyLabelStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	// SQL Preview Box
	SQLBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	SQLTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(AccentColor)

	SQLCodeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(InfoColor)

	// User & AI Message Tags
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

	WarningBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(WarningColor)

	MetricsStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	AIResponseStyle = lipgloss.NewStyle().
			Foreground(TextNormal).
			PaddingLeft(1)

	ErrorMsgStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ErrorColor).
			PaddingLeft(1)

	PromptPrefixStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(PrimaryColor)

	// Collapsible Tool Call Badges
	ToolCollapsedBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94A3B8"}).
				Background(lipgloss.AdaptiveColor{Light: "#E2E8F0", Dark: "#1E293B"}).
				Padding(0, 1)

	ToolExpandedBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(WarningColor).
				Padding(0, 1)

	ToolDetailStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(MutedColor).
			PaddingLeft(1).
			Foreground(MutedColor)
)
