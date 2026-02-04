package db

import (
	"testing"

	"github.com/zx06/xsql/internal/errors"
)

// TestIsReadOnlySQL_BasicCases 基础测试用例
func TestIsReadOnlySQL_BasicCases(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		// 基础 SELECT
		{"select 1", true},
		{"SELECT * FROM users", true},
		{"select count(*) from orders", true},
		{"SELECT DISTINCT name FROM products", true},

		// 注释处理
		{"  -- c\nSELECT 1", true},
		{"/*c*/select 1", true},
		{"/* multi\nline */ select 1", true},
		{"-- line1\n-- line2\nselect 1", true},

		// CTE (Common Table Expression)
		{"with t as (select 1) select * from t", true},
		{"WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a, b", true},
		{"with recursive t as (select 1 union all select n+1 from t where n < 5) select * from t", true},

		// 其他只读语句
		{"explain select 1", true},
		{"show databases", true},
		{"describe t", true},
		{"DESC users", true},
		{"TABLE users", true}, // MySQL 8.0+
		{"VALUES (1, 2), (3, 4)", true},

		// 字符串中包含"危险"单词但不应拦截
		{"SELECT 'delete from users' AS warning_msg", true},
		{"SELECT * FROM logs WHERE msg = 'INSERT failed'", true},
		{"SELECT * FROM products WHERE name LIKE '%update%'", true},
	}

	for _, tc := range cases {
		got, reason := IsReadOnlySQL(tc.sql)
		if got != tc.want {
			t.Errorf("sql=%q got=%v want=%v reason=%s", tc.sql, got, tc.want, reason)
		}
	}
}

// TestIsReadOnlySQL_WriteOperations 写入操作拦截测试
func TestIsReadOnlySQL_WriteOperations(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		// DML
		{"insert into t values (1)", false},
		{"INSERT INTO users (name) VALUES ('test')", false},
		{"update t set a=1", false},
		{"UPDATE users SET status = 'active' WHERE id = 1", false},
		{"delete from t", false},
		{"DELETE FROM users WHERE created_at < '2020-01-01'", false},
		{"MERGE INTO target USING source ON target.id = source.id", false},

		// DDL
		{"create table t(a int)", false},
		{"CREATE INDEX idx_name ON users(name)", false},
		{"alter table t add column b int", false},
		{"drop table t", false},
		{"TRUNCATE TABLE logs", false},

		// 其他危险操作
		{"grant select on users to readonly", false},
		{"revoke all privileges from user", false},
		{"call cleanup_procedure()", false},
		{"DO SLEEP(5)", false},
		{"COPY users TO '/tmp/users.csv'", false},

		// 存储过程和预处理
		{"EXECUTE stmt", false},
		{"PREPARE stmt FROM 'SELECT 1'", false},
		{"DEALLOCATE PREPARE stmt", false},

		// SELECT locking clauses
		{"SELECT * FROM users FOR SHARE", false},
		{"SELECT * FROM users FOR KEY SHARE", false},
	}

	for _, tc := range cases {
		got, reason := IsReadOnlySQL(tc.sql)
		if got != tc.want {
			t.Errorf("sql=%q got=%v want=%v reason=%s", tc.sql, got, tc.want, reason)
		}
	}
}

// TestIsReadOnlySQL_TransactionAndSessionControl 事务和会话控制拦截
func TestIsReadOnlySQL_TransactionAndSessionControl(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		// 事务控制 - 必须拦截以防止解除只读
		{"BEGIN", false},
		{"COMMIT", false},
		{"ROLLBACK", false},
		{"SAVEPOINT sp1", false},
		{"RELEASE SAVEPOINT sp1", false},

		// SET 语句 - 可能解除只读
		{"SET TRANSACTION READ WRITE", false},
		{"SET default_transaction_read_only = off", false},
		{"SET autocommit = 1", false},

		// 锁定
		{"LOCK TABLE users IN SHARE MODE", false},
		{"UNLOCK TABLES", false},

		// LOAD DATA
		{"LOAD DATA INFILE '/tmp/data.csv' INTO TABLE users", false},

		// REPLACE (MySQL)
		{"REPLACE INTO users (id, name) VALUES (1, 'test')", false},
	}

	for _, tc := range cases {
		got, reason := IsReadOnlySQL(tc.sql)
		if got != tc.want {
			t.Errorf("sql=%q got=%v want=%v reason=%s", tc.sql, got, tc.want, reason)
		}
	}
}

