package pg

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

// ListTables returns the lightweight PostgreSQL table list.
func (d *Driver) ListTables(ctx context.Context, conn *sql.DB, opts db.SchemaOptions) (*db.TableList, *errors.XError) {
	database, xe := currentDatabase(ctx, conn)
	if xe != nil {
		return nil, xe
	}

	schemas, xe := d.listSchemas(ctx, conn, opts.IncludeSystem)
	if xe != nil {
		return nil, xe
	}

	tablePattern := toLikePattern(opts.TablePattern)
	tables, xe := d.listTables(ctx, conn, schemas, tablePattern)
	if xe != nil {
		return nil, xe
	}

	return &db.TableList{
		Database: database,
		Tables:   tables,
	}, nil
}

// DescribeTable returns the schema details for a single PostgreSQL table.
func (d *Driver) DescribeTable(ctx context.Context, conn *sql.DB, opts db.TableDescribeOptions) (*db.Table, *errors.XError) {
	table, xe := d.loadTableSummary(ctx, conn, opts.Schema, opts.Name)
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
		result, xe := d.getColumns(gctx, conn, opts.Schema, opts.Name)
		if xe != nil {
			return xe
		}
		columns = result
		return nil
	})
	g.Go(func() error {
		result, xe := d.getIndexes(gctx, conn, opts.Schema, opts.Name)
		if xe != nil {
			return xe
		}
		indexes = result
		return nil
	})
	g.Go(func() error {
		result, xe := d.getForeignKeys(gctx, conn, opts.Schema, opts.Name)
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
	if err := conn.QueryRowContext(ctx, "SELECT current_database()").Scan(&database); err != nil {
		return "", errors.Wrap(errors.CodeDBExecFailed, "failed to get database name", nil, err)
	}
	return database, nil
}

func (d *Driver) listSchemas(ctx context.Context, conn *sql.DB, includeSystem bool) ([]string, *errors.XError) {
	query := `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
	`
	if !includeSystem {
		query += " AND schema_name NOT LIKE 'pg_%'"
	}
	query += " ORDER BY schema_name"

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to list schemas", nil, err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan schema row", nil, err)
		}
		schemas = append(schemas, schema)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "rows iteration error", nil, err)
	}
	return schemas, nil
}

func (d *Driver) listTables(ctx context.Context, conn *sql.DB, schemas []string, tablePattern string) ([]db.TableSummary, *errors.XError) {
	query := `
		SELECT
			t.table_schema,
			t.table_name,
			obj_description((quote_ident(t.table_schema) || '.' || quote_ident(t.table_name))::regclass, 'pg_class') AS table_comment
		FROM information_schema.tables t
		WHERE t.table_schema = ANY($1) AND t.table_type = 'BASE TABLE'
	`
	args := []any{schemas}
	if tablePattern != "" {
		query += " AND t.table_name LIKE $2"
		args = append(args, tablePattern)
	}
	query += " ORDER BY t.table_schema, t.table_name"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to list tables", nil, err)
	}
	defer rows.Close()

	var tables []db.TableSummary
	for rows.Next() {
		var schema, name string
		var comment sql.NullString
		if err := rows.Scan(&schema, &name, &comment); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan table row", nil, err)
		}
		tables = append(tables, db.TableSummary{
			Schema:  schema,
			Name:    name,
			Comment: comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "rows iteration error", nil, err)
	}
	return tables, nil
}

func (d *Driver) loadTableSummary(ctx context.Context, conn *sql.DB, schemaName, tableName string) (*db.Table, *errors.XError) {
	const query = `
		SELECT
			t.table_schema,
			t.table_name,
			obj_description((quote_ident(t.table_schema) || '.' || quote_ident(t.table_name))::regclass, 'pg_class') AS table_comment
		FROM information_schema.tables t
		WHERE t.table_schema = $1 AND t.table_type = 'BASE TABLE' AND t.table_name = $2
	`

	var schema, name string
	var comment sql.NullString
	if err := conn.QueryRowContext(ctx, query, schemaName, tableName).Scan(&schema, &name, &comment); err != nil {
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
		Schema:  schema,
		Name:    name,
		Comment: comment.String,
	}, nil
}

func (d *Driver) getColumns(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]db.Column, *errors.XError) {
	const query = `
		SELECT
			c.column_name,
			CASE
				WHEN c.data_type = 'USER-DEFINED' THEN c.udt_name
				WHEN c.character_maximum_length IS NOT NULL THEN
					c.data_type || '(' || c.character_maximum_length || ')'
				WHEN c.numeric_precision IS NOT NULL AND c.numeric_scale IS NOT NULL THEN
					c.data_type || '(' || c.numeric_precision || ',' || c.numeric_scale || ')'
				WHEN c.numeric_precision IS NOT NULL THEN
					c.data_type || '(' || c.numeric_precision || ')'
				ELSE c.data_type
			END AS column_type,
			c.is_nullable,
			c.column_default,
			col_description((quote_ident(c.table_schema) || '.' || quote_ident(c.table_name))::regclass, c.ordinal_position) AS column_comment,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END AS is_primary
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT kcu.table_schema, kcu.table_name, kcu.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
				ON tc.constraint_name = kcu.constraint_name
				AND tc.table_schema = kcu.table_schema
			WHERE tc.constraint_type = 'PRIMARY KEY'
		) pk ON c.table_schema = pk.table_schema
			AND c.table_name = pk.table_name
			AND c.column_name = pk.column_name
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position
	`

	rows, err := conn.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get columns", nil, err)
	}
	defer rows.Close()

	var columns []db.Column
	for rows.Next() {
		var name, colType, nullable string
		var defaultValue, comment sql.NullString
		var isPrimary bool
		if err := rows.Scan(&name, &colType, &nullable, &defaultValue, &comment, &isPrimary); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan column row", nil, err)
		}

		col := db.Column{
			Name:       name,
			Type:       colType,
			Nullable:   nullable == "YES",
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
			i.relname AS index_name,
			a.attname AS column_name,
			NOT ix.indisunique AS is_non_unique,
			ix.indisprimary AS is_primary,
			array_position(ix.indkey, a.attnum) AS column_position
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON t.relnamespace = n.oid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE n.nspname = $1 AND t.relname = $2
		ORDER BY i.relname, array_position(ix.indkey, a.attnum)
	`

	rows, err := conn.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get indexes", nil, err)
	}
	defer rows.Close()

	indexMap := make(map[string]*db.Index)
	for rows.Next() {
		var indexName, columnName string
		var isNonUnique, isPrimary bool
		var columnPosition int
		if err := rows.Scan(&indexName, &columnName, &isNonUnique, &isPrimary, &columnPosition); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan index row", nil, err)
		}

		if idx, exists := indexMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			indexMap[indexName] = &db.Index{
				Name:    indexName,
				Columns: []string{columnName},
				Unique:  !isNonUnique,
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
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS referenced_table,
			ccu.column_name AS referenced_column,
			kcu.ordinal_position
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name
			AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1
			AND tc.table_name = $2
		ORDER BY tc.constraint_name, kcu.ordinal_position
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
