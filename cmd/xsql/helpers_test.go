package main

import (
	"testing"

	"github.com/zx06/xsql/internal/output"
)

// TestResolveAuto tests the resolveAuto helper function
func TestResolveAuto(t *testing.T) {
	tests := []struct {
		name   string
		format output.Format
		want   output.Format
	}{
		{
			name:   "auto_returns_json_or_table",
			format: output.FormatAuto,
			want:   output.FormatJSON, // In test environment, not a TTY
		},
		{
			name:   "json_passthrough",
			format: output.FormatJSON,
			want:   output.FormatJSON,
		},
		{
			name:   "table_passthrough",
			format: output.FormatTable,
			want:   output.FormatTable,
		},
		{
			name:   "csv_passthrough",
			format: output.FormatCSV,
			want:   output.FormatCSV,
		},
		{
			name:   "yaml_passthrough",
			format: output.FormatYAML,
			want:   output.FormatYAML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAuto(tt.format)
			if got != tt.want {
				t.Errorf("resolveAuto(%v) = %v, want %v", tt.format, got, tt.want)
			}
		})
	}
}
