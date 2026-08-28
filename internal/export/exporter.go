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
	FormatCSV  ExportFormat = "csv"
	FormatJSON ExportFormat = "json"
)

// ExpandPath expands leading ~ to user's home directory.
func ExpandPath(filePath string) (string, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", nil
	}

	if filePath == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}

	if strings.HasPrefix(filePath, "~/") || strings.HasPrefix(filePath, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, filePath[2:]), nil
	}

	return filePath, nil
}

func ExportQueryResult(result *db.QueryResult, format ExportFormat, filePath string) (string, *errors.XError) {
	if result == nil {
		return "", errors.New(errors.CodeCfgInvalid, "cannot export nil QueryResult", nil)
	}

	format = ExportFormat(strings.ToLower(strings.TrimSpace(string(format))))
	switch format {
	case FormatCSV, FormatJSON:
	default:
		return "", errors.New(errors.CodeCfgInvalid, "unsupported export format", map[string]any{
			"format": format,
		})
	}

	expandedPath, err := ExpandPath(filePath)
	if err != nil {
		return "", errors.New(errors.CodeInternal, "failed to expand export file path", map[string]any{
			"path": filePath,
			"err":  err.Error(),
		})
	}
	if expandedPath == "" {
		expandedPath = fmt.Sprintf("export_%s.%s", format, format)
	}

	// Ensure directory exists
	dir := filepath.Dir(expandedPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", errors.New(errors.CodeInternal, "failed to create export directory", map[string]any{
				"dir": dir,
				"err": err.Error(),
			})
		}
	}

	f, err := os.Create(expandedPath)
	if err != nil {
		return "", errors.New(errors.CodeInternal, "failed to create export file", map[string]any{
			"path": expandedPath,
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

	absPath, _ := filepath.Abs(expandedPath)
	return absPath, nil
}

// ExportReport writes the Markdown/text report content to the target file path.
func ExportReport(content string, filePath string) (string, *errors.XError) {
	expandedPath, err := ExpandPath(filePath)
	if err != nil {
		return "", errors.New(errors.CodeInternal, "failed to expand report file path", map[string]any{
			"path": filePath,
			"err":  err.Error(),
		})
	}
	if expandedPath == "" {
		expandedPath = "report.md"
	}

	dir := filepath.Dir(expandedPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", errors.New(errors.CodeInternal, "failed to create report directory", map[string]any{
				"dir": dir,
				"err": err.Error(),
			})
		}
	}

	if err := os.WriteFile(expandedPath, []byte(content), 0644); err != nil {
		return "", errors.New(errors.CodeInternal, "failed to write report file", map[string]any{
			"path": expandedPath,
			"err":  err.Error(),
		})
	}

	absPath, _ := filepath.Abs(expandedPath)
	return absPath, nil
}