// TestIsReadOnlySQL_StringBypass 字符串伪装测试（确保不会误判也不会漏判）
func TestIsReadOnlySQL_StringBypass(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
		desc string
	}{
		// 字符串中包含危险词，但只是 SELECT，应该允许
		{"SELECT 'delete' FROM dual", true, "quoted DELETE should be allowed in SELECT"},
		{"SELECT * FROM t WHERE name = 'insert test'", true, "INSERT in string is OK"},
		{"SELECT \"update\" as col", true, "double quoted UPDATE"},

		// MySQL 反引号标识符
		{"SELECT * FROM `delete`", true, "backtick identifier named delete"},
		{"SELECT `insert` FROM t", true, "backtick column named insert"},

		// PostgreSQL 美元引号
		{"SELECT $tag$delete from users$tag$", true, "dollar quoted string"},

		// 真正的写入语句应该被拦截，即使包含字符串
		{"DELETE FROM t WHERE name = 'keep'", false, "real DELETE should be blocked"},
		{"UPDATE t SET col = 'insert' WHERE id = 1", false, "real UPDATE should be blocked"},
	}

	for _, tc := range cases {
		got, reason := IsReadOnlySQL(tc.sql)
		if got != tc.want {
			t.Errorf("%s: sql=%q got=%v want=%v reason=%s", tc.desc, tc.sql, got, tc.want, reason)
		}
	}
}

// TestIsReadOnlySQL_MultipleStatements 多语句检测
func TestIsReadOnlySQL_MultipleStatements(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		// 多语句应该被拒绝
		{"select 1; select 2", false},
		{"SELECT * FROM users; DELETE FROM logs", false},
		{"INSERT INTO t VALUES (1); SELECT 1", false},

		// 单个语句（即使有分号在末尾）
		{"select 1;", true},
		{"SELECT * FROM users;", true},

		// 注释中的分号不应被误认为多语句
		{"SELECT 1 /* ; comment */ FROM t", true},
		{"-- select 2;\nSELECT 1", true},
	}

	for _, tc := range cases {
		got, reason := IsReadOnlySQL(tc.sql)
		if got != tc.want {
			t.Errorf("sql=%q got=%v want=%v reason=%s", tc.sql, got, tc.want, reason)
		}
	}
}

// TestIsReadOnlySQL_CTEWrites CTE 写入检测
func TestIsReadOnlySQL_CTEWrites(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
		desc string
	}{
		// 纯读取 CTE
		{"WITH t AS (SELECT 1) SELECT * FROM t", true, "simple read CTE"},
		{"WITH a AS (SELECT * FROM t1), b AS (SELECT * FROM t2) SELECT * FROM a, b", true, "multiple read CTEs"},

		// Data-modifying CTE (PostgreSQL)
		{"WITH d AS (DELETE FROM t RETURNING id) SELECT * FROM d", false, "DELETE in CTE"},
		{"WITH u AS (UPDATE t SET col = 1 RETURNING *) SELECT * FROM u", false, "UPDATE in CTE"},
		{"WITH i AS (INSERT INTO t VALUES (1) RETURNING id) SELECT * FROM i", false, "INSERT in CTE"},
	}

	for _, tc := range cases {
		got, reason := IsReadOnlySQL(tc.sql)
		if got != tc.want {
			t.Errorf("%s: sql=%q got=%v want=%v reason=%s", tc.desc, tc.sql, got, tc.want, reason)
		}
	}
}

// TestIsReadOnlySQL_EdgeCases 边界情况
func TestIsReadOnlySQL_EdgeCases(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		// 空和空白
		{"", false},
		{"   ", false},
		{"\n\t  ", false},

		// 只有注释
		{"-- just a comment", false},
		{"/* comment only */", false},

		// 以括号开头（子查询）应被拒绝
		{"(select 1)", false},
		{"SELECT * FROM (SELECT 1) t", true}, // 但整体是 SELECT
	}

	for _, tc := range cases {
		got, reason := IsReadOnlySQL(tc.sql)
		if got != tc.want {
			t.Errorf("sql=%q got=%v want=%v reason=%s", tc.sql, got, tc.want, reason)
		}
	}
}

