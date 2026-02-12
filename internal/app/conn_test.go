package app

import (
	"testing"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
)

func TestResolveConnection_UnsupportedDriver(t *testing.T) {
	profile := config.Profile{
		DB: "unsupported",
	}

	conn, err := ResolveConnection(nil, ConnectionOptions{
		Profile: profile,
	})

	if conn != nil {
		t.Fatal("expected nil connection")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != errors.CodeDBDriverUnsupported {
		t.Errorf("expected CodeDBDriverUnsupported, got %s", err.Code)
	}
}

func TestResolveSSH_NoSSHConfig(t *testing.T) {
	profile := config.Profile{}

	client, err := ResolveSSH(nil, profile, false, false)

	if client != nil {
		t.Fatal("expected nil client")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveConnection_PasswordNotAllowed(t *testing.T) {
	profile := config.Profile{
		DB:       "mysql",
		Password: "plaintext_password",
	}

	conn, err := ResolveConnection(nil, ConnectionOptions{
		Profile:        profile,
		AllowPlaintext: false,
	})

	if conn != nil {
		t.Fatal("expected nil connection")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", err.Code)
	}
}

func TestResolveConnection_PasswordAllowed(t *testing.T) {
	profile := config.Profile{
		DB:       "mysql",
		Password: "plaintext_password",
	}

	conn, err := ResolveConnection(nil, ConnectionOptions{
		Profile:        profile,
		AllowPlaintext: true,
	})

	if err == nil {
		if conn != nil {
			conn.Close()
		}
	}
}

func TestResolveSSH_PassphraseNotAllowed(t *testing.T) {
	profile := config.Profile{
		SSHConfig: &config.SSHProxy{
			Host:       "example.com",
			Port:       22,
			User:       "user",
			Passphrase: "some_passphrase",
		},
	}

	client, err := ResolveSSH(nil, profile, false, false)

	if client != nil {
		t.Fatal("expected nil client")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", err.Code)
	}
}

func TestResolveSSH_PassphraseAllowed(t *testing.T) {
	profile := config.Profile{
		SSHConfig: &config.SSHProxy{
			Host:       "example.com",
			Port:       22,
			User:       "user",
			Passphrase: "some_passphrase",
		},
	}

	client, err := ResolveSSH(nil, profile, true, false)

	if err == nil {
		if client != nil {
			client.Close()
		}
	}
}

func TestConnectionOptions_Fields(t *testing.T) {
	opts := ConnectionOptions{
		Profile: config.Profile{
			DB: "pg",
		},
		AllowPlaintext:   true,
		SkipHostKeyCheck: true,
	}

	if opts.Profile.DB != "pg" {
		t.Errorf("expected db pg, got %s", opts.Profile.DB)
	}
	if !opts.AllowPlaintext {
		t.Error("expected AllowPlaintext to be true")
	}
	if !opts.SkipHostKeyCheck {
		t.Error("expected SkipHostKeyCheck to be true")
	}
}
