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

// Allowed leading keywords (allowlist)
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

// Denied keywords (denylist) - rejected if found anywhere
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
	"SET":        true, // may disable read-only: SET TRANSACTION READ WRITE
	"BEGIN":      true, // user may BEGIN READ WRITE
	"COMMIT":     true,
	"ROLLBACK":   true,
	"SAVEPOINT":  true,
	"RELEASE":    true,
	"LOCK":       true,
	"UNLOCK":     true,
	"LOAD":       true,
	"REPLACE":    true, // MySQL REPLACE
	// Side-effecting SELECT forms (MySQL / PostgreSQL)
	"INTO":     true, // SELECT INTO @var / OUTFILE / PG SELECT INTO table
	"OUTFILE":  true, // MySQL SELECT ... INTO OUTFILE
	"DUMPFILE": true, // MySQL SELECT ... INTO DUMPFILE
}

// IsReadOnlySQL performs a conservative check: deny by default; only allow
// explicitly documented read-only statements.
// Uses lexical analysis instead of simple string matching to correctly handle
// strings and comments. Returns false on parse failure, multiple statements,
// or presence of forbidden keywords.
func IsReadOnlySQL(sql string) (bool, string) {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return false, "empty"
	}

	trimmed := stripLeadingCommentsAndSpace(sql)
	if trimmed == "" {
		return false, "empty_after_comment"
	}

	// Allow a single outer parenthesized read-only statement, e.g. "(SELECT 1)".
	if trimmed[0] == '(' {
		inner, ok := unwrapOuterParenthesizedSQL(trimmed)
		if !ok {
			return false, "invalid_outer_paren"
		}
		return IsReadOnlySQL(inner)
	}

	// Conservative: if the first character is not a letter, treat as unparseable.
	if !unicode.IsLetter(rune(trimmed[0])) {
		return false, "non_letter_start"
	}

	// Tokenize and validate
	tokens, err := tokenize(sql)
	if err != nil {
		return false, "tokenize_error: " + err.Error()
	}

	// Check for multiple statements (multiple valid statements separated by semicolons)
	if hasMultipleValidStatements(tokens) {
		return false, "multiple_statements"
	}

	// Find the first valid keyword (skipping comments)
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

	// Check if the first keyword is in the allowlist
	if !allowedFirstKeywords[firstKeyword] {
		return false, "forbidden_start: " + firstKeyword
	}

	// Check for any forbidden keywords
	for _, tok := range tokens {
		if tok.Type == TokenKeyword && forbiddenKeywords[tok.Value] {
			// e.g. WITH ... INSERT/UPDATE/DELETE
			return false, "forbidden_keyword: " + tok.Value
		}
	}

	// Check for SELECT ... FOR SHARE / FOR KEY SHARE
	if hasSelectShareLock(tokens) {
		return false, "forbidden_share_lock"
	}

	// MySQL user-variable assignment in SELECT (e.g. SELECT @v := 1)
	if hasMySQLUserVarAssignment(tokens) {
		return false, "forbidden_user_var_assignment"
	}

	// Special handling for WITH (CTE) - check for write operations
	if firstKeyword == "WITH" {
		if hasWriteInCTE(tokens) {
			return false, "cte_write_operation"
		}
	}

	return true, firstKeyword
}

func hasSelectShareLock(tokens []SQLToken) bool {
	keywords := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if tok.Type == TokenKeyword {
			keywords = append(keywords, tok.Value)
		}
	}

	return hasKeywordSequence(keywords, []string{"FOR", "SHARE"}) ||
		hasKeywordSequence(keywords, []string{"FOR", "KEY", "SHARE"})
}

