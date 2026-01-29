package main

import (
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
)

var version = "dev"

func main() {
	exit := run()
	os.Exit(exit)
}

func run() int {
	a := app.New(version)
	w := output.New(os.Stdout, os.Stderr)

	var formatStr string
	root := &cobra.Command{
		Use:           "xsql",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// CLI > ENV > Config（本阶段仅落地 format 的 ENV 读取）
			if !cmd.Flags().Changed("format") {
				if v := os.Getenv("XSQL_FORMAT"); v != "" {
					formatStr = v
				}
			}
			return nil
		},
	}
	root.PersistentFlags().StringVarP(&formatStr, "format", "f", "auto", "Output format: json|yaml|table|csv|auto")

	specCmd := &cobra.Command{
		Use:   "spec",
		Short: "Export tool spec for AI/agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(formatStr)
			if err != nil {
				return err
			}
			return w.WriteOK(format, a.BuildSpec())
		},
	}

	verCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(formatStr)
			if err != nil {
				return err
			}
			return w.WriteOK(format, a.VersionInfo())
		},
	}

	root.AddCommand(specCmd, verCmd)

	if err := root.Execute(); err != nil {
		xe := normalizeErr(err)
		format := resolveFormatForError(formatStr)
		_ = w.WriteError(format, xe)
		return int(errors.ExitCodeFor(xe.Code))
	}
	return int(errors.ExitOK)
}

func parseOutputFormat(s string) (output.Format, error) {
	f := output.Format(s)
	if !output.IsValid(f) {
		return "", errors.New(errors.CodeCfgInvalid, "invalid output format", map[string]any{"format": s})
	}
	return resolveAuto(f), nil
}

func resolveFormatForError(s string) output.Format {
	f := output.Format(s)
	if !output.IsValid(f) {
		f = output.FormatAuto
	}
	return resolveAuto(f)
}

func resolveAuto(f output.Format) output.Format {
	if f != output.FormatAuto {
		return f
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return output.FormatTable
	}
	return output.FormatJSON
}

func normalizeErr(err error) *errors.XError {
	if xe, ok := errors.As(err); ok {
		return xe
	}
	return errors.Wrap(errors.CodeInternal, "internal error", nil, err)
}
