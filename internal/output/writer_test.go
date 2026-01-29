package output

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/zx06/xsql/internal/errors"
)

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
	if env.Error.Details == nil || env.Error.Details["cause"] == nil {
		t.Error("error details should contain cause")
	}
	if env.Error.Details["cause"] != "underlying error" {
		t.Errorf("cause=%v, want 'underlying error'", env.Error.Details["cause"])
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
