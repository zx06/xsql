package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/stats"
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
	Stats      stats.StatsConfig
	Attrs      map[string]string
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
				WorkDir:       os.Getenv("XSQL_WORKDIR"),
				HomeDir:       os.Getenv("XSQL_HOMEDIR"),
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

	var cliAttrs []string

	root.PersistentFlags().StringVar(&GlobalConfig.ConfigStr, "config", "", "Config file path (YAML); default: ./xsql.yaml or $HOME/.config/xsql/xsql.yaml")
	root.PersistentFlags().StringVarP(&GlobalConfig.ProfileStr, "profile", "p", "", "Profile name (config: profiles.<name>)")
	root.PersistentFlags().StringVarP(&GlobalConfig.FormatStr, "format", "f", "auto", "Output format: json|yaml|table|csv|auto")
	root.PersistentFlags().StringArrayVar(&cliAttrs, "attr", nil, "Attribute key=value pair (repeatable)")

	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
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
			WorkDir:       os.Getenv("XSQL_WORKDIR"),
			HomeDir:       os.Getenv("XSQL_HOMEDIR"),
		})
		if xe != nil {
			return xe
		}
		GlobalConfig.Resolved = r
		GlobalConfig.FormatStr = r.Format
		GlobalConfig.ProfileStr = r.ProfileName

		// Parse attributes: CLI > ENV
		GlobalConfig.Attrs = stats.ParseAttrs(cliAttrs, stats.GetXSQLAttrEnv())

		// Load stats config from file
		GlobalConfig.Stats = loadStatsConfig(GlobalConfig.ConfigStr)

		return nil
	}

	return root
}

// loadStatsConfig loads stats configuration from the config file.
func loadStatsConfig(configPath string) stats.StatsConfig {
	cfg, _, xe := config.LoadConfig(config.Options{ConfigPath: configPath})
	if xe != nil {
		return stats.StatsConfig{}
	}
	return cfg.Stats
}
