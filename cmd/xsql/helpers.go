package main

import (
	"os"

	"golang.org/x/term"

	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
)

// parseOutputFormat parses and validates the output format string
func parseOutputFormat(s string) (output.Format, error) {
	f := output.Format(s)
	if !output.IsValid(f) {
		return "", errors.New(errors.CodeCfgInvalid, "invalid output format", map[string]any{"format": s})
	}
	return resolveAuto(f), nil
}

// resolveFormatForError resolves the format for error output
func resolveFormatForError(s string) output.Format {
	f := output.Format(s)
	if !output.IsValid(f) {
		f = output.FormatAuto
	}
	return resolveAuto(f)
}

// resolveAuto resolves "auto" format to appropriate format based on TTY
func resolveAuto(f output.Format) output.Format {
	if f != output.FormatAuto {
		return f
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return output.FormatTable
	}
	return output.FormatJSON
}

// normalizeErr normalizes any error to XError
func normalizeErr(err error) *errors.XError {
	if xe, ok := errors.As(err); ok {
		return xe
	}
	// Preserve original error message
	return errors.Wrap(errors.CodeInternal, err.Error(), nil, err)
}
