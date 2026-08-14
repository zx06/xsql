package stats

import (
	"sort"
	"strings"
	"time"
)

// Record is a single usage statistics entry.
type Record struct {
	Timestamp  time.Time         `json:"ts"`
	Cmd        string            `json:"cmd"`
	Profile    string            `json:"profile"`
	OK         bool              `json:"ok"`
	DurationMs int64             `json:"duration_ms"`
	ErrorCode  string            `json:"error_code,omitempty"`
	SQL        string            `json:"sql,omitempty"`
	Attrs      map[string]string `json:"attrs,omitempty"`
}

// StatsConfig holds the statistics configuration.
type StatsConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	LogSQL        bool   `yaml:"log_sql" json:"log_sql"`
	FilePath      string `yaml:"file_path" json:"file_path,omitempty"`
	RetentionDays int    `yaml:"retention_days" json:"retention_days,omitempty"`
}

// AggregatedRecord is an aggregated statistics entry.
type AggregatedRecord struct {
	Cmd     string            `json:"cmd"`
	Profile string            `json:"profile"`
	Attrs   map[string]string `json:"attrs,omitempty"`
	OK      int               `json:"ok"`
	Fail    int               `json:"fail"`
	AvgMs   int64             `json:"avg_ms"`
}

// StatsResult is the output of stats aggregation.
type StatsResult struct {
	Records []*AggregatedRecord `json:"records" yaml:"records"`
}

// LogResult is the output of stats log.
type LogResult struct {
	Records []Record `json:"records" yaml:"records"`
	Total   int      `json:"total" yaml:"total"`
}

// StatsTableFormatter implements output.TableFormatter for stats aggregation.
type StatsTableFormatter struct {
	Records []*AggregatedRecord
}

// ToTableData returns table columns and rows for stats aggregation.
func (f *StatsTableFormatter) ToTableData() ([]string, []map[string]any, bool) {
	if len(f.Records) == 0 {
		return nil, nil, false
	}
	columns := []string{"cmd", "profile", "attrs", "ok", "fail", "avg_ms"}
	rows := make([]map[string]any, 0, len(f.Records))
	for _, r := range f.Records {
		attrs := formatAttrs(r.Attrs)
		rows = append(rows, map[string]any{
			"cmd":     r.Cmd,
			"profile": r.Profile,
			"attrs":   attrs,
			"ok":      r.OK,
			"fail":    r.Fail,
			"avg_ms":  r.AvgMs,
		})
	}
	return columns, rows, true
}

// StatsLogTableFormatter implements output.TableFormatter for stats log.
type StatsLogTableFormatter struct {
	Records []Record
}

// ToTableData returns table columns and rows for stats log.
func (f *StatsLogTableFormatter) ToTableData() ([]string, []map[string]any, bool) {
	if len(f.Records) == 0 {
		return nil, nil, false
	}
	columns := []string{"ts", "cmd", "profile", "attrs", "ok", "duration_ms", "error_code"}
	rows := make([]map[string]any, 0, len(f.Records))
	for _, r := range f.Records {
		attrs := formatAttrs(r.Attrs)
		okStr := "false"
		if r.OK {
			okStr = "true"
		}
		rows = append(rows, map[string]any{
			"ts":          r.Timestamp.Format(time.RFC3339),
			"cmd":         r.Cmd,
			"profile":     r.Profile,
			"attrs":       attrs,
			"ok":          okStr,
			"duration_ms": r.DurationMs,
			"error_code":  r.ErrorCode,
		})
	}
	return columns, rows, true
}

func formatAttrs(attrs map[string]string) string {
	if len(attrs) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(attrs[k])
	}
	return sb.String()
}
