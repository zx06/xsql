package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
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

// TableFormatter is the interface for data structures that support table output.
type TableFormatter interface {
	ToTableData() (columns []string, rows []map[string]any, ok bool)
}

// SchemaFormatter is the interface for data structures that support schema output.
type SchemaFormatter interface {
	ToSchemaData() (database string, tables []SchemaTable, ok bool)
}

// SchemaTable is a simplified structure for schema table output.
type SchemaTable struct {
	Schema  string
	Name    string
	Comment string
	Columns []SchemaColumn
}

// ProfileListFormatter is the interface for data structures that support profile list output.
type ProfileListFormatter interface {
	ToProfileListData() (configPath string, profiles []ProfileListItem, ok bool)
}

func writeTable(out io.Writer, env Envelope) error {
	if !env.OK {
		// Error output: display error message concisely
		if env.Error != nil {
			_, _ = fmt.Fprintf(out, "Error [%s]: %s\n", env.Error.Code, env.Error.Message)
		}
		return nil
	}

	// First check if the data implements the TableFormatter interface (no JSON encode/decode)
	if formatter, ok := env.Data.(TableFormatter); ok {
		if cols, rows, ok := formatter.ToTableData(); ok {
			return writeQueryResultTable(out, cols, rows)
		}
	}

	// Check if the data implements the ProfileListFormatter interface
	if formatter, ok := env.Data.(ProfileListFormatter); ok {
		if cfgPath, profiles, ok := formatter.ToProfileListData(); ok {
			return writeProfileListTable(out, cfgPath, profiles)
		}
	}

	// Check if the data implements the SchemaFormatter interface
	if formatter, ok := env.Data.(SchemaFormatter); ok {
		if database, tables, ok := formatter.ToSchemaData(); ok {
			return writeSchemaTable(out, database, tables)
		}
	}

	// Fallback: use type assertion to extract from map[string]any
	if m, ok := env.Data.(map[string]any); ok {
		// Try to extract query result
		if cols, ok := extractStringSlice(m["columns"]); ok {
			if rows, ok := extractMapSlice(m["rows"]); ok {
				return writeQueryResultTable(out, cols, rows)
			}
		}

		// Try to extract profile list
		if profilesRaw, hasProfiles := m["profiles"]; hasProfiles {
			if profileList, ok := tryAsProfileList(profilesRaw); ok {
				cfgPath, _ := m["config_path"].(string)
				return writeProfileListTable(out, cfgPath, profileList)
			}
		}

		// Default: output key-value pairs (sorted by key for stability)
		tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
		for _, k := range sortedMapKeys(m) {
			_, _ = fmt.Fprintf(tw, "%s\t%v\n", k, m[k])
		}
		return tw.Flush()
	}

	// Last resort: try reflection-based extraction (no JSON encode/decode)
	if result, ok := tryAsQueryResultReflect(env.Data); ok {
		return writeQueryResultTable(out, result.columns, result.rows)
	}

	// Default: output data as key-value pairs
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	if env.Data != nil {
		if m, ok := env.Data.(map[string]any); ok {
			for _, k := range sortedMapKeys(m) {
				_, _ = fmt.Fprintf(tw, "%s\t%v\n", k, m[k])
			}
		} else {
			b, err := json.MarshalIndent(env.Data, "", "  ")
			if err == nil {
				_, _ = fmt.Fprintf(tw, "%s\n", b)
			} else {
				_, _ = fmt.Fprintf(tw, "%v\n", env.Data)
			}
		}
	}
	return tw.Flush()
}

// writeProfileListTable writes the profile list as a table.
func writeProfileListTable(out io.Writer, cfgPath string, profiles []ProfileListItem) error {
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	// Output config_path first
	if cfgPath != "" {
		_, _ = fmt.Fprintf(tw, "Config: %s\n\n", cfgPath)
	}
	// Output profiles table
	_, _ = fmt.Fprintln(tw, "NAME\tDESCRIPTION\tDB\tMODE")
	_, _ = fmt.Fprintln(tw, "----\t-----------\t--\t----")
	for _, p := range profiles {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Name, p.Description, p.DB, p.Mode)
	}
	// Use correct singular/plural form
	suffix := "profiles"
	if len(profiles) == 1 {
		suffix = "profile"
	}
	_, _ = fmt.Fprintf(tw, "\n(%d %s)\n", len(profiles), suffix)
	return tw.Flush()
}

// extractStringSlice extracts a []string from any.
func extractStringSlice(v any) ([]string, bool) {
	if v == nil {
		return nil, false
	}
	// Already a []string
	if ss, ok := v.([]string); ok {
		return ss, true
	}
	// Try []any
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

// extractMapSlice extracts a []map[string]any from any.
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

type ProfileListItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	DB          string `json:"db"`
	Mode        string `json:"mode"`
}

