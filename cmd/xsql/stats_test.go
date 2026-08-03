package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/stats"
)

func TestRecordCmdStats_Disabled(t *testing.T) {
	// When stats is disabled, nothing should be recorded
	GlobalConfig.Stats.Enabled = false
	GlobalConfig.Stats.FilePath = filepath.Join(t.TempDir(), "stats.jsonl")

	recordCmdStats("query", "dev", true, 100*time.Millisecond, "", "")

	// Verify no file was created
	if _, err := os.Stat(GlobalConfig.Stats.FilePath); !os.IsNotExist(err) {
		t.Error("expected no stats file when disabled")
	}
}

func TestRecordCmdStats_Enabled(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.jsonl")

	GlobalConfig.Stats.Enabled = true
	GlobalConfig.Stats.FilePath = statsPath
	GlobalConfig.Stats.LogSQL = false
	GlobalConfig.Attrs = map[string]string{"env": "test"}

	recordCmdStats("query", "dev", true, 150*time.Millisecond, "", "SELECT 1")

	store := stats.NewStore(statsPath)
	records, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Cmd != "query" {
		t.Errorf("expected cmd=query, got %s", records[0].Cmd)
	}
	if records[0].Profile != "dev" {
		t.Errorf("expected profile=dev, got %s", records[0].Profile)
	}
	if !records[0].OK {
		t.Error("expected ok=true")
	}
	if records[0].Attrs["env"] != "test" {
		t.Errorf("expected attrs.env=test, got %s", records[0].Attrs["env"])
	}
	// SQL should not be recorded when LogSQL is false
	if records[0].SQL != "" {
		t.Errorf("expected empty SQL when LogSQL=false, got %s", records[0].SQL)
	}
}

func TestRecordCmdStats_WithErrorCode(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.jsonl")

	GlobalConfig.Stats.Enabled = true
	GlobalConfig.Stats.FilePath = statsPath
	GlobalConfig.Stats.LogSQL = false
	GlobalConfig.Attrs = nil

	recordCmdStats("query", "dev", false, 100*time.Millisecond, errors.CodeDBExecFailed, "")

	store := stats.NewStore(statsPath)
	records, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].OK {
		t.Error("expected ok=false")
	}
	if records[0].ErrorCode != string(errors.CodeDBExecFailed) {
		t.Errorf("expected error_code=%s, got %s", errors.CodeDBExecFailed, records[0].ErrorCode)
	}
}

func TestRecordCmdStats_WithLogSQL(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.jsonl")

	GlobalConfig.Stats.Enabled = true
	GlobalConfig.Stats.FilePath = statsPath
	GlobalConfig.Stats.LogSQL = true
	GlobalConfig.Attrs = nil

	recordCmdStats("query", "dev", true, 100*time.Millisecond, "", "SELECT * FROM users")

	store := stats.NewStore(statsPath)
	records, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].SQL != "SELECT * FROM users" {
		t.Errorf("expected SQL=SELECT * FROM users, got %s", records[0].SQL)
	}
}

func TestRunStats_Empty(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.jsonl")

	GlobalConfig.Stats.FilePath = statsPath

	var buf bytes.Buffer
	w := output.New(&buf, &buf)
	flags := &StatsFlags{}

	err := runStats(flags, &w)
	if err != nil {
		t.Fatalf("runStats: %v", err)
	}
}

func TestRunStatsLog_Empty(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.jsonl")

	GlobalConfig.Stats.FilePath = statsPath

	var buf bytes.Buffer
	w := output.New(&buf, &buf)
	flags := &StatsLogFlags{Limit: 100}

	err := runStatsLog(flags, &w)
	if err != nil {
		t.Fatalf("runStatsLog: %v", err)
	}
}

func TestRunStatsReset(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.jsonl")

	GlobalConfig.Stats.FilePath = statsPath

	// Create a stats file
	store := stats.NewStore(statsPath)
	_ = store.Append(&stats.Record{
		Timestamp: time.Now(),
		Cmd:       "query",
		Profile:   "dev",
		OK:        true,
	})

	var buf bytes.Buffer
	w := output.New(&buf, &buf)

	err := runStatsReset(&w)
	if err != nil {
		t.Fatalf("runStatsReset: %v", err)
	}

	// Verify file is removed
	if _, err := os.Stat(statsPath); !os.IsNotExist(err) {
		t.Error("expected stats file to be removed")
	}
}

func TestNewStatsCommand(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, &buf)

	cmd := NewStatsCommand(&w)
	if cmd.Use != "stats" {
		t.Errorf("expected use=stats, got %s", cmd.Use)
	}

	// Check subcommands
	subcommands := cmd.Commands()
	if len(subcommands) != 2 {
		t.Errorf("expected 2 subcommands, got %d", len(subcommands))
	}

	// Check flags
	if cmd.Flags().Lookup("profile") == nil {
		t.Error("expected --profile flag")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("expected --json flag")
	}
}
