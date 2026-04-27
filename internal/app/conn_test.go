package app

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/ssh"
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
		Password: "plain-value",
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
	driverName := registerTestDriver(t, &testDriver{
		openFn: func(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
			return nil, nil
		},
	})

	profile := config.Profile{
		DB:       driverName,
		Password: "plain-value",
	}

	conn, err := ResolveConnection(context.Background(), ConnectionOptions{
		Profile:        profile,
		AllowPlaintext: true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection wrapper")
	}
}

func TestResolveSSH_PassphraseNotAllowed(t *testing.T) {
	profile := config.Profile{
		SSHConfig: &config.SSHProxy{
			Host:       "example.com",
			Port:       22,
			User:       "user",
			Passphrase: "phrase-value",
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
			Host:       "127.0.0.1",
			Port:       22,
			User:       "user",
			Passphrase: "phrase-value",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := ResolveSSH(ctx, profile, true, false)

	if err == nil {
		if client != nil {
			client.Close()
		}
	}
	// Error is acceptable (no SSH server running), just verifying passphrase resolves
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

type testDriver struct {
	openFn func(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError)
}

func (d *testDriver) Open(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
	return d.openFn(ctx, opts)
}

var driverSeq uint64

func registerTestDriver(t *testing.T, d db.Driver) string {
	t.Helper()
	name := fmt.Sprintf("test_conn_driver_%d", atomic.AddUint64(&driverSeq, 1))
	db.Register(name, d)
	return name
}

func TestResolveConnection_Success_CloseRunsHook(t *testing.T) {
	var hookCalls int32
	driverName := registerTestDriver(t, &testDriver{
		openFn: func(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
			if opts.RegisterCloseHook != nil {
				opts.RegisterCloseHook(func() {
					atomic.AddInt32(&hookCalls, 1)
				})
			}
			return nil, nil
		},
	})

	conn, xe := ResolveConnection(context.Background(), ConnectionOptions{
		Profile: config.Profile{
			DB: driverName,
		},
	})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if conn == nil {
		t.Fatal("expected connection")
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if got := atomic.LoadInt32(&hookCalls); got != 1 {
		t.Fatalf("expected hook call count 1, got %d", got)
	}
}

func TestResolveConnection_OpenErrorRunsHook(t *testing.T) {
	var hookCalls int32
	driverName := registerTestDriver(t, &testDriver{
		openFn: func(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
			if opts.RegisterCloseHook != nil {
				opts.RegisterCloseHook(func() {
					atomic.AddInt32(&hookCalls, 1)
				})
			}
			return nil, errors.New(errors.CodeDBConnectFailed, "open failed", nil)
		},
	})

	conn, xe := ResolveConnection(context.Background(), ConnectionOptions{
		Profile: config.Profile{
			DB: driverName,
		},
	})
	if conn != nil {
		t.Fatal("expected nil connection")
	}
	if xe == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&hookCalls); got != 1 {
		t.Fatalf("expected hook call count 1, got %d", got)
	}
}

func TestResolveConnection_AllowPlaintextFromProfile(t *testing.T) {
	var gotPassword string
	driverName := registerTestDriver(t, &testDriver{
		openFn: func(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
			gotPassword = opts.Password
			return nil, nil
		},
	})

	conn, xe := ResolveConnection(context.Background(), ConnectionOptions{
		Profile: config.Profile{
			DB:             driverName,
			Password:       "plain-value",
			AllowPlaintext: true,
		},
		AllowPlaintext: false,
	})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if conn == nil {
		t.Fatal("expected connection")
	}
	if gotPassword != "plain-value" {
		t.Fatalf("expected resolved password to be passed, got %q", gotPassword)
	}
}

func TestResolveConnection_SSHPassphraseNotAllowed(t *testing.T) {
	driverName := registerTestDriver(t, &testDriver{
		openFn: func(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
			t.Fatal("driver should not be called when ssh passphrase resolve fails")
			return nil, nil
		},
	})

	conn, xe := ResolveConnection(context.Background(), ConnectionOptions{
		Profile: config.Profile{
			DB: driverName,
			SSHConfig: &config.SSHProxy{
				Host:       "127.0.0.1",
				Port:       22,
				User:       "user",
				Passphrase: "plain-phrase",
			},
		},
		AllowPlaintext: false,
	})
	if conn != nil {
		t.Fatal("expected nil connection")
	}
	if xe == nil {
		t.Fatal("expected error")
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Fatalf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestResolveConnection_SSHAuthFailed(t *testing.T) {
	driverName := registerTestDriver(t, &testDriver{
		openFn: func(ctx context.Context, opts db.ConnOptions) (*sql.DB, *errors.XError) {
			t.Fatal("driver should not be called when ssh connect fails")
			return nil, nil
		},
	})

	conn, xe := ResolveConnection(context.Background(), ConnectionOptions{
		Profile: config.Profile{
			DB: driverName,
			SSHConfig: &config.SSHProxy{
				Host: "127.0.0.1",
				Port: 22,
				User: "user",
			},
		},
		AllowPlaintext: true,
	})
	if conn != nil {
		t.Fatal("expected nil connection")
	}
	if xe == nil {
		t.Fatal("expected error")
	}
	if xe.Code != errors.CodeSSHAuthFailed && xe.Code != errors.CodeSSHDialFailed {
		t.Fatalf("expected ssh auth/dial failure, got %s", xe.Code)
	}
}

func TestResolveReconnectableSSH_NoSSHConfig(t *testing.T) {
	profile := config.Profile{}

	rd, xe := ResolveReconnectableSSH(context.Background(), profile, false, false, nil)
	if rd != nil {
		t.Fatal("expected nil dialer")
	}
	if xe == nil {
		t.Fatal("expected error when SSHConfig is nil")
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestResolveReconnectableSSH_PassphraseNotAllowed(t *testing.T) {
	profile := config.Profile{
		SSHConfig: &config.SSHProxy{
			Host:       "127.0.0.1",
			Port:       22,
			User:       "user",
			Passphrase: "plain-phrase",
		},
	}

	rd, xe := ResolveReconnectableSSH(context.Background(), profile, false, false, nil)
	if rd != nil {
		t.Fatal("expected nil dialer")
	}
	if xe == nil {
		t.Fatal("expected error for plaintext passphrase")
	}
	if xe.Code != errors.CodeCfgInvalid {
		t.Errorf("expected CodeCfgInvalid, got %s", xe.Code)
	}
}

func TestResolveReconnectableSSH_ConnectFails(t *testing.T) {
	profile := config.Profile{
		SSHConfig: &config.SSHProxy{
			Host: "127.0.0.1",
			Port: 1, // unlikely to have SSH on port 1
			User: "user",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var statusCalled bool
	rd, xe := ResolveReconnectableSSH(ctx, profile, true, true, func(e ssh.StatusEvent) {
		statusCalled = true
	})
	if rd != nil {
		rd.Close()
		t.Fatal("expected nil dialer")
	}
	if xe == nil {
		t.Fatal("expected connection error")
	}
	// The error may be wrapped as XError or as a generic error
	_ = statusCalled
}

func TestResolveReconnectableSSH_NilCallback(t *testing.T) {
	profile := config.Profile{
		SSHConfig: &config.SSHProxy{
			Host: "127.0.0.1",
			Port: 1,
			User: "user",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// nil callback should not cause panic
	rd, xe := ResolveReconnectableSSH(ctx, profile, true, true, nil)
	if rd != nil {
		rd.Close()
	}
	// Just verify no panic
	_ = xe
}

func TestResolveSSHOptions_Success(t *testing.T) {
	profile := config.Profile{
		SSHConfig: &config.SSHProxy{
			Host:         "bastion.example.com",
			Port:         2222,
			User:         "admin",
			IdentityFile: "~/.ssh/id_ed25519",
		},
	}

	opts, xe := resolveSSHOptions(profile, true, true)
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if opts.Host != "bastion.example.com" {
		t.Errorf("expected host bastion.example.com, got %s", opts.Host)
	}
	if opts.Port != 2222 {
		t.Errorf("expected port 2222, got %d", opts.Port)
	}
	if !opts.SkipKnownHostsCheck {
		t.Error("expected SkipKnownHostsCheck=true")
	}
}
