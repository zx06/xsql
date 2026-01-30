package integration

import (
	"fmt"
	"testing"

	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/secret"
)

// mockKeyring 模拟 keyring，用于集成测试（不依赖真实 OS keyring）
type mockKeyring struct {
	data map[string]map[string]string
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

func TestSecretResolve_Integration(t *testing.T) {
	kr := newMockKeyring()
	kr.set("xsql", "dev/mysql_password", "dev_secret")
	kr.set("xsql", "prod/mysql_password", "prod_secret")
	kr.set("xsql", "ssh/passphrase", "ssh_pass")

	tests := []struct {
		name           string
		raw            string
		allowPlaintext bool
		wantValue      string
		wantErrCode    errors.Code
	}{
		{
			name:      "keyring ref with nested account",
			raw:       "keyring:dev/mysql_password",
			wantValue: "dev_secret",
		},
		{
			name:      "keyring ref prod password",
			raw:       "keyring:prod/mysql_password",
			wantValue: "prod_secret",
		},
		{
			name:      "keyring ref ssh passphrase",
			raw:       "keyring:ssh/passphrase",
			wantValue: "ssh_pass",
		},
		{
			name:        "keyring not found",
			raw:         "keyring:nonexistent",
			wantErrCode: errors.CodeSecretNotFound,
		},
		{
			name:        "invalid format - empty account",
			raw:         "keyring:",
			wantErrCode: errors.CodeCfgInvalid,
		},
		{
			name:           "plaintext allowed",
			raw:            "plain_password",
			allowPlaintext: true,
			wantValue:      "plain_password",
		},
		{
			name:           "plaintext denied",
			raw:            "plain_password",
			allowPlaintext: false,
			wantErrCode:    errors.CodeCfgInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, xe := secret.Resolve(tt.raw, secret.Options{
				Keyring:        kr,
				AllowPlaintext: tt.allowPlaintext,
			})

			if tt.wantErrCode != errors.Code("") {
				if xe == nil {
					t.Fatalf("expected error code %s, got nil", tt.wantErrCode)
				}
				if xe.Code != tt.wantErrCode {
					t.Fatalf("expected error code %s, got %s", tt.wantErrCode, xe.Code)
				}
				return
			}

			if xe != nil {
				t.Fatalf("unexpected error: %v", xe)
			}
			if val != tt.wantValue {
				t.Fatalf("got value %q, want %q", val, tt.wantValue)
			}
		})
	}
}

func TestSecretResolve_MultipleProfiles(t *testing.T) {
	kr := newMockKeyring()
	kr.set("xsql", "profiles/dev/mysql", "dev_pass")
	kr.set("xsql", "profiles/staging/mysql", "staging_pass")
	kr.set("xsql", "profiles/prod/mysql", "prod_pass")

	profiles := []struct {
		ref  string
		want string
	}{
		{"keyring:profiles/dev/mysql", "dev_pass"},
		{"keyring:profiles/staging/mysql", "staging_pass"},
		{"keyring:profiles/prod/mysql", "prod_pass"},
	}

	for _, p := range profiles {
		val, xe := secret.Resolve(p.ref, secret.Options{Keyring: kr})
		if xe != nil {
			t.Errorf("Resolve(%q): unexpected error %v", p.ref, xe)
			continue
		}
		if val != p.want {
			t.Errorf("Resolve(%q) = %q, want %q", p.ref, val, p.want)
		}
	}
}

// =============================================================================
// SSH Passphrase 集成测试
// =============================================================================

func TestSecretResolve_SSHPassphrase(t *testing.T) {
	kr := newMockKeyring()
	kr.set("xsql", "ssh/bastion/passphrase", "ssh_secret")
	kr.set("xsql", "ssh/jump/passphrase", "jump_secret")

	tests := []struct {
		ref  string
		want string
	}{
		{"keyring:ssh/bastion/passphrase", "ssh_secret"},
		{"keyring:ssh/jump/passphrase", "jump_secret"},
	}

	for _, tt := range tests {
		val, xe := secret.Resolve(tt.ref, secret.Options{Keyring: kr})
		if xe != nil {
			t.Errorf("Resolve(%q): unexpected error %v", tt.ref, xe)
			continue
		}
		if val != tt.want {
			t.Errorf("Resolve(%q) = %q, want %q", tt.ref, val, tt.want)
		}
	}
}

// =============================================================================
// 边界情况集成测试
// =============================================================================

