package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler_HandleQuery_MaxRowsNegative(t *testing.T) {
	configPath := createConfigFile(t, `
profiles:
  dev:
    db: mysql
    dsn: "root:root@tcp(127.0.0.1:3306)/testdb"
`)
	handler := NewHandler(HandlerOptions{
		ConfigPath: configPath,
		Assets:     fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	body := []byte(`{"profile":"dev","sql":"SELECT 1","max_rows":-1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_HandleQuery_MaxRowsExplicit(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}
	configPath := createConfigFile(t, `
profiles:
  dev:
    db: mysql
    dsn: "`+dsn+`"
`)
	handler := NewHandler(HandlerOptions{
		ConfigPath: configPath,
		Assets:     fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	body := []byte(`{"profile":"dev","sql":"WITH RECURSIVE seq AS (SELECT 1 AS n UNION ALL SELECT n+1 FROM seq WHERE n < 10) SELECT n FROM seq","max_rows":2}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("expected ok: %+v", resp)
	}
}

func TestHandler_HandleQuery_InvalidEnvMaxRows(t *testing.T) {
	t.Setenv("XSQL_MAX_ROWS", "not-a-number")
	configPath := createConfigFile(t, `
profiles:
  dev:
    db: mysql
    dsn: "root:root@tcp(127.0.0.1:3306)/testdb"
`)
	handler := NewHandler(HandlerOptions{
		ConfigPath: configPath,
		Assets:     fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	body := []byte(`{"profile":"dev","sql":"SELECT 1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
