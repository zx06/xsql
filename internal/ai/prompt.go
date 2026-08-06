package ai

import (
	"encoding/json"
	"fmt"

	"github.com/zx06/xsql/internal/db"
)

const SystemPromptTemplate = `You are an AI SQL Generator and Data Analyst for the %s database.

DATABASE SCHEMA:
%s

%s

ENVIRONMENT & SPECIFICATIONS:
- Database Mode: Default to READ-ONLY SELECT queries.
- JavaScript Environment: Strict ES5 (ECMAScript 5.1) engine. Active session datasets (e.g. res1, res2) are available in global context.
`

func BuildSystemPrompt(dbType string, schemaInfo *db.SchemaInfo, catalog string) string {
	schemaJSON := "{}"
	if schemaInfo != nil {
		if bytes, err := json.MarshalIndent(schemaInfo, "", "  "); err == nil {
			schemaJSON = string(bytes)
		}
	}
	if dbType == "" {
		dbType = "MySQL/PostgreSQL"
	}
	catalogBlock := ""
	if catalog != "" {
		catalogBlock = fmt.Sprintf("SESSION DATASETS CATALOG:\n%s\n", catalog)
	}
	return fmt.Sprintf(SystemPromptTemplate, dbType, schemaJSON, catalogBlock)
}
