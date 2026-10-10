//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
)

func TestAppQuery_TruncationHint_MySQL(t *testing.T) {
	dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("XSQL_TEST_MYSQL_DSN not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, xe := app.Query(ctx, app.QueryRequest{
		Profile: config.Profile{
			DB:  "mysql",
			DSN: dsn,
		},
		SQL: `WITH RECURSIVE seq AS (SELECT 1 AS n UNION ALL SELECT n+1 FROM seq WHERE n < 8) SELECT n FROM seq`,
		MaxRows:        3,
		TruncationHint: app.QueryTruncationHint,
	})
	if xe != nil {
		t.Fatalf("query failed: %v", xe)
	}
	if len(result.Rows) != 3 || !result.Truncated {
		t.Fatalf("expected truncated 3 rows, got %d truncated=%v", len(result.Rows), result.Truncated)
	}
	if result.Hint != app.QueryTruncationHint {
		t.Fatalf("hint=%q", result.Hint)
	}
}
