package stats

import (
	"testing"
	"time"
)

func TestStatsTableFormatter_ToTableData_Empty(t *testing.T) {
	f := &StatsTableFormatter{}
	cols, rows, ok := f.ToTableData()
	if ok {
		t.Error("expected ok=false for empty records")
	}
	if cols != nil {
		t.Error("expected nil columns")
	}
	if rows != nil {
		t.Error("expected nil rows")
	}
}

func TestStatsTableFormatter_ToTableData(t *testing.T) {
	f := &StatsTableFormatter{
		Records: []*AggregatedRecord{
			{
				Cmd:     "query",
				Profile: "dev",
				Attrs:   map[string]string{"env": "dev"},
				OK:      10,
				Fail:    2,
				AvgMs:   150,
			},
			{
				Cmd:     "schema dump",
				Profile: "prod",
				OK:      5,
				Fail:    0,
				AvgMs:   200,
			},
		},
	}
	cols, rows, ok := f.ToTableData()
	if !ok {
		t.Error("expected ok=true")
	}
	if len(cols) != 6 {
		t.Errorf("expected 6 columns, got %d", len(cols))
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["cmd"] != "query" {
		t.Errorf("expected cmd=query, got %v", rows[0]["cmd"])
	}
	if rows[0]["attrs"] != "env=dev" {
		t.Errorf("expected attrs=env=dev, got %v", rows[0]["attrs"])
	}
	if rows[1]["attrs"] != "-" {
		t.Errorf("expected attrs=-, got %v", rows[1]["attrs"])
	}
}

func TestStatsLogTableFormatter_ToTableData_Empty(t *testing.T) {
	f := &StatsLogTableFormatter{}
	cols, rows, ok := f.ToTableData()
	if ok {
		t.Error("expected ok=false for empty records")
	}
	if cols != nil {
		t.Error("expected nil columns")
	}
	if rows != nil {
		t.Error("expected nil rows")
	}
}

func TestStatsLogTableFormatter_ToTableData(t *testing.T) {
	f := &StatsLogTableFormatter{
		Records: []Record{
			{
				Timestamp:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
				Cmd:        "query",
				Profile:    "dev",
				OK:         true,
				DurationMs: 100,
				Attrs:      map[string]string{"env": "dev"},
			},
			{
				Timestamp:  time.Date(2025, 1, 15, 10, 31, 0, 0, time.UTC),
				Cmd:        "query",
				Profile:    "prod",
				OK:         false,
				DurationMs: 200,
				ErrorCode:  "XSQL_DB_EXEC_FAILED",
			},
		},
	}
	cols, rows, ok := f.ToTableData()
	if !ok {
		t.Error("expected ok=true")
	}
	if len(cols) != 7 {
		t.Errorf("expected 7 columns, got %d", len(cols))
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["ok"] != "true" {
		t.Errorf("expected ok=true, got %v", rows[0]["ok"])
	}
	if rows[1]["ok"] != "false" {
		t.Errorf("expected ok=false, got %v", rows[1]["ok"])
	}
	if rows[1]["error_code"] != "XSQL_DB_EXEC_FAILED" {
		t.Errorf("expected error_code=XSQL_DB_EXEC_FAILED, got %v", rows[1]["error_code"])
	}
}

func TestFormatAttrs(t *testing.T) {
	tests := []struct {
		name  string
		attrs map[string]string
		want  string
	}{
		{"nil", nil, "-"},
		{"empty", map[string]string{}, "-"},
		{"single", map[string]string{"a": "1"}, "a=1"},
		{"multiple", map[string]string{"a": "1", "b": "2"}, "a=1,b=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAttrs(tt.attrs)
			if got != tt.want {
				t.Errorf("formatAttrs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetXSQLAttrEnv(t *testing.T) {
	// Just verify it returns a string (may be empty)
	_ = GetXSQLAttrEnv()
}
