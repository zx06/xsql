package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/ai"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/secret"
	"github.com/zx06/xsql/internal/tui"
)

type CmdAIFlags struct {
	Model            string
	BaseURL          string
	APIKey           string
	UnsafeAllowWrite bool
	Prompt           string
}

func NewAICommand() *cobra.Command {
	flags := &CmdAIFlags{}

	cmd := &cobra.Command{
		Use:   "ai [PROMPT]",
		Short: "Interactive AI terminal mode (TUI) to write and execute SQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				flags.Prompt = args[0]
			}
			return runCmdAI(cmd, flags)
		},
	}

	cmd.Flags().StringVar(&flags.Model, "model", "", "AI model name (default: gpt-4o)")
	cmd.Flags().StringVar(&flags.BaseURL, "base-url", "", "AI service base URL")
	cmd.Flags().StringVar(&flags.APIKey, "api-key", "", "AI service API key")
	cmd.Flags().BoolVar(&flags.UnsafeAllowWrite, "unsafe-allow-write", false, "Allow write operations (bypasses read-only protection)")
	cmd.Flags().StringVar(&flags.Prompt, "prompt", "", "Initial prompt for AI query")

	return cmd
}

func runCmdAI(cmd *cobra.Command, flags *CmdAIFlags) error {
	opts := config.Options{
		ConfigPath:      GlobalConfig.ConfigStr,
		CLIProfile:      GlobalConfig.ProfileStr,
		CLIProfileSet:   cmd.Flags().Changed("profile") || GlobalConfig.ProfileStr != "",
		CLIAIModel:      flags.Model,
		CLIAIModelSet:   cmd.Flags().Changed("model"),
		CLIAIBaseURL:    flags.BaseURL,
		CLIAIBaseURLSet: cmd.Flags().Changed("base-url"),
		CLIAIAPIKey:     flags.APIKey,
		CLIAIAPIKeySet:  cmd.Flags().Changed("api-key"),
	}

	resolved, xe := config.Resolve(opts)
	if xe != nil {
		return xe
	}

	apiKey := resolved.AI.APIKey
	if secret.IsKeyringRef(apiKey) {
		if resolvedKey, xe := secret.Resolve(apiKey, secret.Options{AllowPlaintext: true}); xe == nil {
			apiKey = resolvedKey
		}
	}
	resolved.AI.APIKey = apiKey

	aiClient := ai.NewClient(resolved.AI, nil)
	aiService := ai.NewService(resolved.AI, aiClient)

	model := tui.NewModel(opts, resolved, aiService, flags.Prompt, flags.UnsafeAllowWrite)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}
