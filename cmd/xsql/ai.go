package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/ai"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
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
	var allowPlaintext bool
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

			resolved := GlobalConfig.Resolved
			if opts.CLIAIModelSet && opts.CLIAIModel != "" {
				resolved.AI.Model = opts.CLIAIModel
			}
			if opts.CLIAIBaseURLSet && opts.CLIAIBaseURL != "" {
				resolved.AI.BaseURL = opts.CLIAIBaseURL
			}
			apiKeyFromCLI := opts.CLIAIAPIKeySet && opts.CLIAIAPIKey != ""
			if apiKeyFromCLI {
				resolved.AI.APIKey = opts.CLIAIAPIKey
			}
			if resolved.ProfileName == "" || resolved.Profile.DB == "" {
				return errors.New(errors.CodeCfgInvalid, "no profile specified and no 'default' profile found in config", nil)
			}

			// Runtime CLI/ENV values are explicit plaintext inputs. Plaintext from
			// the config file requires an explicit config or CLI opt-in.
			apiKey := resolved.AI.APIKey
			if apiKey != "" {
				allowAPIKeyPlaintext := allowPlaintext || resolved.AI.AllowPlaintext || apiKeyFromCLI || os.Getenv("XSQL_AI_API_KEY") != ""
				resolvedKey, xe := secret.Resolve(apiKey, secret.Options{AllowPlaintext: allowAPIKeyPlaintext})
				if xe != nil {
					return xe
				}
				apiKey = resolvedKey
			}
			resolved.AI.APIKey = apiKey
			opts.CLIAIModel = resolved.AI.Model

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
	cmd.Flags().BoolVar(&allowPlaintext, "allow-plaintext", false, "Allow plaintext AI API key in config")
	cmd.Flags().BoolVar(&unsafeAllowWrite, "unsafe-allow-write", false, "Allow writes when the profile also sets unsafe_allow_write: true")

	return cmd
}
