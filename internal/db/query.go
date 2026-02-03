package db

import (
	"context"
	"database/sql"

	"github.com/zx06/xsql/internal/errors"
)

// QueryResult 是通用查询结果。
type QueryResult struct {
	Columns []string         `json:"columns" yaml:"columns"`
	Rows    []map[string]any `json:"rows" yaml:"rows"`
}

// ToTableData 实现 output.TableFormatter 接口，支持无 JSON 编解码的表格输出。
func (r *QueryResult) ToTableData() (columns []string, rows []map[string]any, ok bool) {
	if r == nil {
		return nil, nil, false
	}
	return r.Columns, r.Rows, true
}

// QueryOptions 包含查询执行的选项。
type QueryOptions struct {
	UnsafeAllowWrite bool   // 允许写操作（绕过只读保护）
	DBType           string // 数据库类型：mysql 或 pg
}

// Query 执行 SQL 查询并返回结果。
// 当 opts.UnsafeAllowWrite=false 时，会启用双重只读保护：
// 1. SQL 语句静态分析（客户端）
// 2. 数据库事务级只读模式（服务端）
// 当 opts.UnsafeAllowWrite=true 时，绕过所有只读保护。
func Query(ctx context.Context, db *sql.DB, query string, opts QueryOptions) (*QueryResult, *errors.XError) {
	// UnsafeAllowWrite 绕过所有只读保护
	if opts.UnsafeAllowWrite {
		return executeQuery(ctx, db, query)
	}

	// 默认启用双重只读保护
	// 第一层保护：SQL 静态分析
	if xe := EnforceReadOnly(query, false); xe != nil {
		return nil, xe
	}
	// 第二层保护：数据库事务级只读
	return queryWithReadOnlyTx(ctx, db, query, opts.DBType)
}

// queryWithReadOnlyTx 在只读事务中执行查询。
func queryWithReadOnlyTx(ctx context.Context, db *sql.DB, query string, dbType string) (*QueryResult, *errors.XError) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to begin read-only transaction", nil, err)
	}
	defer func() {
		// 只读事务无需 commit，直接 rollback
		_ = tx.Rollback()
	}()

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "query failed", nil, err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// executeQuery 直接执行查询（不使用事务）。
func executeQuery(ctx context.Context, db *sql.DB, query string) (*QueryResult, *errors.XError) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "query failed", nil, err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// scanRows 扫描查询结果。
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
