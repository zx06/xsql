package db

import (
	"context"
	"database/sql"

	"github.com/zx06/xsql/internal/errors"
)

// QueryResult represents a generic query result.
type QueryResult struct {
	Columns []string         `json:"columns" yaml:"columns"`
	Rows    []map[string]any `json:"rows" yaml:"rows"`
}

// ToTableData implements the output.TableFormatter interface for table output without JSON encoding/decoding.
func (r *QueryResult) ToTableData() (columns []string, rows []map[string]any, ok bool) {
	if r == nil {
		return nil, nil, false
	}
	return r.Columns, r.Rows, true
}

// QueryOptions contains options for query execution.
type QueryOptions struct {
	UnsafeAllowWrite bool   // Allow write operations (bypass read-only protection)
	DBType           string // Database type: mysql or pg
}

// Query executes a SQL query and returns the result.
// When opts.UnsafeAllowWrite is false, dual read-only protection is enabled:
// 1. SQL statement static analysis (client-side)
// 2. Database transaction-level read-only mode (server-side)
// When opts.UnsafeAllowWrite is true, all read-only protections are bypassed.
func Query(ctx context.Context, db *sql.DB, query string, opts QueryOptions) (*QueryResult, *errors.XError) {
	// UnsafeAllowWrite bypasses all read-only protections
	if opts.UnsafeAllowWrite {
		return executeQuery(ctx, db, query)
	}

	// Enable dual read-only protection by default
	// First layer: SQL static analysis
	if xe := EnforceReadOnly(query, false); xe != nil {
		return nil, xe
	}
	// Second layer: database transaction-level read-only
	return queryWithReadOnlyTx(ctx, db, query, opts.DBType)
}

// queryWithReadOnlyTx executes a query within a read-only transaction.
func queryWithReadOnlyTx(ctx context.Context, db *sql.DB, query string, dbType string) (*QueryResult, *errors.XError) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to begin read-only transaction", nil, err)
	}
	defer func() {
		// Read-only transaction needs no commit; just rollback
		_ = tx.Rollback()
	}()

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "query failed", nil, err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// executeQuery executes a query directly (without a transaction).
func executeQuery(ctx context.Context, db *sql.DB, query string) (*QueryResult, *errors.XError) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "query failed", nil, err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// scanRows scans query result rows.
func scanRows(rows *sql.Rows) (*QueryResult, *errors.XError) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get columns", nil, err)
	}

	result := &QueryResult{Columns: cols, Rows: []map[string]any{}}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan row", nil, err)
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = convertValue(vals[i])
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "rows iteration error", nil, err)
	}
	return result, nil
}

func convertValue(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	default:
		return val
	}
}
