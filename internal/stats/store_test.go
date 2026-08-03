package stats

import (
	"os"
	"path/filepath"
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
