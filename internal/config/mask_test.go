package config

import "testing"

func TestMaskSecretForDisplay(t *testing.T) {
	if MaskSecretForDisplay("") != "" {
		t.Fatal("empty secret should stay empty")
	}
	if got := MaskSecretForDisplay("keyring:prod/password"); got != "keyring:prod/password" {
		t.Fatalf("keyring ref should be visible, got %q", got)
	}
	if got := MaskSecretForDisplay("super-secret"); got != MaskedSecret {
		t.Fatalf("plaintext should be masked, got %q", got)
	}
}

func TestCoalesceSecret(t *testing.T) {
	if got := CoalesceSecret("", "stored"); got != "stored" {
		t.Fatalf("empty incoming should keep existing, got %q", got)
	}
	if got := CoalesceSecret(MaskedSecret, "stored"); got != "stored" {
		t.Fatalf("mask placeholder should keep existing, got %q", got)
	}
	if got := CoalesceSecret("new", "stored"); got != "new" {
		t.Fatalf("explicit value should win, got %q", got)
	}
}

func TestFileForAPI_RedactsSecrets(t *testing.T) {
	f := File{
		Profiles: map[string]Profile{
			"dev": {
				Password: "plain-pass",
				DSN:      "user:pass@tcp(localhost)/db",
			},
		},
		SSHProxies: map[string]SSHProxy{
			"bastion": {
				Passphrase: "ssh-secret",
			},
		},
		MCP: MCPConfig{
			HTTP: MCPHTTPConfig{AuthToken: "mcp-token"},
		},
		Web: WebConfig{
			HTTP: WebHTTPConfig{AuthToken: "web-token"},
		},
	}
	masked := FileForAPI(f)
	if masked.Profiles["dev"].Password != MaskedSecret {
		t.Fatalf("password not masked: %q", masked.Profiles["dev"].Password)
	}
	if masked.Profiles["dev"].DSN != MaskedSecret {
		t.Fatalf("dsn not masked: %q", masked.Profiles["dev"].DSN)
	}
	if masked.SSHProxies["bastion"].Passphrase != MaskedSecret {
		t.Fatalf("passphrase not masked")
	}
	if masked.MCP.HTTP.AuthToken != MaskedSecret {
		t.Fatalf("mcp token not masked")
	}
	if masked.Web.HTTP.AuthToken != MaskedSecret {
		t.Fatalf("web token not masked")
	}
}

func TestFileForAPI_ShowsKeyringReferences(t *testing.T) {
	f := File{
		Profiles: map[string]Profile{
			"dev": {Password: "keyring:prod/db"},
		},
	}
	masked := FileForAPI(f)
	if masked.Profiles["dev"].Password != "keyring:prod/db" {
		t.Fatalf("expected keyring ref for password, got %q", masked.Profiles["dev"].Password)
	}
}

func TestMergeProfileSecrets_PreservesOnMask(t *testing.T) {
	existing := Profile{Password: "stored-pass", DSN: "stored-dsn"}
	incoming := Profile{Password: MaskedSecret, DSN: MaskedSecret, Host: "new-host"}
	merged := MergeProfileSecrets(incoming, existing)
	if merged.Password != "stored-pass" || merged.DSN != "stored-dsn" {
		t.Fatalf("secrets not preserved: password=%q dsn=%q", merged.Password, merged.DSN)
	}
	if merged.Host != "new-host" {
		t.Fatalf("non-secret fields should copy through")
	}
}
