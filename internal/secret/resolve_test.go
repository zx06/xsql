package secret

import (
	"fmt"
	"testing"

	"github.com/zx06/xsql/internal/errors"
)

// mockKeyring 模拟 keyring 实现，用于单元测试
type mockKeyring struct {
	data map[string]map[string]string // service -> account -> value
}

func newMockKeyring() *mockKeyring {
	return &mockKeyring{data: make(map[string]map[string]string)}
}

func (m *mockKeyring) set(service, account, value string) {
	if m.data[service] == nil {
		m.data[service] = make(map[string]string)
	}
	m.data[service][account] = value
}

func (m *mockKeyring) Get(service, account string) (string, error) {
	if svc, ok := m.data[service]; ok {
		if v, ok := svc[account]; ok {
			return v, nil
		}
	}
	return "", fmt.Errorf("not found: %s/%s", service, account)
}

func (m *mockKeyring) Set(service, account, value string) error {
	m.set(service, account, value)
	return nil
}

func (m *mockKeyring) Delete(service, account string) error {
	if svc, ok := m.data[service]; ok {
		delete(svc, account)
	}
	return nil
}

// =============================================================================
// parseKeyringRef 单元测试
// =============================================================================

func TestParseKeyringRef(t *testing.T) {
	tests := []struct {
		name        string
		ref         string
		wantService string
		wantAccount string
		wantErr     bool
	}{
		{
			name:        "simple account",
			ref:         "password",
			wantService: "xsql",
			wantAccount: "password",
		},
		{
			name:        "account with path",
			ref:         "prod/db_password",
			wantService: "xsql",
			wantAccount: "prod/db_password",
		},
		{
			name:        "deeply nested account",
			ref:         "profiles/prod/mysql/readonly",
			wantService: "xsql",
			wantAccount: "profiles/prod/mysql/readonly",
		},
		{
			name:        "account with hyphen",
			ref:         "prod/my-password",
			wantService: "xsql",
			wantAccount: "prod/my-password",
		},
		{
			name:        "account with underscore",
			ref:         "prod/my_password",
			wantService: "xsql",
			wantAccount: "prod/my_password",
		},
		{
			name:    "empty ref",
			ref:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, account, err := parseKeyringRef(tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseKeyringRef(%q) expected error, got nil", tt.ref)
				}
				return
			}
			if err != nil {
				t.Errorf("parseKeyringRef(%q) unexpected error: %v", tt.ref, err)
				return
			}
			if service != tt.wantService {
				t.Errorf("parseKeyringRef(%q) service = %q, want %q", tt.ref, service, tt.wantService)
			}
			if account != tt.wantAccount {
				t.Errorf("parseKeyringRef(%q) account = %q, want %q", tt.ref, account, tt.wantAccount)
			}
		})
	}
}

// =============================================================================
// KeyringAPI 接口测试
// =============================================================================

func TestMockKeyring_GetSetDelete(t *testing.T) {
	kr := newMockKeyring()

	// Set
	if err := kr.Set("svc", "acc", "val"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get
	val, err := kr.Get("svc", "acc")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "val" {
		t.Errorf("Get = %q, want %q", val, "val")
	}

	// Get not found
	_, err = kr.Get("svc", "notexist")
	if err == nil {
		t.Error("Get should fail for non-existent account")
	}

	// Delete
	if err := kr.Delete("svc", "acc"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get after delete
	_, err = kr.Get("svc", "acc")
	if err == nil {
		t.Error("Get should fail after Delete")
	}
}

func TestMockKeyring_MultipleServices(t *testing.T) {
	kr := newMockKeyring()
	kr.set("svc1", "acc1", "val1")
	kr.set("svc1", "acc2", "val2")
	kr.set("svc2", "acc1", "val3")

	tests := []struct {
		service string
		account string
		want    string
	}{
		{"svc1", "acc1", "val1"},
		{"svc1", "acc2", "val2"},
		{"svc2", "acc1", "val3"},
	}

	for _, tt := range tests {
		val, err := kr.Get(tt.service, tt.account)
		if err != nil {
			t.Errorf("Get(%q, %q) failed: %v", tt.service, tt.account, err)
			continue
		}
		if val != tt.want {
			t.Errorf("Get(%q, %q) = %q, want %q", tt.service, tt.account, val, tt.want)
		}
	}
}

func TestResolve_KeyringRef(t *testing.T) {
	kr := newMockKeyring()
	kr.set("xsql", "prod/db_password", "secret123")

	val, xe := Resolve("keyring:prod/db_password", Options{Keyring: kr})
	if xe != nil {
		t.Fatalf("unexpected err: %v", xe)
	}
	if val != "secret123" {
		t.Fatalf("val=%q, want %q", val, "secret123")
	}
}

func TestResolve_KeyringRef_NestedAccount(t *testing.T) {
	kr := newMockKeyring()
	kr.set("xsql", "prod/mysql/readonly", "nestedpass")

	val, xe := Resolve("keyring:prod/mysql/readonly", Options{Keyring: kr})
	if xe != nil {
		t.Fatalf("unexpected err: %v", xe)
	}
	if val != "nestedpass" {
		t.Fatalf("val=%q, want %q", val, "nestedpass")
	}
}

func TestResolve_KeyringNotFound(t *testing.T) {
	kr := newMockKeyring()
	_, xe := Resolve("keyring:no_such", Options{Keyring: kr})
	if xe == nil || xe.Code != errors.CodeSecretNotFound {
		t.Fatalf("expected XSQL_SECRET_NOT_FOUND, got %v", xe)
	}
}

func TestResolve_KeyringInvalidFormat(t *testing.T) {
	kr := newMockKeyring()

	tests := []string{
		"keyring:", // 空引用
	}
	for _, raw := range tests {
		_, xe := Resolve(raw, Options{Keyring: kr})
		if xe == nil || xe.Code != errors.CodeCfgInvalid {
			t.Errorf("Resolve(%q): expected XSQL_CFG_INVALID, got %v", raw, xe)
		}
	}
}

func TestResolve_PlaintextAllowed(t *testing.T) {
	val, xe := Resolve("plaintext_password", Options{AllowPlaintext: true})
	if xe != nil {
		t.Fatalf("unexpected err: %v", xe)
	}
	if val != "plaintext_password" {
		t.Fatalf("val=%q", val)
	}
}

func TestResolve_PlaintextDenied(t *testing.T) {
	_, xe := Resolve("plaintext_password", Options{AllowPlaintext: false})
	if xe == nil || xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected XSQL_CFG_INVALID")
	}
}

func TestIsKeyringRef(t *testing.T) {
	if !IsKeyringRef("keyring:foo") {
		t.Fatal("expected true")
	}
	if IsKeyringRef("plaintext") {
		t.Fatal("expected false")
	}
}
