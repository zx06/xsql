package main

import (
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/output"
)

// NewSpecCommand creates the spec command
func NewSpecCommand(a *app.App, w *output.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "spec",
		Short: "Export tool spec for AI/agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(GlobalConfig.FormatStr)
			if err != nil {
				return err
			}
			return w.WriteOK(format, a.BuildSpec())
		},
	}
}
