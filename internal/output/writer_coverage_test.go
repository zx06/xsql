package output

import (
	"bytes"
	"strings"
	"testing"
)

// ============================================================================
// Helper Function Tests (for coverage)
// ============================================================================

func TestExtractStringSlice(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  []string
		ok    bool
	}{
		{"nil", nil, nil, false},
		{"empty slice", []string{}, []string{}, true},
		{"string slice", []string{"a", "b", "c"}, []string{"a", "b", "c"}, true},
		{"[]any with strings", []any{"x", "y", "z"}, []string{"x", "y", "z"}, true},
		{"[]any with mixed", []any{"a", 1, "b"}, nil, false},
		{"other type", "not a slice", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractStringSlice(tc.input)
			if ok != tc.ok {
				t.Errorf("extractStringSlice(%T) ok=%v, want %v", tc.input, ok, tc.ok)
				return
			}
			if ok && len(got) != len(tc.want) {
				t.Errorf("extractStringSlice(%T)=%v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestExtractMapSlice(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  []map[string]any
		ok    bool
	}{
		{"nil", nil, nil, false},
		{"empty slice", []map[string]any{}, []map[string]any{}, true},
		{"map slice", []map[string]any{{"a": 1}}, []map[string]any{{"a": 1}}, true},
		{"[]any with maps", []any{map[string]any{"x": 1}}, []map[string]any{{"x": 1}}, true},
		{"[]any with non-map", []any{"not a map"}, nil, false},
		{"other type", "not a slice", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractMapSlice(tc.input)
			if ok != tc.ok {
				t.Errorf("extractMapSlice(%T) ok=%v, want %v", tc.input, ok, tc.ok)
				return
			}
			if ok && len(got) != len(tc.want) {
				t.Errorf("extractMapSlice(%T)=%v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestTryAsProfileList(t *testing.T) {
	// Test with []profileListItem
	t.Run("profileListItem slice", func(t *testing.T) {
		input := []profileListItem{
			{Name: "dev", Description: "Dev", DB: "mysql", Mode: "read-only"},
		}
		got, ok := tryAsProfileList(input)
		if !ok {
			t.Error("expected ok=true for []profileListItem")
		}
		if len(got) != 1 || got[0].Name != "dev" {
			t.Errorf("got %+v, want [{dev Dev mysql read-only}]", got)
		}
	})

	// Test with []map[string]any
	t.Run("map slice", func(t *testing.T) {
		input := []map[string]any{
			{"name": "prod", "description": "Prod", "db": "pg", "mode": "read-write"},
		}
		got, ok := tryAsProfileList(input)
		if !ok {
			t.Error("expected ok=true for []map[string]any")
		}
		if len(got) != 1 || got[0].Name != "prod" {
			t.Errorf("got %+v, want [{prod Prod pg read-write}]", got)
		}
	})

	// Test with empty slice
	t.Run("empty slice", func(t *testing.T) {
		_, ok := tryAsProfileList([]map[string]any{})
		if ok {
			t.Error("expected ok=false for empty slice")
		}
	})

	// Test with missing name field
	t.Run("missing name", func(t *testing.T) {
		input := []map[string]any{
			{"description": "No name", "db": "mysql"},
		}
		_, ok := tryAsProfileList(input)
		if ok {
			t.Error("expected ok=false for missing name")
		}
	})
}

func TestTryAsQueryResultReflect(t *testing.T) {
	// Test with struct that has Columns and Rows
	t.Run("struct with Columns and Rows", func(t *testing.T) {
		type MyResult struct {
			Columns []string
			Rows    []map[string]any
		}
		input := MyResult{
			Columns: []string{"id", "name"},
			Rows:    []map[string]any{{"id": 1, "name": "test"}},
		}
		got, ok := tryAsQueryResultReflect(input)
		if !ok {
			t.Error("expected ok=true for struct with Columns/Rows")
		}
		if len(got.columns) != 2 || got.columns[0] != "id" {
			t.Errorf("columns=%v, want [id name]", got.columns)
		}
	})

	// Test with pointer to struct (lowercase fields)
	t.Run("pointer to struct", func(t *testing.T) {
		type MyResult struct {
			Columns []string
			Rows    []map[string]any
		}
		input := &MyResult{
			Columns: []string{"col1"},
			Rows:    []map[string]any{{"col1": "val"}},
		}
		got, ok := tryAsQueryResultReflect(input)
		if !ok {
			t.Error("expected ok=true for pointer to struct")
		}
		if len(got.columns) != 1 {
			t.Errorf("columns=%v, want [col1]", got.columns)
		}
	})

	// Test with nil
	t.Run("nil", func(t *testing.T) {
		_, ok := tryAsQueryResultReflect(nil)
		if ok {
			t.Error("expected ok=false for nil")
		}
	})

	// Test with non-struct
	t.Run("non-struct", func(t *testing.T) {
		_, ok := tryAsQueryResultReflect("not a struct")
		if ok {
			t.Error("expected ok=false for non-struct")
		}
	})

	// Test with struct missing fields
	t.Run("missing fields", func(t *testing.T) {
		type NoColumns struct {
			Rows []map[string]any
		}
		_, ok := tryAsQueryResultReflect(NoColumns{})
		if ok {
			t.Error("expected ok=false for struct missing Columns")
		}
	})
}

func TestWriteOK_YAMLFormat_EmptyData(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	// Test with nil data
	if err := w.WriteOK(FormatYAML, nil); err != nil {
		t.Fatal(err)
	}

	result := out.String()
	if !strings.Contains(result, "ok: true") {
		t.Errorf("YAML should contain 'ok: true', got: %s", result)
	}
}

func TestWriteOK_TableFormat_NilData(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	if err := w.WriteOK(FormatTable, nil); err != nil {
		t.Fatal(err)
	}

	result := out.String()
	// Should handle nil gracefully (no panic)
	t.Logf("nil data output: %s", result)
}
