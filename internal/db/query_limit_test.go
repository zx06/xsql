package db

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestScanRows_MaxRowsTruncated(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()

	rows := sqlmock.NewRows([]string{"n"}).
		AddRow(1).
		AddRow(2).
		AddRow(3)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	sqlRows, err := mockDB.Query("SELECT n FROM t")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlRows.Close()

	result, xe := scanRows(sqlRows, 2)
	if xe != nil {
		t.Fatalf("scanRows: %v", xe)
	}
	if len(result.Rows) != 2 || !result.Truncated {
		t.Fatalf("expected 2 rows truncated, got %d truncated=%v", len(result.Rows), result.Truncated)
	}
	if result.RowCount != 2 || result.MaxRows != 2 {
		t.Fatalf("meta row_count=%d max_rows=%d", result.RowCount, result.MaxRows)
	}
}

func TestScanRows_MaxRowsExact(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()

	rows := sqlmock.NewRows([]string{"n"}).AddRow(1).AddRow(2)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	sqlRows, err := mockDB.Query("SELECT n FROM t")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlRows.Close()

	result, xe := scanRows(sqlRows, 2)
	if xe != nil {
		t.Fatalf("scanRows: %v", xe)
	}
	if len(result.Rows) != 2 || result.Truncated {
		t.Fatalf("expected 2 rows not truncated, got truncated=%v", result.Truncated)
	}
}

func TestScanRows_Unlimited(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()

	rows := sqlmock.NewRows([]string{"n"}).AddRow(1).AddRow(2).AddRow(3)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	sqlRows, err := mockDB.Query("SELECT n FROM t")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlRows.Close()

	result, xe := scanRows(sqlRows, 0)
	if xe != nil {
		t.Fatalf("scanRows: %v", xe)
	}
	if len(result.Rows) != 3 || result.Truncated {
		t.Fatalf("expected 3 rows, got %d truncated=%v", len(result.Rows), result.Truncated)
	}
}
