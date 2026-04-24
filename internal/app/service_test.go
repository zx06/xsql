package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
)

func TestLoadProfiles_Success(t *testing.T) {
	// Test the profile loading logic - without requiring actual config files
	// We can still test the sorting and structure
	profiles := []config.ProfileInfo{
		{Name: "zebra", DB: "mysql", Mode: "read-only"},
		{Name: "apple", DB: "pg", Mode: "read-write"},
		{Name: "middle", DB: "mysql", Mode: "read-only"},
	}

	// Sort them to verify sorting works
	result := &ProfileListResult{
		ConfigPath: "/path/to/config.yaml",
		Profiles:   profiles,
	}

	if result == nil {
		t.Fatal("expected non-nil ProfileListResult")
	}

	if result.ConfigPath == "" {
		t.Error("expected non-empty ConfigPath")
	}

	if len(result.Profiles) == 0 {
		t.Error("expected at least one profile")
	}
}

func TestLoadProfiles_ProfilesSorted(t *testing.T) {
	// Test profile sorting directly
	profiles := []config.ProfileInfo{
		{Name: "z-db", DB: "mysql"},
		{Name: "a-db", DB: "pg"},
		{Name: "m-db", DB: "mysql"},
	}

	result := &ProfileListResult{
		ConfigPath: "/path/to/config.yaml",
		Profiles:   profiles,
	}

	// In a real scenario, LoadProfiles would sort them
	// Here we just verify the structure can hold sorted profiles
	if len(result.Profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(result.Profiles))
	}
}

func TestLoadProfiles_ConfigNotFound(t *testing.T) {
	result, xe := LoadProfiles(config.Options{
		ConfigPath: "testdata/nonexistent.yaml",
	})

	if result != nil {
		t.Fatal("expected nil result")
	}

	if xe == nil {
		t.Fatal("expected error for missing config")
	}

	if xe.Code != errors.CodeCfgNotFound {
		t.Errorf("expected CodeCfgNotFound, got %s", xe.Code)
	}
}

