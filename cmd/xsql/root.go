package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
)

// Build-time variables (set by goreleaser)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Config holds the resolved configuration
type Config struct {
	FormatStr  string
	ConfigStr  string
	ProfileStr string
	Resolved   config.Resolved
}

// GlobalConfig holds the global configuration state
var GlobalConfig = &Config{}

// NewRootCommand creates the root command
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "xsql",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// CLI > ENV > Config
			formatSet := cmd.Flags().Changed("format")
			profileSet := cmd.Flags().Changed("profile")
			configSet := cmd.Flags().Changed("config")
			if configSet && GlobalConfig.ConfigStr == "" {
				return errors.New(errors.CodeCfgInvalid, "config path is empty", nil)
			}

			r, xe := config.Resolve(config.Options{
				ConfigPath:    GlobalConfig.ConfigStr,
				CLIProfile:    GlobalConfig.ProfileStr,
				CLIProfileSet: profileSet,
				CLIFormat:     GlobalConfig.FormatStr,
				CLIFormatSet:  formatSet,
				EnvProfile:    os.Getenv("XSQL_PROFILE"),
				EnvFormat:     os.Getenv("XSQL_FORMAT"),
				WorkDir:       "",
				HomeDir:       "",
			})
			if xe != nil {
				return xe
			}
			GlobalConfig.Resolved = r
			GlobalConfig.FormatStr = r.Format
			GlobalConfig.ProfileStr = r.ProfileName
			return nil
		},
	}

	root.PersistentFlags().StringVar(&GlobalConfig.ConfigStr, "config", "", "Config file path (YAML); default: ./xsql.yaml or $HOME/.config/xsql/xsql.yaml")
	root.PersistentFlags().StringVarP(&GlobalConfig.ProfileStr, "profile", "p", "", "Profile name (config: profiles.<name>)")
	root.PersistentFlags().StringVarP(&GlobalConfig.FormatStr, "format", "f", "auto", "Output format: json|yaml|table|csv|auto")

	return root
}