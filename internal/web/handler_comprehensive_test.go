package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/zx06/xsql/internal/errors"
)

// TestHandler_Health_Success tests successful health endpoint
func TestHandler_Health_Success(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		InitialProfile: "default",
		Assets:         fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Errorf("invalid JSON response: %v", err)
	}

	if !resp.OK {
		t.Error("expected OK=true")
	}
}

// TestHandler_Profiles_NoConfig tests profiles endpoint without config
func TestHandler_Profiles_NoConfig(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		ConfigPath: "/nonexistent/config.yaml",
		Assets:     fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should fail to load config
	if rec.Code == http.StatusOK {
		t.Errorf("expected error status, got 200")
	}
}

// TestHandler_Profiles_WithConfig tests profiles endpoint with valid config
func TestHandler_Profiles_WithConfig(t *testing.T) {
	configPath := createConfigFile(t, `
profiles:
  default:
    driver: mysql
    host: localhost
    port: 3306
    user: root
    password: root
  test:
    driver: mysql
    host: 127.0.0.1
    port: 3307
    user: testuser
    password: testpass
`)

	handler := NewHandler(HandlerOptions{
		ConfigPath: configPath,
		Assets:     fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if !resp.OK {
		t.Error("expected OK=true")
	}
}

// TestHandler_ProfileShow_Success tests getting a specific profile
func TestHandler_ProfileShow_Success(t *testing.T) {
	configPath := createConfigFile(t, `
profiles:
  dev:
    driver: mysql
    host: localhost
    port: 3306
    user: root
    password: root
`)

	handler := NewHandler(HandlerOptions{
		ConfigPath: configPath,
		Assets:     fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/dev", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if !resp.OK {
		t.Error("expected OK=true")
	}
}

// TestHandler_PostNotAllowed tests POST method not allowed
func TestHandler_PostNotAllowed(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	// POST to health should fail
	req := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

// TestHandler_ConfigJS_Format tests config.js format
func TestHandler_ConfigJS_Format(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		InitialProfile: "test-profile",
		AuthRequired:   true,
		Assets:         fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	req := httptest.NewRequest(http.MethodGet, "/config.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "window.__XSQL_WEB_CONFIG__") {
		t.Error("expected window.__XSQL_WEB_CONFIG__ variable")
	}

	if !strings.Contains(body, "test-profile") {
		t.Error("expected initial profile in config")
	}

	if !strings.Contains(body, "true") {
		t.Error("expected authRequired in config")
	}
}

// TestHandler_Frontend_Index tests serving index.html
func TestHandler_Frontend_Index(t *testing.T) {
	assets := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><body>Welcome</body></html>")},
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

	if !strings.Contains(rec.Body.String(), "Welcome") {
		t.Error("expected Welcome in index.html")
	}
}

// TestHandler_Frontend_SubPath tests serving nested assets
func TestHandler_Frontend_SubPath(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":   &fstest.MapFile{Data: []byte("<html>index</html>")},
		"css/style.css": &fstest.MapFile{Data: []byte("body { color: red; }")},
	}

	handler := NewHandler(HandlerOptions{
		Assets: assets,
	})

	req := httptest.NewRequest(http.MethodGet, "/css/style.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "color: red") {
		t.Error("expected CSS content")
	}
}

// TestHandler_Auth_BearerToken tests Bearer token extraction
func TestHandler_Auth_BearerToken(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		AuthRequired: true,
		AuthToken:    "my-secret-token",
		Assets:       fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	// Valid token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer my-secret-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected not 401 with valid token, got %d", rec.Code)
	}
}

// TestHandler_Auth_InvalidToken tests invalid token
func TestHandler_Auth_InvalidToken(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		AuthRequired: true,
		AuthToken:    "correct-token",
		Assets:       fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// TestHandler_Health_WrongMethod tests wrong HTTP method
func TestHandler_Health_WrongMethod(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/v1/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for %s, got %d", method, rec.Code)
		}
	}
}

// TestHandler_ResponseContentType tests correct content types
func TestHandler_ResponseContentType(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}},
	})

	// Test JSON API response content type
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected application/json, got %s", contentType)
	}

	// Test JavaScript content type for config.js
	req = httptest.NewRequest(http.MethodGet, "/config.js", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	contentType = rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/javascript") {
		t.Errorf("expected application/javascript, got %s", contentType)
	}
}

