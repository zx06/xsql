package output

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/zx06/xsql/internal/errors"
)

type tableFormatterData struct{}

func (tableFormatterData) ToTableData() ([]string, []map[string]any, bool) {
	return []string{"id"}, []map[string]any{{"id": 1}}, true
}

func TestWriteOK_JSONEnvelope(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})
	if err := w.WriteOK(FormatJSON, map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK || env.SchemaVersion != SchemaVersion {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestWriteError_JSONEnvelope(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})
	xe := errors.New(errors.CodeCfgInvalid, "bad", map[string]any{"x": 1})
	if err := w.WriteError(FormatJSON, xe); err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.OK || env.Error == nil || env.Error.Code != errors.CodeCfgInvalid {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestWriteError_WithCause(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})
	cause := stderrors.New("underlying error")
	xe := errors.Wrap(errors.CodeDBExecFailed, "query failed", nil, cause)
	if err := w.WriteError(FormatJSON, xe); err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Details != nil && env.Error.Details["cause"] != nil {
		t.Errorf("error details should not expose cause, got: %v", env.Error.Details["cause"])
	}
}

func TestWriteOK_YAMLFormat(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})
	if err := w.WriteOK(FormatYAML, map[string]any{"version": "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	result := out.String()
	if !strings.Contains(result, "ok: true") {
		t.Errorf("YAML should contain 'ok: true', got: %s", result)
	}
	if !strings.Contains(result, "version: 1.0.0") {
		t.Errorf("YAML should contain version, got: %s", result)
	}
}

func TestWriteOK_TableFormat_QueryResult(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	// 模拟查询结果
	data := map[string]any{
		"columns": []string{"id", "name"},
		"rows": []map[string]any{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		},
	}

	if err := w.WriteOK(FormatTable, data); err != nil {
		t.Fatal(err)
	}

	result := out.String()

	// 检查表头
	if !strings.Contains(result, "id") || !strings.Contains(result, "name") {
		t.Errorf("table should contain column headers, got: %s", result)
	}

	// 检查数据
	if !strings.Contains(result, "Alice") || !strings.Contains(result, "Bob") {
		t.Errorf("table should contain row data, got: %s", result)
	}

	// 检查行数统计
	if !strings.Contains(result, "(2 rows)") {
		t.Errorf("table should contain row count, got: %s", result)
	}

	// 不应包含 ok 和 schema_version
	if strings.Contains(result, "schema_version") {
		t.Errorf("table format should not contain schema_version, got: %s", result)
	}
}

func TestWriteOK_TableFormat_NonQueryResult(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	// 非查询结果数据（如 version 命令）
	data := map[string]any{"version": "1.0.0"}

	if err := w.WriteOK(FormatTable, data); err != nil {
		t.Fatal(err)
	}

	result := out.String()

	// 应该包含 version 数据
	if !strings.Contains(result, "version") || !strings.Contains(result, "1.0.0") {
		t.Errorf("table should contain version data, got: %s", result)
	}

	// 不应包含 ok 和 schema_version
	if strings.Contains(result, "schema_version") {
		t.Errorf("table format should not contain schema_version, got: %s", result)
	}
}

func TestWriteOK_TableFormat_EmptyResult(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	data := map[string]any{
		"columns": []string{"id", "name"},
		"rows":    []map[string]any{},
	}

	if err := w.WriteOK(FormatTable, data); err != nil {
		t.Fatal(err)
	}

	result := out.String()
	if !strings.Contains(result, "(0 rows)") {
		t.Errorf("table should show 0 rows, got: %s", result)
	}
}

func TestWriteOK_TableFormat_NullValue(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	data := map[string]any{
		"columns": []string{"id", "email"},
		"rows": []map[string]any{
			{"id": 1, "email": nil},
		},
	}

	if err := w.WriteOK(FormatTable, data); err != nil {
		t.Fatal(err)
	}

	result := out.String()
	if !strings.Contains(result, "<null>") {
		t.Errorf("table should render null as <null>, got: %s", result)
	}
}

func TestWriteOK_CSVFormat_QueryResult(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	// 模拟查询结果
	data := map[string]any{
		"columns": []string{"id", "name"},
		"rows": []map[string]any{
			{"id": 1, "name": "Alice"},
		},
	}

	if err := w.WriteOK(FormatCSV, data); err != nil {
		t.Fatal(err)
	}

	result := out.String()

	// 检查表头
	if !strings.Contains(result, "id,name") {
		t.Errorf("csv should contain column headers, got: %s", result)
	}

	// 检查数据
	if !strings.Contains(result, "Alice") {
		t.Errorf("csv should contain row data, got: %s", result)
	}

	// 不应包含 ok 和 schema_version
	if strings.Contains(result, "schema_version") {
		t.Errorf("csv format should not contain schema_version, got: %s", result)
	}
}

func TestWriteOK_CSVFormat_NullValue(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	data := map[string]any{
		"columns": []string{"id", "email"},
		"rows": []map[string]any{
			{"id": 1, "email": nil},
		},
	}

	if err := w.WriteOK(FormatCSV, data); err != nil {
		t.Fatal(err)
	}

	result := out.String()
	if !strings.Contains(result, "1,") {
		t.Errorf("csv should render null as empty, got: %s", result)
	}
}

func TestWriteOK_CSVFormat_NonQueryResult(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	data := map[string]any{"version": "1.0.0", "build": "abc123"}

	if err := w.WriteOK(FormatCSV, data); err != nil {
		t.Fatal(err)
	}

	result := out.String()
	// Non-query results output as key,value pairs
	if !strings.Contains(result, "version") || !strings.Contains(result, "1.0.0") {
		t.Errorf("csv should contain version data, got: %s", result)
	}
}

func TestWriteError_TableFormat(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})
	xe := errors.New(errors.CodeDBExecFailed, "query failed", nil)
	if err := w.WriteError(FormatTable, xe); err != nil {
		t.Fatal(err)
	}
	result := out.String()
	if !strings.Contains(result, "XSQL_DB_EXEC_FAILED") {
		t.Errorf("table should contain error code, got: %s", result)
	}
	if !strings.Contains(result, "query failed") {
		t.Errorf("table should contain error message, got: %s", result)
	}
}