func TestSecretResolve_EdgeCases(t *testing.T) {
	kr := newMockKeyring()

	// 特殊字符密码
	kr.set("xsql", "special", "p@ss!#$%^&*()")
	// Unicode 密码
	kr.set("xsql", "unicode", "密码пароль")
	// 空格密码
	kr.set("xsql", "spaces", "pass word with spaces")
	// 很长的 account 路径
	kr.set("xsql", "a/b/c/d/e/f/g/h", "deep_nested")

	tests := []struct {
		name    string
		ref     string
		want    string
		wantErr bool
	}{
		{"special chars", "keyring:special", "p@ss!#$%^&*()", false},
		{"unicode", "keyring:unicode", "密码пароль", false},
		{"spaces", "keyring:spaces", "pass word with spaces", false},
		{"deep nested", "keyring:a/b/c/d/e/f/g/h", "deep_nested", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, xe := secret.Resolve(tt.ref, secret.Options{Keyring: kr})
			if tt.wantErr {
				if xe == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if xe != nil {
				t.Fatalf("unexpected error: %v", xe)
			}
			if val != tt.want {
				t.Errorf("got %q, want %q", val, tt.want)
			}
		})
	}
}

// =============================================================================
// Keyring 操作集成测试
// =============================================================================

func TestKeyring_SetGetDelete(t *testing.T) {
	kr := newMockKeyring()

	// Set
	if err := kr.Set("test-service", "test-account", "test-value"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get
	val, err := kr.Get("test-service", "test-account")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "test-value" {
		t.Errorf("Get = %q, want %q", val, "test-value")
	}

	// Delete
	if err := kr.Delete("test-service", "test-account"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get after delete should fail
	_, err = kr.Get("test-service", "test-account")
	if err == nil {
		t.Error("Get after Delete should fail")
	}
}

func TestKeyring_MultipleAccounts(t *testing.T) {
	kr := newMockKeyring()

	// 所有账户都存储在固定的 xsql service 下
	accounts := []struct {
		account string
		value   string
	}{
		{"prod/mysql", "mysql_pass"},
		{"prod/pg", "pg_pass"},
		{"dev/api_key", "api_secret"},
		{"staging/token", "token_value"},
	}

	// Set all
	for _, a := range accounts {
		kr.set("xsql", a.account, a.value)
	}

	// Verify all via Resolve
	for _, a := range accounts {
		ref := fmt.Sprintf("keyring:%s", a.account)
		val, xe := secret.Resolve(ref, secret.Options{Keyring: kr})
		if xe != nil {
			t.Errorf("Resolve(%q) failed: %v", ref, xe)
			continue
		}
		if val != a.value {
			t.Errorf("Resolve(%q) = %q, want %q", ref, val, a.value)
		}
	}
}

// =============================================================================
// 配置文件场景集成测试
// =============================================================================

func TestSecretResolve_ConfigScenarios(t *testing.T) {
	kr := newMockKeyring()

	// 模拟配置文件中的各种场景
	kr.set("xsql", "dev/mysql_password", "dev_mysql_123")
	kr.set("xsql", "prod/mysql_password", "prod_mysql_456")
	kr.set("xsql", "ssh/passphrase", "ssh_key_pass")

	scenarios := []struct {
		name           string
		configPassword string
		allowPlaintext bool
		wantValue      string
		wantErr        bool
	}{
		{
			name:           "dev mysql from keyring",
			configPassword: "keyring:dev/mysql_password",
			wantValue:      "dev_mysql_123",
		},
		{
			name:           "prod mysql from keyring",
			configPassword: "keyring:prod/mysql_password",
			wantValue:      "prod_mysql_456",
		},
		{
			name:           "ssh passphrase from keyring",
			configPassword: "keyring:ssh/passphrase",
			wantValue:      "ssh_key_pass",
		},
		{
			name:           "plaintext allowed",
			configPassword: "plain_dev_pass",
			allowPlaintext: true,
			wantValue:      "plain_dev_pass",
		},
		{
			name:           "plaintext denied",
			configPassword: "plain_prod_pass",
			allowPlaintext: false,
			wantErr:        true,
		},
		{
			name:           "keyring not found",
			configPassword: "keyring:nonexistent",
			wantErr:        true,
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			val, xe := secret.Resolve(sc.configPassword, secret.Options{
				Keyring:        kr,
				AllowPlaintext: sc.allowPlaintext,
			})

			if sc.wantErr {
				if xe == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if xe != nil {
				t.Fatalf("unexpected error: %v", xe)
			}
			if val != sc.wantValue {
				t.Errorf("got %q, want %q", val, sc.wantValue)
			}
		})
	}
}
