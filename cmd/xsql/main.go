package main

import (
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
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

	var (
		formatStr  string
		configStr  string
		profileStr string
	)
	root := &cobra.Command{
		Use:           "xsql",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// CLI > ENV > Config
			formatSet := cmd.Flags().Changed("format")
			profileSet := cmd.Flags().Changed("profile")
			configSet := cmd.Flags().Changed("config")
			if configSet && configStr == "" {
				return errors.New(errors.CodeCfgInvalid, "config path is empty", nil)
			}

			resolved, xe := config.Resolve(config.Options{
				ConfigPath:    configStr,
				CLIProfile:    profileStr,
				CLIProfileSet: profileSet,
				CLIFormat:     formatStr,
				CLIFormatSet:  formatSet,
				EnvProfile:    os.Getenv("XSQL_PROFILE"),
				EnvFormat:     os.Getenv("XSQL_FORMAT"),
				WorkDir:       "",
				HomeDir:       "",
			})
			if xe != nil {
				return xe
			}
			formatStr = resolved.Format
			profileStr = resolved.ProfileName
			return nil
		},
	}
	root.PersistentFlags().StringVar(&configStr, "config", "", "Config file path (YAML); default: ./xsql.yaml or $HOME/.config/xsql/xsql.yaml")
	root.PersistentFlags().StringVar(&profileStr, "profile", "", "Profile name (config: profiles.<name>)")
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
