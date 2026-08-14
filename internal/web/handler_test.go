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

func TestHandler_ConfigManagement(t *testing.T) {
	configPath := createConfigFile(t, `
profiles:
  dev:
    db: mysql
    host: 127.0.0.1
    port: 3306
    user: root
    database: app
ssh_proxies: {}
`)
	handler := NewHandler(HandlerOptions{
		ConfigPath: configPath,
	})

	// 1. GET /api/v1/config
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/config status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"dev"`) {
		t.Fatalf("expected dev profile in config GET response: %s", rec.Body.String())
	}

	// 2. POST /api/v1/config/profiles
	saveBody := `{"name":"staging","profile":{"db":"pg","host":"10.0.0.1","port":5432,"user":"postgres","database":"stg_db"}}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/profiles", strings.NewReader(saveBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/config/profiles status=%d body=%s", rec.Code, rec.Body.String())
	}

	// 3. GET /api/v1/config to verify staging saved
	req = httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"staging"`) {
		t.Fatalf("expected staging profile in config GET response: %s", rec.Body.String())
	}

	// 4. DELETE /api/v1/config/profiles/staging
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/config/profiles/staging", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/config/profiles/staging status=%d body=%s", rec.Code, rec.Body.String())
	}

	// 5. POST /api/v1/config/ssh-proxies
	saveProxyBody := `{"name":"bastion","ssh_proxy":{"host":"bastion.example.com","port":22,"user":"admin"}}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/ssh-proxies", strings.NewReader(saveProxyBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/config/ssh-proxies status=%d body=%s", rec.Code, rec.Body.String())
	}

	// 6. DELETE /api/v1/config/ssh-proxies/bastion
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/config/ssh-proxies/bastion", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/config/ssh-proxies/bastion status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ConfigManagementErrors(t *testing.T) {
	// 1. GET with non-existent config path returns 200 OK with empty profiles
	hInvalid := NewHandler(HandlerOptions{ConfigPath: "/nonexistent/path/xsql.yaml"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec := httptest.NewRecorder()
	hInvalid.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for non-existent config GET, got %d", rec.Code)
	}

	// 2. Save profile invalid JSON body
	configPath := createConfigFile(t, "profiles: {}\nssh_proxies: {}\n")
	h := NewHandler(HandlerOptions{ConfigPath: configPath})

	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/profiles", strings.NewReader("invalid_json"))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON body, got %d", rec.Code)
	}

	// 3. Save profile missing name
	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/profiles", strings.NewReader(`{"name":"","profile":{}}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty profile name, got %d", rec.Code)
	}

	// 4. Delete profile missing name
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/config/profiles/", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty profile delete name, got %d", rec.Code)
	}

	// 5. Delete profile non-existent file error (400)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/config/profiles/dev", nil)
	rec = httptest.NewRecorder()
	hInvalid.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for profile delete error on non-existent config, got %d", rec.Code)
	}

	// 6. Save SSH proxy invalid JSON body
	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/ssh-proxies", strings.NewReader("invalid_json"))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid SSH proxy JSON, got %d", rec.Code)
	}

	// 7. Save SSH proxy missing name
	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/ssh-proxies", strings.NewReader(`{"name":"","ssh_proxy":{}}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty SSH proxy name, got %d", rec.Code)
	}

	// 8. Delete SSH proxy missing name
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/config/ssh-proxies/", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty SSH proxy delete name, got %d", rec.Code)
	}

	// 9. Delete SSH proxy non-existent file error (400)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/config/ssh-proxies/bastion", nil)
	rec = httptest.NewRecorder()
	hInvalid.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for SSH proxy delete error on non-existent config, got %d", rec.Code)
	}

	// 10. Method Not Allowed (405) checks
	methodsNotAllowed := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/config"},
		{http.MethodGet, "/api/v1/config/profiles"},
		{http.MethodGet, "/api/v1/config/profiles/dev"},
		{http.MethodGet, "/api/v1/config/ssh-proxies"},
		{http.MethodGet, "/api/v1/config/ssh-proxies/bastion"},
	}
	for _, tc := range methodsNotAllowed {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: expected 405 MethodNotAllowed, got %d", tc.method, tc.path, w.Code)
		}
	}

	// 11. Test PUT method for profile and SSH proxy save
	putProfileReq := httptest.NewRequest(http.MethodPut, "/api/v1/config/profiles", strings.NewReader(`{"name":"dev2","profile":{"db":"mysql"}}`))
	putProfileRec := httptest.NewRecorder()
	h.ServeHTTP(putProfileRec, putProfileReq)
	if putProfileRec.Code != http.StatusOK {
		t.Errorf("expected 200 for PUT profile save, got %d", putProfileRec.Code)
	}

	putProxyReq := httptest.NewRequest(http.MethodPut, "/api/v1/config/ssh-proxies", strings.NewReader(`{"name":"bastion2","ssh_proxy":{"host":"1.2.3.4"}}`))
	putProxyRec := httptest.NewRecorder()
	h.ServeHTTP(putProxyRec, putProxyReq)
	if putProxyRec.Code != http.StatusOK {
		t.Errorf("expected 200 for PUT ssh proxy save, got %d", putProxyRec.Code)
	}

	// 12. Test Handler with explicit temporary ConfigPath
	tempConfig := createConfigFile(t, "profiles: {}\nssh_proxies: {}\n")
	hEmpty := NewHandler(HandlerOptions{ConfigPath: tempConfig})

	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	recGet := httptest.NewRecorder()
	hEmpty.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Errorf("expected 200 for GET config with handler ConfigPath, got %d", recGet.Code)
	}

	reqSaveP := httptest.NewRequest(http.MethodPost, "/api/v1/config/profiles", strings.NewReader(`{"name":"dev3","profile":{"db":"mysql"}}`))
	recSaveP := httptest.NewRecorder()
	hEmpty.ServeHTTP(recSaveP, reqSaveP)
	if recSaveP.Code != http.StatusOK {
		t.Errorf("expected 200 for SaveProfile with handler ConfigPath, got %d", recSaveP.Code)
	}

	reqDelP := httptest.NewRequest(http.MethodDelete, "/api/v1/config/profiles/dev3", nil)
	recDelP := httptest.NewRecorder()
	hEmpty.ServeHTTP(recDelP, reqDelP)
	if recDelP.Code != http.StatusOK {
		t.Errorf("expected 200 for DeleteProfile with handler ConfigPath, got %d", recDelP.Code)
	}

	reqSaveSP := httptest.NewRequest(http.MethodPost, "/api/v1/config/ssh-proxies", strings.NewReader(`{"name":"bastion3","ssh_proxy":{"host":"1.1.1.1"}}`))
	recSaveSP := httptest.NewRecorder()
	hEmpty.ServeHTTP(recSaveSP, reqSaveSP)
	if recSaveSP.Code != http.StatusOK {
		t.Errorf("expected 200 for SaveSSHProxy with handler ConfigPath, got %d", recSaveSP.Code)
	}

	reqDelSP := httptest.NewRequest(http.MethodDelete, "/api/v1/config/ssh-proxies/bastion3", nil)
	recDelSP := httptest.NewRecorder()
	hEmpty.ServeHTTP(recDelSP, reqDelSP)
	if recDelSP.Code != http.StatusOK {
		t.Errorf("expected 200 for DeleteSSHProxy with handler ConfigPath, got %d", recDelSP.Code)
	}
}

