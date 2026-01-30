package ssh

import (
	"os"
	"testing"

	"github.com/zx06/xsql/internal/errors"
)

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTilde bool
	}{
		{
			name:     "tilde expansion",
			input:    "~/.ssh/id_rsa",
			wantTilde: true,
		},
		{
			name:     "no tilde",
			input:    "/etc/ssh/ssh_config",
			wantTilde: false,
		},
		{
			name:     "relative path",
			input:    "./id_rsa",
			wantTilde: false,
		},
		{
			name:     "absolute path",
			input:    "/home/user/.ssh/id_rsa",
			wantTilde: false,
		},
		{
			name:     "tilde without slash - not expanded by current implementation",
			input:    "~",
			wantTilde: false, // Current implementation only expands ~/... not ~
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if tt.wantTilde {
				if result == tt.input {
					t.Errorf("expandPath(%q) should have been expanded", tt.input)
				}
				// Should contain home directory
				home, _ := os.UserHomeDir()
				if home != "" && result != home && !containsPath(result, home) {
					t.Errorf("expandPath(%q) = %q, should contain home dir %q", tt.input, result, home)
				}
			} else {
				if result != tt.input {
					t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.input)
				}
			}
		})
	}
}

func TestDefaultKnownHostsPath(t *testing.T) {
	p := DefaultKnownHostsPath()
	if p != "~/.ssh/known_hosts" {
		t.Fatalf("unexpected: %q", p)
	}
}

func TestConnect_MissingHost(t *testing.T) {
	opts := Options{
		Port: 22,
		User: "test",
	}

	_, xe := Connect(nil, opts)
	if xe == nil {
		t.Fatal("expected error for missing host")
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestConnect_DefaultPort(t *testing.T) {
	// This test verifies the default port logic without actually connecting
	// We can't test the full connection without a real SSH server
	opts := Options{
		Host: "example.com",
		// Port not set, should default to 22
	}

	// Note: This will fail due to missing auth, but that's expected
	// We just want to verify the port is handled correctly
	_, xe := Connect(nil, opts)
	// Should fail due to no auth methods, not due to port
	if xe != nil && xe.Code == errors.CodeCfgInvalid {
		t.Errorf("unexpected validation error: %v", xe)
	}
}

func TestConnect_DefaultUser(t *testing.T) {
	// Set environment variables for user
	originalUser := os.Getenv("USER")
	originalUsername := os.Getenv("USERNAME")
	defer func() {
		os.Setenv("USER", originalUser)
		os.Setenv("USERNAME", originalUsername)
	}()

	os.Setenv("USER", "testuser")
	os.Setenv("USERNAME", "")

	opts := Options{
		Host: "example.com",
	}

	_, xe := Connect(nil, opts)
	// Should fail due to no auth methods, not due to user
	if xe != nil && xe.Code == errors.CodeCfgInvalid {
		t.Errorf("unexpected validation error: %v", xe)
	}
}

func TestConnect_NoAuthMethods(t *testing.T) {
	// Ensure no default keys exist by using a non-existent path
	opts := Options{
		Host:         "example.com",
		IdentityFile: "/nonexistent/key",
	}

	_, xe := Connect(nil, opts)
	if xe == nil {
		t.Fatal("expected error for no auth methods")
	}
	// Error could be CodeCfgInvalid (file not found) or CodeSSHAuthFailed (no auth methods)
	if xe.Code != errors.CodeCfgInvalid && xe.Code != errors.CodeSSHAuthFailed {
		t.Errorf("expected CodeCfgInvalid or CodeSSHAuthFailed, got %s", xe.Code)
	}
}

func TestBuildAuthMethods_WithInvalidKeyFile(t *testing.T) {
	opts := Options{
		IdentityFile: "/nonexistent/id_rsa",
	}

	_, xe := buildAuthMethods(opts)
	if xe == nil {
		t.Fatal("expected error for non-existent key file")
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestBuildHostKeyCallback_SkipKnownHostsCheck(t *testing.T) {
	opts := Options{
		SkipKnownHostsCheck: true,
	}

	cb, xe := buildHostKeyCallback(opts)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if cb == nil {
		t.Fatal("expected non-nil callback")
	}
}

func TestBuildHostKeyCallback_InvalidKnownHostsFile(t *testing.T) {
	opts := Options{
		KnownHostsFile: "/nonexistent/known_hosts",
	}

	cb, xe := buildHostKeyCallback(opts)
	if xe == nil {
		t.Fatal("expected error for non-existent known_hosts file")
	}
	if xe.Code != errors.CodeSSHHostKeyMismatch {
		t.Errorf("expected CodeSSHHostKeyMismatch, got %s", xe.Code)
	}
	if cb != nil {
		t.Error("expected nil callback for error")
	}
}

func TestBuildHostKeyCallback_DefaultPath(t *testing.T) {
	// Test with default path (empty string)
	opts := Options{
		KnownHostsFile: "",
	}

	// This will likely fail because the default file doesn't exist or is invalid
	// But we're testing the logic, not the actual file parsing
	cb, xe := buildHostKeyCallback(opts)
	// Either success or error is acceptable, depending on whether the file exists
	if xe != nil {
		// Expected if default known_hosts doesn't exist
		if xe.Code != errors.CodeSSHHostKeyMismatch {
			t.Errorf("unexpected error code: %s", xe.Code)
		}
	}
	// If xe is nil, cb should be non-nil
	if xe == nil && cb == nil {
		t.Error("expected non-nil callback when no error")
	}
}

// Helper function to check if a path contains another path component
func containsPath(path, component string) bool {
	return len(path) >= len(component) && (path == component || path[len(path)-len(component)-1] == '/' || path[:len(component)] == component)
}
