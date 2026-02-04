//go:build e2e

package e2e

import (
	"encoding/json"
	"testing"
)

func TestQuery_ReadOnlyBlocksForShare(t *testing.T) {
	dsn := mysqlDSN(t)
	config := createTempConfig(t, `profiles:
  test:
    db: mysql
    dsn: "`+dsn+`"
`)

	stdout, _, exitCode := runXSQL(t, "query", "SELECT 1 FOR SHARE",
		"--config", config, "--profile", "test", "--format", "json")

	if exitCode != 4 {
		t.Fatalf("expected exit code 4, got %d", exitCode)
	}

	var resp Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("stdout should contain error JSON: %v\nstdout: %s", err, stdout)
	}
	if resp.OK {
		t.Error("expected ok=false for read-only block")
	}
	if resp.Error == nil || resp.Error.Code != "XSQL_RO_BLOCKED" {
		t.Fatalf("expected XSQL_RO_BLOCKED, got %+v", resp.Error)
	}
}
