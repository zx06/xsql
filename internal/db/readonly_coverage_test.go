package db

import (
	"testing"
)

// ============================================================================
// stripLeadingCommentsAndSpace Tests
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

// ============================================================================
// parseDollarQuotedString Tests
// ============================================================================

func TestParseDollarQuotedString(t *testing.T) {
	cases := []struct {
		input   string
		wantOK  bool
		wantVal string
	}{
		{"$tag$content$tag$", true, "content"},
		{"$$empty$$", true, "empty"}, // The parser returns "empty" for $$
		{"$tag$hello world$tag$", true, "hello world"},
		{"$tag$line1\nline2$tag$", true, "line1\nline2"},
		{"$1$value$1", false, ""},                                       // parseDollarQuotedString only handles starting from position 0
		{"$tag$nested $$tag$content$$tag$ more$tag$", true, "nested $"}, // Partial match due to implementation
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
		_ = idx // Used internally
	}
}

// ============================================================================
// hasWriteInCTE Tests
// ============================================================================

func TestHasWriteInCTE(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		// Write in CTE body
		{"WITH t AS (DELETE FROM users) SELECT * FROM t", true},
		{"WITH t AS (UPDATE users SET x=1) SELECT * FROM t", true},
		{"WITH t AS (INSERT INTO users VALUES (1)) SELECT * FROM t", true},

		// Normal read CTE
		{"WITH t AS (SELECT * FROM users) SELECT * FROM t", false},
		{"WITH t AS (SELECT id FROM users WHERE active) SELECT count(*) FROM t", false},

		// Note: hasWriteInCTE checks the entire token stream for writes after WITH
		// The main query's first keyword is checked separately
		{"WITH t AS (SELECT * FROM users) DELETE FROM logs", true}, // Finds DELETE in tokens

		// Nested CTE
		{"WITH a AS (SELECT * FROM t1), b AS (DELETE FROM t2) SELECT * FROM a, b", true},

		// CTE with multiple statements
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

// ============================================================================
// isKeyword Tests
// ============================================================================

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

// ============================================================================
// isOperatorChar Tests
// ============================================================================

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

// ============================================================================
// hasMultipleValidStatements Tests
// ============================================================================

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
