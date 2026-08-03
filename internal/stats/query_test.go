package stats

import (
	"testing"
	"time"
)

func TestQuery_Empty(t *testing.T) {
	result := Query(nil, Filter{})
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestQuery_AggregateByCmdAndProfile(t *testing.T) {
	now := time.Now()
	records := []*Record{
		{Timestamp: now, Cmd: "query", Profile: "dev", OK: true, DurationMs: 100},
		{Timestamp: now, Cmd: "query", Profile: "dev", OK: true, DurationMs: 200},
		{Timestamp: now, Cmd: "query", Profile: "dev", OK: false, DurationMs: 300},
		{Timestamp: now, Cmd: "schema dump", Profile: "dev", OK: true, DurationMs: 150},
		{Timestamp: now, Cmd: "query", Profile: "prod", OK: true, DurationMs: 80},
	}

	result := Query(records, Filter{})
	if len(result) != 3 {
		t.Fatalf("expected 3 aggregated records, got %d", len(result))
	}

	for _, r := range result {
		if r.Cmd == "query" && r.Profile == "dev" {
			if r.OK != 2 {
				t.Errorf("expected ok=2, got %d", r.OK)
			}
			if r.Fail != 1 {
				t.Errorf("expected fail=1, got %d", r.Fail)
			}
			if r.AvgMs != 200 {
				t.Errorf("expected avg_ms=200, got %d", r.AvgMs)
			}
		}
	}
}

func TestQuery_FilterByProfile(t *testing.T) {
	now := time.Now()
	records := []*Record{
		{Timestamp: now, Cmd: "query", Profile: "dev", OK: true, DurationMs: 100},
		{Timestamp: now, Cmd: "query", Profile: "prod", OK: true, DurationMs: 200},
	}

	result := Query(records, Filter{Profile: "dev"})
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Profile != "dev" {
		t.Errorf("expected profile=dev, got %s", result[0].Profile)
	}
}

func TestQuery_FilterByCmd(t *testing.T) {
	now := time.Now()
	records := []*Record{
		{Timestamp: now, Cmd: "query", Profile: "dev", OK: true, DurationMs: 100},
		{Timestamp: now, Cmd: "schema dump", Profile: "dev", OK: true, DurationMs: 200},
	}

	result := Query(records, Filter{Cmd: "query"})
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Cmd != "query" {
		t.Errorf("expected cmd=query, got %s", result[0].Cmd)
	}
}

func TestQuery_FilterByAttrs(t *testing.T) {
	now := time.Now()
	records := []*Record{
		{Timestamp: now, Cmd: "query", Profile: "dev", OK: true, DurationMs: 100, Attrs: map[string]string{"env": "dev"}},
		{Timestamp: now, Cmd: "query", Profile: "dev", OK: true, DurationMs: 200, Attrs: map[string]string{"env": "prod"}},
	}

	result := Query(records, Filter{Attrs: map[string]string{"env": "prod"}})
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Attrs["env"] != "prod" {
		t.Errorf("expected attrs.env=prod, got %s", result[0].Attrs["env"])
	}
}

func TestQuery_FilterByTime(t *testing.T) {
	now := time.Now()
	old := now.Add(-2 * time.Hour)
	records := []*Record{
		{Timestamp: old, Cmd: "query", Profile: "dev", OK: true, DurationMs: 100},
		{Timestamp: now, Cmd: "query", Profile: "dev", OK: true, DurationMs: 200},
	}

	since := now.Add(-1 * time.Hour)
	result := Query(records, Filter{Since: &since})
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
}

func TestQuery_SortOrder(t *testing.T) {
	now := time.Now()
	records := []*Record{
		{Timestamp: now, Cmd: "schema dump", Profile: "prod", OK: true, DurationMs: 100},
		{Timestamp: now, Cmd: "query", Profile: "prod", OK: true, DurationMs: 100},
		{Timestamp: now, Cmd: "query", Profile: "dev", OK: true, DurationMs: 100},
	}

	result := Query(records, Filter{})
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}

	if result[0].Cmd != "query" || result[0].Profile != "dev" {
		t.Errorf("expected first: query/dev, got %s/%s", result[0].Cmd, result[0].Profile)
	}
	if result[1].Cmd != "query" || result[1].Profile != "prod" {
		t.Errorf("expected second: query/prod, got %s/%s", result[1].Cmd, result[1].Profile)
	}
	if result[2].Cmd != "schema dump" {
		t.Errorf("expected third: schema dump, got %s", result[2].Cmd)
	}
}

func TestMatchAttrs(t *testing.T) {
	tests := []struct {
		name   string
		record map[string]string
		filter map[string]string
		want   bool
	}{
		{"empty filter", map[string]string{"a": "1"}, nil, true},
		{"match", map[string]string{"a": "1", "b": "2"}, map[string]string{"a": "1"}, true},
		{"no match", map[string]string{"a": "1"}, map[string]string{"a": "2"}, false},
		{"missing key", map[string]string{"a": "1"}, map[string]string{"b": "1"}, false},
		{"empty record", nil, map[string]string{"a": "1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchAttrs(tt.record, tt.filter); got != tt.want {
				t.Errorf("matchAttrs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAttrsKey(t *testing.T) {
	tests := []struct {
		name  string
		attrs map[string]string
		want  string
	}{
		{"nil", nil, ""},
		{"single", map[string]string{"a": "1"}, "a=1"},
		{"multiple sorted", map[string]string{"b": "2", "a": "1"}, "a=1,b=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attrsKey(tt.attrs); got != tt.want {
				t.Errorf("attrsKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
