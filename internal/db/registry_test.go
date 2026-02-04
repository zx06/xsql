package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/zx06/xsql/internal/errors"
)

type mockDriver struct{}

func (m *mockDriver) Open(ctx context.Context, opts ConnOptions) (*sql.DB, *errors.XError) {
	return nil, errors.New(errors.CodeDBConnectFailed, "mock", nil)
}

func TestRegistry(t *testing.T) {
	name := "test_driver_mock"
	Register(name, &mockDriver{})
	if _, ok := Get(name); !ok {
		t.Fatalf("expected driver")
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	name := "test_driver_dup"
	Register(name, &mockDriver{})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for duplicate driver")
		}
	}()
	Register(name, &mockDriver{})
}

func TestRegistry_InvalidArgsPanics(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for empty name")
			}
		}()
		Register("", &mockDriver{})
	})

	t.Run("nil driver", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for nil driver")
			}
		}()
		Register("test_driver_nil", nil)
	})
}

func TestRegisteredNames(t *testing.T) {
	name := "test_driver_names"
	Register(name, &mockDriver{})
	names := RegisteredNames()
	found := false
	for _, n := range names {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s in RegisteredNames", name)
	}
}
