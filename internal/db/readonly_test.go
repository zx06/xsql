package db

import (
	"testing"

	"github.com/zx06/xsql/internal/errors"
)

func TestIsReadOnlySQL(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"select 1", true},
		{"  -- c\nSELECT 1", true},
		{"/*c*/select 1", true},
		{"with t as (select 1) select * from t", true},
		{"with t as (select 1) insert into x values (1)", false},
		{"explain select 1", true},
		{"show databases", true},
		{"describe t", true},
		{"insert into t values (1)", false},
		{"update t set a=1", false},
		{"create table t(a int)", false},
		{"select 1; delete from t", false},
		{"", false},
		{"   ", false},
		{"(select 1)", false},
	}
	for _, tc := range cases {
		got, _ := IsReadOnlySQL(tc.sql)
		if got != tc.want {
			t.Fatalf("sql=%q got=%v want=%v", tc.sql, got, tc.want)
		}
	}
}

func TestEnforceReadOnly(t *testing.T) {
	if xe := EnforceReadOnly("select 1", false); xe != nil {
		t.Fatalf("unexpected: %v", xe)
	}
	xe := EnforceReadOnly("insert into t values (1)", false)
	if xe == nil || xe.Code != errors.CodeROBlocked {
		t.Fatalf("expected RO blocked error")
	}
	if xe := EnforceReadOnly("insert into t values (1)", true); xe != nil {
		t.Fatalf("expected nil in unsafe mode")
	}
}