// TestEnforceReadOnly EnforceReadOnly 测试
func TestEnforceReadOnly(t *testing.T) {
	// 安全模式
	if xe := EnforceReadOnly("select 1", false); xe != nil {
		t.Errorf("unexpected error for SELECT: %v", xe)
	}

	xe := EnforceReadOnly("insert into t values (1)", false)
	if xe == nil {
		t.Fatal("expected error for INSERT")
	}
	if xe.Code != errors.CodeROBlocked {
		t.Errorf("expected CodeROBlocked, got %v", xe.Code)
	}

	// 不安全模式
	if xe := EnforceReadOnly("insert into t values (1)", true); xe != nil {
		t.Errorf("expected no error in unsafe mode: %v", xe)
	}

	// 错误详情应包含原因
	xe = EnforceReadOnly("DELETE FROM users", false)
	if xe == nil || xe.Details == nil {
		t.Fatal("expected error details")
	}
	reason, ok := xe.Details["reason"]
	if !ok {
		t.Error("expected 'reason' in error details")
	}
	t.Logf("DELETE blocked with reason: %v", reason)
}

// TestTokenize 词法分析器测试
func TestTokenize(t *testing.T) {
	tests := []struct {
		sql       string
		wantTypes []TokenType
		wantVals  []string
	}{
		{
			sql:       "SELECT 1",
			wantTypes: []TokenType{TokenKeyword, TokenNumber, TokenEOF},
			wantVals:  []string{"SELECT", "1", ""},
		},
		{
			sql:       "SELECT 'hello'",
			wantTypes: []TokenType{TokenKeyword, TokenString, TokenEOF},
			wantVals:  []string{"SELECT", "hello", ""},
		},
		{
			sql:       "SELECT * FROM t;",
			wantTypes: []TokenType{TokenKeyword, TokenOperator, TokenKeyword, TokenIdentifier, TokenSemicolon, TokenEOF},
			wantVals:  []string{"SELECT", "*", "FROM", "t", ";", ""},
		},
	}

	for _, tc := range tests {
		tokens, err := tokenize(tc.sql)
		if err != nil {
			t.Errorf("tokenize(%q) error: %v", tc.sql, err)
			continue
		}

		if len(tokens) != len(tc.wantTypes) {
			t.Errorf("tokenize(%q) got %d tokens, want %d", tc.sql, len(tokens), len(tc.wantTypes))
			continue
		}

		for i, tok := range tokens {
			if tok.Type != tc.wantTypes[i] {
				t.Errorf("tokenize(%q) token[%d] type=%v, want %v", tc.sql, i, tok.Type, tc.wantTypes[i])
			}
			if tok.Value != tc.wantVals[i] {
				t.Errorf("tokenize(%q) token[%d] value=%q, want %q", tc.sql, i, tok.Value, tc.wantVals[i])
			}
		}
	}
}

// BenchmarkIsReadOnlySQL 性能基准测试
func BenchmarkIsReadOnlySQL(b *testing.B) {
	sql := "SELECT u.id, u.name, u.email FROM users u JOIN orders o ON u.id = o.user_id WHERE u.status = 'active' AND o.created_at > '2024-01-01' ORDER BY o.created_at DESC LIMIT 100"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsReadOnlySQL(sql)
	}
}

// BenchmarkIsReadOnlySQL_ComplexCTE 复杂 CTE 基准测试
func BenchmarkIsReadOnlySQL_ComplexCTE(b *testing.B) {
	sql := `WITH 
		recent_orders AS (SELECT * FROM orders WHERE created_at > '2024-01-01'),
		order_stats AS (SELECT user_id, COUNT(*) as cnt, SUM(amount) as total FROM recent_orders GROUP BY user_id)
		SELECT u.name, s.cnt, s.total 
		FROM users u 
		JOIN order_stats s ON u.id = s.user_id 
		WHERE s.total > 1000`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsReadOnlySQL(sql)
	}
}

// ============================================================================
// Lexer Helper Function Tests (for coverage)
// ============================================================================

