package stats

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultFilePath returns the default stats file path.
func DefaultFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "stats.jsonl"
	}
	return filepath.Join(home, ".config", "xsql", "stats.jsonl")
}

// Store manages the JSONL stats file.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore creates a stats store with the given path.
// If path is empty, uses the default path.
func NewStore(path string) *Store {
	if path == "" {
		path = DefaultFilePath()
	}
	return &Store{path: path}
}

// Append atomically appends a record to the JSONL file.
func (s *Store) Append(r *Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if _, err := f.Write(data); err != nil {
		return err
	}
	return nil
}

// Load reads all records from the JSONL file.
func (s *Store) Load() ([]*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []*Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue // skip corrupted lines
		}
		records = append(records, &r)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

// Reset clears the stats file.
func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Cleanup removes records older than retentionDays.
func (s *Store) Cleanup(retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	records, err := s.Load()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	var kept []*Record
	removed := 0
	for _, r := range records {
		if r.Timestamp.Before(cutoff) {
			removed++
		} else {
			kept = append(kept, r)
		}
	}

	if removed == 0 {
		return 0, nil
	}

	// Rewrite file with kept records
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return 0, err
	}

	f, err := os.Create(s.path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, r := range kept {
		if err := enc.Encode(r); err != nil {
			return removed, err
		}
	}

	return removed, nil
}
