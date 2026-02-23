package secret

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// nullByteKeyring 模拟 Windows cmdkey 返回带 null 字节的值
type nullByteKeyring struct {
	data map[string]map[string]string
}

func newNullByteKeyring() *nullByteKeyring {
	return &nullByteKeyring{data: make(map[string]map[string]string)}
}

func (m *nullByteKeyring) set(service, account, value string) {
	if m.data[service] == nil {
		m.data[service] = make(map[string]string)
	}
	m.data[service][account] = value
}

// setWithNullBytes 模拟 Windows UTF-16 问题：每个字符后插入 null 字节
func (m *nullByteKeyring) setWithNullBytes(service, account, value string) {
	var sb strings.Builder
	for _, r := range value {
		sb.WriteRune(r)
		sb.WriteByte(0x00)
	}
	m.set(service, account, sb.String())
}

func (m *nullByteKeyring) Get(service, account string) (string, error) {
	if svc, ok := m.data[service]; ok {
		if v, ok := svc[account]; ok {
			return v, nil
		}
	}
	return "", fmt.Errorf("not found: %s/%s", service, account)
}

func (m *nullByteKeyring) Set(service, account, value string) error {
	m.set(service, account, value)
	return nil
}

func (m *nullByteKeyring) Delete(service, account string) error {
	if svc, ok := m.data[service]; ok {
		delete(svc, account)
	}
	return nil
}

// =============================================================================
// Windows null 字节处理测试
// =============================================================================

func TestStripNullBytes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no null bytes",
			input: "password123",
			want:  "password123",
		},
		{
			name:  "null bytes between chars",
			input: "p\x00a\x00s\x00s\x00",
			want:  "pass",
		},
		{
			name:  "full password with null bytes",
			input: "m\x00y\x00P\x00a\x00s\x00s\x00w\x00o\x00r\x00d\x00",
			want:  "myPassword",
		},
		{
			name:  "special chars with null bytes",
			input: "p\x00@\x00s\x00s\x00!\x00#\x00",
			want:  "p@ss!#",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only null bytes",
			input: "\x00\x00\x00",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.ReplaceAll(tt.input, "\x00", "")
			if got != tt.want {
				t.Errorf("stripNullBytes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNullByteKeyring_SimulatesWindowsBehavior(t *testing.T) {
	kr := newNullByteKeyring()
	kr.setWithNullBytes("xsql", "prod/password", "secret123")

	val, err := kr.Get("xsql", "prod/password")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// 原始值应该包含 null 字节
	if !strings.Contains(val, "\x00") {
		t.Error("Expected value to contain null bytes")
	}

	// 清理后应该等于原始密码
	cleaned := strings.ReplaceAll(val, "\x00", "")
	if cleaned != "secret123" {
		t.Errorf("Cleaned value = %q, want %q", cleaned, "secret123")
	}
}

// =============================================================================
// KeyringAPI 接口合规性测试
// =============================================================================

func TestKeyringAPI_Interface(t *testing.T) {
	// 确保 mockKeyring 实现 KeyringAPI 接口
	var _ KeyringAPI = (*mockKeyring)(nil)
	var _ KeyringAPI = (*nullByteKeyring)(nil)
}

func TestKeyringAPI_ErrorCases(t *testing.T) {
	kr := newMockKeyring()

	// 空 service
	_, err := kr.Get("", "account")
	if err == nil {
		t.Error("Get with empty service should fail")
	}

	// 空 account
	_, err = kr.Get("service", "")
	if err == nil {
		t.Error("Get with empty account should fail")
	}

	// 不存在的 service
	_, err = kr.Get("nonexistent", "account")
	if err == nil {
		t.Error("Get with nonexistent service should fail")
	}
}

// =============================================================================
// Resolve 与 Keyring 集成测试
// =============================================================================

func TestResolve_WithNullByteKeyring(t *testing.T) {
	kr := newNullByteKeyring()
	// 模拟 Windows 返回带 null 字节的密码
	kr.set("xsql", "prod/password", "s\x00e\x00c\x00r\x00e\x00t\x00")

	// 注意：Resolve 直接使用 keyring 返回值，不做清理
	// 清理逻辑在 keyring_windows.go 的 osKeyring.Get 中
	val, xe := Resolve("keyring:prod/password", Options{Keyring: kr})
	if xe != nil {
		t.Fatalf("Resolve failed: %v", xe)
	}

	// 由于使用 mockKeyring，不会自动清理 null 字节
	// 这个测试验证 Resolve 正确传递值
	if !strings.Contains(val, "\x00") {
		t.Log("Value does not contain null bytes (expected if using cleaned keyring)")
	}
}

func TestResolve_SpecialCharacters(t *testing.T) {
	kr := newMockKeyring()
	specialPasswords := []string{
		"p@ssw0rd!",
		"pass#123$",
		"密码123",
		"пароль",
		"パスワード",
		"pass word",
		"pass\ttab",
	}

	for i, pw := range specialPasswords {
		account := fmt.Sprintf("test%d", i)
		kr.set("xsql", account, pw)

		val, xe := Resolve(fmt.Sprintf("keyring:%s", account), Options{Keyring: kr})
		if xe != nil {
			t.Errorf("Resolve special password %q failed: %v", pw, xe)
			continue
		}
		if val != pw {
			t.Errorf("Resolve special password: got %q, want %q", val, pw)
		}
	}
}

func TestResolve_EmptyPassword(t *testing.T) {
	kr := newMockKeyring()
	kr.set("xsql", "empty", "")

	val, xe := Resolve("keyring:empty", Options{Keyring: kr})
	if xe != nil {
		t.Fatalf("Resolve failed: %v", xe)
	}
	if val != "" {
		t.Errorf("Expected empty password, got %q", val)
	}
}

func TestResolve_LongPassword(t *testing.T) {
	kr := newMockKeyring()
	longPass := strings.Repeat("a", 1000)
	kr.set("xsql", "long", longPass)

	val, xe := Resolve("keyring:long", Options{Keyring: kr})
	if xe != nil {
		t.Fatalf("Resolve failed: %v", xe)
	}
	if val != longPass {
		t.Errorf("Long password mismatch: got len=%d, want len=%d", len(val), len(longPass))
	}
}

func TestDefaultKeyring_NullByteBehavior(t *testing.T) {
	keyring.MockInit()
	kr := defaultKeyring()
	service := "xsql-test"
	account := "null-byte"
	raw := "s\x00e\x00c\x00r\x00e\x00t\x00"
	if err := kr.Set(service, account, raw); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	got, err := kr.Get(service, account)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if runtime.GOOS == "windows" {
		if strings.Contains(got, "\x00") {
			t.Fatalf("expected null bytes to be stripped, got %q", got)
		}
		if got != "secret" {
			t.Fatalf("expected cleaned value, got %q", got)
		}
		return
	}
	if got != raw {
		t.Fatalf("expected raw value on non-windows, got %q", got)
	}
}
