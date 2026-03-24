package main

import (
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
)

// NewConfigCommand creates the config command group
func NewConfigCommand(w *output.Writer) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	configCmd.AddCommand(newConfigInitCommand(w))
	configCmd.AddCommand(newConfigSetCommand(w))

	return configCmd
}

// newConfigInitCommand creates the config init command
func newConfigInitCommand(w *output.Writer) *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a template configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(GlobalConfig.FormatStr)
			if err != nil {
				return err
			}

			cfgPath, xe := config.InitConfig(path)
			if xe != nil {
				return xe
			}

			return w.WriteOK(format, map[string]any{
				"config_path": cfgPath,
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Config file path (default: $HOME/.config/xsql/xsql.yaml)")

	return cmd
}

// newConfigSetCommand creates the config set command
func newConfigSetCommand(w *output.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value (e.g., profile.dev.host localhost)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			format, err := parseOutputFormat(GlobalConfig.FormatStr)
			if err != nil {
				return err
			}

			cfgPath := config.FindConfigPath(config.Options{
				ConfigPath: GlobalConfig.ConfigStr,
			})
			if cfgPath == "" {
				return errors.New(errors.CodeCfgNotFound, "no config file found; run 'xsql config init' first", nil)
			}

			if xe := config.SetConfigValue(cfgPath, key, value); xe != nil {
				return xe
			}

			return w.WriteOK(format, map[string]any{
				"config_path": cfgPath,
				"key":         key,
				"value":       value,
			})
		},
	}
}
