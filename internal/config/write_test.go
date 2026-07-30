package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInitConfig(t *testing.T) {
	t.Run("creates config at specified path", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "subdir", "xsql.yaml")

		cfgPath, xe := InitConfig(path)
		if xe != nil {
			t.Fatalf("unexpected error: %v", xe)
		}
		if cfgPath != path {
			t.Errorf("expected %s, got %s", path, cfgPath)
		}

		// File should exist
		if _, err := os.Stat(path); err != nil {
			t.Errorf("config file should exist: %v", err)
		}

		// File should be valid YAML
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		var f File
		if err := yaml.Unmarshal(data, &f); err != nil {
			t.Errorf("config should be valid YAML: %v", err)
		}
	})

	t.Run("creates config at default path", func(t *testing.T) {
		// Create a temp HOME
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		t.Setenv("USERPROFILE", dir) // Windows compatibility

		cfgPath, xe := InitConfig("")
		if xe != nil {
			t.Fatalf("unexpected error: %v", xe)
		}

		expected := filepath.Join(dir, ".config", "xsql", "xsql.yaml")
		if cfgPath != expected {
			t.Errorf("expected %s, got %s", expected, cfgPath)
		}
	})

	t.Run("fails if file already exists", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}

		_, xe := InitConfig(path)
		if xe == nil {
			t.Error("expected error when file exists")
		}
	})
}

