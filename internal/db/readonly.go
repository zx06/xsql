package db

import (
	"strings"
	"unicode"

	"github.com/zx06/xsql/internal/errors"
)

// SQLToken represents a tokenized SQL element
type SQLToken struct {
	Type  TokenType
	Value string
}

// TokenType represents the type of SQL token
type TokenType int

const (
	TokenUnknown TokenType = iota
	TokenKeyword
	TokenIdentifier
	TokenString
	TokenNumber
	TokenComment
	TokenOperator
	TokenSemicolon
	TokenEOF
)

// 允许的首关键字（allowlist）
var allowedFirstKeywords = map[string]bool{
	"SELECT":   true,
	"SHOW":     true,
	"DESCRIBE": true,
	"DESC":     true,
	"EXPLAIN":  true,
	"WITH":     true,
	"TABLE":    true, // MySQL TABLE statement (8.0+)
	"VALUES":   true, // PostgreSQL VALUES
}

// 禁止的关键字（denylist）- 任何位置出现都拒绝
var forbiddenKeywords = map[string]bool{
	"INSERT":     true,
	"UPDATE":     true,
	"DELETE":     true,
	"CREATE":     true,
	"ALTER":      true,
	"DROP":       true,
	"TRUNCATE":   true,
	"GRANT":      true,
	"REVOKE":     true,
	"CALL":       true,
	"DO":         true,
	"COPY":       true,
	"MERGE":      true,
	"UPSERT":     true,
	"EXECUTE":    true,
	"PREPARE":    true,
	"DEALLOCATE": true,
	"SET":        true, // 可能解除只读: SET TRANSACTION READ WRITE
	"BEGIN":      true, // 用户可能 BEGIN READ WRITE
	"COMMIT":     true,
	"ROLLBACK":   true,
	"SAVEPOINT":  true,
	"RELEASE":    true,
	"LOCK":       true,
	"UNLOCK":     true,
	"LOAD":       true,
	"REPLACE":    true, // MySQL REPLACE
}

// IsReadOnlySQL 做保守判定：默认拒绝；仅允许文档明确的只读语句。
// 使用词法分析而非简单字符串匹配，正确处理字符串、注释。
// 解析失败/多语句/包含禁止关键字时一律返回 false。
func IsReadOnlySQL(sql string) (bool, string) {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return false, "empty"
	}

	// 保守：首字符若不是字母，则视为无法解析（例如 "(select 1)"）
	// 这是为了防止子查询绕过检测
	trimmed := stripLeadingCommentsAndSpace(sql)
	if trimmed == "" {
		return false, "empty_after_comment"
	}
	if !unicode.IsLetter(rune(trimmed[0])) {
		return false, "non_letter_start"
	}

	// 词法分析并检查
	tokens, err := tokenize(sql)
	if err != nil {
		return false, "tokenize_error: " + err.Error()
	}

	// 检查是否包含多语句（多个分号分隔的有效语句）
	if hasMultipleValidStatements(tokens) {
		return false, "multiple_statements"
	}

	// 找到第一个有效关键字（跳过注释）
	firstKeyword := ""
	for _, tok := range tokens {
		if tok.Type == TokenKeyword {
			firstKeyword = tok.Value
			break
		}
	}

	if firstKeyword == "" {
		return false, "no_keyword"
	}

	// 检查首关键字是否在 allowlist 中
	if !allowedFirstKeywords[firstKeyword] {
		return false, "forbidden_start: " + firstKeyword
	}

	// 检查是否包含任何禁止的关键字
	for _, tok := range tokens {
		if tok.Type == TokenKeyword && forbiddenKeywords[tok.Value] {
			// WITH 后面跟 INSERT/UPDATE/DELETE 的情况
			return false, "forbidden_keyword: " + tok.Value
		}
	}

	// 特殊处理 WITH (CTE) - 检查是否包含写入操作
	if firstKeyword == "WITH" {
		if hasWriteInCTE(tokens) {
			return false, "cte_write_operation"
		}
	}

	return true, firstKeyword
}

// stripLeadingCommentsAndSpace 去除前导注释和空白
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

