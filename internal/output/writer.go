package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
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

// TableFormatter 接口：支持表格输出的数据结构实现此接口
type TableFormatter interface {
	ToTableData() (columns []string, rows []map[string]any, ok bool)
}

// ProfileListFormatter 接口：支持 profile list 输出的结构实现此接口
type ProfileListFormatter interface {
	ToProfileListData() (configPath string, profiles []profileListItem, ok bool)
}

func writeTable(out io.Writer, env Envelope) error {
	if !env.OK {
		// 错误输出：简洁显示错误信息
		if env.Error != nil {
			_, _ = fmt.Fprintf(out, "Error [%s]: %s\n", env.Error.Code, env.Error.Message)
		}
		return nil
	}

	// 优先检查是否实现了 TableFormatter 接口（无 JSON 编解码）
	if formatter, ok := env.Data.(TableFormatter); ok {
		if cols, rows, ok := formatter.ToTableData(); ok {
			return writeQueryResultTable(out, cols, rows)
		}
	}

	// 优先检查是否实现了 ProfileListFormatter 接口
	if formatter, ok := env.Data.(ProfileListFormatter); ok {
		if cfgPath, profiles, ok := formatter.ToProfileListData(); ok {
			return writeProfileListTable(out, cfgPath, profiles)
		}
	}

	// 回退：使用类型断言从 map[string]any 提取
	if m, ok := env.Data.(map[string]any); ok {
		// 尝试提取查询结果
		if cols, ok := extractStringSlice(m["columns"]); ok {
			if rows, ok := extractMapSlice(m["rows"]); ok {
				return writeQueryResultTable(out, cols, rows)
			}
		}

		// 尝试提取 profile list
		if profilesRaw, hasProfiles := m["profiles"]; hasProfiles {
			if profileList, ok := tryAsProfileList(profilesRaw); ok {
				cfgPath, _ := m["config_path"].(string)
				return writeProfileListTable(out, cfgPath, profileList)
			}
		}

		// 默认：直接输出 key-value
		tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
		for k, v := range m {
			_, _ = fmt.Fprintf(tw, "%s\t%v\n", k, v)
		}
		return tw.Flush()
	}

	// 最后尝试反射提取（无 JSON 编解码）
	if result, ok := tryAsQueryResultReflect(env.Data); ok {
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
			// 只有这里不得已才使用 JSON 格式化
			b, _ := json.MarshalIndent(env.Data, "", "  ")
			_, _ = fmt.Fprintf(tw, "%s\n", b)
		}
	}
	return tw.Flush()
}

// writeProfileListTable 输出 profile list 表格
func writeProfileListTable(out io.Writer, cfgPath string, profiles []profileListItem) error {
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	// 先输出 config_path
	if cfgPath != "" {
		_, _ = fmt.Fprintf(tw, "Config: %s\n\n", cfgPath)
	}
	// 输出 profiles 表格
	_, _ = fmt.Fprintln(tw, "NAME\tDESCRIPTION\tDB\tMODE")
	_, _ = fmt.Fprintln(tw, "----\t-----------\t--\t----")
	for _, p := range profiles {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Name, p.Description, p.DB, p.Mode)
	}
	// 使用正确的单数/复数形式
	suffix := "profiles"
	if len(profiles) == 1 {
		suffix = "profile"
	}
	_, _ = fmt.Fprintf(tw, "\n(%d %s)\n", len(profiles), suffix)
	return tw.Flush()
}

// extractStringSlice 从 any 提取 []string
func extractStringSlice(v any) ([]string, bool) {
	if v == nil {
		return nil, false
	}
	// 已经是 []string
	if ss, ok := v.([]string); ok {
		return ss, true
	}
	// 尝试 []any
	if arr, ok := v.([]any); ok {
		result := make([]string, len(arr))
		for i, item := range arr {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			result[i] = s
		}
		return result, true
	}
	return nil, false
}

// extractMapSlice 从 any 提取 []map[string]any
func extractMapSlice(v any) ([]map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	if arr, ok := v.([]map[string]any); ok {
		return arr, true
	}
	if arr, ok := v.([]any); ok {
		result := make([]map[string]any, len(arr))
		for i, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			result[i] = m
		}
		return result, true
	}
	return nil, false
}

type profileListItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	DB          string `json:"db"`
	Mode        string `json:"mode"`
}

