package mysql

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
	"golang.org/x/sync/errgroup"
)

// ListTables returns the lightweight MySQL table list.
func (d *Driver) ListTables(ctx context.Context, conn *sql.DB, opts db.SchemaOptions) (*db.TableList, *errors.XError) {
	database, xe := currentDatabase(ctx, conn)
	if xe != nil {
		return nil, xe
	}

	tables, xe := d.listTables(ctx, conn, database, opts.TablePattern)
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

	table, xe := d.loadTableSummary(ctx, conn, schemaName, opts.Name)
	if xe != nil {
		return nil, xe
	}

	var (
		columns []db.Column
		indexes []db.Index
		fks     []db.ForeignKey
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		result, xe := d.getColumns(gctx, conn, schemaName, opts.Name)
		if xe != nil {
			return xe
		}
		columns = result
		return nil
	})
	g.Go(func() error {
		result, xe := d.getIndexes(gctx, conn, schemaName, opts.Name)
		if xe != nil {
			return xe
		}
		indexes = result
		return nil
	})
	g.Go(func() error {
		result, xe := d.getForeignKeys(gctx, conn, schemaName, opts.Name)
		if xe != nil {
			return xe
		}
		fks = result
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, errors.AsOrWrap(err)
	}

	table.Columns = columns
	table.Indexes = indexes
	table.ForeignKeys = fks
	return table, nil
}

func currentDatabase(ctx context.Context, conn *sql.DB) (string, *errors.XError) {
	var database string
	if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database); err != nil {
		return "", errors.Wrap(errors.CodeDBExecFailed, "failed to get database name", nil, err)
	}
	return database, nil
}

func (d *Driver) listTables(ctx context.Context, conn *sql.DB, database, tablePattern string) ([]db.TableSummary, *errors.XError) {
	query := `
		SELECT table_name, table_comment
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE'
	`
	args := []any{database}
	if tablePattern != "" {
		query += " AND table_name LIKE ?"
		args = append(args, toLikePattern(tablePattern))
	}
	query += " ORDER BY table_name"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to list tables", nil, err)
	}
	defer rows.Close()

	var tables []db.TableSummary
	for rows.Next() {
		var name, comment string
		if err := rows.Scan(&name, &comment); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan table row", nil, err)
		}
		tables = append(tables, db.TableSummary{
			Schema:  database,
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

	fkMap := make(map[string]*db.ForeignKey)
	for rows.Next() {
		var constraintName, columnName, refTable, refColumn string
		var ordinalPosition int
		if err := rows.Scan(&constraintName, &columnName, &refTable, &refColumn, &ordinalPosition); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan foreign key row", nil, err)
		}

		if fk, exists := fkMap[constraintName]; exists {
			fk.Columns = append(fk.Columns, columnName)
			fk.ReferencedColumns = append(fk.ReferencedColumns, refColumn)
		} else {
			fkMap[constraintName] = &db.ForeignKey{
				Name:              constraintName,
				Columns:           []string{columnName},
				ReferencedTable:   refTable,
				ReferencedColumns: []string{refColumn},
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "rows iteration error", nil, err)
	}

	names := make([]string, 0, len(fkMap))
	for name := range fkMap {
		names = append(names, name)
	}
	sort.Strings(names)

	fks := make([]db.ForeignKey, 0, len(names))
	for _, name := range names {
		fks = append(fks, *fkMap[name])
	}
	return fks, nil
}

func toLikePattern(pattern string) string {
	pattern = strings.ReplaceAll(pattern, "*", "%")
	return strings.ReplaceAll(pattern, "?", "_")
}
