//go:build !windows

package secret

import (
	"testing"

	"github.com/zalando/go-keyring"
)

func TestDefaultKeyringCRUD(t *testing.T) {
	keyring.MockInit()

	kr := defaultKeyring()
	if _, ok := kr.(*osKeyring); !ok {
		t.Fatalf("expected *osKeyring, got %T", kr)
	}

	service := "xsql-test"
	account := "acct"
	value := "secret"

	if err := kr.Set(service, account, value); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := kr.Get(service, account)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != value {
		t.Fatalf("Get returned %q, want %q", got, value)
	}

	if err := kr.Delete(service, account); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := kr.Get(service, account); err == nil {
		t.Fatal("expected error after Delete")
	}
}
