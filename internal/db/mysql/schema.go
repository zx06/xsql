package mysql

import (
	"context"
	"database/sql"
	"strings"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

// DumpSchema exports the MySQL database schema.
func (d *Driver) DumpSchema(ctx context.Context, conn *sql.DB, opts db.SchemaOptions) (*db.SchemaInfo, *errors.XError) {
	info := &db.SchemaInfo{}

	// Get the current database name
	var database string
	if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database); err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get database name", nil, err)
	}
	info.Database = database

	// Get table list
	tables, xe := d.listTables(ctx, conn, database, opts)
	if xe != nil {
		return nil, xe
	}

	// Get detailed information for each table
	for _, table := range tables {
		// Get column information
		columns, xe := d.getColumns(ctx, conn, database, table.Name)
		if xe != nil {
			return nil, xe
		}
		table.Columns = columns

		// Get index information
		indexes, xe := d.getIndexes(ctx, conn, database, table.Name)
		if xe != nil {
			return nil, xe
		}
		table.Indexes = indexes

		// Get foreign key information
		fks, xe := d.getForeignKeys(ctx, conn, database, table.Name)
		if xe != nil {
			return nil, xe
		}
		table.ForeignKeys = fks

		info.Tables = append(info.Tables, table)
	}

	return info, nil
}

// listTables retrieves the list of tables.
func (d *Driver) listTables(ctx context.Context, conn *sql.DB, database string, opts db.SchemaOptions) ([]db.Table, *errors.XError) {
	query := `
		SELECT table_name, table_comment
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE'
	`
	args := []any{database}

	// Table name filter
	if opts.TablePattern != "" {
		// Convert wildcards * and ? to SQL LIKE patterns
		likePattern := strings.ReplaceAll(opts.TablePattern, "*", "%")
		likePattern = strings.ReplaceAll(likePattern, "?", "_")
		query += " AND table_name LIKE ?"
		args = append(args, likePattern)
	}

	query += " ORDER BY table_name"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to list tables", nil, err)
	}
	defer rows.Close()

	var tables []db.Table
	for rows.Next() {
		var name, comment string
		if err := rows.Scan(&name, &comment); err != nil {
			return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to scan table row", nil, err)
		}
		tables = append(tables, db.Table{
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

// getColumns retrieves column information for a table.
func (d *Driver) getColumns(ctx context.Context, conn *sql.DB, database, tableName string) ([]db.Column, *errors.XError) {
	query := `
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

	rows, err := conn.QueryContext(ctx, query, database, tableName)
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

// getIndexes retrieves index information for a table.
func (d *Driver) getIndexes(ctx context.Context, conn *sql.DB, database, tableName string) ([]db.Index, *errors.XError) {
	query := `
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

	rows, err := conn.QueryContext(ctx, query, database, tableName)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get indexes", nil, err)
	}
	defer rows.Close()

	// Group by index_name
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

	// Convert to slice
	indexes := make([]db.Index, 0, len(indexMap))
	for _, idx := range indexMap {
		indexes = append(indexes, *idx)
	}

	return indexes, nil
}

// getForeignKeys retrieves foreign key information for a table.
func (d *Driver) getForeignKeys(ctx context.Context, conn *sql.DB, database, tableName string) ([]db.ForeignKey, *errors.XError) {
	query := `
		SELECT
			kcu.constraint_name,
			kcu.column_name,
			kcu.referenced_table_name,
			kcu.referenced_column_name,
			kcu.ordinal_position
		FROM information_schema.key_column_usage kcu
		WHERE kcu.table_schema = ?
		  AND kcu.table_name = ?
		  AND kcu.referenced_table_name IS NOT NULL
		ORDER BY kcu.constraint_name, kcu.ordinal_position
	`

	rows, err := conn.QueryContext(ctx, query, database, tableName)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBExecFailed, "failed to get foreign keys", nil, err)
	}
	defer rows.Close()

	// Group by constraint_name
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

	// Convert to slice
	fks := make([]db.ForeignKey, 0, len(fkMap))
	for _, fk := range fkMap {
		fks = append(fks, *fk)
	}

	return fks, nil
}
