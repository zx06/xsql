package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
)

type ExportFormat string

const (
	FormatCSV      ExportFormat = "csv"
	FormatJSON     ExportFormat = "json"
	FormatMarkdown ExportFormat = "markdown"
)

func ExportQueryResult(result *db.QueryResult, format ExportFormat, filePath string) (string, *errors.XError) {
	if result == nil {
		return "", errors.New(errors.CodeCfgInvalid, "cannot export nil QueryResult", nil)
	}

	format = ExportFormat(strings.ToLower(strings.TrimSpace(string(format))))
	switch format {
	case FormatCSV, FormatJSON, FormatMarkdown:
	default:
		return "", errors.New(errors.CodeCfgInvalid, "unsupported export format", map[string]any{
			"format": format,
		})
	}

	if filePath == "" {
		filePath = fmt.Sprintf("export_%s.%s", format, format)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", errors.New(errors.CodeInternal, "failed to create export directory", map[string]any{
				"dir": dir,
				"err": err.Error(),
			})
		}
	}

	f, err := os.Create(filePath)
	if err != nil {
		return "", errors.New(errors.CodeInternal, "failed to create export file", map[string]any{
			"path": filePath,
			"err":  err.Error(),
		})
	}
	defer func() { _ = f.Close() }()

	switch format {
	case FormatJSON:
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result.Rows); err != nil {
			return "", errors.New(errors.CodeInternal, "failed to write JSON export", map[string]any{"err": err.Error()})
		}

	case FormatMarkdown:
		var sb strings.Builder
		sb.WriteString("| " + strings.Join(result.Columns, " | ") + " |\n")
		var sep []string
		for range result.Columns {
			sep = append(sep, "---")
		}
		sb.WriteString("| " + strings.Join(sep, " | ") + " |\n")

		for _, row := range result.Rows {
			var vals []string
			for _, col := range result.Columns {
				val := row[col]
				if val == nil {
					vals = append(vals, "NULL")
				} else {
					cellStr := fmt.Sprintf("%v", val)
					cellStr = strings.ReplaceAll(cellStr, "\n", " ")
					cellStr = strings.ReplaceAll(cellStr, "|", "\\|")
					vals = append(vals, cellStr)
				}
			}
			sb.WriteString("| " + strings.Join(vals, " | ") + " |\n")
		}
		if _, err := f.WriteString(sb.String()); err != nil {
			return "", errors.New(errors.CodeInternal, "failed to write Markdown export", map[string]any{"err": err.Error()})
		}

	case FormatCSV:
		w := csv.NewWriter(f)
		if err := w.Write(result.Columns); err != nil {
			return "", errors.New(errors.CodeInternal, "failed to write CSV header", map[string]any{"err": err.Error()})
		}
		for _, row := range result.Rows {
			var vals []string
			for _, col := range result.Columns {
				val := row[col]
				if val == nil {
					vals = append(vals, "")
				} else {
					vals = append(vals, fmt.Sprintf("%v", val))
				}
			}
			if err := w.Write(vals); err != nil {
				return "", errors.New(errors.CodeInternal, "failed to write CSV row", map[string]any{"err": err.Error()})
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return "", errors.New(errors.CodeInternal, "failed to flush CSV writer", map[string]any{"err": err.Error()})
		}
	}

	absPath, _ := filepath.Abs(filePath)
	return absPath, nil
}