func TestStripLeadingCommentsAndSpace(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"SELECT 1", "SELECT 1"},
		{"   SELECT 1", "SELECT 1"},
		{"\n\t  SELECT 1", "SELECT 1"},
		{"-- comment\nSELECT 1", "SELECT 1"},
		{"/* comment */ SELECT 1", "SELECT 1"},
		{"/* multi\nline */ SELECT 1", "SELECT 1"},
		{"-- line1\n-- line2\nSELECT 1", "SELECT 1"},
		{"\ufeffSELECT 1", "SELECT 1"}, // BOM
		{"   \ufeffSELECT 1", "SELECT 1"},
		{"-- only comment", ""},
		{"/* only comment */", ""},
		{"", ""},
	}

	for _, tc := range cases {
		got := stripLeadingCommentsAndSpace(tc.input)
		if got != tc.expected {
			t.Errorf("stripLeadingCommentsAndSpace(%q)=%q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestParseDollarQuotedString(t *testing.T) {
	cases := []struct {
		input   string
		wantOK  bool
		wantVal string
	}{
		{"$tag$content$tag$", true, "content"},
		{"$$empty$$", true, "empty"},
		{"$tag$hello world$tag$", true, "hello world"},
		{"$tag$line1\nline2$tag$", true, "line1\nline2"},
		{"$1$value$1", false, ""},
		{"$tag$nested $$tag$content$$tag$ more$tag$", true, "nested $"},
		{"not a dollar string", false, ""},
		{"$ incomplete", false, ""},
		{"$tag$unclosed", false, ""},
	}

	for _, tc := range cases {
		got, idx, ok := parseDollarQuotedString(tc.input, 0)
		if ok != tc.wantOK {
			t.Errorf("parseDollarQuotedString(%q) ok=%v, want %v", tc.input, ok, tc.wantOK)
			continue
		}
		if ok && got != tc.wantVal {
			t.Errorf("parseDollarQuotedString(%q)=%q, want %q", tc.input, got, tc.wantVal)
		}
		_ = idx
	}
}

func TestHasWriteInCTE(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"WITH t AS (DELETE FROM users) SELECT * FROM t", true},
		{"WITH t AS (UPDATE users SET x=1) SELECT * FROM t", true},
		{"WITH t AS (INSERT INTO users VALUES (1)) SELECT * FROM t", true},
		{"WITH t AS (SELECT * FROM users) SELECT * FROM t", false},
		{"WITH t AS (SELECT id FROM users WHERE active) SELECT count(*) FROM t", false},
		{"WITH t AS (SELECT * FROM users) DELETE FROM logs", true},
		{"WITH a AS (SELECT * FROM t1), b AS (DELETE FROM t2) SELECT * FROM a, b", true},
		{"WITH t AS (BEGIN; DELETE FROM users; COMMIT;) SELECT * FROM t", true},
	}

	for _, tc := range cases {
		tokens, err := tokenize(tc.sql)
		if err != nil {
			t.Errorf("tokenize(%q) error: %v", tc.sql, err)
			continue
		}
		got := hasWriteInCTE(tokens)
		if got != tc.want {
			t.Errorf("hasWriteInCTE(%q)=%v, want %v", tc.sql, got, tc.want)
		}
	}
}

func TestIsKeyword(t *testing.T) {
	keywords := []string{
		"SELECT", "FROM", "WHERE", "INSERT", "UPDATE", "DELETE",
		"CREATE", "DROP", "BEGIN", "COMMIT", "WITH",
	}

	for _, kw := range keywords {
		if !isKeyword(kw) {
			t.Errorf("isKeyword(%q) should be true", kw)
		}
	}

	nonKeywords := []string{
		"not_a_keyword", "my_function", "user_table",
	}

	for _, kw := range nonKeywords {
		if isKeyword(kw) {
			t.Errorf("isKeyword(%q) should be false", kw)
		}
	}
}

func TestIsOperatorChar(t *testing.T) {
	ops := []rune{'+', '-', '*', '/', '=', '<', '>', '!', '~', '|', '&', '%', '^', '@', '#', '?', ':'}
	for _, r := range ops {
		if !isOperatorChar(r) {
			t.Errorf("isOperatorChar(%q) should be true", r)
		}
	}

	nonOps := []rune{'a', '1', '(', ')', ' ', '\n'}
	for _, r := range nonOps {
		if isOperatorChar(r) {
			t.Errorf("isOperatorChar(%q) should be false", r)
		}
	}
}

func TestHasMultipleValidStatements(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"SELECT 1", false},
		{"SELECT 1;", false},
		{"SELECT 1; SELECT 2", true},
		{"SELECT 1;   SELECT 2;  SELECT 3", true},
		{"SELECT 1; -- comment\nSELECT 2", true},
		{"SELECT /* comment; */ 1; SELECT 2", true},
		{"SELECT 1; INSERT INTO t VALUES (1)", true},
	}

	for _, tc := range cases {
		tokens, err := tokenize(tc.sql)
		if err != nil {
			t.Errorf("tokenize(%q) error: %v", tc.sql, err)
			continue
		}
		got := hasMultipleValidStatements(tokens)
		if got != tc.want {
			t.Errorf("hasMultipleValidStatements(%q)=%v, want %v", tc.sql, got, tc.want)
		}
	}
}