func hasKeywordSequence(keywords []string, seq []string) bool {
	if len(seq) == 0 || len(keywords) < len(seq) {
		return false
	}
	for i := 0; i <= len(keywords)-len(seq); i++ {
		match := true
		for j := range seq {
			if keywords[i+j] != seq[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// unwrapOuterParenthesizedSQL returns the inner SQL when s is exactly "( ... )" (ignoring trailing semicolon).
func unwrapOuterParenthesizedSQL(s string) (string, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(strings.TrimSpace(s), ";")
	if len(s) < 2 || s[0] != '(' {
		return "", false
	}
	closeIdx, ok := findOuterWrapperCloseParen(s)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(s[1:closeIdx]), true
}

func findOuterWrapperCloseParen(s string) (closeIdx int, ok bool) {
	depth := 0
	for i := 0; i < len(s); i++ {
		if !advanceStringOrComment(s, &i) {
			return 0, false
		}
		if i >= len(s) {
			break
		}
		var done, valid bool
		depth, done, valid = stepOuterParenDepth(depth, s[i], i, len(s)-1)
		if !valid {
			return 0, false
		}
		if done {
			return i, true
		}
	}
	return 0, false
}

func stepOuterParenDepth(depth int, ch byte, idx, lastIdx int) (newDepth int, closed bool, ok bool) {
	switch ch {
	case '(':
		return depth + 1, false, true
	case ')':
		nd := depth - 1
		if nd < 0 {
			return nd, false, false
		}
		if nd == 0 {
			return 0, true, idx == lastIdx
		}
		return nd, false, true
	default:
		return depth, false, true
	}
}

// advanceStringOrComment skips a leading string literal or comment at *i. Returns false on unclosed literals.
func advanceStringOrComment(s string, i *int) bool {
	if *i >= len(s) {
		return true
	}
	switch {
	case hasLineCommentPrefix(s, *i):
		skipToLineEnd(s, i)
		return true
	case hasBlockCommentPrefix(s, *i):
		return skipBlockCommentBody(s, i)
	case s[*i] == '\'':
		return skipDoubledQuoteLiteral(s, i, '\'')
	case s[*i] == '"':
		return skipDoubledQuoteLiteral(s, i, '"')
	case s[*i] == '`':
		return skipBacktickLiteral(s, i)
	default:
		return true
	}
}

func hasLineCommentPrefix(s string, i int) bool {
	return s[i] == '-' && i+1 < len(s) && s[i+1] == '-'
}

func hasBlockCommentPrefix(s string, i int) bool {
	return s[i] == '/' && i+1 < len(s) && s[i+1] == '*'
}

func skipToLineEnd(s string, i *int) {
	for *i < len(s) && s[*i] != '\n' {
		*i++
	}
}

func skipBlockCommentBody(s string, i *int) bool {
	*i += 2
	for *i < len(s)-1 {
		if s[*i] == '*' && s[*i+1] == '/' {
			*i += 2
			return true
		}
		*i++
	}
	return false
}

func skipDoubledQuoteLiteral(s string, i *int, quote byte) bool {
	*i++
	for *i < len(s) {
		if s[*i] != quote {
			*i++
			continue
		}
		if *i+1 < len(s) && s[*i+1] == quote {
			*i += 2
			continue
		}
		*i++
		return true
	}
	return false
}

func skipBacktickLiteral(s string, i *int) bool {
	*i++
	for *i < len(s) {
		if s[*i] == '`' {
			*i++
			return true
		}
		*i++
	}
	return false
}

func hasMySQLUserVarAssignment(tokens []SQLToken) bool {
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type != TokenOperator || tokens[i].Value != "@" {
			continue
		}
		if i+2 >= len(tokens) {
			continue
		}
		if tokens[i+1].Type != TokenIdentifier {
			continue
		}
		if tokens[i+2].Type == TokenOperator && tokens[i+2].Value == ":=" {
			return true
		}
	}
	return false
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

// tokenize performs SQL lexical analysis, correctly handling strings, comments, and identifiers
func tokenize(sql string) ([]SQLToken, error) {
	var tokens []SQLToken
	i := 0
	sqlLen := len(sql)

	for i < sqlLen {
		r := rune(sql[i])

		// Skip whitespace
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// BOM header
		if r == '\ufeff' {
			i++
			continue
		}

		// Line comment --
		if r == '-' && i+1 < sqlLen && sql[i+1] == '-' {
			// Skip to end of line
			for i < sqlLen && sql[i] != '\n' {
				i++
			}
			continue
		}

		// Block comment /* */
		if r == '/' && i+1 < sqlLen && sql[i+1] == '*' {
			// Skip to */
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

		// String literal '...' (MySQL/PostgreSQL with escape support)
		if r == '\'' {
			str, newIdx := parseString(sql, i, '\'')
			tokens = append(tokens, SQLToken{Type: TokenString, Value: str})
			i = newIdx
			continue
		}

		// String literal "..." (PostgreSQL/ANSI standard)
		if r == '"' {
			str, newIdx := parseString(sql, i, '"')
			tokens = append(tokens, SQLToken{Type: TokenString, Value: str})
			i = newIdx
			continue
		}

		// Backtick `...` (MySQL identifier)
		if r == '`' {
			str, newIdx := parseString(sql, i, '`')
			tokens = append(tokens, SQLToken{Type: TokenIdentifier, Value: str})
			i = newIdx
			continue
		}

		// PostgreSQL dollar-quoted string $tag$...$tag$
		if r == '$' {
			if str, newIdx, ok := parseDollarQuotedString(sql, i); ok {
				tokens = append(tokens, SQLToken{Type: TokenString, Value: str})
				i = newIdx
				continue
			}
		}

		// Semicolon
		if r == ';' {
			tokens = append(tokens, SQLToken{Type: TokenSemicolon, Value: ";"})
			i++
			continue
		}

		// Number
		if unicode.IsDigit(r) || (r == '.' && i+1 < sqlLen && unicode.IsDigit(rune(sql[i+1]))) {
			start := i
			for i < sqlLen && (unicode.IsDigit(rune(sql[i])) || sql[i] == '.' || sql[i] == 'e' || sql[i] == 'E' || sql[i] == '+' || sql[i] == '-') {
				i++
			}
			tokens = append(tokens, SQLToken{Type: TokenNumber, Value: sql[start:i]})
			continue
		}

		// Keyword or identifier
		if unicode.IsLetter(r) || r == '_' {
			start := i
			for i < sqlLen && (unicode.IsLetter(rune(sql[i])) || unicode.IsDigit(rune(sql[i])) || sql[i] == '_' || sql[i] == '$') {
				i++
			}
			originalWord := sql[start:i]
			upperWord := strings.ToUpper(originalWord)
			// Check if it's a keyword (keywords stored uppercase, identifiers keep original case)
			if isKeyword(upperWord) {
				tokens = append(tokens, SQLToken{Type: TokenKeyword, Value: upperWord})
			} else {
				tokens = append(tokens, SQLToken{Type: TokenIdentifier, Value: originalWord})
			}
			continue
		}

		// Operators and other characters
		if isOperatorChar(r) {
			start := i
			for i < sqlLen && isOperatorChar(rune(sql[i])) {
				i++
			}
			tokens = append(tokens, SQLToken{Type: TokenOperator, Value: sql[start:i]})
			continue
		}

		// Other characters (possibly unsupported)
		tokens = append(tokens, SQLToken{Type: TokenUnknown, Value: string(r)})
		i++
	}

	tokens = append(tokens, SQLToken{Type: TokenEOF, Value: ""})
	return tokens, nil
}

// parseString parses a quoted string, handling escape sequences
func parseString(sql string, start int, quote rune) (string, int) {
	i := start + 1 // skip opening quote
	sqlLen := len(sql)
	var result strings.Builder

	for i < sqlLen {
		r := rune(sql[i])
		if r == quote {
			// Check for escape ('' or "" within quoted strings)
			if i+1 < sqlLen && rune(sql[i+1]) == quote {
				result.WriteRune(quote)
				i += 2
				continue
			}
			// End of string
			return result.String(), i + 1
		}
		// Handle standard escape \' or ''
		if r == '\\' && i+1 < sqlLen && rune(sql[i+1]) == quote {
			result.WriteRune(quote)
			i += 2
			continue
		}
		result.WriteRune(r)
		i++
	}

	// Unclosed string
	return result.String(), i
}

// parseDollarQuotedString parses PostgreSQL dollar-quoted strings $tag$...$tag$
func parseDollarQuotedString(sql string, start int) (string, int, bool) {
	sqlLen := len(sql)
	if sql[start] != '$' {
		return "", start, false
	}

	// Parse tag name
	tagStart := start + 1
	tagEnd := tagStart
	for tagEnd < sqlLen && (unicode.IsLetter(rune(sql[tagEnd])) || unicode.IsDigit(rune(sql[tagEnd])) || sql[tagEnd] == '_') {
		tagEnd++
	}

	if tagEnd >= sqlLen || sql[tagEnd] != '$' {
		return "", start, false // not a valid dollar-quoted string
	}

	tag := sql[tagStart:tagEnd]
	closingTag := "$" + tag + "$"
	contentStart := tagEnd + 1

	// Find closing tag
	for i := contentStart; i < sqlLen; i++ {
		if i+len(closingTag) <= sqlLen && sql[i:i+len(closingTag)] == closingTag {
			content := sql[contentStart:i]
			return content, i + len(closingTag), true
		}
	}

	return "", start, false // closing tag not found
}

// isKeyword checks whether a word is a SQL keyword
func isKeyword(word string) bool {
	upper := strings.ToUpper(word)
	// Check allowed and forbidden keyword lists
	if allowedFirstKeywords[upper] || forbiddenKeywords[upper] {
		return true
	}
	// Other common keywords
	commonKeywords := []string{
		"FROM", "WHERE", "AND", "OR", "NOT", "NULL", "IS", "IN", "EXISTS",
		"AS", "ON", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "CROSS",
		"GROUP", "BY", "ORDER", "HAVING", "LIMIT", "OFFSET", "UNION",
		"ALL", "DISTINCT", "CASE", "WHEN", "THEN", "ELSE", "END",
		"CAST", "CONVERT", "LIKE", "BETWEEN", "AND", "OR", "XOR",
		"IF", "ELSEIF", "ELSE", "WHILE", "FOR", "LOOP", "RETURN",
		"FUNCTION", "PROCEDURE", "TRIGGER", "VIEW", "INDEX", "SCHEMA",
		"DATABASE", "TABLESPACE", "SEQUENCE", "DOMAIN", "TYPE",
		"INTO", "VALUES", "DEFAULT", "PRIMARY", "KEY", "FOREIGN", "SHARE",
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

// isOperatorChar checks whether a rune is an operator character
func isOperatorChar(r rune) bool {
	return r == '+' || r == '-' || r == '*' || r == '/' || r == '=' || r == '<' ||
		r == '>' || r == '!' || r == '~' || r == '|' || r == '&' || r == '%' ||
		r == '^' || r == '@' || r == '#' || r == '?' || r == ':'
}

// hasMultipleValidStatements checks if tokens contain multiple valid statements
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

	// The trailing segment (not terminated by semicolon) counts as a statement if non-empty
	if hasNonTrivialContent {
		stmtCount++
	}

	return stmtCount > 1
}

func isDataModifyingKeyword(value string) bool {
	switch value {
	case "INSERT", "UPDATE", "DELETE":
		return true
	default:
		return false
	}
}

// hasWriteInCTE checks if a WITH statement contains write operations (data-modifying CTE)
func hasWriteInCTE(tokens []SQLToken) bool {
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

		switch tok.Value {
		case "(":
			parenDepth++
		case ")":
			parenDepth--
			if verdict, decided := cteMainStatementVerdict(parenDepth, tokens, i); decided {
				return verdict
			}
		default:
			if parenDepth > 0 && isDataModifyingKeyword(tok.Value) {
				return true
			}
		}
	}
	return false
}

func cteMainStatementVerdict(parenDepth int, tokens []SQLToken, closeIdx int) (writeDetected bool, decided bool) {
	if parenDepth != 0 || closeIdx+1 >= len(tokens) {
		return false, false
	}
	switch tokens[closeIdx+1].Value {
	case "INSERT", "UPDATE", "DELETE":
		return true, true
	case "SELECT":
		return false, true
	default:
		return false, false
	}
}
