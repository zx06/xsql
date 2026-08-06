package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/ai"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/secret"
	"github.com/zx06/xsql/internal/tui"
)

var newAIProgramFunc = func(model tea.Model) *tea.Program {
	return tea.NewProgram(model, tea.WithAltScreen())
}

func NewAICommand() *cobra.Command {
	var modelStr string
	var baseURLStr string
	var apiKeyStr string
	var unsafeAllowWrite bool

	cmd := &cobra.Command{
		Use:   "ai [PROMPT]",
		Short: "Interactive AI assistant mode (TUI)",
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := ""
			if len(args) > 0 {
				prompt = strings.Join(args, " ")
			}

			opts := config.Options{
				ConfigPath:      GlobalConfig.ConfigStr,
				CLIProfile:      GlobalConfig.ProfileStr,
				CLIProfileSet:   cmd.Flags().Changed("profile"),
				CLIAIModel:      modelStr,
				CLIAIModelSet:   cmd.Flags().Changed("model"),
				CLIAIBaseURL:    baseURLStr,
				CLIAIBaseURLSet: cmd.Flags().Changed("base-url"),
				CLIAIAPIKey:     apiKeyStr,
				CLIAIAPIKeySet:  cmd.Flags().Changed("api-key"),
			}

			resolved, xe := config.Resolve(opts)
			if xe != nil {
				return xe
			}
			if resolved.ProfileName == "" || resolved.Profile.DB == "" {
				return fmt.Errorf("config error [XSQL_CFG_INVALID]: no profile specified and no 'default' profile found in config")
			}

			// Resolve API key if keyring reference or plaintext
			apiKey := resolved.AI.APIKey
			if secret.IsKeyringRef(apiKey) {
				resolvedKey, xe := secret.Resolve(apiKey, secret.Options{AllowPlaintext: true})
				if xe == nil {
					apiKey = resolvedKey
				}
			}
			resolved.AI.APIKey = apiKey

			aiClient := ai.NewClient(resolved.AI, nil)
			aiService := ai.NewService(resolved.AI, aiClient)

			m := tui.NewModel(opts, resolved, aiService, prompt, unsafeAllowWrite)
			p := newAIProgramFunc(m)
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("error running TUI: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&modelStr, "model", "", "AI model name (default: gpt-4o)")
	cmd.Flags().StringVar(&baseURLStr, "base-url", "", "AI service base URL")
	cmd.Flags().StringVar(&apiKeyStr, "api-key", "", "AI service API key")
	cmd.Flags().BoolVar(&unsafeAllowWrite, "unsafe-allow-write", false, "Allow write operations (bypasses read-only protection)")

	return cmd
}