func TestSetConfigValue(t *testing.T) {
	t.Run("set profile field on new config", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		// Create minimal config
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "profile.dev.host", "localhost")
		if xe != nil {
			t.Fatalf("unexpected error: %v", xe)
		}

		// Read back and verify
		f, xe2 := readFile(path)
		if xe2 != nil {
			t.Fatalf("failed to read config: %v", xe2)
		}

		p, ok := f.Profiles["dev"]
		if !ok {
			t.Fatal("profile 'dev' not found")
		}
		if p.Host != "localhost" {
			t.Errorf("expected host=localhost, got %s", p.Host)
		}
	})

	t.Run("set profile port as number", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "profile.dev.port", "3306")
		if xe != nil {
			t.Fatalf("unexpected error: %v", xe)
		}

		f, _ := readFile(path)
		if f.Profiles["dev"].Port != 3306 {
			t.Errorf("expected port=3306, got %d", f.Profiles["dev"].Port)
		}
	})

	t.Run("set profile local_port", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "profile.prod.local_port", "13306")
		if xe != nil {
			t.Fatalf("unexpected error: %v", xe)
		}

		f, _ := readFile(path)
		if f.Profiles["prod"].LocalPort != 13306 {
			t.Errorf("expected local_port=13306, got %d", f.Profiles["prod"].LocalPort)
		}
	})

	t.Run("set ssh_proxy field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "ssh_proxy.bastion.host", "bastion.example.com")
		if xe != nil {
			t.Fatalf("unexpected error: %v", xe)
		}

		f, _ := readFile(path)
		sp, ok := f.SSHProxies["bastion"]
		if !ok {
			t.Fatal("ssh_proxy 'bastion' not found")
		}
		if sp.Host != "bastion.example.com" {
			t.Errorf("expected host=bastion.example.com, got %s", sp.Host)
		}
	})

	t.Run("set ssh_proxy port", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "ssh_proxy.bastion.port", "2222")
		if xe != nil {
			t.Fatalf("unexpected error: %v", xe)
		}

		f, _ := readFile(path)
		if f.SSHProxies["bastion"].Port != 2222 {
			t.Errorf("expected port=2222, got %d", f.SSHProxies["bastion"].Port)
		}
	})

	t.Run("invalid key format", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "invalidkey", "value")
		if xe == nil {
			t.Error("expected error for invalid key format")
		}
	})

	t.Run("invalid section", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "unknown.name.field", "value")
		if xe == nil {
			t.Error("expected error for unknown section")
		}
	})

	t.Run("invalid port value", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "profile.dev.port", "notanumber")
		if xe == nil {
			t.Error("expected error for invalid port")
		}
	})

	t.Run("unknown profile field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "profile.dev.nonexistent", "value")
		if xe == nil {
			t.Error("expected error for unknown field")
		}
	})

	t.Run("unknown ssh_proxy field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "ssh_proxy.bastion.nonexistent", "value")
		if xe == nil {
			t.Error("expected error for unknown field")
		}
	})

	t.Run("empty config path", func(t *testing.T) {
		xe := SetConfigValue("", "profile.dev.host", "localhost")
		if xe == nil {
			t.Error("expected error for empty config path")
		}
	})

	t.Run("set all profile fields", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		fields := map[string]string{
			"db":                 "mysql",
			"host":               "localhost",
			"user":               "root",
			"password":           "secret",
			"database":           "mydb",
			"dsn":                "root:secret@tcp(localhost:3306)/mydb",
			"description":        "test db",
			"format":             "json",
			"ssh_proxy":          "bastion",
			"unsafe_allow_write": "true",
			"allow_plaintext":    "true",
		}

		for field, value := range fields {
			xe := SetConfigValue(path, "profile.test."+field, value)
			if xe != nil {
				t.Fatalf("failed to set %s: %v", field, xe)
			}
		}

		f, _ := readFile(path)
		p := f.Profiles["test"]
		if p.DB != "mysql" {
			t.Errorf("db: expected mysql, got %s", p.DB)
		}
		if p.Host != "localhost" {
			t.Errorf("host: expected localhost, got %s", p.Host)
		}
		if !p.UnsafeAllowWrite {
			t.Error("unsafe_allow_write should be true")
		}
		if !p.AllowPlaintext {
			t.Error("allow_plaintext should be true")
		}
	})

	t.Run("set all ssh_proxy fields", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		fields := map[string]string{
			"host":             "bastion.example.com",
			"user":             "admin",
			"identity_file":    "~/.ssh/id_ed25519",
			"passphrase":       "keyring:ssh/passphrase",
			"known_hosts_file": "~/.ssh/known_hosts",
			"skip_host_key":    "true",
		}

		for field, value := range fields {
			xe := SetConfigValue(path, "ssh_proxy.bastion."+field, value)
			if xe != nil {
				t.Fatalf("failed to set %s: %v", field, xe)
			}
		}

		f, _ := readFile(path)
		sp := f.SSHProxies["bastion"]
		if sp.Host != "bastion.example.com" {
			t.Errorf("host: expected bastion.example.com, got %s", sp.Host)
		}
		if sp.User != "admin" {
			t.Errorf("user: expected admin, got %s", sp.User)
		}
		if !sp.SkipHostKey {
			t.Error("skip_host_key should be true")
		}
	})

	t.Run("invalid local_port value", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "profile.dev.local_port", "abc")
		if xe == nil {
			t.Error("expected error for non-numeric local_port")
		}
	})

	t.Run("invalid ssh_proxy port value", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		xe := SetConfigValue(path, "ssh_proxy.bastion.port", "abc")
		if xe == nil {
			t.Error("expected error for non-numeric ssh_proxy port")
		}
	})
}

func TestFindConfigPath(t *testing.T) {
	t.Run("returns explicit config path", func(t *testing.T) {
		path := FindConfigPath(Options{ConfigPath: "/explicit/path.yaml"})
		if path != "/explicit/path.yaml" {
			t.Errorf("expected /explicit/path.yaml, got %s", path)
		}
	})

	t.Run("finds config in work dir", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(cfgPath, []byte("profiles: {}"), 0600); err != nil {
			t.Fatal(err)
		}

		path := FindConfigPath(Options{WorkDir: dir})
		if path != cfgPath {
			t.Errorf("expected %s, got %s", cfgPath, path)
		}
	})

	t.Run("returns default home path when not found", func(t *testing.T) {
		dir := t.TempDir()
		path := FindConfigPath(Options{WorkDir: "/nonexistent", HomeDir: dir})
		expected := filepath.Join(dir, ".config", "xsql", "xsql.yaml")
		if path != expected {
			t.Errorf("expected %s, got %s", expected, path)
		}
	})
}

