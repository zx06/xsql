package main

import (
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/output"
)

// NewProfileCommand creates the profile command group
func NewProfileCommand(w *output.Writer) *cobra.Command {
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles",
	}

	profileCmd.AddCommand(newProfileListCommand(w))
	profileCmd.AddCommand(newProfileShowCommand(w))

	return profileCmd
}

// newProfileListCommand creates the profile list command
func newProfileListCommand(w *output.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(GlobalConfig.FormatStr)
			if err != nil {
				return err
			}

			result, xe := app.LoadProfiles(config.Options{
				ConfigPath: GlobalConfig.ConfigStr,
			})
			if xe != nil {
				return xe
			}

			return w.WriteOK(format, result)
		},
	}
}

// newProfileShowCommand creates the profile show command
func newProfileShowCommand(w *output.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show profile details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			format, err := parseOutputFormat(GlobalConfig.FormatStr)
			if err != nil {
				return err
			}

			result, xe := app.LoadProfileDetail(config.Options{
				ConfigPath: GlobalConfig.ConfigStr,
			}, name)
			if xe != nil {
				return xe
			}

			return w.WriteOK(format, result)
		},
	}
}
