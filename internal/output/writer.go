package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"

	"github.com/zx06/xsql/internal/errors"
)

type Writer struct {
	Out io.Writer
	Err io.Writer
}

func New(out, err io.Writer) Writer {
	return Writer{Out: out, Err: err}
}

func (w Writer) WriteOK(format Format, data any) error {
	return w.write(format, Envelope{OK: true, SchemaVersion: SchemaVersion, Data: data})
}

func (w Writer) WriteError(format Format, xe *errors.XError) error {
	errObj := &ErrorObject{Code: xe.Code, Message: xe.Message, Details: xe.Details}
	// 如果有底层错误，添加到 details
	if xe.Unwrap() != nil && errObj.Details == nil {
		errObj.Details = map[string]any{"cause": xe.Unwrap().Error()}
	} else if xe.Unwrap() != nil {
		errObj.Details["cause"] = xe.Unwrap().Error()
	}
	return w.write(format, Envelope{OK: false, SchemaVersion: SchemaVersion, Error: errObj})
}

func (w Writer) write(format Format, env Envelope) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w.Out)
		enc.SetEscapeHTML(false)
		return enc.Encode(env)
	case FormatYAML:
		b, err := yaml.Marshal(env)
		if err != nil {
			return err
		}
		_, err = w.Out.Write(b)
		if err != nil {
			return err
		}
		if len(b) == 0 || b[len(b)-1] != '\n' {
			_, _ = w.Out.Write([]byte("\n"))
		}
		return nil
	case FormatTable:
		return writeTable(w.Out, env)
	case FormatCSV:
		return writeCSV(w.Out, env)
	default:
		return errors.New(errors.CodeCfgInvalid, "invalid output format", map[string]any{"format": string(format)})
	}
}

func writeTable(out io.Writer, env Envelope) error {
	if !env.OK {
		// 错误输出：简洁显示错误信息
		if env.Error != nil {
			_, _ = fmt.Fprintf(out, "Error [%s]: %s\n", env.Error.Code, env.Error.Message)
		}
		return nil
	}

	// 将 data 转为 map[string]any（通过 JSON 序列化确保一致性）
	var data map[string]any
	if m, ok := env.Data.(map[string]any); ok {
		data = m
	} else {
		b, err := json.Marshal(env.Data)
		if err == nil {
			_ = json.Unmarshal(b, &data)
		}
	}

	if data != nil {
		// 尝试将 data 解析为查询结果表格
		if cols, hasColumns := data["columns"].([]string); hasColumns {
			if rows, hasRows := data["rows"].([]map[string]any); hasRows {
				return writeQueryResultTable(out, cols, rows)
			}
		}

		// 处理 profile list 输出
		if profiles, hasProfiles := data["profiles"]; hasProfiles {
			if profileList, ok := tryAsProfileList(profiles); ok {
				tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
				// 先输出 config_path
				if cfgPath, ok := data["config_path"].(string); ok {
					_, _ = fmt.Fprintf(tw, "Config: %s\n\n", cfgPath)
				}
				// 输出 profiles 表格
				_, _ = fmt.Fprintln(tw, "NAME\tDESCRIPTION\tDB\tMODE")
				_, _ = fmt.Fprintln(tw, "----\t-----------\t--\t----")
				for _, p := range profileList {
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.name, p.description, p.db, p.mode)
				}
				_, _ = fmt.Fprintf(tw, "\n(%d profiles)\n", len(profileList))
				return tw.Flush()
			}
		}
	}

	// 尝试类型断言为 *db.QueryResult 或类似结构
	if result, ok := tryAsQueryResult(env.Data); ok {
		return writeQueryResultTable(out, result.columns, result.rows)
	}

	// 默认：直接输出数据的 key-value
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	if env.Data != nil {
		if m, ok := env.Data.(map[string]any); ok {
			for k, v := range m {
				_, _ = fmt.Fprintf(tw, "%s\t%v\n", k, v)
			}
		} else {
			b, _ := json.MarshalIndent(env.Data, "", "  ")
			_, _ = fmt.Fprintf(tw, "%s\n", b)
		}
	}
	return tw.Flush()
}

