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

// Build-time variables (set by goreleaser)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	exit := run()
	os.Exit(exit)
}

func run() int {
	a := app.New(version, commit, date)
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
	root.PersistentFlags().StringVarP(&profileStr, "profile", "p", "", "Profile name (config: profiles.<name>)")
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
		queryUnsafeAllowWrite bool
		queryAllowPlaintext   bool
		querySSHSkipHostKey   bool
	)
	queryCmd := &cobra.Command{
		Use:   "query [SQL]",
		Short: "Execute a SQL query (read-only by default)",
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

			// 允许明文密码（CLI > Config）
			allowPlaintext := queryAllowPlaintext || p.AllowPlaintext

			// 解析 password（支持 keyring）
			password := p.Password
			if password != "" {
				pw, xe := secret.Resolve(password, secret.Options{AllowPlaintext: allowPlaintext})
				if xe != nil {
					return xe
				}
				password = pw
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// SSH proxy（如配置）
			var sshClient *ssh.Client
			if p.SSHConfig != nil {
				passphrase := p.SSHConfig.Passphrase
				if passphrase != "" {
					pp, xe := secret.Resolve(passphrase, secret.Options{AllowPlaintext: allowPlaintext})
					if xe != nil {
						return xe
					}
					passphrase = pp
				}
				sshOpts := ssh.Options{
					Host:                p.SSHConfig.Host,
					Port:                p.SSHConfig.Port,
					User:                p.SSHConfig.User,
					IdentityFile:        p.SSHConfig.IdentityFile,
					Passphrase:          passphrase,
					KnownHostsFile:      p.SSHConfig.KnownHostsFile,
					SkipKnownHostsCheck: querySSHSkipHostKey || p.SSHConfig.SkipHostKey,
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
			}
			if sshClient != nil {
				connOpts.Dialer = sshClient
			}

			conn, xe := drv.Open(ctx, connOpts)
			if xe != nil {
				return xe
			}
			defer conn.Close()

			unsafeAllowWrite := queryUnsafeAllowWrite || p.UnsafeAllowWrite
			result, xe := db.Query(ctx, conn, sql, db.QueryOptions{
				ReadOnly:         !unsafeAllowWrite,
				UnsafeAllowWrite: unsafeAllowWrite,
				DBType:           p.DB,
			})
			if xe != nil {
				return xe
			}
			return w.WriteOK(format, result)
		},
	}
	queryCmd.Flags().BoolVar(&queryUnsafeAllowWrite, "unsafe-allow-write", false, "Allow write operations (bypasses read-only protection)")
	queryCmd.Flags().BoolVar(&queryAllowPlaintext, "allow-plaintext", false, "Allow plaintext secrets in config")
	queryCmd.Flags().BoolVar(&querySSHSkipHostKey, "ssh-skip-known-hosts-check", false, "Skip SSH known_hosts check (dangerous)")

	root.AddCommand(specCmd, verCmd, queryCmd)

	// profile 命令组
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles",
	}

	profileListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(formatStr)
			if err != nil {
				return err
			}

			cfg, cfgPath, xe := config.LoadConfig(config.Options{
				ConfigPath: configStr,
			})
			if xe != nil {
				return xe
			}

			type profileInfo struct {
				Name string `json:"name"`
				DB   string `json:"db"`
				Mode string `json:"mode"` // "read-only" or "read-write"
			}
			profiles := make([]profileInfo, 0, len(cfg.Profiles))
			for name, p := range cfg.Profiles {
				mode := "read-only"
				if p.UnsafeAllowWrite {
					mode = "read-write"
				}
				profiles = append(profiles, profileInfo{
					Name: name,
					DB:   p.DB,
					Mode: mode,
				})
			}

			result := map[string]any{
				"config_path": cfgPath,
				"profiles":    profiles,
			}
			return w.WriteOK(format, result)
		},
	}

	profileShowCmd := &cobra.Command{
		Use:   "show [name]",
		Short: "Show profile details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			format, err := parseOutputFormat(formatStr)
			if err != nil {
				return err
			}

			cfg, cfgPath, xe := config.LoadConfig(config.Options{
				ConfigPath: configStr,
			})
			if xe != nil {
				return xe
			}

			profile, ok := cfg.Profiles[name]
			if !ok {
				return errors.New(errors.CodeCfgInvalid, "profile not found", map[string]any{"name": name})
			}

			// 脱敏输出：隐藏密码
			result := map[string]any{
				"config_path":        cfgPath,
				"name":               name,
				"db":                 profile.DB,
				"host":               profile.Host,
				"port":               profile.Port,
				"user":               profile.User,
				"database":           profile.Database,
				"unsafe_allow_write": profile.UnsafeAllowWrite,
				"allow_plaintext":    profile.AllowPlaintext,
			}
			if profile.DSN != "" {
				result["dsn"] = "***"
			}
			if profile.Password != "" {
				result["password"] = "***"
			}
			if profile.SSHProxy != "" {
				result["ssh_proxy"] = profile.SSHProxy
				if proxy, ok := cfg.SSHProxies[profile.SSHProxy]; ok {
					result["ssh_host"] = proxy.Host
					result["ssh_port"] = proxy.Port
					result["ssh_user"] = proxy.User
					if proxy.IdentityFile != "" {
						result["ssh_identity_file"] = proxy.IdentityFile
					}
				}
			}
			return w.WriteOK(format, result)
		},
	}

	profileCmd.AddCommand(profileListCmd, profileShowCmd)
	root.AddCommand(profileCmd)

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
	// 保留原始错误信息
	return errors.Wrap(errors.CodeInternal, err.Error(), nil, err)
}