// TestParseSchemaTablePath_Complex tests path parsing with special characters
func TestParseSchemaTablePath_Complex(t *testing.T) {
	cases := []struct {
		path      string
		wantOK    bool
		wantSchema string
		wantTable string
	}{
		{"/api/v1/schema/tables/public/users", true, "public", "users"},
		{"/api/v1/schema/tables/my_schema/my_table", true, "my_schema", "my_table"},
		{"/api/v1/schema/tables/public", false, "", ""},
		{"/api/v1/schema/tables/public/", false, "", ""},
		{"/api/v1/schema/tables/public%20schema/public%20table", true, "public schema", "public table"},
	}

	for _, tc := range cases {
		schema, table, ok := parseSchemaTablePath(tc.path)
		if ok != tc.wantOK {
			t.Errorf("path=%q: ok=%v want %v", tc.path, ok, tc.wantOK)
			continue
		}
		if ok && (schema != tc.wantSchema || table != tc.wantTable) {
			t.Errorf("path=%q: got (%q,%q) want (%q,%q)", tc.path, schema, table, tc.wantSchema, tc.wantTable)
		}
	}
}

// TestIsLoopbackAddr_Various tests loopback address detection
func TestIsLoopbackAddr_Various(t *testing.T) {
	cases := []struct {
		addr       string
		wantResult bool
	}{
		{"127.0.0.1:8080", true},
		{"[::1]:8080", true},
		{"localhost:8080", true},
		{"0.0.0.0:8080", false},
		{"192.168.1.1:8080", false},
		{"10.0.0.1:8080", false},
		{":8080", false},
		{"8080", false},
	}

	for _, tc := range cases {
		result := IsLoopbackAddr(tc.addr)
		if result != tc.wantResult {
			t.Errorf("IsLoopbackAddr(%q)=%v want %v", tc.addr, result, tc.wantResult)
		}
	}
}

// TestPublicURL_Various tests public URL conversion
func TestPublicURL_Various(t *testing.T) {
	cases := []struct {
		addr    string
		wantURL string
	}{
		{"0.0.0.0:8080", "http://127.0.0.1:8080/"},
		{"127.0.0.1:8080", "http://127.0.0.1:8080/"},
		{":8080", "http://127.0.0.1:8080/"},
		{"192.168.1.1:8080", "http://192.168.1.1:8080/"},
	}

	for _, tc := range cases {
		result := PublicURL(tc.addr)
		if result != tc.wantURL {
			t.Errorf("PublicURL(%q)=%q want %q", tc.addr, result, tc.wantURL)
		}
	}
}

// TestStatusCodeFor_AllCases tests all error code mappings to HTTP status codes
func TestStatusCodeFor_AllCases(t *testing.T) {
	tests := []struct {
		code     errors.Code
		expected int
	}{
		{errors.CodeCfgInvalid, http.StatusBadRequest},
		{errors.CodeCfgNotFound, http.StatusBadRequest},
		{errors.CodeSecretNotFound, http.StatusBadRequest},
		{errors.CodeAuthRequired, http.StatusUnauthorized},
		{errors.CodeAuthInvalid, http.StatusUnauthorized},
		{errors.CodeROBlocked, http.StatusForbidden},
		{errors.CodeDBConnectFailed, http.StatusBadGateway},
		{errors.CodeDBAuthFailed, http.StatusBadGateway},
		{errors.CodeSSHDialFailed, http.StatusBadGateway},
		{errors.CodeSSHAuthFailed, http.StatusBadGateway},
		{errors.CodeSSHHostKeyMismatch, http.StatusBadGateway},
		{errors.CodeDBDriverUnsupported, http.StatusBadRequest},
		{errors.CodeDBExecFailed, http.StatusBadRequest},
		{errors.CodeInternal, http.StatusInternalServerError},
	}

	for _, test := range tests {
		result := statusCodeFor(test.code)
		if result != test.expected {
			t.Errorf("statusCodeFor(%q) = %d, want %d", test.code, result, test.expected)
		}
	}
}
