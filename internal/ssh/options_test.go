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
}
