package secret

import (
	"fmt"
	"testing"

	"github.com/zx06/xsql/internal/errors"
)

type mockKeyring struct {
	data map[string]string
}

func (m *mockKeyring) Get(key string) (string, error) {
	v, ok := m.data[key]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return v, nil
}
func (m *mockKeyring) Set(key, value string) error { m.data[key] = value; return nil }
func (m *mockKeyring) Delete(key string) error     { delete(m.data, key); return nil }

func TestResolve_KeyringRef(t *testing.T) {
	kr := &mockKeyring{data: map[string]string{"profiles/dev/db_password": "secret123"}}
	val, xe := Resolve("keyring:profiles/dev/db_password", Options{Keyring: kr})
	if xe != nil {
		t.Fatalf("unexpected err: %v", xe)
	}
	if val != "secret123" {
		t.Fatalf("val=%q", val)
	}
}

func TestResolve_KeyringNotFound(t *testing.T) {
	kr := &mockKeyring{data: map[string]string{}}
	_, xe := Resolve("keyring:no_such", Options{Keyring: kr})
	if xe == nil || xe.Code != errors.CodeSecretNotFound {
		t.Fatalf("expected XSQL_SECRET_NOT_FOUND")
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