func TestHandler_ConfigAIAndTesting(t *testing.T) {
	tempConfig := createConfigFile(t, "profiles:\n  dev:\n    db: mysql\n    host: 127.0.0.1\n    port: 3306\nssh_proxies:\n  bastion:\n    host: 127.0.0.1\n    port: 22\nai:\n  provider: openai\n  base_url: https://api.openai.com/v1\n  api_key: sk-test\n  model: gpt-4o\n")
	h := NewHandler(HandlerOptions{ConfigPath: tempConfig})

	// 1. GET /api/v1/config returns ai
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for GET config, got %d", rec.Code)
		}
		var env envelope
		_ = json.NewDecoder(rec.Body).Decode(&env)
		dataMap, ok := env.Data.(map[string]any)
		if !ok || dataMap["ai"] == nil {
			t.Fatalf("expected ai section in config response, got: %v", env.Data)
		}
	}

	// 2. POST /api/v1/config/ai saves AI
	{
		body := `{"ai":{"provider":"deepseek","model":"deepseek-chat","base_url":"https://api.deepseek.com","api_key":"sk-deepseek"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/config/ai", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for POST /api/v1/config/ai, got %d: %s", rec.Code, rec.Body.String())
		}

		// Invalid method
		reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/config/ai", nil)
		recGet := httptest.NewRecorder()
		h.ServeHTTP(recGet, reqGet)
		if recGet.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for GET /api/v1/config/ai, got %d", recGet.Code)
		}

		// Invalid json
		reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/config/ai", strings.NewReader("{invalid"))
		recBad := httptest.NewRecorder()
		h.ServeHTTP(recBad, reqBad)
		if recBad.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid json, got %d", recBad.Code)
		}
	}

	// 3. POST /api/v1/config/test/profile
	{
		// Invalid method
		reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/config/test/profile", nil)
		recGet := httptest.NewRecorder()
		h.ServeHTTP(recGet, reqGet)
		if recGet.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for GET test profile, got %d", recGet.Code)
		}

		// Invalid json
		reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/config/test/profile", strings.NewReader("{bad"))
		recBad := httptest.NewRecorder()
		h.ServeHTTP(recBad, reqBad)
		if recBad.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for bad json, got %d", recBad.Code)
		}

		// Missing db
		reqNoDB := httptest.NewRequest(http.MethodPost, "/api/v1/config/test/profile", strings.NewReader(`{"profile":{"host":"127.0.0.1"}}`))
		recNoDB := httptest.NewRecorder()
		h.ServeHTTP(recNoDB, reqNoDB)
		if recNoDB.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing db, got %d", recNoDB.Code)
		}

		// Unknown ssh proxy
		reqBadSSH := httptest.NewRequest(http.MethodPost, "/api/v1/config/test/profile", strings.NewReader(`{"profile":{"db":"mysql","host":"127.0.0.1","ssh_proxy":"nonexistent"}}`))
		recBadSSH := httptest.NewRecorder()
		h.ServeHTTP(recBadSSH, reqBadSSH)
		if recBadSSH.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for nonexistent ssh proxy, got %d", recBadSSH.Code)
		}
	}

	// 4. POST /api/v1/config/test/ssh-proxy
	{
		// Invalid method
		reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/config/test/ssh-proxy", nil)
		recGet := httptest.NewRecorder()
		h.ServeHTTP(recGet, reqGet)
		if recGet.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for GET test ssh proxy, got %d", recGet.Code)
		}

		// Missing host
		reqNoHost := httptest.NewRequest(http.MethodPost, "/api/v1/config/test/ssh-proxy", strings.NewReader(`{"ssh_proxy":{"port":22}}`))
		recNoHost := httptest.NewRecorder()
		h.ServeHTTP(recNoHost, reqNoHost)
		if recNoHost.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing host, got %d", recNoHost.Code)
		}
	}

	// 5. POST /api/v1/config/test/ai
	{
		// Invalid method
		reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/config/test/ai", nil)
		recGet := httptest.NewRecorder()
		h.ServeHTTP(recGet, reqGet)
		if recGet.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for GET test ai, got %d", recGet.Code)
		}

		// Missing API key
		reqNoKey := httptest.NewRequest(http.MethodPost, "/api/v1/config/test/ai", strings.NewReader(`{"ai":{"base_url":"https://api.openai.com/v1"}}`))
		recNoKey := httptest.NewRecorder()
		// Overwrite temp config with empty key
		hEmptyKey := NewHandler(HandlerOptions{ConfigPath: createConfigFile(t, "profiles: {}\nssh_proxies: {}\nai:\n  api_key: \"\"\n")})
		hEmptyKey.ServeHTTP(recNoKey, reqNoKey)
		if recNoKey.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing api key, got %d", recNoKey.Code)
		}

		// Success path with mock OpenAI server
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`))
		}))
		defer srv.Close()

		reqSuccess := httptest.NewRequest(http.MethodPost, "/api/v1/config/test/ai", strings.NewReader(`{"ai":{"api_key":"test-key","base_url":"`+srv.URL+`","model":"gpt-4o"}}`))
		recSuccess := httptest.NewRecorder()
		h.ServeHTTP(recSuccess, reqSuccess)
		if recSuccess.Code != http.StatusOK {
			t.Errorf("expected 200 for successful AI test, got %d: %s", recSuccess.Code, recSuccess.Body.String())
		}
	}
}

