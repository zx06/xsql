package db

import (
	"strings"
	"unicode"

	"github.com/zx06/xsql/internal/errors"
)

var writeKeywords = []string{
	"INSERT",
	"UPDATE",
	"DELETE",
	"CREATE",
	"ALTER",
	"DROP",
	"TRUNCATE",
	"GRANT",
	"REVOKE",
	"CALL",
	"DO",
	"COPY",
	"MERGE",
}

// IsReadOnlySQL 做保守判定：默认拒绝；仅允许文档明确的只读语句。
// 解析失败/多语句时一律返回 false。
func IsReadOnlySQL(sql string) (bool, string) {
	sql = stripLeadingCommentsAndSpace(sql)
	if sql == "" {
		return false, "empty"
	}
	if hasMultipleStatements(sql) {
		return false, "multiple_statements"
	}
	kw := firstKeyword(sql)
	switch kw {
	case "SELECT", "SHOW", "DESCRIBE", "DESC", "EXPLAIN":
		return true, kw
	case "WITH":
		// WITH 可能承载写入（例如 WITH ... INSERT/UPDATE ...）；保守起见，只要出现写关键词就拒绝。
		u := strings.ToUpper(sql)
		for _, w := range writeKeywords {
			if containsWord(u, w) {
				return false, "WITH_" + w
			}
		}
		return true, kw
	default:
		return false, kw
	}
}

func EnforceReadOnly(sql string, unsafeAllowWrite bool) *errors.XError {
	if unsafeAllowWrite {
		return nil
	}
	ok, reason := IsReadOnlySQL(sql)
	if ok {
		return nil
	}
	return errors.New(errors.CodeROBlocked, "write blocked by read-only policy", map[string]any{"reason": reason})
}

func hasMultipleStatements(sql string) bool {
	parts := strings.Split(sql, ";")
	nonEmpty := 0
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty++
		}
		if nonEmpty > 1 {
			return true
		}
	}
	return false
}

func firstKeyword(sql string) string {
	sql = strings.TrimLeftFunc(sql, unicode.IsSpace)
	if sql == "" {
		return ""
	}
	// 保守：首字符若不是字母，则视为无法解析（例如 "(select 1)"）。
	if !unicode.IsLetter(rune(sql[0])) {
		return ""
	}
	start := 0
	end := 0
	for end < len(sql) {
		r := rune(sql[end])
		if !unicode.IsLetter(r) {
			break
		}
		end++
	}
	if end <= start {
		return ""
	}
	return strings.ToUpper(sql[start:end])
}

func stripLeadingCommentsAndSpace(s string) string {
	for {
		s = strings.TrimLeftFunc(s, unicode.IsSpace)
		s = strings.TrimPrefix(s, "\ufeff")
		if strings.HasPrefix(s, "--") {
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
				continue
			}
			return ""
		}
		if strings.HasPrefix(s, "/*") {
			if i := strings.Index(s, "*/"); i >= 0 {
				s = s[i+2:]
				continue
			}
			return ""
		}
		return s
	}
}

func containsWord(s, word string) bool {
	// 简单边界匹配：避免把 "COPY" 命中在 "SCOPY" 里。
	idx := strings.Index(s, word)
	for idx >= 0 {
		leftOK := idx == 0 || !isIdentChar(rune(s[idx-1]))
		right := idx + len(word)
		rightOK := right >= len(s) || !isIdentChar(rune(s[right]))
		if leftOK && rightOK {
			return true
		}
		next := strings.Index(s[idx+1:], word)
		if next < 0 {
			return false
		}
		idx = idx + 1 + next
	}
	return false
}

func isIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
