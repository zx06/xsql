package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
)

func TestStreamableHTTPAuthRequired(t *testing.T) {
	server, err := CreateServer("test", &config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
	})
	if err != nil {
		t.Fatalf("CreateServer error: %v", err)
	}
	handler, err := NewStreamableHTTPHandler(server, "secret-token")
	if err != nil {
		t.Fatalf("NewStreamableHTTPHandler error: %v", err)
	}

	ts := httptest.NewServer(handler)
	defer ts.Close()

	cases := []struct {
		name             string
		authHeader       string
		wantUnauthorized bool
	}{
		{name: "missing", authHeader: "", wantUnauthorized: true},
		{name: "wrong-scheme", authHeader: "Token secret-token", wantUnauthorized: true},
		{name: "wrong-token", authHeader: "Bearer bad-token", wantUnauthorized: true},
		{name: "ok", authHeader: "Bearer secret-token", wantUnauthorized: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader("{}"))
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			req.Header.Set("Accept", "application/json, text/event-stream")
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("http request error: %v", err)
			}
			resp.Body.Close()
			if tc.wantUnauthorized && resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected unauthorized, got %d", resp.StatusCode)
			}
			if !tc.wantUnauthorized && resp.StatusCode == http.StatusUnauthorized {
				t.Fatalf("expected non-unauthorized status, got %d", resp.StatusCode)
			}
		})
	}
}

func TestNewStreamableHTTPHandler_Validation(t *testing.T) {
	_, err := NewStreamableHTTPHandler(nil, "token")
	if err == nil {
		t.Fatal("expected error for nil server")
	}
	xe, ok := err.(*errors.XError)
	if !ok {
		t.Fatalf("expected XError, got %T", err)
	}
	if xe.Code != errors.CodeInternal {
		t.Fatalf("expected CodeInternal, got %s", xe.Code)
	}

	server, err := CreateServer("test", &config.File{
		Profiles:   map[string]config.Profile{},
		SSHProxies: map[string]config.SSHProxy{},
	})
	if err != nil {
		t.Fatalf("CreateServer error: %v", err)
	}
	_, err = NewStreamableHTTPHandler(server, "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	xe, ok = err.(*errors.XError)
	if !ok {
		t.Fatalf("expected XError, got %T", err)
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}