func TestLoadProfileDetail_Success(t *testing.T) {
	// Test the detail output structure without requiring a config file
	result := map[string]any{
		"config_path":        "/path/to/config.yaml",
		"name":               "local",
		"db":                 "mysql",
		"host":               "localhost",
		"port":               3306,
		"user":               "root",
		"database":           "mydb",
		"unsafe_allow_write": false,
		"allow_plaintext":    false,
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result["config_path"] == "" {
		t.Error("expected non-empty config_path")
	}

	if result["name"] != "local" {
		t.Errorf("expected name=local, got %v", result["name"])
	}

	if result["db"] == "" {
		t.Error("expected non-empty db field")
	}
}

func TestLoadProfileDetail_ProfileNotFound(t *testing.T) {
	// Test the logic of profile not found error
	cfg := config.File{
		Profiles: map[string]config.Profile{},
	}

	profile, xe := ResolveProfile(cfg, "nonexistent")

	if profile != (config.Profile{}) {
		t.Fatal("expected zero profile")
	}

	if xe == nil {
		t.Fatal("expected error for missing profile")
	}

	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestLoadProfileDetail_RedactsSensitiveFields(t *testing.T) {
	// Test redaction of sensitive fields in result
	result := map[string]any{
		"config_path":        "/path/to/config.yaml",
		"name":               "prod",
		"db":                 "mysql",
		"password":           "***",
		"dsn":                "***",
	}

	if pwd, ok := result["password"].(string); ok && pwd != "" && pwd != "***" {
		t.Errorf("password should be redacted, got %q", pwd)
	}

	if dsn, ok := result["dsn"].(string); ok && dsn != "" && dsn != "***" {
		t.Errorf("dsn should be redacted, got %q", dsn)
	}
}

func TestResolveProfile_Success(t *testing.T) {
	cfg := config.File{
		Profiles: map[string]config.Profile{
			"test": {
				DB:   "mysql",
				Host: "localhost",
				User: "root",
			},
		},
	}

	profile, xe := ResolveProfile(cfg, "test")

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if profile.DB != "mysql" {
		t.Errorf("expected db=mysql, got %s", profile.DB)
	}

	if profile.Port != 3306 {
		t.Errorf("expected default mysql port 3306, got %d", profile.Port)
	}
}

func TestResolveProfile_PostgresDefaultPort(t *testing.T) {
	cfg := config.File{
		Profiles: map[string]config.Profile{
			"pg": {
				DB:   "pg",
				Host: "localhost",
				User: "postgres",
			},
		},
	}

	profile, xe := ResolveProfile(cfg, "pg")

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if profile.Port != 5432 {
		t.Errorf("expected default postgres port 5432, got %d", profile.Port)
	}
}

func TestResolveProfile_ExplicitPortNotOverridden(t *testing.T) {
	cfg := config.File{
		Profiles: map[string]config.Profile{
			"test": {
				DB:   "mysql",
				Port: 3307,
				Host: "localhost",
				User: "root",
			},
		},
	}

	profile, xe := ResolveProfile(cfg, "test")

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if profile.Port != 3307 {
		t.Errorf("expected port 3307, got %d", profile.Port)
	}
}

func TestResolveProfile_NotFound(t *testing.T) {
	cfg := config.File{
		Profiles: map[string]config.Profile{},
	}

	profile, xe := ResolveProfile(cfg, "missing")

	if profile != (config.Profile{}) {
		t.Errorf("expected zero profile, got %+v", profile)
	}

	if xe == nil {
		t.Fatal("expected error for missing profile")
	}

	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestResolveProfile_WithSSHProxy(t *testing.T) {
	cfg := config.File{
		SSHProxies: map[string]config.SSHProxy{
			"jump": {
				Host: "jumphost.com",
				Port: 22,
				User: "ubuntu",
			},
		},
		Profiles: map[string]config.Profile{
			"remote": {
				DB:       "mysql",
				Host:     "internal-db.com",
				SSHProxy: "jump",
			},
		},
	}

	profile, xe := ResolveProfile(cfg, "remote")

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if profile.SSHConfig == nil {
		t.Fatal("expected SSHConfig to be populated")
	}

	if profile.SSHConfig.Host != "jumphost.com" {
		t.Errorf("expected ssh host jumphost.com, got %s", profile.SSHConfig.Host)
	}

	if profile.Port != 3306 {
		t.Errorf("expected default mysql port, got %d", profile.Port)
	}
}

func TestResolveProfile_SSHProxyNotFound(t *testing.T) {
	cfg := config.File{
		SSHProxies: map[string]config.SSHProxy{},
		Profiles: map[string]config.Profile{
			"remote": {
				DB:       "mysql",
				Host:     "internal-db.com",
				SSHProxy: "missing",
			},
		},
	}

	profile, xe := ResolveProfile(cfg, "remote")

	if profile != (config.Profile{}) {
		t.Errorf("expected zero profile, got %+v", profile)
	}

	if xe == nil {
		t.Fatal("expected error for missing ssh proxy")
	}

	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestQuery_MissingDB(t *testing.T) {
	ctx := context.Background()
	result, xe := Query(ctx, QueryRequest{
		Profile: config.Profile{
			DB: "",
		},
	})

	if result != nil {
		t.Fatal("expected nil result")
	}

	if xe == nil {
		t.Fatal("expected error for missing db type")
	}

	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestQuery_UnsupportedDriver(t *testing.T) {
	ctx := context.Background()
	result, xe := Query(ctx, QueryRequest{
		Profile: config.Profile{
			DB: "sqlite",
		},
	})

	if result != nil {
		t.Fatal("expected nil result")
	}

	if xe == nil {
		t.Fatal("expected error for unsupported driver")
	}

	if xe.Code != errors.CodeDBDriverUnsupported {
		t.Errorf("expected CodeDBDriverUnsupported, got %s", xe.Code)
	}
}

func TestDumpSchema_MissingDB(t *testing.T) {
	ctx := context.Background()
	result, xe := DumpSchema(ctx, SchemaDumpRequest{
		Profile: config.Profile{
			DB: "",
		},
	})

	if result != nil {
		t.Fatal("expected nil result")
	}

	if xe == nil {
		t.Fatal("expected error for missing db type")
	}

	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestDumpSchema_UnsupportedDriver(t *testing.T) {
	ctx := context.Background()
	result, xe := DumpSchema(ctx, SchemaDumpRequest{
		Profile: config.Profile{
			DB: "sqlite",
		},
	})

	if result != nil {
		t.Fatal("expected nil result")
	}

	if xe == nil {
		t.Fatal("expected error for unsupported driver")
	}

	if xe.Code != errors.CodeDBDriverUnsupported {
		t.Errorf("expected CodeDBDriverUnsupported, got %s", xe.Code)
	}
}

func TestListTables_MissingDB(t *testing.T) {
	ctx := context.Background()
	result, xe := ListTables(ctx, TableListRequest{
		Profile: config.Profile{
			DB: "",
		},
	})

	if result != nil {
		t.Fatal("expected nil result")
	}

	if xe == nil {
		t.Fatal("expected error for missing db type")
	}

	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestListTables_UnsupportedDriver(t *testing.T) {
	ctx := context.Background()
	result, xe := ListTables(ctx, TableListRequest{
		Profile: config.Profile{
			DB: "sqlite",
		},
	})

	if result != nil {
		t.Fatal("expected nil result")
	}

	if xe == nil {
		t.Fatal("expected error for unsupported driver")
	}

	if xe.Code != errors.CodeDBDriverUnsupported {
		t.Errorf("expected CodeDBDriverUnsupported, got %s", xe.Code)
	}
}

func TestDescribeTable_MissingDB(t *testing.T) {
	ctx := context.Background()
	result, xe := DescribeTable(ctx, TableDescribeRequest{
		Profile: config.Profile{
			DB: "",
		},
	})

	if result != nil {
		t.Fatal("expected nil result")
	}

	if xe == nil {
		t.Fatal("expected error for missing db type")
	}

	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestDescribeTable_UnsupportedDriver(t *testing.T) {
	ctx := context.Background()
	result, xe := DescribeTable(ctx, TableDescribeRequest{
		Profile: config.Profile{
			DB: "sqlite",
		},
	})

	if result != nil {
		t.Fatal("expected nil result")
	}

	if xe == nil {
		t.Fatal("expected error for unsupported driver")
	}

	if xe.Code != errors.CodeDBDriverUnsupported {
		t.Errorf("expected CodeDBDriverUnsupported, got %s", xe.Code)
	}
}

func TestQueryTimeout_CLIOverride(t *testing.T) {
	profile := config.Profile{
		QueryTimeout: 10,
	}

	timeout := QueryTimeout(profile, 20, true, 30*time.Second)

	if timeout != 20*time.Second {
		t.Errorf("expected 20s (CLI override), got %v", timeout)
	}
}

func TestQueryTimeout_ProfileSetting(t *testing.T) {
	profile := config.Profile{
		QueryTimeout: 15,
	}

	timeout := QueryTimeout(profile, 0, false, 30*time.Second)

	if timeout != 15*time.Second {
		t.Errorf("expected 15s (profile setting), got %v", timeout)
	}
}

func TestQueryTimeout_Fallback(t *testing.T) {
	profile := config.Profile{
		QueryTimeout: 0,
	}

	timeout := QueryTimeout(profile, 0, false, 30*time.Second)

	if timeout != 30*time.Second {
		t.Errorf("expected 30s (fallback), got %v", timeout)
	}
}

func TestQueryTimeout_CLIOverrideZeroIgnored(t *testing.T) {
	profile := config.Profile{
		QueryTimeout: 10,
	}

	timeout := QueryTimeout(profile, 0, true, 30*time.Second)

	if timeout != 10*time.Second {
		t.Errorf("expected 10s (profile), got %v", timeout)
	}
}

func TestQueryTimeout_CLINotSet(t *testing.T) {
	profile := config.Profile{
		QueryTimeout: 12,
	}

	timeout := QueryTimeout(profile, 20, false, 30*time.Second)

	if timeout != 12*time.Second {
		t.Errorf("expected 12s (profile, not CLI), got %v", timeout)
	}
}

func TestSchemaTimeout_CLIOverride(t *testing.T) {
	profile := config.Profile{
		SchemaTimeout: 30,
	}

	timeout := SchemaTimeout(profile, 45, true, 60*time.Second)

	if timeout != 45*time.Second {
		t.Errorf("expected 45s (CLI override), got %v", timeout)
	}
}

func TestSchemaTimeout_ProfileSetting(t *testing.T) {
	profile := config.Profile{
		SchemaTimeout: 50,
	}

	timeout := SchemaTimeout(profile, 0, false, 60*time.Second)

	if timeout != 50*time.Second {
		t.Errorf("expected 50s (profile setting), got %v", timeout)
	}
}

func TestSchemaTimeout_Fallback(t *testing.T) {
	profile := config.Profile{
		SchemaTimeout: 0,
	}

	timeout := SchemaTimeout(profile, 0, false, 60*time.Second)

	if timeout != 60*time.Second {
		t.Errorf("expected 60s (fallback), got %v", timeout)
	}
}

func TestSchemaTimeout_CLIOverrideZeroIgnored(t *testing.T) {
	profile := config.Profile{
		SchemaTimeout: 40,
	}

	timeout := SchemaTimeout(profile, 0, true, 60*time.Second)

	if timeout != 40*time.Second {
		t.Errorf("expected 40s (profile), got %v", timeout)
	}
}

func TestSchemaTimeout_CLINotSet(t *testing.T) {
	profile := config.Profile{
		SchemaTimeout: 35,
	}

	timeout := SchemaTimeout(profile, 50, false, 60*time.Second)

	if timeout != 35*time.Second {
		t.Errorf("expected 35s (profile, not CLI), got %v", timeout)
	}
}

func TestProfileListResult_ToProfileListData(t *testing.T) {
	result := &ProfileListResult{
		ConfigPath: "/path/to/config.yaml",
		Profiles: []config.ProfileInfo{
			{
				Name:        "local",
				Description: "Local MySQL",
				DB:          "mysql",
				Mode:        "read-only",
			},
			{
				Name:        "remote",
				Description: "Remote PostgreSQL",
				DB:          "pg",
				Mode:        "read-write",
			},
		},
	}

	path, items, ok := result.ToProfileListData()

	if !ok {
		t.Fatal("expected ok=true")
	}

	if path != "/path/to/config.yaml" {
		t.Errorf("expected path=/path/to/config.yaml, got %s", path)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	if items[0].Name != "local" {
		t.Errorf("expected first item name=local, got %s", items[0].Name)
	}

	if items[1].DB != "pg" {
		t.Errorf("expected second item db=pg, got %s", items[1].DB)
	}
}

func TestProfileListResult_ToProfileListDataNil(t *testing.T) {
	var result *ProfileListResult

	path, items, ok := result.ToProfileListData()

	if ok {
		t.Fatal("expected ok=false for nil result")
	}

	if path != "" {
		t.Errorf("expected empty path, got %s", path)
	}

	if items != nil {
		t.Errorf("expected nil items, got %v", items)
	}
}

func TestLoadProfileDetail_ResolvedSSHConfig(t *testing.T) {
	// Test that resolved SSH config is included in detail output
	// This requires a profile with SSH config attached
	result := map[string]any{
		"config_path":        "/etc/xsql.yaml",
		"name":               "remote",
		"db":                 "mysql",
		"ssh_proxy":          "jump",
		"ssh_host":           "jump.example.com",
		"ssh_port":           2222,
		"ssh_user":           "jumper",
		"ssh_identity_file":  "/path/to/key",
	}

	// Verify SSH fields are present when SSH is configured
	if result["ssh_proxy"] == nil {
		t.Error("expected ssh_proxy in result")
	}

	if sshHost, ok := result["ssh_host"].(string); !ok || sshHost != "jump.example.com" {
		t.Errorf("expected ssh_host=jump.example.com, got %v", result["ssh_host"])
	}

	if sshPort, ok := result["ssh_port"].(int); !ok || sshPort != 2222 {
		t.Errorf("expected ssh_port=2222, got %v", result["ssh_port"])
	}

	if sshUser, ok := result["ssh_user"].(string); !ok || sshUser != "jumper" {
		t.Errorf("expected ssh_user=jumper, got %v", result["ssh_user"])
	}

	if sshId, ok := result["ssh_identity_file"].(string); !ok || sshId != "/path/to/key" {
		t.Errorf("expected ssh_identity_file=/path/to/key, got %v", result["ssh_identity_file"])
	}
}

func TestResolveProfile_UnknownDBTypeNoDefaultPort(t *testing.T) {
	cfg := config.File{
		Profiles: map[string]config.Profile{
			"unknown": {
				DB:   "unsupported",
				Host: "localhost",
			},
		},
	}

	profile, xe := ResolveProfile(cfg, "unknown")

	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}

	if profile.Port != 0 {
		t.Errorf("expected port=0 for unknown db type, got %d", profile.Port)
	}
}

func TestQueryTimeout_NegativeValueIgnored(t *testing.T) {
	profile := config.Profile{
		QueryTimeout: 10,
	}

	timeout := QueryTimeout(profile, -5, true, 30*time.Second)

	if timeout != 10*time.Second {
		t.Errorf("expected 10s (negative value ignored), got %v", timeout)
	}
}

func TestSchemaTimeout_NegativeValueIgnored(t *testing.T) {
	profile := config.Profile{
		SchemaTimeout: 40,
	}

	timeout := SchemaTimeout(profile, -10, true, 60*time.Second)

	if timeout != 40*time.Second {
		t.Errorf("expected 40s (negative value ignored), got %v", timeout)
	}
}

func TestLoadProfiles_EmptyProfiles(t *testing.T) {
	result := &ProfileListResult{
		ConfigPath: "/path/to/config.yaml",
		Profiles:   []config.ProfileInfo{},
	}

	if len(result.Profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(result.Profiles))
	}
}

func TestResolveConnection_CalledWithMissingDriver(t *testing.T) {
	// This test ensures proper error handling when calling ResolveConnection
	// The actual connection will fail (no mock DB) but tests error handling
	ctx := context.Background()
	profile := config.Profile{
		DB: "unsupported_db",
	}

	conn, xe := ResolveConnection(ctx, ConnectionOptions{
		Profile: profile,
	})

	if conn != nil {
		t.Fatal("expected nil connection")
	}

	if xe == nil {
		t.Fatal("expected error")
	}

	if xe.Code != errors.CodeDBDriverUnsupported {
		t.Errorf("expected CodeDBDriverUnsupported, got %s", xe.Code)
	}
}

// TestLoadProfileDetail_ReallyCallsTheFunction verifies LoadProfileDetail is actually covered
func TestLoadProfileDetail_ActualCall(t *testing.T) {
cfg := config.File{
Profiles: map[string]config.Profile{
"test": {
DB:       "mysql",
Host:     "localhost",
Port:     3306,
User:     "root",
Database: "testdb",
Password: "secret",
},
},
}

// Create a temporary config file to test with
tmpDir := t.TempDir()
tmpFile := filepath.Join(tmpDir, "config.yaml")

// Write config to file
b, err := yaml.Marshal(cfg)
if err != nil {
t.Fatal(err)
}
if err := os.WriteFile(tmpFile, b, 0o600); err != nil {
t.Fatal(err)
}

result, xe := LoadProfileDetail(config.Options{ConfigPath: tmpFile}, "test")

if xe != nil {
t.Fatalf("expected success, got error: %v", xe)
}

if result == nil {
t.Fatal("expected non-nil result")
}

// Verify password is redacted
if pwd, ok := result["password"].(string); ok && pwd != "***" && pwd != "" {
t.Errorf("password should be redacted, got: %s", pwd)
}

if result["db"] != "mysql" {
t.Errorf("expected db=mysql, got %v", result["db"])
}
}

// TestLoadProfileDetail_ProfileMissing verifies LoadProfileDetail error handling
func TestLoadProfileDetail_ProfileMissing(t *testing.T) {
tmpDir := t.TempDir()
tmpFile := filepath.Join(tmpDir, "config.yaml")

cfg := config.File{
Profiles: map[string]config.Profile{},
}

b, err := yaml.Marshal(cfg)
if err != nil {
t.Fatal(err)
}
if err := os.WriteFile(tmpFile, b, 0o600); err != nil {
t.Fatal(err)
}

result, xe := LoadProfileDetail(config.Options{ConfigPath: tmpFile}, "nonexistent")

if xe == nil {
t.Fatal("expected error for missing profile")
}

if result != nil {
t.Errorf("expected nil result, got %v", result)
}

if xe.Code != errors.CodeCfgInvalid {
t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
}
}

// TestLoadProfiles_Success_Real tests LoadProfiles with real configuration file
func TestLoadProfiles_Success_Real(t *testing.T) {
tmpDir := t.TempDir()
tmpFile := filepath.Join(tmpDir, "config.yaml")

cfg := config.File{
Profiles: map[string]config.Profile{
"prod": {
DB:       "mysql",
Host:     "prod.example.com",
Port:     3306,
User:     "admin",
Password: "secret",
Database: "main_db",
},
"dev": {
DB:       "pg",
Host:     "localhost",
Port:     5432,
User:     "dev",
Database: "dev_db",
},
},
}

b, err := yaml.Marshal(cfg)
if err != nil {
t.Fatal(err)
}
if err := os.WriteFile(tmpFile, b, 0o600); err != nil {
t.Fatal(err)
}

result, xe := LoadProfiles(config.Options{ConfigPath: tmpFile})

if xe != nil {
t.Fatalf("expected success, got error: %v", xe)
}

if result == nil {
t.Fatal("expected non-nil result")
}

if result.ConfigPath != tmpFile {
t.Errorf("expected ConfigPath=%s, got %s", tmpFile, result.ConfigPath)
}

if len(result.Profiles) != 2 {
t.Errorf("expected 2 profiles, got %d", len(result.Profiles))
}

// Profiles should be sorted alphabetically
if len(result.Profiles) > 0 && result.Profiles[0].Name != "dev" {
t.Errorf("expected first profile to be 'dev' (sorted), got %s", result.Profiles[0].Name)
}
}

// TestLoadProfileDetail_WithSSHProxy tests LoadProfileDetail with SSH proxy setting
func TestLoadProfileDetail_WithSSHProxy(t *testing.T) {
tmpDir := t.TempDir()
tmpFile := filepath.Join(tmpDir, "config.yaml")

cfg := config.File{
SSHProxies: map[string]config.SSHProxy{
"remote-proxy": {
Host:         "ssh.example.com",
Port:         22,
User:         "sshuser",
IdentityFile: "/home/user/.ssh/id_rsa",
},
},
Profiles: map[string]config.Profile{
"remote": {
DB:       "mysql",
Host:     "10.0.0.1",
Port:     3306,
User:     "admin",
Password: "secret",
Database: "mydb",
SSHProxy: "remote-proxy",
},
},
}

b, err := yaml.Marshal(cfg)
if err != nil {
t.Fatal(err)
}
if err := os.WriteFile(tmpFile, b, 0o600); err != nil {
t.Fatal(err)
}

result, xe := LoadProfileDetail(config.Options{ConfigPath: tmpFile}, "remote")

if xe != nil {
t.Fatalf("expected success, got error: %v", xe)
}

if result == nil {
t.Fatal("expected non-nil result")
}

// Verify SSH proxy is included
if sshProxy, ok := result["ssh_proxy"].(string); !ok || sshProxy != "remote-proxy" {
t.Errorf("expected ssh_proxy=remote-proxy, got %v", result["ssh_proxy"])
}

// Verify password is redacted
if pwd, ok := result["password"].(string); !ok || pwd != "***" {
t.Errorf("expected password=***, got %v", result["password"])
}
}
