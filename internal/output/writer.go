package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/zx06/xsql/internal/errors"
	"gopkg.in/yaml.v3"
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

func writeTable(out io.Writer, env Envelope) error {
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	if env.OK {
		_, _ = fmt.Fprintf(tw, "ok\t%v\n", true)
		_, _ = fmt.Fprintf(tw, "schema_version\t%d\n", env.SchemaVersion)
		if env.Data != nil {
			b, _ := json.MarshalIndent(env.Data, "", "  ")
			_, _ = fmt.Fprintf(tw, "data\t%s\n", strings.ReplaceAll(string(b), "\n", " "))
		}
	} else {
		_, _ = fmt.Fprintf(tw, "ok\t%v\n", false)
		_, _ = fmt.Fprintf(tw, "schema_version\t%d\n", env.SchemaVersion)
		if env.Error != nil {
			_, _ = fmt.Fprintf(tw, "error.code\t%s\n", env.Error.Code)
			_, _ = fmt.Fprintf(tw, "error.message\t%s\n", env.Error.Message)
		}
	}
	return tw.Flush()
}

func writeCSV(out io.Writer, env Envelope) error {
	// CSV 仅作为人类可读/流式占位；结构化场景建议用 json/yaml。
	cw := csv.NewWriter(out)
	defer cw.Flush()
	if env.OK {
		_ = cw.Write([]string{"ok", "true"})
		_ = cw.Write([]string{"schema_version", fmt.Sprintf("%d", env.SchemaVersion)})
		return cw.Error()
	}
	_ = cw.Write([]string{"ok", "false"})
	_ = cw.Write([]string{"schema_version", fmt.Sprintf("%d", env.SchemaVersion)})
	if env.Error != nil {
		_ = cw.Write([]string{"error.code", string(env.Error.Code)})
		_ = cw.Write([]string{"error.message", env.Error.Message})
	}
	return cw.Error()
}
