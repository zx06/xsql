package ssh

import "testing"

func TestOptions_Defaults(t *testing.T) {
	opts := Options{
		Host: "example.com",
		User: "admin",
	}

	// Port 默认值应在 Connect 时处理
	if opts.Port != 0 {
		t.Errorf("expected Port to be 0 before processing, got %d", opts.Port)
	}
	// 确保 Host 和 User 被正确设置
	if opts.Host != "example.com" {
		t.Errorf("expected Host=example.com, got %s", opts.Host)
	}
	if opts.User != "admin" {
		t.Errorf("expected User=admin, got %s", opts.User)
	}
}

func TestOptions_WithIdentityFile(t *testing.T) {
	opts := Options{
		Host:         "example.com",
		User:         "admin",
		IdentityFile: "~/.ssh/id_rsa",
	}

	if opts.IdentityFile != "~/.ssh/id_rsa" {
		t.Errorf("unexpected identity file: %s", opts.IdentityFile)
	}
	if opts.Host != "example.com" || opts.User != "admin" {
		t.Error("Host or User not set correctly")
	}
}

func TestOptions_SkipKnownHostsCheck(t *testing.T) {
	opts := Options{
		Host:                "example.com",
		User:                "admin",
		SkipKnownHostsCheck: true,
	}

	if !opts.SkipKnownHostsCheck {
		t.Error("expected SkipKnownHostsCheck to be true")
	}
	if opts.Host != "example.com" || opts.User != "admin" {
		t.Error("Host or User not set correctly")
	}
}
