package output

type Format string

const (
	FormatAuto  Format = "auto"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatTable Format = "table"
	FormatCSV   Format = "csv"
)

func IsValid(f Format) bool {
	switch f {
	case FormatAuto, FormatJSON, FormatYAML, FormatTable, FormatCSV:
		return true
	default:
		return false
	}
}