func TestParseBool(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"Yes", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"", false},
	}

	for _, tc := range cases {
		got := parseBool(tc.input)
		if got != tc.want {
			t.Errorf("parseBool(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestLocalPortInProfile(t *testing.T) {
	// Test that local_port is properly deserialized from YAML
	yamlContent := `profiles:
  dev:
    db: mysql
    host: localhost
    port: 3306
    local_port: 13306
ssh_proxies: {}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "xsql.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0600); err != nil {
		t.Fatal(err)
	}

	f, xe := readFile(path)
	if xe != nil {
		t.Fatalf("failed to read config: %v", xe)
	}

	p, ok := f.Profiles["dev"]
	if !ok {
		t.Fatal("profile 'dev' not found")
	}

	if p.LocalPort != 13306 {
		t.Errorf("expected local_port=13306, got %d", p.LocalPort)
	}
}

func TestSaveAndDeleteProfile(t *testing.T) {
	t.Run("SaveProfile creates and updates profile", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		prof := Profile{
			DB:          "mysql",
			Host:        "127.0.0.1",
			Port:        3306,
			User:        "root",
			Database:    "mydb",
			Description: "test profile",
		}

		if err := SaveProfile(path, "dev", prof); err != nil {
			t.Fatalf("unexpected error saving profile: %v", err)
		}

		f, err := readFile(path)
		if err != nil {
			t.Fatalf("failed to read back config: %v", err)
		}
		if p, ok := f.Profiles["dev"]; !ok || p.Host != "127.0.0.1" || p.Database != "mydb" {
			t.Errorf("expected profile dev with host=127.0.0.1, got %+v", p)
		}
	})

	t.Run("SaveProfile with empty configPath initializes default path", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		t.Setenv("USERPROFILE", dir)

		prof := Profile{DB: "mysql", Host: "127.0.0.1"}
		if err := SaveProfile("", "dev", prof); err != nil {
			t.Fatalf("unexpected error saving profile with empty path: %v", err)
		}

		// Also test SaveProfile when file already exists
		if err := SaveProfile("", "prod", prof); err != nil {
			t.Fatalf("unexpected error updating profile: %v", err)
		}
	})

	t.Run("SaveProfile validation errors", func(t *testing.T) {
		dummyFile := filepath.Join(t.TempDir(), "dummy_file")
		_ = os.WriteFile(dummyFile, []byte("x"), 0600)
		invalidPath := filepath.Join(dummyFile, "xsql.yaml")

		if err := SaveProfile(invalidPath, "", Profile{}); err == nil {
			t.Error("expected error when profile name is empty")
		}
		if err := SaveProfile(invalidPath, "dev", Profile{}); err == nil {
			t.Error("expected error when config file cannot be read/written")
		}
	})

	t.Run("DeleteProfile removes profile", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles:\n  dev:\n    db: mysql\n"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := DeleteProfile(path, "dev"); err != nil {
			t.Fatalf("unexpected error deleting profile: %v", err)
		}

		f, err := readFile(path)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}
		if _, ok := f.Profiles["dev"]; ok {
			t.Error("expected profile 'dev' to be deleted")
		}
	})

	t.Run("DeleteProfile with empty configPath", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		t.Setenv("USERPROFILE", dir)

		prof := Profile{DB: "mysql", Host: "127.0.0.1"}
		_ = SaveProfile("", "dev", prof)

		if err := DeleteProfile("", "dev"); err != nil {
			t.Fatalf("unexpected error deleting profile with empty path: %v", err)
		}
	})

	t.Run("DeleteProfile validation errors", func(t *testing.T) {
		dummyFile := filepath.Join(t.TempDir(), "dummy_file")
		_ = os.WriteFile(dummyFile, []byte("x"), 0600)
		invalidPath := filepath.Join(dummyFile, "xsql.yaml")

		if err := DeleteProfile(invalidPath, ""); err == nil {
			t.Error("expected error when profile name is empty")
		}
	})
}

func TestSaveAndDeleteSSHProxy(t *testing.T) {
	t.Run("SaveSSHProxy creates and updates proxy", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("profiles: {}\nssh_proxies: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}

		proxy := SSHProxy{
			Host: "bastion.example.com",
			Port: 22,
			User: "ubuntu",
		}

		if err := SaveSSHProxy(path, "bastion", proxy); err != nil {
			t.Fatalf("unexpected error saving ssh proxy: %v", err)
		}

		f, err := readFile(path)
		if err != nil {
			t.Fatalf("failed to read back config: %v", err)
		}
		if sp, ok := f.SSHProxies["bastion"]; !ok || sp.Host != "bastion.example.com" {
			t.Errorf("expected ssh proxy bastion with host=bastion.example.com, got %+v", sp)
		}
	})

	t.Run("SaveSSHProxy with empty configPath initializes default path", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		t.Setenv("USERPROFILE", dir)

		proxy := SSHProxy{Host: "bastion.example.com"}
		if err := SaveSSHProxy("", "bastion", proxy); err != nil {
			t.Fatalf("unexpected error saving ssh proxy with empty path: %v", err)
		}

		if err := SaveSSHProxy("", "bastion2", proxy); err != nil {
			t.Fatalf("unexpected error updating ssh proxy: %v", err)
		}
	})

	t.Run("SaveSSHProxy validation errors", func(t *testing.T) {
		dummyFile := filepath.Join(t.TempDir(), "dummy_file")
		_ = os.WriteFile(dummyFile, []byte("x"), 0600)
		invalidPath := filepath.Join(dummyFile, "xsql.yaml")

		if err := SaveSSHProxy(invalidPath, "", SSHProxy{}); err == nil {
			t.Error("expected error when ssh proxy name is empty")
		}
		if err := SaveSSHProxy(invalidPath, "bastion", SSHProxy{}); err == nil {
			t.Error("expected error when config file cannot be read/written")
		}
	})

	t.Run("DeleteSSHProxy removes ssh proxy", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("ssh_proxies:\n  bastion:\n    host: bastion.example.com\n"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := DeleteSSHProxy(path, "bastion"); err != nil {
			t.Fatalf("unexpected error deleting ssh proxy: %v", err)
		}

		f, err := readFile(path)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}
		if _, ok := f.SSHProxies["bastion"]; ok {
			t.Error("expected ssh proxy 'bastion' to be deleted")
		}
	})

	t.Run("DeleteSSHProxy with empty configPath", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		t.Setenv("USERPROFILE", dir)

		proxy := SSHProxy{Host: "bastion.example.com"}
		_ = SaveSSHProxy("", "bastion", proxy)

		if err := DeleteSSHProxy("", "bastion"); err != nil {
			t.Fatalf("unexpected error deleting ssh proxy with empty path: %v", err)
		}
	})

	t.Run("DeleteSSHProxy validation errors", func(t *testing.T) {
		dummyFile := filepath.Join(t.TempDir(), "dummy_file")
		_ = os.WriteFile(dummyFile, []byte("x"), 0600)
		invalidPath := filepath.Join(dummyFile, "xsql.yaml")

		if err := DeleteSSHProxy(invalidPath, ""); err == nil {
			t.Error("expected error when ssh proxy name is empty")
		}
	})

	t.Run("SaveProfile and DeleteProfile with invalid YAML file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("invalid_yaml: [unclosed"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := SaveProfile(path, "dev", Profile{}); err == nil {
			t.Error("expected error for invalid YAML in SaveProfile")
		}

		if err := DeleteProfile(path, "dev"); err == nil {
			t.Error("expected error for invalid YAML in DeleteProfile")
		}
	})

	t.Run("SaveSSHProxy and DeleteSSHProxy with invalid YAML file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "xsql.yaml")
		if err := os.WriteFile(path, []byte("invalid_yaml: [unclosed"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := SaveSSHProxy(path, "bastion", SSHProxy{}); err == nil {
			t.Error("expected error for invalid YAML in SaveSSHProxy")
		}

		if err := DeleteSSHProxy(path, "bastion"); err == nil {
			t.Error("expected error for invalid YAML in DeleteSSHProxy")
		}
	})

	t.Run("SaveProfile on non-existent config file creates file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "new_dir", "xsql.yaml")
		// Parent dir created automatically by writeFile
		if err := SaveProfile(path, "dev", Profile{Host: "localhost"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("SaveSSHProxy on non-existent config file creates file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "new_dir", "xsql.yaml")
		if err := SaveSSHProxy(path, "bastion", SSHProxy{Host: "bastion.example.com"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
