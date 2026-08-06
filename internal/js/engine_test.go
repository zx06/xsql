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