func TestWriteError_CSVFormat(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})
	xe := errors.New(errors.CodeCfgInvalid, "bad config", nil)
	if err := w.WriteError(FormatCSV, xe); err != nil {
		t.Fatal(err)
	}
	result := out.String()
	if !strings.Contains(result, "error") {
		t.Errorf("csv should contain error, got: %s", result)
	}
	if !strings.Contains(result, "bad config") {
		t.Errorf("csv should contain message, got: %s", result)
	}
}

func TestIsValid(t *testing.T) {
	validFormats := []Format{FormatJSON, FormatYAML, FormatTable, FormatCSV, FormatAuto}
	for _, f := range validFormats {
		if !IsValid(f) {
			t.Errorf("IsValid(%s) should be true", f)
		}
	}

	if IsValid(Format("invalid")) {
		t.Error("IsValid('invalid') should be false")
	}
	if IsValid(Format("")) {
		t.Error("IsValid('') should be false")
	}
}

func TestWriteOK_InvalidFormat(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})
	err := w.WriteOK(Format("invalid"), map[string]any{})
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestWriteOK_TableFormat_ProfileListWithDescription(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	// 模拟 profile list 数据（包含 description）
	data := map[string]any{
		"config_path": "/path/to/config.yaml",
		"profiles": []map[string]any{
			{
				"name":        "dev",
				"description": "开发环境数据库",
				"db":          "mysql",
				"mode":        "read-only",
			},
			{
				"name":        "prod",
				"description": "生产环境数据库",
				"db":          "pg",
				"mode":        "read-write",
			},
		},
	}

	if err := w.WriteOK(FormatTable, data); err != nil {
		t.Fatal(err)
	}

	result := out.String()

	// 检查表头包含四列
	if !strings.Contains(result, "NAME") || !strings.Contains(result, "DESCRIPTION") || !strings.Contains(result, "DB") || !strings.Contains(result, "MODE") {
		t.Errorf("table should contain NAME, DESCRIPTION, DB, MODE columns, got: %s", result)
	}

	// 检查分隔线正确
	if !strings.Contains(result, "----") || !strings.Contains(result, "-----------") {
		t.Errorf("table should contain column separators, got: %s", result)
	}

	// 检查 profile 数据
	if !strings.Contains(result, "dev") || !strings.Contains(result, "开发环境数据库") {
		t.Errorf("table should contain dev profile with description, got: %s", result)
	}
	if !strings.Contains(result, "prod") || !strings.Contains(result, "生产环境数据库") {
		t.Errorf("table should contain prod profile with description, got: %s", result)
	}

	// 检查 profiles 数量
	if !strings.Contains(result, "(2 profiles)") {
		t.Errorf("table should show 2 profiles, got: %s", result)
	}
}

