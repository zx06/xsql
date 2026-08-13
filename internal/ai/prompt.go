package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zx06/xsql/internal/db"
)

const SystemPromptTemplate = `You are an AI SQL Generator and Data Analyst for %s database.

TARGET DATABASE DIALECT: %s
- Always generate correct %s SQL dialect syntax, functions, and data types.

DATABASE SCHEMA:
%s

%s

ENVIRONMENT & SPECIFICATIONS:
	- Database Mode: Default to READ-ONLY SELECT queries.
	- JavaScript Environment: Strict ES5 (ECMAScript 5.1) engine. Active session datasets (e.g. res1, res2) are available in global context.
	- JavaScript Output: Return only compact derived aggregates needed for the final answer. Never copy or return complete raw datasets.
`

func FormatDBName(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql":
		return "MySQL"
	case "pg", "postgres", "postgresql":
		return "PostgreSQL"
	default:
		if dbType != "" {
			return dbType
		}
		return "MySQL/PostgreSQL"
	}
}

func BuildSystemPrompt(dbType string, schemaInfo *db.SchemaInfo, catalog string) string {
	formattedDB := FormatDBName(dbType)
	schemaJSON := "{}"
	if schemaInfo != nil {
		if bytes, err := json.MarshalIndent(schemaInfo, "", "  "); err == nil {
			schemaJSON = string(bytes)
		}
	}
	catalogBlock := ""
	if catalog != "" {
		catalogBlock = fmt.Sprintf("SESSION DATASETS CATALOG:\n%s\n", catalog)
	}
	return fmt.Sprintf(SystemPromptTemplate, formattedDB, formattedDB, formattedDB, schemaJSON, catalogBlock)
}