func tryAsProfileList(data any) ([]profileListItem, bool) {
	// 处理 []profileListItem
	if arr, ok := data.([]profileListItem); ok {
		return arr, len(arr) > 0
	}

	// 处理 []map[string]any
	if arr, ok := data.([]map[string]any); ok {
		result := make([]profileListItem, 0, len(arr))
		for _, m := range arr {
			p := profileListItem{}
			if v, ok := m["name"].(string); ok {
				p.Name = v
			}
			if v, ok := m["description"].(string); ok {
				p.Description = v
			}
			if v, ok := m["db"].(string); ok {
				p.DB = v
			}
			if v, ok := m["mode"].(string); ok {
				p.Mode = v
			}
			if p.Name == "" {
				return nil, false
			}
			result = append(result, p)
		}
		return result, len(result) > 0
	}

	// 使用反射处理任意结构体切片（如 cmd/xsql/profile.go 中的 []profileInfo）
	v := reflect.ValueOf(data)
	if v.IsValid() && v.Kind() == reflect.Slice {
		result := make([]profileListItem, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			// 解引用指针
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			// 只处理结构体
			if elem.Kind() != reflect.Struct {
				return nil, false
			}
			p := profileListItem{}
			// 读取字段
			if f := elem.FieldByName("Name"); f.IsValid() && f.Kind() == reflect.String {
				p.Name = f.String()
			}
			if f := elem.FieldByName("Description"); f.IsValid() && f.Kind() == reflect.String {
				p.Description = f.String()
			}
			if f := elem.FieldByName("DB"); f.IsValid() && f.Kind() == reflect.String {
				p.DB = f.String()
			}
			if f := elem.FieldByName("Mode"); f.IsValid() && f.Kind() == reflect.String {
				p.Mode = f.String()
			}
			if p.Name == "" {
				return nil, false
			}
			result = append(result, p)
		}
		return result, len(result) > 0
	}

	// 处理 []any
	arr, ok := data.([]any)
	if !ok {
		return nil, false
	}

	result := make([]profileListItem, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		p := profileListItem{}
		if v, ok := m["name"].(string); ok {
			p.Name = v
		}
		if v, ok := m["description"].(string); ok {
			p.Description = v
		}
		if v, ok := m["db"].(string); ok {
			p.DB = v
		}
		if v, ok := m["mode"].(string); ok {
			p.Mode = v
		}
		if p.Name == "" {
			return nil, false
		}
		result = append(result, p)
	}
	return result, len(result) > 0
}

type queryResultLike struct {
	columns []string
	rows    []map[string]any
}

// tryAsQueryResultReflect 使用反射检查字段 Columns 和 Rows（无 JSON 编解码）
func tryAsQueryResultReflect(data any) (*queryResultLike, bool) {
	if data == nil {
		return nil, false
	}

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, false
	}

	// 查找 Columns 和 Rows 字段
	var colsValue, rowsValue reflect.Value
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if field.Name == "Columns" || field.Name == "columns" {
			colsValue = v.Field(i)
		}
		if field.Name == "Rows" || field.Name == "rows" {
			rowsValue = v.Field(i)
		}
	}

	if !colsValue.IsValid() || !rowsValue.IsValid() {
		return nil, false
	}

	// 提取 columns
	if colsValue.Kind() != reflect.Slice {
		return nil, false
	}
	cols := make([]string, colsValue.Len())
	for i := 0; i < colsValue.Len(); i++ {
		elem := colsValue.Index(i)
		if elem.Kind() == reflect.String {
			cols[i] = elem.String()
		} else if elem.Kind() == reflect.Interface {
			s, ok := elem.Interface().(string)
			if !ok {
				return nil, false
			}
			cols[i] = s
		} else {
			return nil, false
		}
	}

	// 提取 rows
	if rowsValue.Kind() != reflect.Slice {
		return nil, false
	}
	rows := make([]map[string]any, rowsValue.Len())
	for i := 0; i < rowsValue.Len(); i++ {
		elem := rowsValue.Index(i)
		if elem.Kind() == reflect.Map {
			row := make(map[string]any)
			for _, key := range elem.MapKeys() {
				val := elem.MapIndex(key)
				k, ok := key.Interface().(string)
				if !ok {
					return nil, false
				}
				row[k] = val.Interface()
			}
			rows[i] = row
		} else if elem.Kind() == reflect.Interface {
			row, ok := elem.Interface().(map[string]any)
			if !ok {
				return nil, false
			}
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
			vals[i] = formatCellValue(row[c], "<null>")
		}
		_, _ = fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}

	// 行数统计
	_, _ = fmt.Fprintf(tw, "\n(%d rows)\n", len(rows))

	return tw.Flush()
}

func formatCellValue(v any, nullValue string) string {
	if v == nil {
		return nullValue
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

	// 尝试解析为查询结果（优先使用接口，然后反射）
	var cols []string
	var rows []map[string]any
	var dataOK bool

	// 先尝试 TableFormatter 接口
	if formatter, isFormatter := env.Data.(TableFormatter); isFormatter {
		cols, rows, dataOK = formatter.ToTableData()
	}

	// 回退：使用反射
	if !dataOK {
		if result, ok2 := tryAsQueryResultReflect(env.Data); ok2 {
			cols = result.columns
			rows = result.rows
			dataOK = true
		}
	}

	// 回退：使用 map 提取
	if !dataOK {
		if m, ok2 := env.Data.(map[string]any); ok2 {
			var ok3 bool
			cols, ok3 = extractStringSlice(m["columns"])
			if ok3 {
				rows, dataOK = extractMapSlice(m["rows"])
			}
		}
	}

	if dataOK {
		// 输出表头
		_ = cw.Write(cols)
		// 输出数据行
		for _, row := range rows {
			vals := make([]string, len(cols))
			for i, c := range cols {
				vals[i] = formatCellValue(row[c], "")
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
