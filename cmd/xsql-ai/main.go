package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/ai"
	"github.com/zx06/xsql/internal/config"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
	"github.com/zx06/xsql/internal/secret"
	"github.com/zx06/xsql/internal/tui"
)

type AIFlags struct {
	ConfigPath       string
	Profile          string
	Model            string
	BaseURL          string
	APIKey           string
	UnsafeAllowWrite bool
	Prompt           string
}

var newProgramFunc = func(model tea.Model) *tea.Program {
	return tea.NewProgram(model, tea.WithAltScreen())
}

func newRootCmd() *cobra.Command {
	flags := &AIFlags{}

	rootCmd := &cobra.Command{
		Use:   "xsql-ai [PROMPT]",
		Short: "xsql AI interactive database query tool (TUI)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				flags.Prompt = strings.Join(args, " ")
			}
			return runAI(cmd, flags)
		},
	}

	rootCmd.Flags().StringVar(&flags.ConfigPath, "config", "", "Config file path (YAML)")
	rootCmd.Flags().StringVarP(&flags.Profile, "profile", "p", "", "Profile name (default: 'default')")
	rootCmd.Flags().StringVar(&flags.Model, "model", "", "AI model name (default: gpt-4o)")
	rootCmd.Flags().StringVar(&flags.BaseURL, "base-url", "", "AI service base URL")
	rootCmd.Flags().StringVar(&flags.APIKey, "api-key", "", "AI service API key")
	rootCmd.Flags().BoolVar(&flags.UnsafeAllowWrite, "unsafe-allow-write", false, "Allow write operations (bypasses read-only protection)")
	rootCmd.Flags().StringVar(&flags.Prompt, "prompt", "", "Initial prompt for AI query")

	return rootCmd
}

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runAI(cmd *cobra.Command, flags *AIFlags) error {
	opts := config.Options{
		ConfigPath:      flags.ConfigPath,
		CLIProfile:      flags.Profile,
		CLIProfileSet:   flags.Profile != "",
		CLIAIModel:      flags.Model,
		CLIAIModelSet:   cmd.Flags().Changed("model"),
		CLIAIBaseURL:    flags.BaseURL,
		CLIAIBaseURLSet: cmd.Flags().Changed("base-url"),
		CLIAIAPIKey:     flags.APIKey,
		CLIAIAPIKeySet:  cmd.Flags().Changed("api-key"),
	}

	resolved, xe := config.Resolve(opts)
	if xe != nil {
		return fmt.Errorf("config error [%s]: %s", xe.Code, xe.Message)
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

	model := tui.NewModel(opts, resolved, aiService, flags.Prompt, flags.UnsafeAllowWrite)

	p := newProgramFunc(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}
