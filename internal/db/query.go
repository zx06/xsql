package db

import (
	"context"
	"database/sql"
	"strings"

	"github.com/zx06/xsql/internal/errors"
)

// QueryResult represents a generic query result.
type QueryResult struct {
	Columns []string         `json:"columns" yaml:"columns"`
	Rows    []map[string]any `json:"rows" yaml:"rows"`

	Truncated bool `json:"truncated,omitempty" yaml:"truncated,omitempty"`
	RowCount  int  `json:"row_count,omitempty" yaml:"row_count,omitempty"`
	MaxRows   int  `json:"max_rows,omitempty" yaml:"max_rows,omitempty"`
	// MaxRowsRequested is set when MCP/user requested more rows than mcp.max_rows_limit.
	MaxRowsRequested int `json:"max_rows_requested,omitempty" yaml:"max_rows_requested,omitempty"`
	// MaxRowsLimitApplied is the configured MCP ceiling when clamping occurred.
	MaxRowsLimitApplied int    `json:"max_rows_limit,omitempty" yaml:"max_rows_limit,omitempty"`
	TruncatedReason     string `json:"truncated_reason,omitempty" yaml:"truncated_reason,omitempty"`
	MaxResponseBytes    int    `json:"max_response_bytes,omitempty" yaml:"max_response_bytes,omitempty"`
	Hint                string `json:"hint,omitempty" yaml:"hint,omitempty"`
}

// ToTableData implements the output.TableFormatter interface for table output without JSON encoding/decoding.
func (r *QueryResult) ToTableData() (columns []string, rows []map[string]any, ok bool) {
	if r == nil {
		return nil, nil, false
	}
	return r.Columns, r.Rows, true
}

// QueryMeta reports truncation metadata for table output.
func (r *QueryResult) QueryMeta() (truncated bool, rowCount, maxRows int, hint string, ok bool) {
	if r == nil || !r.Truncated {
		return false, 0, 0, "", false
	}
	return true, r.RowCount, r.MaxRows, r.Hint, true
}

// QueryOptions contains options for query execution.
type QueryOptions struct {
	UnsafeAllowWrite bool   // Allow write operations (bypass read-only protection)
	DBType           string // Database type: mysql or pg
	// BypassStaticReadOnly skips SQL static analysis (integration tests for DB-layer enforcement only).
	BypassStaticReadOnly bool
	// MaxRows limits rows scanned (0 = unlimited).
	MaxRows int
}

// Query executes a SQL query and returns the result.
// When opts.UnsafeAllowWrite is false, dual read-only protection is enabled:
// 1. SQL statement static analysis (client-side)
// 2. Database transaction-level read-only mode (server-side)
// When opts.UnsafeAllowWrite is true, all read-only protections are bypassed.
func Query(ctx context.Context, db *sql.DB, query string, opts QueryOptions) (*QueryResult, *errors.XError) {
	// UnsafeAllowWrite bypasses all read-only protections
	if opts.UnsafeAllowWrite {
		return executeQuery(ctx, db, query, opts.DBType, opts.MaxRows)
	}

	// Enable dual read-only protection by default
	if !opts.BypassStaticReadOnly {
		if xe := EnforceReadOnly(query, false); xe != nil {
			return nil, xe
		}
	}
	// Second layer: database transaction-level read-only
	return queryWithReadOnlyTx(ctx, db, query, opts.DBType, opts.MaxRows)
}

func normalizeDBType(dbType string) string {
	switch strings.ToLower(strings.TrimSpace(dbType)) {
	case "mysql", "mariadb":
		return "mysql"
	case "pg", "postgres", "postgresql":
		return "pg"
	default:
		return strings.ToLower(strings.TrimSpace(dbType))
	}
}

// beginReadOnlyTx starts a read-only transaction using dialect-appropriate driver settings.
func beginReadOnlyTx(ctx context.Context, db *sql.DB, dbType string) (*sql.Tx, *errors.XError) {
	dialect := normalizeDBType(dbType)
	details := map[string]any{"dialect": dialect}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to begin read-only transaction", details, err)
	}

	// MySQL: go-sql-driver/mysql maps ReadOnly to START TRANSACTION READ ONLY.
	// Do not use @@transaction_read_only here — on MySQL 8.0 it often stays 0 inside a
	// read-only transaction while writes are still rejected (ER 1792). Integration tests
	// assert write blocking for MySQL.
	if dialect == "pg" {
		var ro string
		if err := tx.QueryRowContext(ctx, "SHOW transaction_read_only").Scan(&ro); err == nil && !strings.EqualFold(ro, "on") {
			_ = tx.Rollback()
			return nil, errors.New(errors.CodeDBExecFailed, "postgresql connection is not in a read-only transaction", details)
		}
	}

	return tx, nil
}

// queryWithReadOnlyTx executes a query within a read-only transaction.
func queryWithReadOnlyTx(ctx context.Context, db *sql.DB, query string, dbType string, maxRows int) (*QueryResult, *errors.XError) {
	tx, xe := beginReadOnlyTx(ctx, db, dbType)
	if xe != nil {
		return nil, xe
	}
	defer func() {
		// Read-only transaction needs no commit; just rollback
		_ = tx.Rollback()
	}()

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, wrapQueryFailed(dbType, err)
	}
	defer rows.Close()

	return scanRows(rows, maxRows)
}

// executeQuery executes a query directly (without a transaction).
func executeQuery(ctx context.Context, db *sql.DB, query string, dbType string, maxRows int) (*QueryResult, *errors.XError) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, wrapQueryFailed(dbType, err)
	}
	defer rows.Close()

	return scanRows(rows, maxRows)
}

// scanRows scans query result rows, stopping after maxRows when maxRows > 0.
func scanRows(rows *sql.Rows, maxRows int) (*QueryResult, *errors.XError) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get columns", nil, err)
	}

	result := &QueryResult{Columns: cols, Rows: make([]map[string]any, 0)}
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

		if maxRows > 0 && len(result.Rows) == maxRows {
			if rows.Next() {
				result.Truncated = true
				result.MaxRows = maxRows
				result.TruncatedReason = TruncatedReasonMaxRows
				RefreshTruncationHint(result)
			}
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "rows iteration error", nil, err)
	}
	result.RowCount = len(result.Rows)
	return result, nil
}

func wrapQueryFailed(dbType string, err error) *errors.XError {
	dialect := normalizeDBType(dbType)
	details := map[string]any{}
	if dialect != "" {
		details["dialect"] = dialect
	}
	return errors.Wrap(errors.CodeDBExecFailed, "query failed", details, err)
}

func convertValue(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	default:
		return val
	}
}