type profileListItem struct {
	name        string
	description string
	db          string
	mode        string
}

func tryAsProfileList(data any) ([]profileListItem, bool) {
	// 先尝试直接转换
	arr, ok := data.([]any)
	if !ok {
		// 尝试通过 JSON 序列化/反序列化转换
		b, err := json.Marshal(data)
		if err != nil {
			return nil, false
		}
		if err := json.Unmarshal(b, &arr); err != nil {
			return nil, false
		}
	}

	result := make([]profileListItem, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		name, _ := m["name"].(string)
		description, _ := m["description"].(string)
		db, _ := m["db"].(string)
		mode, _ := m["mode"].(string)
		if name == "" {
			return nil, false
		}
		result = append(result, profileListItem{name: name, description: description, db: db, mode: mode})
	}
	return result, len(result) > 0
}

type queryResultLike struct {
	columns []string
	rows    []map[string]any
}

func tryAsQueryResult(data any) (*queryResultLike, bool) {
	// 使用反射检查是否有 Columns 和 Rows 字段
	b, err := json.Marshal(data)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, false
	}

	colsRaw, hasColumns := m["columns"]
	rowsRaw, hasRows := m["rows"]
	if !hasColumns || !hasRows {
		return nil, false
	}

	// 解析 columns
	colsArr, ok := colsRaw.([]any)
	if !ok {
		return nil, false
	}
	cols := make([]string, len(colsArr))
	for i, c := range colsArr {
		if s, ok := c.(string); ok {
			cols[i] = s
		} else {
			return nil, false
		}
	}

	// 解析 rows
	rowsArr, ok := rowsRaw.([]any)
	if !ok {
		return nil, false
	}
	rows := make([]map[string]any, len(rowsArr))
	for i, r := range rowsArr {
		if row, ok := r.(map[string]any); ok {
			rows[i] = row
		} else {
			return nil, false
		}
	}

	return &queryResultLike{columns: cols, rows: rows}, true
}

func writeQueryResultTable(out io.Writer, cols []string, rows []map[string]any) error {
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)

	// 表头
	_, _ = fmt.Fprintln(tw, strings.Join(cols, "\t"))

	// 分隔线
	dashes := make([]string, len(cols))
	for i, c := range cols {
		dashes[i] = strings.Repeat("-", len(c))
	}
	_, _ = fmt.Fprintln(tw, strings.Join(dashes, "\t"))

	// 数据行
	for _, row := range rows {
		vals := make([]string, len(cols))
		for i, c := range cols {
			vals[i] = formatCellValue(row[c])
		}
		_, _ = fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}

	// 行数统计
	_, _ = fmt.Fprintf(tw, "\n(%d rows)\n", len(rows))

	return tw.Flush()
}

func formatCellValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		// JSON 数字都是 float64
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func writeCSV(out io.Writer, env Envelope) error {
	cw := csv.NewWriter(out)
	defer cw.Flush()

	if !env.OK {
		// 错误输出
		if env.Error != nil {
			_ = cw.Write([]string{"error", string(env.Error.Code), env.Error.Message})
		}
		return cw.Error()
	}

	// 尝试解析为查询结果
	if result, ok := tryAsQueryResult(env.Data); ok {
		// 输出表头
		_ = cw.Write(result.columns)
		// 输出数据行
		for _, row := range result.rows {
			vals := make([]string, len(result.columns))
			for i, c := range result.columns {
				vals[i] = formatCellValue(row[c])
			}
			_ = cw.Write(vals)
		}
		return cw.Error()
	}

	// 默认：输出为 key,value 格式
	if m, ok := env.Data.(map[string]any); ok {
		for k, v := range m {
			_ = cw.Write([]string{k, fmt.Sprintf("%v", v)})
		}
	}
	return cw.Error()
}
