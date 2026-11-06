package mysql

import (
	"context"
	"database/sql"
	"sort"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

// ListTables returns the lightweight MySQL table list.
func (d *Driver) ListTables(ctx context.Context, conn *sql.DB, opts db.SchemaOptions) (*db.TableList, *errors.XError) {
	database, xe := currentDatabase(ctx, conn)
	if xe != nil {
		return nil, xe
	}

	tables, xe := d.listTables(ctx, conn, database, opts.TablePattern, opts.IncludeSystem)
	if xe != nil {
		return nil, xe
	}

	return &db.TableList{
		Database: database,
		Tables:   tables,
	}, nil
}

// DescribeTable returns the schema details for a single MySQL table.
func (d *Driver) DescribeTable(ctx context.Context, conn *sql.DB, opts db.TableDescribeOptions) (*db.Table, *errors.XError) {
	database, xe := currentDatabase(ctx, conn)
	if xe != nil {
		return nil, xe
	}

	schemaName := opts.Schema
	if schemaName == "" {
		schemaName = database
	}
	return db.DescribeTableWithDetails(ctx, conn, schemaName, opts.Name, d.loadTableSummary, d.getColumns, d.getIndexes, d.getForeignKeys)
}

func currentDatabase(ctx context.Context, conn *sql.DB) (string, *errors.XError) {
	var database string
	if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database); err != nil {
		return "", errors.Wrap(errors.CodeDBExecFailed, "failed to get database name", nil, err)
	}
	return database, nil
}

func (d *Driver) listTables(ctx context.Context, conn *sql.DB, database, tablePattern string, includeSystem bool) ([]db.TableSummary, *errors.XError) {
	query := `
		SELECT table_schema, table_name, table_comment
		FROM information_schema.tables
		WHERE table_type = 'BASE TABLE'
	`
	args := []any{}
	if includeSystem {
		query += " AND (table_schema = ? OR table_schema IN ('information_schema','mysql','performance_schema','sys'))"
		args = append(args, database)
	} else {
		query += " AND table_schema = ?"
		args = append(args, database)
	}
	if tablePattern != "" {
		query += " AND table_name LIKE ?"
		args = append(args, toLikePattern(tablePattern))
	}
	query += " ORDER BY table_schema, table_name"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to list tables", nil, err)
	}
	defer rows.Close()

	tables := make([]db.TableSummary, 0)
	for rows.Next() {
		var schemaName, name, comment string
		if err := rows.Scan(&schemaName, &name, &comment); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan table row", nil, err)
		}
		tables = append(tables, db.TableSummary{
			Schema:  schemaName,
			Name:    name,
			Comment: comment,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "rows iteration error", nil, err)
	}
	return tables, nil
}

func (d *Driver) loadTableSummary(ctx context.Context, conn *sql.DB, schemaName, tableName string) (*db.Table, *errors.XError) {
	const query = `
		SELECT table_name, table_comment
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE' AND table_name = ?
	`

	var name, comment string
	if err := conn.QueryRowContext(ctx, query, schemaName, tableName).Scan(&name, &comment); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.CodeCfgInvalid, "table not found", map[string]any{
				"schema": schemaName,
				"name":   tableName,
				"reason": "table_not_found",
			})
		}
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to load table", map[string]any{"schema": schemaName, "name": tableName}, err)
	}

	return &db.Table{
		Schema:  schemaName,
		Name:    name,
		Comment: comment,
	}, nil
}

func (d *Driver) getColumns(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]db.Column, *errors.XError) {
	const query = `
		SELECT
			column_name,
			column_type,
			is_nullable,
			column_default,
			column_comment,
			CASE WHEN column_key = 'PRI' THEN 1 ELSE 0 END AS is_primary
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position
	`

	rows, err := conn.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get columns", nil, err)
	}
	defer rows.Close()

	var columns []db.Column
	for rows.Next() {
		var name, colType, nullable, defaultValue, comment sql.NullString
		var isPrimary bool
		if err := rows.Scan(&name, &colType, &nullable, &defaultValue, &comment, &isPrimary); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan column row", nil, err)
		}

		col := db.Column{
			Name:       name.String,
			Type:       colType.String,
			Nullable:   nullable.String == "YES",
			PrimaryKey: isPrimary,
		}
		if defaultValue.Valid {
			col.Default = defaultValue.String
		}
		if comment.Valid {
			col.Comment = comment.String
		}
		columns = append(columns, col)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "rows iteration error", nil, err)
	}
	return columns, nil
}

func (d *Driver) getIndexes(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]db.Index, *errors.XError) {
	const query = `
		SELECT
			index_name,
			column_name,
			NOT non_unique AS is_unique,
			index_name = 'PRIMARY' AS is_primary,
			seq_in_index
		FROM information_schema.statistics
		WHERE table_schema = ? AND table_name = ?
		ORDER BY index_name, seq_in_index
	`

	rows, err := conn.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get indexes", nil, err)
	}
	defer rows.Close()

	indexMap := make(map[string]*db.Index)
	for rows.Next() {
		var indexName, columnName string
		var isUnique, isPrimary bool
		var seqInIndex int
		if err := rows.Scan(&indexName, &columnName, &isUnique, &isPrimary, &seqInIndex); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan index row", nil, err)
		}

		if idx, exists := indexMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			indexMap[indexName] = &db.Index{
				Name:    indexName,
				Columns: []string{columnName},
				Unique:  isUnique,
				Primary: isPrimary,
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "rows iteration error", nil, err)
	}

	names := make([]string, 0, len(indexMap))
	for name := range indexMap {
		names = append(names, name)
	}
	sort.Strings(names)

	indexes := make([]db.Index, 0, len(names))
	for _, name := range names {
		indexes = append(indexes, *indexMap[name])
	}
	return indexes, nil
}

func (d *Driver) getForeignKeys(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]db.ForeignKey, *errors.XError) {
	const query = `
		SELECT
			constraint_name,
			column_name,
			referenced_table_name,
			referenced_column_name,
			ordinal_position
		FROM information_schema.key_column_usage
		WHERE table_schema = ?
		  AND table_name = ?
		  AND referenced_table_name IS NOT NULL
		ORDER BY constraint_name, ordinal_position
	`

	rows, err := conn.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get foreign keys", nil, err)
	}
	defer rows.Close()

	return db.ScanForeignKeyRows(rows)
}

func toLikePattern(pattern string) string {
	return db.ToLikePattern(pattern)
}
