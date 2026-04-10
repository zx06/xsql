//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestServe_JSONHealthCheck(t *testing.T) {
	cmd := exec.Command(testBinary, "serve", "--addr", "127.0.0.1:0", "--format", "json")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start serve: %v", err)
	}

	reader := bufio.NewReader(stdout)
	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			errCh <- readErr
			return
		}
		lineCh <- line
	}()

	var line string
	select {
	case line = <-lineCh:
	case err := <-errCh:
		t.Fatalf("failed to read startup output: %v stderr=%s", err, stderr.String())
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for startup output; stderr=%s", stderr.String())
	}

	var resp Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("invalid JSON startup output: %v line=%s", err, line)
	}
	if !resp.OK {
		t.Fatalf("startup failed: %+v", resp.Error)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data payload: %#v", resp.Data)
	}
	baseURL, ok := data["url"].(string)
	if !ok || baseURL == "" {
		t.Fatalf("missing url in startup payload: %#v", data)
	}

	httpResp, err := http.Get(strings.TrimRight(baseURL, "/") + "/api/v1/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d", httpResp.StatusCode)
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("interrupt serve: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	select {
	case err := <-waitCh:
		if err != nil {
			t.Fatalf("serve exited with error: %v stderr=%s", err, stderr.String())
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timed out waiting for serve shutdown")
	}
}

func TestServe_RemoteRequiresToken(t *testing.T) {
	stdout, _, exitCode := runXSQL(t, "serve", "--addr", "0.0.0.0:8788", "--format", "json")

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v output=%s", err, stdout)
	}
	if resp.OK {
		t.Fatal("expected ok=false")
	}
	if resp.Error == nil || resp.Error.Code != "XSQL_CFG_INVALID" {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}