func TestWriteOK_TableFormat_TableFormatter(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	if err := w.WriteOK(FormatTable, tableFormatterData{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "id") {
		t.Fatalf("expected table output, got %s", out.String())
	}
}

func TestWriteOK_TableFormat_ProfileListWithoutDescription(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	// 模拟 profile list 数据（不包含 description，向后兼容）
	data := map[string]any{
		"config_path": "/path/to/config.yaml",
		"profiles": []map[string]any{
			{
				"name": "old-style",
				"db":   "mysql",
				"mode": "read-only",
			},
		},
	}

	if err := w.WriteOK(FormatTable, data); err != nil {
		t.Fatal(err)
	}

	result := out.String()

	// 检查表头仍然包含四列（保持一致性）
	if !strings.Contains(result, "NAME") || !strings.Contains(result, "DESCRIPTION") || !strings.Contains(result, "DB") || !strings.Contains(result, "MODE") {
		t.Errorf("table should contain NAME, DESCRIPTION, DB, MODE columns even for profiles without description, got: %s", result)
	}

	// 检查 profile 数据（description 列应该为空）
	if !strings.Contains(result, "old-style") {
		t.Errorf("table should contain old-style profile, got: %s", result)
	}

	// 检查 profiles 数量
	if !strings.Contains(result, "(1 profile)") {
		t.Errorf("table should show 1 profile, got: %s", result)
	}
}

func TestWriteOK_TableFormat_ProfileListWithEmptyDescription(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

	// 模拟 profile list 数据（包含空 description）
	data := map[string]any{
		"config_path": "/path/to/config.yaml",
		"profiles": []map[string]any{
			{
				"name":        "empty-desc",
				"description": "",
				"db":          "mysql",
				"mode":        "read-write",
			},
		},
	}

	if err := w.WriteOK(FormatTable, data); err != nil {
		t.Fatal(err)
	}

	result := out.String()

	// 检查表头
	if !strings.Contains(result, "NAME") || !strings.Contains(result, "DESCRIPTION") {
		t.Errorf("table should contain NAME, DESCRIPTION columns, got: %s", result)
	}

	// 检查 profile 数据
	if !strings.Contains(result, "empty-desc") {
		t.Errorf("table should contain empty-desc profile, got: %s", result)
	}

	// 检查 profiles 数量
	if !strings.Contains(result, "(1 profile)") {
		t.Errorf("table should show 1 profile, got: %s", result)
	}
}

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

	t.Run("empty slice", func(t *testing.T) {
		_, ok := tryAsProfileList([]map[string]any{})
		if ok {
			t.Error("expected ok=false for empty slice")
		}
	})

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

	t.Run("nil", func(t *testing.T) {
		_, ok := tryAsQueryResultReflect(nil)
		if ok {
			t.Error("expected ok=false for nil")
		}
	})

	t.Run("non-struct", func(t *testing.T) {
		_, ok := tryAsQueryResultReflect("not a struct")
		if ok {
			t.Error("expected ok=false for non-struct")
		}
	})

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

func TestFormatCellValue(t *testing.T) {
	if got := formatCellValue(nil, "<null>"); got != "<null>" {
		t.Fatalf("expected null placeholder, got %q", got)
	}
	if got := formatCellValue(float64(10), "<null>"); got != "10" {
		t.Fatalf("expected integer float to render without decimals, got %q", got)
	}
	if got := formatCellValue(float64(10.5), "<null>"); got != "10.5" {
		t.Fatalf("expected float to render with decimals, got %q", got)
	}
}

func TestWriteOK_YAMLFormat_EmptyData(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})

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
	t.Logf("nil data output: %s", result)
}
