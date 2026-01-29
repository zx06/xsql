package ssh

import "testing"

func TestExpandPath(t *testing.T) {
	p := expandPath("~/.ssh/id_rsa")
	if p == "~/.ssh/id_rsa" {
		t.Fatalf("expected expansion, got %q", p)
	}
}

func TestDefaultKnownHostsPath(t *testing.T) {
	p := DefaultKnownHostsPath()
	if p != "~/.ssh/known_hosts" {
		t.Fatalf("unexpected: %q", p)
	}
}
