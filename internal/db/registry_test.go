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
