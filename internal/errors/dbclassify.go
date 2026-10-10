package errors

import (
	stderrors "errors"
	"net"
	"strconv"
	"strings"
	"syscall"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBCategory is a stable, dialect-agnostic classification for database errors.
type DBCategory string

const (
	CategoryUnknownTable      DBCategory = "unknown_table"
	CategoryUnknownColumn     DBCategory = "unknown_column"
	CategorySyntaxError       DBCategory = "syntax_error"
	CategoryPermissionDenied  DBCategory = "permission_denied"
	CategoryTimeout           DBCategory = "timeout"
	CategoryConnection        DBCategory = "connection"
	CategoryReadOnlyViolation DBCategory = "read_only_violation"
	CategoryDuplicateKey      DBCategory = "duplicate_key"
	CategoryOther             DBCategory = "other"
)

// dbClassification holds parsed native DB error metadata and agent guidance.
type dbClassification struct {
	Category       DBCategory
	NativeCode     string
	NativeCodeKind string
	Hint           string
	Retryable      bool
}

func classifyDriverError(dialect string, err error) (dbClassification, bool) {
	if err == nil {
		return dbClassification{}, false
	}
	dialect = normalizeDialect(dialect)

	var myErr *mysql.MySQLError
	if stderrors.As(err, &myErr) {
		return classifyMySQLError(myErr), true
	}
	var pgErr *pgconn.PgError
	if stderrors.As(err, &pgErr) {
		return classifyPgError(pgErr), true
	}

	// Walk the chain for wrapped driver errors.
	for unwrapped := err; unwrapped != nil; unwrapped = stderrors.Unwrap(unwrapped) {
		if stderrors.As(unwrapped, &myErr) {
			return classifyMySQLError(myErr), true
		}
		if stderrors.As(unwrapped, &pgErr) {
			return classifyPgError(pgErr), true
		}
	}

	if dialect == "pg" {
		if c, ok := classifyPgMessage(err.Error()); ok {
			return c, true
		}
	}
	if dialect == "mysql" {
		if c, ok := classifyMySQLMessage(err.Error()); ok {
			return c, true
		}
	}
	return dbClassification{}, false
}

func classifyMySQLError(e *mysql.MySQLError) dbClassification {
	c := dbClassification{
		NativeCode:     intString(e.Number),
		NativeCodeKind: "mysql_errno",
	}
	switch e.Number {
	case 1146, 1049:
		c.Category = CategoryUnknownTable
		c.Hint = "Table or database not found; run `xsql schema dump` (or query information_schema) to list valid tables, then fix the SQL."
		c.Retryable = false
	case 1054, 1109:
		c.Category = CategoryUnknownColumn
		c.Hint = "Column not found; run `xsql schema dump --table <name>` to inspect columns, then fix the SQL."
		c.Retryable = false
	case 1064, 1149, 1060:
		c.Category = CategorySyntaxError
		c.Hint = "Fix SQL syntax; compare with the profile dialect (mysql) and re-run."
		c.Retryable = false
	case 1142, 1227, 1044, 1045:
		c.Category = CategoryPermissionDenied
		c.Hint = "Insufficient database privileges; use a read-only SELECT or ask for a profile with required grants."
		c.Retryable = false
	case 1205, 1213, 3024:
		c.Category = CategoryTimeout
		c.Hint = "Query timed out or lock wait exceeded; narrow the query, add LIMIT, or retry later."
		c.Retryable = true
	case 1792, 1836:
		c.Category = CategoryReadOnlyViolation
		c.Hint = "Write blocked in read-only mode; set profile `unsafe_allow_write: true` AND pass `--unsafe-allow-write` on query/ai/mcp server."
		c.Retryable = false
	case 1062:
		c.Category = CategoryDuplicateKey
		c.Hint = "Duplicate key conflict; adjust INSERT/UPDATE values or use ON DUPLICATE handling if writes are explicitly allowed."
		c.Retryable = false
	case 2002, 2003, 2006, 2013:
		c.Category = CategoryConnection
		c.Hint = "Cannot reach MySQL; verify profile host/port, network, and ssh_proxy settings."
		c.Retryable = true
	default:
		c.Category = CategoryOther
		c.Hint = "Database returned an error; inspect native_code and message, adjust SQL or connection settings."
		c.Retryable = false
	}
	return c
}

func classifyPgError(e *pgconn.PgError) dbClassification {
	c := dbClassification{
		NativeCode:     e.Code,
		NativeCodeKind: "pg_sqlstate",
	}
	switch e.Code {
	case "42P01", "3F000":
		c.Category = CategoryUnknownTable
		c.Hint = "Table or schema not found; run `xsql schema dump` to list tables, then fix the SQL."
		c.Retryable = false
	case "42703":
		c.Category = CategoryUnknownColumn
		c.Hint = "Column not found; run `xsql schema dump --table <name>` to inspect columns, then fix the SQL."
		c.Retryable = false
	case "42601", "42803", "42809":
		c.Category = CategorySyntaxError
		c.Hint = "Fix SQL syntax; compare with the profile dialect (pg) and re-run."
		c.Retryable = false
	case "42501":
		c.Category = CategoryPermissionDenied
		c.Hint = "Insufficient database privileges; use a read-only SELECT or ask for a profile with required grants."
		c.Retryable = false
	case "57014", "55P03":
		c.Category = CategoryTimeout
		c.Hint = "Query canceled or lock timeout; narrow the query, add LIMIT, or retry later."
		c.Retryable = true
	case "25006", "25007":
		c.Category = CategoryReadOnlyViolation
		c.Hint = "Write blocked in read-only transaction; set profile `unsafe_allow_write: true` AND pass `--unsafe-allow-write` on query/ai/mcp server."
		c.Retryable = false
	case "23505":
		c.Category = CategoryDuplicateKey
		c.Hint = "Duplicate key conflict; adjust values or constraints if writes are explicitly allowed."
		c.Retryable = false
	case "08000", "08001", "08003", "08006", "57P01", "53300":
		c.Category = CategoryConnection
		c.Hint = "Cannot reach PostgreSQL; verify profile host/port, sslmode, and ssh_proxy settings."
		c.Retryable = true
	case "40001", "40P01":
		c.Category = CategoryTimeout
		c.Hint = "Serialization or deadlock failure; retry the read query or simplify concurrent access."
		c.Retryable = true
	default:
		c.Category = CategoryOther
		c.Hint = "Database returned an error; inspect native_code and message, adjust SQL or connection settings."
		c.Retryable = false
	}
	return c
}

func classifyMySQLMessage(msg string) (dbClassification, bool) {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "doesn't exist"):
		return dbClassification{
			Category:       CategoryUnknownTable,
			Hint:           "Table or database not found; run `xsql schema dump` to list tables, then fix the SQL.",
			Retryable:      false,
			NativeCodeKind: "mysql_errno",
		}, true
	case strings.Contains(lower, "unknown column"):
		return dbClassification{
			Category:       CategoryUnknownColumn,
			Hint:           "Column not found; run `xsql schema dump --table <name>` to inspect columns, then fix the SQL.",
			Retryable:      false,
			NativeCodeKind: "mysql_errno",
		}, true
	case strings.Contains(lower, "syntax"):
		return dbClassification{
			Category:       CategorySyntaxError,
			Hint:           "Fix SQL syntax; compare with the profile dialect (mysql) and re-run.",
			Retryable:      false,
			NativeCodeKind: "mysql_errno",
		}, true
	default:
		return dbClassification{}, false
	}
}

