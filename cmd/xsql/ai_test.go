package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCmdXSQL_AICommand(t *testing.T) {
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
