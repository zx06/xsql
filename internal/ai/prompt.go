package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zx06/xsql/internal/db"
)

const SystemPromptTemplate = `You are an AI SQL Generator and Data Analyst for %s database.

TARGET DATABASE: %s (Dialect: %s)
- Always generate valid %s SQL dialect queries.

DATABASE SCHEMA:
%s

%s
ENVIRONMENT & SPECIFICATIONS:
- Database Mode: Default to READ-ONLY SELECT queries.
- JavaScript Environment: Strict ES5 (ECMAScript 5.1) engine. Active session datasets (e.g. res1, res2) are available in global context.
- Tool Calling Guidelines:
  * Always use the structured tool calling interface with strictly valid JSON arguments.
  * 'export_data': Use ONLY to export a raw cached session dataset (e.g. res1, res2) to 'csv' or 'json'.
  * 'export_report': When the user asks to generate, save, or export an analysis report / summary / Markdown document, assemble the full comprehensive Markdown content (including titles, insights, conclusions, and embedded markdown tables) and call 'export_report'.
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
	currentDB := "(unknown)"
	if schemaInfo != nil && schemaInfo.Database != "" {
		currentDB = schemaInfo.Database
	}

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
	return fmt.Sprintf(SystemPromptTemplate, formattedDB, currentDB, formattedDB, formattedDB, schemaJSON, catalogBlock)
}