func tryAsProfileList(data any) ([]ProfileListItem, bool) {
	// Handle []ProfileListItem
	if arr, ok := data.([]ProfileListItem); ok {
		return arr, len(arr) > 0
	}

	// Handle []map[string]any
	if arr, ok := data.([]map[string]any); ok {
		result := make([]ProfileListItem, 0, len(arr))
		for _, m := range arr {
			p := ProfileListItem{}
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

	// Use reflection to handle arbitrary struct slices (e.g., []profileInfo in cmd/xsql/profile.go)
	v := reflect.ValueOf(data)
	if v.IsValid() && v.Kind() == reflect.Slice {
		result := make([]ProfileListItem, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			// Dereference pointer
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			// Only handle structs
			if elem.Kind() != reflect.Struct {
				return nil, false
			}
			p := ProfileListItem{}
			// Read fields
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

	// Handle []any
	arr, ok := data.([]any)
	if !ok {
		return nil, false
	}

	result := make([]ProfileListItem, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		p := ProfileListItem{}
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

// tryAsQueryResultReflect uses reflection to check for Columns and Rows fields (no JSON encode/decode).
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

	// Find the Columns and Rows fields
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

	// Extract columns
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

	// Extract rows
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

	// Header
	_, _ = fmt.Fprintln(tw, strings.Join(cols, "\t"))

	// Separator line
	dashes := make([]string, len(cols))
	for i, c := range cols {
		dashes[i] = strings.Repeat("-", len(c))
	}
	_, _ = fmt.Fprintln(tw, strings.Join(dashes, "\t"))

	// Data rows
	for _, row := range rows {
		vals := make([]string, len(cols))
		for i, c := range cols {
			vals[i] = formatCellValue(row[c], "<null>")
		}
		_, _ = fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}

	// Row count
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
		// JSON numbers are always float64
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
		// Error output
		if env.Error != nil {
			_ = cw.Write([]string{"error", string(env.Error.Code), env.Error.Message})
		}
		return cw.Error()
	}

	// Try to parse as query result (prefer interface, then reflection)
	var cols []string
	var rows []map[string]any
	var dataOK bool

	// First try the TableFormatter interface
	if formatter, isFormatter := env.Data.(TableFormatter); isFormatter {
		cols, rows, dataOK = formatter.ToTableData()
	}

	// Fallback: use reflection
	if !dataOK {
		if result, ok2 := tryAsQueryResultReflect(env.Data); ok2 {
			cols = result.columns
			rows = result.rows
			dataOK = true
		}
	}

	// Fallback: extract from map
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
		// Write header
		_ = cw.Write(cols)
		// Write data rows
		for _, row := range rows {
			vals := make([]string, len(cols))
			for i, c := range cols {
				vals[i] = formatCellValue(row[c], "")
			}
			_ = cw.Write(vals)
		}
		return cw.Error()
	}

	// Default: output as key,value format
	if m, ok := env.Data.(map[string]any); ok {
		for _, k := range sortedMapKeys(m) {
			_ = cw.Write([]string{k, fmt.Sprintf("%v", m[k])})
		}
	}
	return cw.Error()
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// writeSchemaTable writes the schema as a table.
func writeSchemaTable(out io.Writer, database string, tables []SchemaTable) error {
	// Output database name
	if database != "" {
		_, _ = fmt.Fprintf(out, "Database: %s\n\n", database)
	}

	// Iterate over each table
	for i, table := range tables {
		if i > 0 {
			_, _ = fmt.Fprintln(out) // Blank line between tables
		}

		// Table header
		header := table.Name
		if table.Schema != "" && table.Schema != database {
			header = table.Schema + "." + table.Name
		}
		if table.Comment != "" {
			header += " (" + table.Comment + ")"
		}
		_, _ = fmt.Fprintf(out, "Table: %s\n", header)

		// Column information
		if len(table.Columns) > 0 {
			_, _ = fmt.Fprintln(out, "  Columns:")
			tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "    name\ttype\tnullable\tdefault\tcomment\tpk")
			_, _ = fmt.Fprintln(tw, "    ----\t----\t--------\t-------\t-------\t--")
			for _, col := range table.Columns {
				defaultVal := col.Default
				if defaultVal == "" {
					defaultVal = "-"
				}
				comment := col.Comment
				if comment == "" {
					comment = "-"
				}
				pk := ""
				if col.PrimaryKey {
					pk = "✓"
				}
				_, _ = fmt.Fprintf(tw, "    %s\t%s\t%v\t%s\t%s\t%s\n",
					col.Name, col.Type, col.Nullable, defaultVal, comment, pk)
			}
			_ = tw.Flush()
		}
	}

	// Table count
	suffix := "tables"
	if len(tables) == 1 {
		suffix = "table"
	}
	_, _ = fmt.Fprintf(out, "\n(%d %s)\n", len(tables), suffix)
	return nil
}

// SchemaColumn is a simplified structure for schema column output.
type SchemaColumn struct {
	Name       string
	Type       string
	Nullable   bool
	Default    string
	Comment    string
	PrimaryKey bool
}
