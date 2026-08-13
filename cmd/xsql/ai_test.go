package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCmdXSQL_AICommand(t *testing.T) {
	oldGlobalConfig := GlobalConfig
	GlobalConfig = &Config{}
	defer func() { GlobalConfig = oldGlobalConfig }()

	oldNewAIProgram := newAIProgramFunc
	defer func() { newAIProgramFunc = oldNewAIProgram }()

	newAIProgramFunc = func(model tea.Model) *tea.Program {
		p := tea.NewProgram(model, tea.WithInput(strings.NewReader("")), tea.WithOutput(os.Stderr), tea.WithoutRenderer())
		go p.Quit()
		return p
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "xsql.yaml")
	cfgContent := `
profiles:
  dev:
    db: mysql
    host: 127.0.0.1
    port: 3306
    user: root
    database: testdb
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	root := NewRootCommand()
	root.AddCommand(NewAICommand())
	root.SetArgs([]string{"ai", "--config", cfgPath, "--profile", "dev", "Show top 10 users"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("expected xsql ai command execution to succeed, got %v", err)
	}
}

func TestCmdXSQL_AICommandUsesEnvironmentConfig(t *testing.T) {
	oldGlobalConfig := GlobalConfig
	GlobalConfig = &Config{}
	defer func() { GlobalConfig = oldGlobalConfig }()

	oldNewAIProgram := newAIProgramFunc
	defer func() { newAIProgramFunc = oldNewAIProgram }()

	var initialView string
	newAIProgramFunc = func(model tea.Model) *tea.Program {
		initialView = model.View()
		p := tea.NewProgram(model, tea.WithInput(strings.NewReader("")), tea.WithOutput(os.Stderr), tea.WithoutRenderer())
		go p.Quit()
		return p
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "xsql.yaml")
	cfgContent := `
profiles:
  default:
    db: mysql
  env-profile:
    db: pg
ai:
  api_key: keyring:must-be-overridden
  model: config-model
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	t.Setenv("XSQL_PROFILE", "env-profile")
	t.Setenv("XSQL_AI_API_KEY", "env-key")
	t.Setenv("XSQL_AI_MODEL", "env-model")

	root := NewRootCommand()
	root.AddCommand(NewAICommand())
	root.SetArgs([]string{"ai", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected environment-backed AI command to succeed, got %v", err)
	}
	if !strings.Contains(initialView, "env-profile") || !strings.Contains(initialView, "pg") {
		t.Fatalf("expected environment-selected pg profile in TUI, got %q", initialView)
	}
}

func TestCmdXSQL_AICommandRejectsPlaintextConfigAPIKey(t *testing.T) {
	oldGlobalConfig := GlobalConfig
	GlobalConfig = &Config{}
	defer func() { GlobalConfig = oldGlobalConfig }()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "xsql.yaml")
	cfgContent := `
profiles:
  default:
    db: mysql
ai:
  api_key: plaintext-config-key
  allow_plaintext: false
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	root := NewRootCommand()
	root.AddCommand(NewAICommand())
	root.SetArgs([]string{"ai", "--config", cfgPath})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "plaintext secret not allowed") {
		t.Fatalf("expected plaintext API key rejection, got %v", err)
	}
}

func TestCmdXSQL_AICommandAllowsPlaintextAPIKey(t *testing.T) {
	tests := []struct {
		name           string
		allowPlaintext bool
		extraArgs      []string
	}{
		{name: "config opt-in", allowPlaintext: true},
		{name: "CLI opt-in", extraArgs: []string{"--allow-plaintext"}},
		{name: "CLI API key", extraArgs: []string{"--api-key", "cli-key"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldGlobalConfig := GlobalConfig
			GlobalConfig = &Config{}
			defer func() { GlobalConfig = oldGlobalConfig }()

			oldNewAIProgram := newAIProgramFunc
			defer func() { newAIProgramFunc = oldNewAIProgram }()

			newAIProgramFunc = func(model tea.Model) *tea.Program {
				p := tea.NewProgram(model, tea.WithInput(strings.NewReader("")), tea.WithOutput(os.Stderr), tea.WithoutRenderer())
				go p.Quit()
				return p
			}

			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "xsql.yaml")
			cfgContent := fmt.Sprintf(`
profiles:
  default:
    db: mysql
ai:
  api_key: plaintext-config-key
  allow_plaintext: %t
`, tt.allowPlaintext)
			if err := os.WriteFile(cfgPath, []byte(cfgContent), 0600); err != nil {
				t.Fatalf("failed to write temp config: %v", err)
			}

			root := NewRootCommand()
			root.AddCommand(NewAICommand())
			args := []string{"ai", "--config", cfgPath}
			root.SetArgs(append(args, tt.extraArgs...))

			if err := root.Execute(); err != nil {
				t.Fatalf("expected plaintext API key to be allowed, got %v", err)
			}
		})
	}
}
