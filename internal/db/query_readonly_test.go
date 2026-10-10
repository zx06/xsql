package db

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	xsqlerrors "github.com/zx06/xsql/internal/errors"
)

func TestNormalizeDBType(t *testing.T) {
	cases := map[string]string{
		"mysql":        "mysql",
		"MYSQL":        "mysql",
		"mariadb":      "mysql",
		"pg":           "pg",
		"postgres":     "pg",
		"postgresql":   "pg",
		"  PG  ":       "pg",
		"custom":       "custom",
	}
	for in, want := range cases {
		if got := normalizeDBType(in); got != want {
			t.Errorf("normalizeDBType(%q)=%q want %q", in, got, want)
		}
	}
}

func TestQuery_StaticLayerBlocksWithoutDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, xe := Query(context.Background(), db, "INSERT INTO t VALUES (1)", QueryOptions{DBType: "mysql"})
	if xe == nil || xe.Code != xsqlerrors.CodeROBlocked {
		t.Fatalf("expected static read-only block, got %v", xe)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected DB calls: %v", err)
	}
}

func TestBeginReadOnlyTx_MySQLNoPgProbe(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()

	tx, xe := beginReadOnlyTx(context.Background(), db, "mysql")
	if xe != nil {
		t.Fatalf("beginReadOnlyTx: %v", xe)
	}
	if tx == nil {
		t.Fatal("expected transaction")
	}
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBeginReadOnlyTx_PostgreSQLChecksReadOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SHOW transaction_read_only").
		WillReturnRows(sqlmock.NewRows([]string{"transaction_read_only"}).AddRow("on"))

	tx, xe := beginReadOnlyTx(context.Background(), db, "postgresql")
	if xe != nil {
		t.Fatalf("beginReadOnlyTx: %v", xe)
	}
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBeginReadOnlyTx_PostgreSQLRejectsWritableSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SHOW transaction_read_only").
		WillReturnRows(sqlmock.NewRows([]string{"transaction_read_only"}).AddRow("off"))
	mock.ExpectRollback()

	_, xe := beginReadOnlyTx(context.Background(), db, "pg")
	if xe == nil || xe.Code != xsqlerrors.CodeDBExecFailed {
		t.Fatalf("expected DB exec failure, got %v", xe)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBeginReadOnlyTx_BeginFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

	_, xe := beginReadOnlyTx(context.Background(), db, "mysql")
	if xe == nil || xe.Code != xsqlerrors.CodeDBExecFailed {
		t.Fatalf("expected begin failure, got %v", xe)
	}
}

func TestQuery_BypassStaticUsesReadOnlyTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"n"}).AddRow(1)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows)
	mock.ExpectRollback()

	result, xe := Query(context.Background(), db, "SELECT 1", QueryOptions{
		DBType:               "mysql",
		BypassStaticReadOnly: true,
	})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQuery_UnsafeAllowWriteSkipsReadOnlyTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"n"}).AddRow(42)
	mock.ExpectQuery("INSERT INTO t").WillReturnRows(rows)

	result, xe := Query(context.Background(), db, "INSERT INTO t VALUES (1)", QueryOptions{
		UnsafeAllowWrite: true,
	})
	if xe != nil {
		t.Fatalf("expected unsafe write to succeed: %v", xe)
	}
	if len(result.Rows) != 1 || result.Rows[0]["n"] != int64(42) {
		t.Fatalf("unexpected rows: %+v", result.Rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQuery_ReadOnlyPathUsesReadOnlyTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"n"}).AddRow(1)
	mock.ExpectQuery("SELECT 2").WillReturnRows(rows)
	mock.ExpectRollback()

	result, xe := Query(context.Background(), db, "SELECT 2", QueryOptions{DBType: "mysql"})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQueryWithReadOnlyTx_QueryFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT 1").WillReturnError(errors.New("query failed"))
	mock.ExpectRollback()

	_, xe := queryWithReadOnlyTx(context.Background(), db, "SELECT 1", "mysql")
	if xe == nil || xe.Code != xsqlerrors.CodeDBExecFailed {
		t.Fatalf("expected query failure, got %v", xe)
	}
}

func TestExecuteQuery_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"msg"}).AddRow([]byte("hi"))
	mock.ExpectQuery("SELECT 'hi'").WillReturnRows(rows)

	result, xe := executeQuery(context.Background(), db, "SELECT 'hi'")
	if xe != nil {
		t.Fatalf("executeQuery failed: %v", xe)
	}
	if result.Rows[0]["msg"] != "hi" {
		t.Fatalf("expected converted []byte, got %v", result.Rows[0]["msg"])
	}
}

func TestExecuteQuery_Failure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT 1").WillReturnError(errors.New("boom"))

	_, xe := executeQuery(context.Background(), db, "SELECT 1")
	if xe == nil || xe.Code != xsqlerrors.CodeDBExecFailed {
		t.Fatalf("expected failure, got %v", xe)
	}
}

func TestScanRows_MultipleRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT n").WillReturnRows(
		sqlmock.NewRows([]string{"n"}).AddRow(1).AddRow(2),
	)
	rows, err := db.Query("SELECT n")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	result, xe := scanRows(rows)
	if xe != nil {
		t.Fatalf("scanRows failed: %v", xe)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}
}

func TestBeginReadOnlyTx_PostgreSQLProbeErrorIgnored(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SHOW transaction_read_only").WillReturnError(errors.New("probe failed"))

	tx, xe := beginReadOnlyTx(context.Background(), db, "pg")
	if xe != nil {
		t.Fatalf("probe errors should not fail begin: %v", xe)
	}
	_ = tx.Rollback()
}

func TestHasMySQLUserVarAssignment(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"SELECT @v := 1", true},
		{"SELECT 1", false},
		{"SELECT @v FROM t", false},
	}
	for _, tc := range cases {
		tokens, err := tokenize(tc.sql)
		if err != nil {
			t.Fatal(err)
		}
		if got := hasMySQLUserVarAssignment(tokens); got != tc.want {
			t.Errorf("sql=%q got=%v want=%v", tc.sql, got, tc.want)
		}
	}
}
