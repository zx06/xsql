package main

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/secret"
	"github.com/zx06/xsql/internal/ssh"
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
		resolved   config.Resolved
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

			r, xe := config.Resolve(config.Options{
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
			resolved = r
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

	var (
		queryReadOnly       bool
		queryUnsafeWrite    bool
		queryAllowPlaintext bool
		querySSHSkipHostKey bool
	)
	queryCmd := &cobra.Command{
		Use:   "query [SQL]",
		Short: "Execute a read-only SQL query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sql := args[0]
			format, err := parseOutputFormat(formatStr)
			if err != nil {
				return err
			}

			p := resolved.Profile
			if p.DB == "" {
				return errors.New(errors.CodeCfgInvalid, "db type is required (mysql|pg)", nil)
			}

			// 解析 password（支持 keyring）
			password := p.Password
			if password != "" {
				pw, xe := secret.Resolve(password, secret.Options{AllowPlaintext: queryAllowPlaintext})
				if xe != nil {
					return xe
				}
				password = pw
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// SSH proxy（如配置）
			var sshClient *ssh.Client
			if p.SSHHost != "" {
				passphrase := p.SSHPassphrase
				if passphrase != "" {
					pp, xe := secret.Resolve(passphrase, secret.Options{AllowPlaintext: queryAllowPlaintext})
					if xe != nil {
						return xe
					}
					passphrase = pp
				}
				sshOpts := ssh.Options{
					Host:                p.SSHHost,
					Port:                p.SSHPort,
					User:                p.SSHUser,
					IdentityFile:        p.SSHIdentityFile,
					Passphrase:          passphrase,
					KnownHostsFile:      p.SSHKnownHostsFile,
					SkipKnownHostsCheck: querySSHSkipHostKey || p.SSHSkipHostKey,
				}
				sc, xe := ssh.Connect(ctx, sshOpts)
				if xe != nil {
					return xe
				}
				defer sc.Close()
				sshClient = sc
			}

			// 获取 driver
			drv, ok := db.Get(p.DB)
			if !ok {
				return errors.New(errors.CodeDBDriverUnsupported, "unsupported db driver", map[string]any{"db": p.DB})
			}

			connOpts := db.ConnOptions{
				DSN:      p.DSN,
				Host:     p.Host,
				Port:     p.Port,
				User:     p.User,
				Password: password,
				Database: p.Database,
				Dialer:   sshClient, // SSH tunnel（nil 时走直连）
			}

			conn, xe := drv.Open(ctx, connOpts)
			if xe != nil {
				return xe
			}
			defer conn.Close()

			readOnly := queryReadOnly || p.ReadOnly
			result, xe := db.Query(ctx, conn, sql, readOnly, queryUnsafeWrite)
			if xe != nil {
				return xe
			}
			return w.WriteOK(format, result)
		},
	}
	queryCmd.Flags().BoolVar(&queryReadOnly, "read-only", true, "Enforce read-only mode (default: true)")
	queryCmd.Flags().BoolVar(&queryUnsafeWrite, "unsafe-allow-write", false, "Bypass read-only check (dangerous)")
	queryCmd.Flags().BoolVar(&queryAllowPlaintext, "allow-plaintext", false, "Allow plaintext secrets in config")
	queryCmd.Flags().BoolVar(&querySSHSkipHostKey, "ssh-skip-known-hosts-check", false, "Skip SSH known_hosts check (dangerous)")

	root.AddCommand(specCmd, verCmd, queryCmd)

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
