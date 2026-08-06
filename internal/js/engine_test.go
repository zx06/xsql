package js

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/session"
)

func TestJSEngine_MultiDatasetRecall(t *testing.T) {
	store := session.NewSessionDataStore()

	res1 := &db.QueryResult{
		Columns: []string{"id", "name", "status"},
		Rows: []map[string]any{
			{"id": 1, "name": "srv1", "status": "ONLINE"},
			{"id": 2, "name": "srv2", "status": "OFFLINE"},
		},
	}
	store.Save("servers query", res1)

	res2 := &db.QueryResult{
		Columns: []string{"id", "server_id", "severity"},
		Rows: []map[string]any{
			{"id": 101, "server_id": 1, "severity": "HIGH"},
		},
	}
	store.Save("alerts query", res2)

	engine := NewJSEngine(5 * time.Second)

	// JS script joining res1 and res2
	jsCode := `
		(function() {
			var alertServerIds = new Set(res2.map(function(a) { return a.server_id; }));
			var onlineWithAlerts = res1.filter(function(s) {
				return s.status === 'ONLINE' && alertServerIds.has(s.id);
			});
			return {
				count: onlineWithAlerts.length,
				servers: onlineWithAlerts
			};
		})();
	`

	result, xe := engine.Execute(context.Background(), jsCode, store)
	if xe != nil {
		t.Fatalf("unexpected execution error: %v", xe)
	}

	if !strings.Contains(result.JSONString, `"count": 1`) || !strings.Contains(result.JSONString, `"srv1"`) {
		t.Fatalf("expected joined result with srv1, got:\n%s", result.JSONString)
	}
}

func TestJSEngine_Timeout(t *testing.T) {
	engine := NewJSEngine(100 * time.Millisecond)

	jsCode := `
		while(true) {}
	`

	_, xe := engine.Execute(context.Background(), jsCode, nil)
	if xe == nil {
		t.Fatal("expected timeout error for infinite loop, got nil")
	}
}

func TestJSEngine_SyntaxAndRuntimeError(t *testing.T) {
	engine := NewJSEngine(1 * time.Second)

	_, xe := engine.Execute(context.Background(), "invalid js {code", nil)
	if xe == nil {
		t.Fatal("expected syntax error for invalid JS")
	}

	_, xe = engine.Execute(context.Background(), "throw new Error('custom js error');", nil)
	if xe == nil {
		t.Fatal("expected runtime error for thrown error")
	}
}

func TestJSEngine_PrimitiveResult(t *testing.T) {
	engine := NewJSEngine(1 * time.Second)

	res, xe := engine.Execute(context.Background(), "'hello world'", nil)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if res.SummaryText != "hello world" {
		t.Fatalf("expected summary 'hello world', got %q", res.SummaryText)
	}

	res, xe = engine.Execute(context.Background(), "[1, 2, 3]", nil)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if !strings.Contains(res.JSONString, "[1, 2, 3]") && !strings.Contains(res.JSONString, "[\n  1") {
		t.Fatalf("expected array json output, got %q", res.JSONString)
	}
}

func TestJSEngine_ConsoleAndNullAndDefaultTimeout(t *testing.T) {
	engine := NewJSEngine(0) // Default 1 min
	if engine.DefaultTimeout <= 0 {
		t.Fatal("expected positive default timeout")
	}

	store := session.NewSessionDataStore()
	store.Save("test query", &db.QueryResult{
		Columns: []string{"id"},
		Rows:    []map[string]any{{"id": 10}},
	})

	// 1. Console log and error with undefined return
	jsCode := `
		console.log("log msg", 123);
		console.error("error msg");
		rows.length;
	`
	res, xe := engine.Execute(nil, jsCode, store)
	if xe != nil {
		t.Fatalf("unexpected execution error: %v", xe)
	}
	if len(res.Logs) != 2 || !strings.Contains(res.SummaryText, "[ERROR] error msg") {
		t.Fatalf("expected 2 log entries in summary, got:\n%s", res.SummaryText)
	}

	// 2. Null/Undefined return with console logs
	resNull, xe := engine.Execute(nil, `console.log("hello"); null;`, nil)
	if xe != nil {
		t.Fatalf("unexpected execution error: %v", xe)
	}
	if resNull.JSONString != "null" || !strings.Contains(resNull.SummaryText, "hello") {
		t.Fatalf("expected null return with logs, got %q", resNull.SummaryText)
	}

	// 3. JSON String return
	resJSON, xe := engine.Execute(nil, `JSON.stringify({status: "ok"})`, nil)
	if xe != nil {
		t.Fatalf("unexpected execution error: %v", xe)
	}
	if !strings.Contains(resJSON.JSONString, `"status": "ok"`) {
		t.Fatalf("expected pretty formatted JSON string, got %q", resJSON.JSONString)
	}
}
