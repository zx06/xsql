package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	stdErrors "errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/errors"
)

// 纯函数单元测试，不需要数据库连接
func TestConvertValue(t *testing.T) {
	cases := []struct {
		input any
		want  any
	}{
		{[]byte("hello"), "hello"},
		{[]byte{}, ""},
		{"string", "string"},
		{42, 42},
		{3.14, 3.14},
		{nil, nil},
		{true, true},
	}

	for _, tc := range cases {
		got := convertValue(tc.input)
		if got != tc.want {
			t.Errorf("convertValue(%v)=%v, want %v", tc.input, got, tc.want)
		}
	}
}

// Query 函数的集成测试在 tests/integration/query_test.go 中

type stubDriver struct {
	responseRows  map[string]*stubRows
	beginCalled   bool
	beginReadOnly bool
}

type stubConnector struct {
	driver *stubDriver
}

func (c *stubConnector) Connect(context.Context) (driver.Conn, error) {
	return &stubConn{driver: c.driver}, nil
}

func (c *stubConnector) Driver() driver.Driver {
	return c.driver
}

func (d *stubDriver) Open(string) (driver.Conn, error) {
	return &stubConn{driver: d}, nil
}

type stubConn struct {
	driver *stubDriver
}

func (c *stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, stdErrors.New("prepare not supported")
}

func (c *stubConn) Close() error {
	return nil
}

func (c *stubConn) Begin() (driver.Tx, error) {
	return &stubTx{}, nil
}

func (c *stubConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.driver.beginCalled = true
	c.driver.beginReadOnly = opts.ReadOnly
	return &stubTx{}, nil
}

func (c *stubConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if rows, ok := c.driver.responseRows[query]; ok {
		return rows, nil
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type stubTx struct{}

func (t *stubTx) Commit() error {
	return nil
}

func (t *stubTx) Rollback() error {
	return nil
}

type stubRows struct {
	columns []string
	rows    [][]driver.Value
	idx     int
	err     error
}

func (r *stubRows) Columns() []string {
	return r.columns
}

func (r *stubRows) Close() error {
	return nil
}

func (r *stubRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		if r.err != nil {
			return r.err
		}
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

func newStubDB(t *testing.T, rows map[string]*stubRows) (*sql.DB, *stubDriver) {
	t.Helper()
	driver := &stubDriver{responseRows: rows}
	db := sql.OpenDB(&stubConnector{driver: driver})
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db, driver
}

func TestQueryResultToTableData(t *testing.T) {
	var result *QueryResult
	cols, rows, ok := result.ToTableData()
	if ok || cols != nil || rows != nil {
		t.Fatalf("nil result should return ok=false, got ok=%v cols=%v rows=%v", ok, cols, rows)
	}

	result = &QueryResult{Columns: []string{"id"}, Rows: []map[string]any{{"id": 1}}}
	cols, rows, ok = result.ToTableData()
	if !ok || len(cols) != 1 || len(rows) != 1 {
		t.Fatalf("expected table data, got ok=%v cols=%v rows=%v", ok, cols, rows)
	}
}

func TestQuery_UnsafeAllowWrite_UsesDirectQuery(t *testing.T) {
	db, driver := newStubDB(t, map[string]*stubRows{
		"select 1": {
			columns: []string{"value"},
			rows:    [][]driver.Value{{1}},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, xe := Query(ctx, db, "select 1", QueryOptions{UnsafeAllowWrite: true})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if driver.beginCalled {
		t.Fatalf("expected no transaction when UnsafeAllowWrite=true")
	}
	if len(result.Rows) != 1 || result.Rows[0]["value"] != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestQuery_ReadOnly_UsesTransaction(t *testing.T) {
	db, driver := newStubDB(t, map[string]*stubRows{
		"select 1": {
			columns: []string{"value"},
			rows:    [][]driver.Value{{1}},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, xe := Query(ctx, db, "select 1", QueryOptions{})
	if xe != nil {
		t.Fatalf("unexpected error: %v", xe)
	}
	if !driver.beginCalled || !driver.beginReadOnly {
		t.Fatalf("expected read-only transaction, beginCalled=%v readOnly=%v", driver.beginCalled, driver.beginReadOnly)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected one row, got %#v", result)
	}
}

func TestQuery_ReadOnlyBlocksWrite(t *testing.T) {
	db, _ := newStubDB(t, map[string]*stubRows{})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, xe := Query(ctx, db, "INSERT INTO t VALUES (1)", QueryOptions{})
	if xe == nil {
		t.Fatal("expected error for write query")
	}
	if xe.Code != errors.CodeROBlocked {
		t.Fatalf("expected CodeROBlocked, got %s", xe.Code)
	}
}

func TestScanRows_ReportsIterationError(t *testing.T) {
	db, _ := newStubDB(t, map[string]*stubRows{
		"select error": {
			columns: []string{"value"},
			rows:    [][]driver.Value{{1}},
			err:     stdErrors.New("iteration error"),
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, xe := executeQuery(ctx, db, "select error")
	if xe == nil {
		t.Fatal("expected error from rows iteration")
	}
	if xe.Code != errors.CodeDBExecFailed {
		t.Fatalf("expected CodeDBExecFailed, got %s", xe.Code)
	}
}
