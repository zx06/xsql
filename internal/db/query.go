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

// Query 执行只读查询并返回结果。
func Query(ctx context.Context, db *sql.DB, query string, readOnly bool, unsafeAllowWrite bool) (*QueryResult, *errors.XError) {
	if readOnly {
		if xe := EnforceReadOnly(query, unsafeAllowWrite); xe != nil {
			return nil, xe
		}
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "query failed", nil, err)
	}
	defer rows.Close()

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
