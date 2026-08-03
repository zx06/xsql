//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStatsCommand(t *testing.T) {
	t.Run("stats shows empty when no data", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "xsql.yaml")
		statsPath := filepath.Join(tmpDir, "stats.jsonl")
		writeStatsTestConfig(t, configPath, statsPath)

		stdout, _, _ := runXSQL(t, "--config", configPath, "stats", "--json")
		var resp Response
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if !resp.OK {
			t.Fatalf("expected OK, got error: %v", resp.Error)
		}
	})

	t.Run("stats with --attr flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "xsql.yaml")
		statsPath := filepath.Join(tmpDir, "stats.jsonl")
		writeStatsTestConfig(t, configPath, statsPath)

		// Run a command with --attr (it will fail due to no DB, but stats should record)
		_, _, _ = runXSQL(t, "--config", configPath, "-p", "dev", "--attr", "env=test", "query", "SELECT 1")

		// Check stats
		stdout, _, _ := runXSQL(t, "--config", configPath, "stats", "--json")
		var resp Response
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if !resp.OK {
			t.Fatalf("expected OK, got error: %v", resp.Error)
		}
	})

	t.Run("stats reset", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "xsql.yaml")
		statsPath := filepath.Join(tmpDir, "stats.jsonl")
		writeStatsTestConfig(t, configPath, statsPath)

		// Run a command to generate stats
		_, _, _ = runXSQL(t, "--config", configPath, "-p", "dev", "query", "SELECT 1")

		// Reset stats
		_, stderr, _ := runXSQL(t, "--config", configPath, "stats", "reset")
		if stderr != "" {
			t.Logf("stderr: %s", stderr)
		}

		// Verify stats file is gone or empty
		if _, err := os.Stat(statsPath); !os.IsNotExist(err) {
			data, _ := os.ReadFile(statsPath)
			if len(data) > 0 {
				t.Fatalf("expected empty stats file after reset, got %d bytes", len(data))
			}
		}
	})
}

func writeStatsTestConfig(t *testing.T, path, statsPath string) {
	t.Helper()
	cfg := map[string]any{
		"stats": map[string]any{
			"enabled":   true,
			"file_path": statsPath,
		},
		"profiles": map[string]any{
			"dev": map[string]any{
				"db":   "mysql",
				"host": "localhost",
				"port": 3306,
			},
		},
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
}
