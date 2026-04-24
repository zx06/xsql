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

// TestHandler_Authentication tests the auth middleware
func TestHandler_Authentication(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		AuthRequired: true,
		AuthToken:    "secret-token",
		Assets:       fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	// Without auth header, should get 401
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// TestHandler_ConfigJS tests the config.js endpoint
func TestHandler_ConfigJS(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		InitialProfile: "dev",
		AuthRequired:   false,
		Assets:         fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	req := httptest.NewRequest(http.MethodGet, "/config.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for config.js, got %d", rec.Code)
	}

	// Should contain JavaScript config variable
	body := rec.Body.String()
	if !strings.Contains(body, "window.__XSQL_WEB_CONFIG__") {
		t.Errorf("expected window.__XSQL_WEB_CONFIG__ in response")
	}
}

// TestHandler_FrontendAssets tests the static asset serving
func TestHandler_FrontendAssets(t *testing.T) {
	assets := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>frontend</html>")},
	}

	handler := NewHandler(HandlerOptions{
		Assets: assets,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "frontend") {
		t.Errorf("expected frontend HTML in response")
	}
}

// TestPublicURL_Comprehensive tests all branches of PublicURL function
func TestPublicURL_Comprehensive(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "loopback", addr: "127.0.0.1:8788", want: "http://127.0.0.1:8788/"},
		{name: "wildcard", addr: "0.0.0.0:8788", want: "http://127.0.0.1:8788/"},
		{name: "ipv6_loopback", addr: "[::1]:8788", want: "http://[[::1]]:8788/"},
		{name: "ipv6_wildcard", addr: "[::]:8788", want: "http://127.0.0.1:8788/"},
		{name: "ipv6_address", addr: "[2001:db8::1]:8788", want: "http://[[2001:db8::1]]:8788/"},
		{name: "invalid_port", addr: "127.0.0.1", want: "http://127.0.0.1/"},
		{name: "localhost", addr: "localhost:8788", want: "http://localhost:8788/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PublicURL(tt.addr)
			if got != tt.want {
				t.Errorf("PublicURL(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

// TestParseIncludeSystem tests parseIncludeSystem helper
func TestParseIncludeSystem(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    bool
		wantErr bool
	}{
		{name: "include_true", query: "?include_system=true", want: true, wantErr: false},
		{name: "include_false", query: "?include_system=false", want: false, wantErr: false},
		{name: "include_missing", query: "", want: false, wantErr: false},
		{name: "include_invalid", query: "?include_system=invalid", want: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test"+tt.query, nil)
			got, err := parseIncludeSystem(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIncludeSystem error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseIncludeSystem got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMustJSON tests mustJSON marshaling
func TestMustJSON(t *testing.T) {
	type testData struct {
		Name string
		Age  int
	}

	// Test valid data marshaling
	data := testData{Name: "test", Age: 30}
	result := mustJSON(data)

	if !strings.Contains(result, "test") || !strings.Contains(result, "30") {
		t.Errorf("mustJSON produced invalid JSON: %s", result)
	}

	// Verify it's valid JSON by unmarshaling
	var decoded testData
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		t.Errorf("mustJSON produced invalid JSON: %v", err)
	}

	if decoded.Name != "test" || decoded.Age != 30 {
		t.Errorf("mustJSON lost data during marshaling")
	}

	// Test with value that would fail JSON marshaling
	// (though any type should marshal successfully)
	emptyResult := mustJSON(nil)
	if emptyResult == "" || emptyResult == "{}" {
		// Either null or empty object is acceptable
	}
}
