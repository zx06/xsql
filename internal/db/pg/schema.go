package pg

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

// DumpSchema 导出 PostgreSQL 数据库结构
func (d *Driver) DumpSchema(ctx context.Context, conn *sql.DB, opts db.SchemaOptions) (*db.SchemaInfo, *errors.XError) {
	info := &db.SchemaInfo{}

	// 获取当前数据库名
	var database string
	if err := conn.QueryRowContext(ctx, "SELECT current_database()").Scan(&database); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get database name", nil, err)
	}
	info.Database = database

	// 获取 schema 列表（排除系统 schema）
	schemas, xe := d.listSchemas(ctx, conn, opts)
	if xe != nil {
		return nil, xe
	}

	// 获取每个 schema 下的表
	for _, schema := range schemas {
		tables, xe := d.listTables(ctx, conn, schema, opts)
		if xe != nil {
			return nil, xe
		}

		// 获取每个表的详细信息
		for _, table := range tables {
			// 获取列信息
			columns, xe := d.getColumns(ctx, conn, schema, table.Name)
			if xe != nil {
				return nil, xe
			}
			table.Columns = columns

			// 获取索引信息
			indexes, xe := d.getIndexes(ctx, conn, schema, table.Name)
			if xe != nil {
				return nil, xe
			}
			table.Indexes = indexes

			// 获取外键信息
			fks, xe := d.getForeignKeys(ctx, conn, schema, table.Name)
			if xe != nil {
				return nil, xe
			}
			table.ForeignKeys = fks

			info.Tables = append(info.Tables, table)
		}
	}

	return info, nil
}

// listSchemas 获取 schema 列表
func (d *Driver) listSchemas(ctx context.Context, conn *sql.DB, opts db.SchemaOptions) ([]string, *errors.XError) {
	query := `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
	`

	if !opts.IncludeSystem {
		// 排除更多系统 schema
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

// listTables 获取表列表
func (d *Driver) listTables(ctx context.Context, conn *sql.DB, schema string, opts db.SchemaOptions) ([]db.Table, *errors.XError) {
	query := `
		SELECT
			t.table_name,
			obj_description((quote_ident($1) || '.' || quote_ident(t.table_name))::regclass, 'pg_class') as table_comment
		FROM information_schema.tables t
		WHERE t.table_schema = $1 AND t.table_type = 'BASE TABLE'
	`
	args := []any{schema}

	// 表名过滤
	if opts.TablePattern != "" {
		// 将通配符 * 和 ? 转换为 SQL LIKE 模式
		likePattern := strings.ReplaceAll(opts.TablePattern, "*", "%")
		likePattern = strings.ReplaceAll(likePattern, "?", "_")
		query += " AND t.table_name LIKE $2"
		args = append(args, likePattern)
	}

	query += " ORDER BY t.table_name"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to list tables", nil, err)
	}
	defer rows.Close()

	var tables []db.Table
	for rows.Next() {
		var name string
		var comment sql.NullString
		if err := rows.Scan(&name, &comment); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan table row", nil, err)
		}
		tables = append(tables, db.Table{
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

// getColumns 获取表的列信息
func (d *Driver) getColumns(ctx context.Context, conn *sql.DB, schema, tableName string) ([]db.Column, *errors.XError) {
	query := `
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
			END as column_type,
			c.is_nullable,
			c.column_default,
			col_description((quote_ident(c.table_schema) || '.' || quote_ident(c.table_name))::regclass, c.ordinal_position) as column_comment,
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

	rows, err := conn.QueryContext(ctx, query, schema, tableName)
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

// getIndexes 获取表的索引信息
func (d *Driver) getIndexes(ctx context.Context, conn *sql.DB, schema, tableName string) ([]db.Index, *errors.XError) {
	query := `
		SELECT
			i.relname as index_name,
			a.attname as column_name,
			NOT ix.indisunique as is_non_unique,
			ix.indisprimary as is_primary,
			array_position(ix.indkey, a.attnum) as column_position
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON t.relnamespace = n.oid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE n.nspname = $1 AND t.relname = $2
		ORDER BY i.relname, array_position(ix.indkey, a.attnum)
	`

	rows, err := conn.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get indexes", nil, err)
	}
	defer rows.Close()

	// 按 index_name 分组
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

	// 转换为切片
	indexes := make([]db.Index, 0, len(indexMap))
	for _, idx := range indexMap {
		indexes = append(indexes, *idx)
	}

	return indexes, nil
}

// getForeignKeys 获取表的外键信息
func (d *Driver) getForeignKeys(ctx context.Context, conn *sql.DB, schema, tableName string) ([]db.ForeignKey, *errors.XError) {
	query := `
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

	rows, err := conn.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get foreign keys", nil, err)
	}
	defer rows.Close()

	// 按 constraint_name 分组
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

	// 转换为切片
	fks := make([]db.ForeignKey, 0, len(fkMap))
	for _, fk := range fkMap {
		fks = append(fks, *fk)
	}

	return fks, nil
}

// matchPattern 检查表名是否匹配通配符模式
func matchPattern(pattern, name string) bool {
	// 简单实现：使用 filepath.Match
	matched, _ := filepath.Match(pattern, name)
	return matched
}
