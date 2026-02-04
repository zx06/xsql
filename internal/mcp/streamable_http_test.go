package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zx06/xsql/internal/config"
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

	req, err := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http request error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.StatusCode)
	}

	reqAuth, err := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	reqAuth.Header.Set("Accept", "application/json, text/event-stream")
	reqAuth.Header.Set("Authorization", "Bearer secret-token")
	respAuth, err := http.DefaultClient.Do(reqAuth)
	if err != nil {
		t.Fatalf("http request error: %v", err)
	}
	respAuth.Body.Close()
	if respAuth.StatusCode == http.StatusUnauthorized {
		t.Fatalf("expected authorized request to not be unauthorized")
	}
}