func classifyPgMessage(msg string) (dbClassification, bool) {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "does not exist") && strings.Contains(lower, "relation"):
		return dbClassification{
			Category:       CategoryUnknownTable,
			Hint:           "Table or schema not found; run `xsql schema dump` to list tables, then fix the SQL.",
			Retryable:      false,
			NativeCodeKind: "pg_sqlstate",
		}, true
	case strings.Contains(lower, "column") && strings.Contains(lower, "does not exist"):
		return dbClassification{
			Category:       CategoryUnknownColumn,
			Hint:           "Column not found; run `xsql schema dump --table <name>` to inspect columns, then fix the SQL.",
			Retryable:      false,
			NativeCodeKind: "pg_sqlstate",
		}, true
	case strings.Contains(lower, "syntax error"):
		return dbClassification{
			Category:       CategorySyntaxError,
			Hint:           "Fix SQL syntax; compare with the profile dialect (pg) and re-run.",
			Retryable:      false,
			NativeCodeKind: "pg_sqlstate",
		}, true
	default:
		return dbClassification{}, false
	}
}

func classifyNetworkError(err error) (dbClassification, bool) {
	var netErr net.Error
	if stderrors.As(err, &netErr) {
		if netErr.Timeout() {
			return dbClassification{
				Category:  CategoryTimeout,
				Hint:      "Network timeout; verify host reachability, ssh_proxy, and firewall rules, then retry.",
				Retryable: true,
			}, true
		}
		return dbClassification{
			Category:  CategoryConnection,
			Hint:      "Network connection failed; verify profile host/port and ssh_proxy settings.",
			Retryable: true,
		}, true
	}
	var opErr *net.OpError
	if stderrors.As(err, &opErr) {
		return dbClassification{
			Category:  CategoryConnection,
			Hint:      "Network connection failed; verify profile host/port and ssh_proxy settings.",
			Retryable: true,
		}, true
	}
	if stderrors.Is(err, syscall.ECONNREFUSED) || stderrors.Is(err, syscall.ECONNRESET) {
		return dbClassification{
			Category:  CategoryConnection,
			Hint:      "Connection refused or reset; verify the database is listening and profile port is correct.",
			Retryable: true,
		}, true
	}
	return dbClassification{}, false
}

func normalizeDialect(d string) string {
	switch strings.ToLower(strings.TrimSpace(d)) {
	case "mysql", "mariadb":
		return "mysql"
	case "pg", "postgres", "postgresql":
		return "pg"
	default:
		return strings.ToLower(strings.TrimSpace(d))
	}
}

func intString(n uint16) string {
	return strconv.FormatUint(uint64(n), 10)
}
