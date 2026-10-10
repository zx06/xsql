//go:build e2e

package e2e

import (
	"encoding/json"
	"testing"
)

type queryData struct {
	Columns   []string         `json:"columns"`
	Rows      []map[string]any `json:"rows"`
	Truncated bool             `json:"truncated"`
	RowCount  int              `json:"row_count"`
	MaxRows   int              `json:"max_rows"`
	Hint      string           `json:"hint"`
}

func TestQuery_MaxRows_DefaultTruncation_MySQL(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)
	sql := `WITH RECURSIVE seq AS (SELECT 1 AS n UNION ALL SELECT n+1 FROM seq WHERE n < 30) SELECT n FROM seq`
	stdout, _, exitCode := runXSQL(t, "query", sql, "--config", config, "--profile", "test", "--format", "json", "--max-rows", "5")
	if exitCode != 0 {
		t.Fatalf("exit %d stdout=%s", exitCode, stdout)
	}
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatal(err)
	}
	data, err := decodeQueryData(resp.Data)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 5 || !data.Truncated || data.MaxRows != 5 {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestQuery_MaxRows_Unlimited_MySQL(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)
	sql := `WITH RECURSIVE seq AS (SELECT 1 AS n UNION ALL SELECT n+1 FROM seq WHERE n < 4) SELECT n FROM seq`
	stdout, _, exitCode := runXSQL(t, "query", sql, "--config", config, "--profile", "test", "--format", "json", "--max-rows", "0")
	if exitCode != 0 {
		t.Fatalf("exit %d stdout=%s", exitCode, stdout)
	}
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatal(err)
	}
	data, err := decodeQueryData(resp.Data)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 4 || data.Truncated {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestQuery_MaxRows_PG(t *testing.T) {
	dsn := pgDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: pg
    dsn: "`+dsn+`"
`)
	stdout, _, exitCode := runXSQL(t, "query", "SELECT generate_series(1,20) AS n",
		"--config", config, "--profile", "test", "--format", "json", "--max-rows", "3")
	if exitCode != 0 {
		t.Fatalf("exit %d stdout=%s", exitCode, stdout)
	}
	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatal(err)
	}
	data, err := decodeQueryData(resp.Data)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 3 || !data.Truncated {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func decodeQueryData(raw any) (queryData, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return queryData{}, err
	}
	var data queryData
	err = json.Unmarshal(b, &data)
	return data, err
}
