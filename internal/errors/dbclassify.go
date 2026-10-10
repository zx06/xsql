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
	CategoryAuthFailed        DBCategory = "auth_failed"
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
		c.Hint = hintUnknownTableMySQL
		c.Retryable = false
	case 1054, 1109:
		c.Category = CategoryUnknownColumn
		c.Hint = hintUnknownColumn
		c.Retryable = false
	case 1064, 1149, 1060:
		c.Category = CategorySyntaxError
		c.Hint = hintSyntaxMySQL
		c.Retryable = false
	case 1045:
		c.Category = CategoryAuthFailed
		c.Hint = hintAuthFailed
		c.Retryable = false
	case 1142, 1227, 1044:
		c.Category = CategoryPermissionDenied
		c.Hint = hintPermissionDenied
		c.Retryable = false
	case 1205, 1213, 3024:
		c.Category = CategoryTimeout
		c.Hint = hintQueryTimeoutMySQL
		c.Retryable = true
	case 1792, 1836:
		c.Category = CategoryReadOnlyViolation
		c.Hint = hintReadOnlyWrite
		c.Retryable = false
	case 1062:
		c.Category = CategoryDuplicateKey
		c.Hint = hintDuplicateKey
		c.Retryable = false
	case 2002, 2003, 2006, 2013:
		c.Category = CategoryConnection
		c.Hint = hintMySQLConnection
		c.Retryable = true
	default:
		c.Category = CategoryOther
		c.Hint = hintDBOther
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
		c.Hint = hintUnknownTablePG
		c.Retryable = false
	case "42703":
		c.Category = CategoryUnknownColumn
		c.Hint = hintUnknownColumn
		c.Retryable = false
	case "42601", "42803", "42809":
		c.Category = CategorySyntaxError
		c.Hint = hintSyntaxPG
		c.Retryable = false
	case "28P01", "28000":
		c.Category = CategoryAuthFailed
		c.Hint = hintAuthFailed
		c.Retryable = false
	case "42501":
		c.Category = CategoryPermissionDenied
		c.Hint = hintPermissionDenied
		c.Retryable = false
	case "57014", "55P03":
		c.Category = CategoryTimeout
		c.Hint = hintQueryTimeoutPG
		c.Retryable = true
	case "25006", "25007":
		c.Category = CategoryReadOnlyViolation
		c.Hint = hintReadOnlyWritePG
		c.Retryable = false
	case "23505":
		c.Category = CategoryDuplicateKey
		c.Hint = hintDuplicateKeyPG
		c.Retryable = false
	case "08000", "08001", "08003", "08006", "57P01", "53300":
		c.Category = CategoryConnection
		c.Hint = hintPGConnection
		c.Retryable = true
	case "40001", "40P01":
		c.Category = CategoryTimeout
		c.Hint = hintSerializationPG
		c.Retryable = true
	default:
		c.Category = CategoryOther
		c.Hint = hintDBOther
		c.Retryable = false
	}
	return c
}

func classifyMySQLMessage(msg string) (dbClassification, bool) {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "access denied"):
		return dbClassification{
			Category:       CategoryAuthFailed,
			Hint:           hintAuthFailed,
			Retryable:      false,
			NativeCodeKind: "mysql_errno",
		}, true
	case strings.Contains(lower, "doesn't exist"):
		return dbClassification{
			Category:       CategoryUnknownTable,
			Hint:           hintUnknownTableMySQL,
			Retryable:      false,
			NativeCodeKind: "mysql_errno",
		}, true
	case strings.Contains(lower, "unknown column"):
		return dbClassification{
			Category:       CategoryUnknownColumn,
			Hint:           hintUnknownColumn,
			Retryable:      false,
			NativeCodeKind: "mysql_errno",
		}, true
	case strings.Contains(lower, "syntax"):
		return dbClassification{
			Category:       CategorySyntaxError,
			Hint:           hintSyntaxMySQL,
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
	case strings.Contains(lower, "password authentication failed"), strings.Contains(lower, "authentication failed"):
		return dbClassification{
			Category:       CategoryAuthFailed,
			Hint:           hintAuthFailed,
			Retryable:      false,
			NativeCodeKind: "pg_sqlstate",
		}, true
	case strings.Contains(lower, "does not exist") && strings.Contains(lower, "relation"):
		return dbClassification{
			Category:       CategoryUnknownTable,
			Hint:           hintUnknownTablePG,
			Retryable:      false,
			NativeCodeKind: "pg_sqlstate",
		}, true
	case strings.Contains(lower, "column") && strings.Contains(lower, "does not exist"):
		return dbClassification{
			Category:       CategoryUnknownColumn,
			Hint:           hintUnknownColumn,
			Retryable:      false,
			NativeCodeKind: "pg_sqlstate",
		}, true
	case strings.Contains(lower, "syntax error"):
		return dbClassification{
			Category:       CategorySyntaxError,
			Hint:           hintSyntaxPG,
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
