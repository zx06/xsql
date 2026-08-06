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
