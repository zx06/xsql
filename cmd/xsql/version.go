package main

import (
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/output"
)

// NewVersionCommand creates the version command
func NewVersionCommand(a *app.App, w *output.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(GlobalConfig.FormatStr)
			if err != nil {
				return err
			}
			return w.WriteOK(format, a.VersionInfo())
		},
	}
}