// tokenize 执行 SQL 词法分析，正确处理字符串、注释、标识符
func tokenize(sql string) ([]SQLToken, error) {
	var tokens []SQLToken
	i := 0
	sqlLen := len(sql)

	for i < sqlLen {
		r := rune(sql[i])

		// 跳过空白字符
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// BOM 头
		if r == '\ufeff' {
			i++
			continue
		}

		// 行注释 --
		if r == '-' && i+1 < sqlLen && sql[i+1] == '-' {
			// 跳过到行尾
			for i < sqlLen && sql[i] != '\n' {
				i++
			}
			continue
		}

		// 块注释 /* */
		if r == '/' && i+1 < sqlLen && sql[i+1] == '*' {
			// 跳过到 */
			i += 2
			for i < sqlLen-1 {
				if sql[i] == '*' && sql[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		// 字符串 '...' (MySQL/PostgreSQL/PG 支持 escape)
		if r == '\'' {
			str, newIdx := parseString(sql, i, '\'')
			tokens = append(tokens, SQLToken{Type: TokenString, Value: str})
			i = newIdx
			continue
		}

		// 字符串 "..." (PostgreSQL/ANSI 标准)
		if r == '"' {
			str, newIdx := parseString(sql, i, '"')
			tokens = append(tokens, SQLToken{Type: TokenString, Value: str})
			i = newIdx
			continue
		}

		// 反引号 `...` (MySQL 标识符)
		if r == '`' {
			str, newIdx := parseString(sql, i, '`')
			tokens = append(tokens, SQLToken{Type: TokenIdentifier, Value: str})
			i = newIdx
			continue
		}

		// PostgreSQL 美元引号字符串 $tag$...$tag$
		if r == '$' {
			if str, newIdx, ok := parseDollarQuotedString(sql, i); ok {
				tokens = append(tokens, SQLToken{Type: TokenString, Value: str})
				i = newIdx
				continue
			}
		}

		// 分号
		if r == ';' {
			tokens = append(tokens, SQLToken{Type: TokenSemicolon, Value: ";"})
			i++
			continue
		}

		// 数字
		if unicode.IsDigit(r) || (r == '.' && i+1 < sqlLen && unicode.IsDigit(rune(sql[i+1]))) {
			start := i
			for i < sqlLen && (unicode.IsDigit(rune(sql[i])) || sql[i] == '.' || sql[i] == 'e' || sql[i] == 'E' || sql[i] == '+' || sql[i] == '-') {
				i++
			}
			tokens = append(tokens, SQLToken{Type: TokenNumber, Value: sql[start:i]})
			continue
		}

		// 关键字或标识符
		if unicode.IsLetter(r) || r == '_' {
			start := i
			for i < sqlLen && (unicode.IsLetter(rune(sql[i])) || unicode.IsDigit(rune(sql[i])) || sql[i] == '_' || sql[i] == '$') {
				i++
			}
			originalWord := sql[start:i]
			upperWord := strings.ToUpper(originalWord)
			// 检查是否为关键字（关键字存大写，标识符保留原样）
			if isKeyword(upperWord) {
				tokens = append(tokens, SQLToken{Type: TokenKeyword, Value: upperWord})
			} else {
				tokens = append(tokens, SQLToken{Type: TokenIdentifier, Value: originalWord})
			}
			continue
		}

		// 运算符和其他字符
		if isOperatorChar(r) {
			start := i
			for i < sqlLen && isOperatorChar(rune(sql[i])) {
				i++
			}
			tokens = append(tokens, SQLToken{Type: TokenOperator, Value: sql[start:i]})
			continue
		}

		// 其他字符（可能是不支持的字符）
		tokens = append(tokens, SQLToken{Type: TokenUnknown, Value: string(r)})
		i++
	}

	tokens = append(tokens, SQLToken{Type: TokenEOF, Value: ""})
	return tokens, nil
}

// parseString 解析引号包围的字符串，处理转义
func parseString(sql string, start int, quote rune) (string, int) {
	i := start + 1 // 跳过开引号
	sqlLen := len(sql)
	var result strings.Builder

	for i < sqlLen {
		r := rune(sql[i])
		if r == quote {
			// 检查是否是转义（'' 或 "" 在引号字符串中）
			if i+1 < sqlLen && rune(sql[i+1]) == quote {
				result.WriteRune(quote)
				i += 2
				continue
			}
			// 字符串结束
			return result.String(), i + 1
		}
		// 处理普通转义 \' 或 ''
		if r == '\\' && i+1 < sqlLen && rune(sql[i+1]) == quote {
			result.WriteRune(quote)
			i += 2
			continue
		}
		result.WriteRune(r)
		i++
	}

	// 未闭合的字符串
	return result.String(), i
}

// parseDollarQuotedString 解析 PostgreSQL 美元引号字符串 $tag$...$tag$
func parseDollarQuotedString(sql string, start int) (string, int, bool) {
	sqlLen := len(sql)
	if sql[start] != '$' {
		return "", start, false
	}

	// 解析标签名
	tagStart := start + 1
	tagEnd := tagStart
	for tagEnd < sqlLen && (unicode.IsLetter(rune(sql[tagEnd])) || unicode.IsDigit(rune(sql[tagEnd])) || sql[tagEnd] == '_') {
		tagEnd++
	}

	if tagEnd >= sqlLen || sql[tagEnd] != '$' {
		return "", start, false // 不是有效的美元引号
	}

	tag := sql[tagStart:tagEnd]
	closingTag := "$" + tag + "$"
	contentStart := tagEnd + 1

	// 查找结束标记
	for i := contentStart; i < sqlLen; i++ {
		if i+len(closingTag) <= sqlLen && sql[i:i+len(closingTag)] == closingTag {
			content := sql[contentStart:i]
			return content, i + len(closingTag), true
		}
	}

	return "", start, false // 未找到闭合标记
}

// isKeyword 判断是否为 SQL 关键字
func isKeyword(word string) bool {
	upper := strings.ToUpper(word)
	// 合并允许的和禁止的关键字
	if allowedFirstKeywords[upper] || forbiddenKeywords[upper] {
		return true
	}
	// 其他常见关键字
	commonKeywords := []string{
		"FROM", "WHERE", "AND", "OR", "NOT", "NULL", "IS", "IN", "EXISTS",
		"AS", "ON", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "CROSS",
		"GROUP", "BY", "ORDER", "HAVING", "LIMIT", "OFFSET", "UNION",
		"ALL", "DISTINCT", "CASE", "WHEN", "THEN", "ELSE", "END",
		"CAST", "CONVERT", "LIKE", "BETWEEN", "AND", "OR", "XOR",
		"IF", "ELSEIF", "ELSE", "WHILE", "FOR", "LOOP", "RETURN",
		"FUNCTION", "PROCEDURE", "TRIGGER", "VIEW", "INDEX", "SCHEMA",
		"DATABASE", "TABLESPACE", "SEQUENCE", "DOMAIN", "TYPE",
		"INTO", "VALUES", "DEFAULT", "PRIMARY", "KEY", "FOREIGN",
		"REFERENCES", "UNIQUE", "CHECK", "CONSTRAINT", "CASCADE",
		"RESTRICT", "COLLATE", "ESCAPE", "REGEXP", "RLIKE",
	}
	for _, kw := range commonKeywords {
		if upper == kw {
			return true
		}
	}
	return false
}

// isOperatorChar 判断是否为运算符字符
func isOperatorChar(r rune) bool {
	return r == '+' || r == '-' || r == '*' || r == '/' || r == '=' || r == '<' ||
		r == '>' || r == '!' || r == '~' || r == '|' || r == '&' || r == '%' ||
		r == '^' || r == '@' || r == '#' || r == '?' || r == ':'
}

// hasMultipleValidStatements 检查是否包含多个有效语句
func hasMultipleValidStatements(tokens []SQLToken) bool {
	stmtCount := 0
	hasNonTrivialContent := false

	for _, tok := range tokens {
		switch tok.Type {
		case TokenKeyword, TokenIdentifier, TokenString, TokenNumber:
			hasNonTrivialContent = true
		case TokenSemicolon:
			if hasNonTrivialContent {
				stmtCount++
				hasNonTrivialContent = false
			}
		}
	}

	// 最后一段（没有以分号结尾的部分）如果有内容也算一个语句
	if hasNonTrivialContent {
		stmtCount++
	}

	return stmtCount > 1
}

// hasWriteInCTE 检查 WITH 语句是否包含写入操作（data-modifying CTE）
func hasWriteInCTE(tokens []SQLToken) bool {
	// 简单的启发式检测：在 WITH 之后查找 INSERT/UPDATE/DELETE
	inCTE := false
	parenDepth := 0
	for i, tok := range tokens {
		if tok.Value == "WITH" {
			inCTE = true
			continue
		}
		if !inCTE {
			continue
		}

		// 跟踪括号深度以识别 CTE 边界
		switch tok.Value {
		case "(":
			parenDepth++
		case ")":
			parenDepth--
			// CTE 定义结束后的 SELECT 是正常的
			if parenDepth == 0 && i+1 < len(tokens) {
				nextTok := tokens[i+1]
				switch nextTok.Value {
				case "INSERT", "UPDATE", "DELETE":
					return true
				case "SELECT":
					return false
				}
			}
		default:
			// 在 CTE 体内检查写入关键字
			if parenDepth > 0 {
				switch tok.Value {
				case "INSERT", "UPDATE", "DELETE":
					return true
				}
			}
		}
	}
	return false
}
