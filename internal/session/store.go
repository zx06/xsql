package session

import (
	"fmt"
	"strings"
	"sync"

	"github.com/zx06/xsql/internal/db"
)

type DatasetEntry struct {
	ID          string          `json:"id"`
	Description string          `json:"description"`
	Result      *db.QueryResult `json:"result"`
}

type SessionDataStore struct {
	mu       sync.RWMutex
	datasets map[string]*DatasetEntry
	order    []string
	counter  int
}

func NewSessionDataStore() *SessionDataStore {
	return &SessionDataStore{
		datasets: make(map[string]*DatasetEntry),
		order:    make([]string, 0),
	}
}

func (s *SessionDataStore) Save(description string, result *db.QueryResult) string {
	if result == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	id := fmt.Sprintf("res%d", s.counter)

	entry := &DatasetEntry{
		ID:          id,
		Description: strings.TrimSpace(description),
		Result:      result,
	}

	s.datasets[id] = entry
	s.order = append(s.order, id)
	return id
}

func (s *SessionDataStore) Get(id string) (*db.QueryResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.datasets[id]
	if !ok || entry == nil {
		return nil, false
	}
	return entry.Result, true
}

func (s *SessionDataStore) Latest() (*db.QueryResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.order) == 0 {
		return nil, false
	}
	lastID := s.order[len(s.order)-1]
	return s.datasets[lastID].Result, true
}

func (s *SessionDataStore) GetAll() map[string]*db.QueryResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make(map[string]*db.QueryResult, len(s.datasets))
	for k, v := range s.datasets {
		res[k] = v.Result
	}
	return res
}

func (s *SessionDataStore) GetCatalog() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.datasets) == 0 {
		return "(No active datasets in current session)"
	}

	var sb strings.Builder
	sb.WriteString("Active Datasets in Session:\n")
	for _, id := range s.order {
		entry := s.datasets[id]
		if entry == nil || entry.Result == nil {
			continue
		}
		cols := strings.Join(entry.Result.Columns, ", ")
		desc := entry.Description
		if desc == "" {
			desc = "Query Result"
		}
		fmt.Fprintf(&sb, "- `%s`: %s (%d rows, columns: [%s])\n", id, desc, len(entry.Result.Rows), cols)
	}
	return sb.String()
}
