package stats

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStore_AppendAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	store := NewStore(path)

	now := time.Now().Truncate(time.Millisecond)
	r := &Record{
		Timestamp:  now,
		Cmd:        "query",
		Profile:    "dev",
		OK:         true,
		DurationMs: 123,
		Attrs:      map[string]string{"env": "dev"},
	}

	if err := store.Append(r); err != nil {
		t.Fatalf("append: %v", err)
	}

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
		t.Errorf("expected ok=true")
	}
	if records[0].DurationMs != 123 {
		t.Errorf("expected duration_ms=123, got %d", records[0].DurationMs)
	}
	if records[0].Attrs["env"] != "dev" {
		t.Errorf("expected attrs.env=dev, got %s", records[0].Attrs["env"])
	}
}

func TestStore_AppendMultiple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	store := NewStore(path)

	for i := 0; i < 5; i++ {
		r := &Record{
			Timestamp:  time.Now(),
			Cmd:        "query",
			Profile:    "dev",
			OK:         i%2 == 0,
			DurationMs: int64(100 + i),
		}
		if err := store.Append(r); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	records, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(records) != 5 {
		t.Fatalf("expected 5 records, got %d", len(records))
	}
}

func TestStore_Reset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	store := NewStore(path)

	r := &Record{Timestamp: time.Now(), Cmd: "query", Profile: "dev", OK: true, DurationMs: 100}
	if err := store.Append(r); err != nil {
		t.Fatalf("append: %v", err)
	}

	if err := store.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	records, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records after reset, got %d", len(records))
	}
}

func TestStore_Reset_NoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	store := NewStore(path)

	// Reset on non-existent file should not error
	if err := store.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}
}

func TestStore_Load_NoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	store := NewStore(path)

	records, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if records != nil {
		t.Fatalf("expected nil, got %d records", len(records))
	}
}

func TestStore_Cleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	store := NewStore(path)

	now := time.Now()
	// Old record (40 days ago)
	old := &Record{Timestamp: now.AddDate(0, 0, -40), Cmd: "query", Profile: "dev", OK: true, DurationMs: 100}
	// Recent record
	recent := &Record{Timestamp: now, Cmd: "query", Profile: "dev", OK: true, DurationMs: 100}

	_ = store.Append(old)
	_ = store.Append(recent)

	removed, err := store.Cleanup(30)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	records, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record after cleanup, got %d", len(records))
	}
}

func TestStore_Cleanup_NoRetention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	store := NewStore(path)

	r := &Record{Timestamp: time.Now(), Cmd: "query", Profile: "dev", OK: true, DurationMs: 100}
	_ = store.Append(r)

	removed, err := store.Cleanup(0)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
}

func TestStore_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping permissions test on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	store := NewStore(path)

	r := &Record{Timestamp: time.Now(), Cmd: "query", Profile: "dev", OK: true, DurationMs: 100}
	if err := store.Append(r); err != nil {
		t.Fatalf("append: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// Check file permissions (0600)
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}
}

func TestDefaultFilePath(t *testing.T) {
	path := DefaultFilePath()
	if path == "" {
		t.Error("expected non-empty default path")
	}
	if filepath.Ext(path) != ".jsonl" {
		t.Errorf("expected .jsonl extension, got %s", filepath.Ext(path))
	}
}

func TestNewStore_EmptyPath(t *testing.T) {
	store := NewStore("")
	if store.path == "" {
		t.Error("expected default path when empty")
	}
	if filepath.Ext(store.path) != ".jsonl" {
		t.Errorf("expected .jsonl extension, got %s", filepath.Ext(store.path))
	}
}

func TestStore_Load_CorruptedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")

	// Write corrupted JSON lines
	content := "not json\n{\"valid\": true}\n{broken json\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	store := NewStore(path)
	records, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// Should skip corrupted lines and load valid ones
	if len(records) != 1 {
		t.Errorf("expected 1 valid record, got %d", len(records))
	}
}

func TestStore_Cleanup_NoOldRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	store := NewStore(path)

	now := time.Now()
	recent := &Record{Timestamp: now, Cmd: "query", Profile: "dev", OK: true, DurationMs: 100}
	_ = store.Append(recent)

	// All records are recent, nothing should be removed
	removed, err := store.Cleanup(30)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
}

func TestStore_Cleanup_AllOld(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	store := NewStore(path)

	now := time.Now()
	old1 := &Record{Timestamp: now.AddDate(0, 0, -60), Cmd: "query", Profile: "dev", OK: true, DurationMs: 100}
	old2 := &Record{Timestamp: now.AddDate(0, 0, -45), Cmd: "query", Profile: "dev", OK: true, DurationMs: 100}
	_ = store.Append(old1)
	_ = store.Append(old2)

	removed, err := store.Cleanup(30)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}

	records, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records after cleanup, got %d", len(records))
	}
}

func TestStore_Append_MkdirError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Try to write to a path where parent dir doesn't exist and can't be created
	store := NewStore("/nonexistent/root/stats.jsonl")
	r := &Record{Timestamp: time.Now(), Cmd: "query", Profile: "dev", OK: true, DurationMs: 100}
	err := store.Append(r)
	if err == nil {
		t.Error("expected error for invalid path")
	}
}
