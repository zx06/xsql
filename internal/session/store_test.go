package session

import (
	"strings"
	"testing"

	"github.com/zx06/xsql/internal/db"
)

func TestSessionDataStore(t *testing.T) {
	store := NewSessionDataStore()

	if catalog := store.GetCatalog(); !strings.Contains(catalog, "No active datasets") {
		t.Fatalf("expected empty catalog warning, got: %s", catalog)
	}

	res1 := &db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    []map[string]any{{"id": 1, "name": "srv1"}},
	}
	id1 := store.Save("servers query", res1)
	if id1 != "res1" {
		t.Fatalf("expected ID 'res1', got %s", id1)
	}

	res2 := &db.QueryResult{
		Columns: []string{"id", "severity"},
		Rows:    []map[string]any{{"id": 1, "severity": "HIGH"}},
	}
	id2 := store.Save("alerts query", res2)
	if id2 != "res2" {
		t.Fatalf("expected ID 'res2', got %s", id2)
	}

	latest, ok := store.Latest()
	if !ok || len(latest.Rows) != 1 || latest.Rows[0]["severity"] != "HIGH" {
		t.Fatal("failed to retrieve latest dataset")
	}

	catalog := store.GetCatalog()
	if !strings.Contains(catalog, "`res1`") || !strings.Contains(catalog, "`res2`") {
		t.Fatalf("expected catalog to list res1 and res2, got:\n%s", catalog)
	}

	got1, ok := store.Get("res1")
	if !ok || got1.Rows[0]["name"] != "srv1" {
		t.Fatal("failed to get res1 from store")
	}

	all := store.GetAll()
	if len(all) != 2 || all["res1"] == nil || all["res2"] == nil {
		t.Fatalf("expected GetAll to return 2 datasets, got %d", len(all))
	}
}
