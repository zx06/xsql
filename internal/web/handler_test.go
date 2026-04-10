package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

type envelope struct {
	OK            bool `json:"ok"`
	SchemaVersion int  `json:"schema_version"`
	Data          any  `json:"data"`
	Error         *struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:8788", want: true},
		{addr: "[::1]:8788", want: true},
		{addr: "localhost:8788", want: true},
		{addr: "0.0.0.0:8788", want: false},
		{addr: ":8788", want: false},
	}

	for _, tc := range cases {
		if got := IsLoopbackAddr(tc.addr); got != tc.want {
			t.Fatalf("IsLoopbackAddr(%q)=%v want %v", tc.addr, got, tc.want)
		}
	}
}

func TestPublicURL(t *testing.T) {
	if got := PublicURL("0.0.0.0:8788"); got != "http://127.0.0.1:8788/" {
		t.Fatalf("PublicURL()=%q", got)
	}
}

func TestHandler_HealthAndConfig(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		InitialProfile: "dev",
		Assets: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/config.js", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config.js status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"initialProfile":"dev"`) {
		t.Fatalf("config.js missing initial profile: %s", rec.Body.String())
	}
}

func TestHandler_ProfilesAuth(t *testing.T) {
	configPath := createConfigFile(t, `
profiles:
  dev:
    db: mysql
    host: 127.0.0.1
    user: root
    database: app
`)
	handler := NewHandler(HandlerOptions{
		ConfigPath:     configPath,
		AuthRequired:   true,
		AuthToken:      "secret",
		InitialProfile: "dev",
		Assets: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	resp := decodeEnvelope(t, rec.Body.Bytes())
	if resp.Error == nil || resp.Error.Code != "XSQL_AUTH_REQUIRED" {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer secret")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	resp = decodeEnvelope(t, rec.Body.Bytes())
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data: %#v", resp.Data)
	}
	profiles, ok := data["profiles"].([]any)
	if !ok || len(profiles) != 1 {
		t.Fatalf("unexpected profiles payload: %#v", data["profiles"])
	}
}

func TestHandler_FrontendFallbackWhenDistMissing(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		Assets: fstest.MapFS{
			"asset.txt": &fstest.MapFile{Data: []byte("x")},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "assets are not built") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandler_SchemaTablesRejectsInvalidIncludeSystem(t *testing.T) {
	configPath := createConfigFile(t, `
profiles:
  dev:
    db: mysql
    host: 127.0.0.1
    user: root
    database: app
`)
	handler := NewHandler(HandlerOptions{
		ConfigPath:     configPath,
		InitialProfile: "dev",
		Assets: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/schema/tables?profile=dev&include_system=wat", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	resp := decodeEnvelope(t, rec.Body.Bytes())
	if resp.Error == nil || resp.Error.Code != "XSQL_CFG_INVALID" {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestHandler_SchemaTableRejectsInvalidPath(t *testing.T) {
	configPath := createConfigFile(t, `
profiles:
  dev:
    db: mysql
    host: 127.0.0.1
    user: root
    database: app
`)
	handler := NewHandler(HandlerOptions{
		ConfigPath:     configPath,
		InitialProfile: "dev",
		Assets: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/schema/tables/public_only?profile=dev", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	resp := decodeEnvelope(t, rec.Body.Bytes())
	if resp.Error == nil || resp.Error.Code != "XSQL_CFG_INVALID" {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestParseSchemaTablePath(t *testing.T) {
	cases := []struct {
		name string
		path string
		ok   bool
		want [2]string
	}{
		{
			name: "plain path",
			path: "/api/v1/schema/tables/public/users",
			ok:   true,
			want: [2]string{"public", "users"},
		},
		{
			name: "escaped path",
			path: "/api/v1/schema/tables/public%20x/user%2Flogs",
			ok:   true,
			want: [2]string{"public x", "user/logs"},
		},
		{
			name: "missing table",
			path: "/api/v1/schema/tables/public",
			ok:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schemaName, tableName, ok := parseSchemaTablePath(tc.path)
			if ok != tc.ok {
				t.Fatalf("ok=%v want %v", ok, tc.ok)
			}
			if !tc.ok {
				return
			}
			if schemaName != tc.want[0] || tableName != tc.want[1] {
				t.Fatalf("got (%q,%q) want (%q,%q)", schemaName, tableName, tc.want[0], tc.want[1])
			}
		})
	}
}

func createConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func decodeEnvelope(t *testing.T, body []byte) envelope {
	t.Helper()
	var resp envelope
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("invalid response: %v body=%s", err, string(body))
	}
	return resp
}
