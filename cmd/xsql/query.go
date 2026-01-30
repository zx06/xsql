package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/secret"
	"github.com/zx06/xsql/internal/ssh"
)

// QueryFlags holds the flags for the query command
type QueryFlags struct {
	UnsafeAllowWrite bool
	AllowPlaintext   bool
	SSHSkipHostKey   bool
}

// NewQueryCommand creates the query command
func NewQueryCommand(w *output.Writer) *cobra.Command {
	flags := &QueryFlags{}

	cmd := &cobra.Command{
		Use:   "query [SQL]",
		Short: "Execute a SQL query (read-only by default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuery(cmd, args, flags, w)
		},
	}

	cmd.Flags().BoolVar(&flags.UnsafeAllowWrite, "unsafe-allow-write", false, "Allow write operations (bypasses read-only protection)")
	cmd.Flags().BoolVar(&flags.AllowPlaintext, "allow-plaintext", false, "Allow plaintext secrets in config")
	cmd.Flags().BoolVar(&flags.SSHSkipHostKey, "ssh-skip-known-hosts-check", false, "Skip SSH known_hosts check (dangerous)")

	return cmd
}

// runQuery executes a SQL query
func runQuery(cmd *cobra.Command, args []string, flags *QueryFlags, w *output.Writer) error {
	sql := args[0]
	format, err := parseOutputFormat(GlobalConfig.FormatStr)
	if err != nil {
		return err
	}

	p := GlobalConfig.Resolved.Profile
	if p.DB == "" {
		return errors.New(errors.CodeCfgInvalid, "db type is required (mysql|pg)", nil)
	}

	// Allow plaintext passwords (CLI > Config)
	allowPlaintext := flags.AllowPlaintext || p.AllowPlaintext

	// Resolve password (supports keyring)
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

	// SSH proxy (if configured)
	sshClient, err := setupSSH(ctx, p, allowPlaintext, flags.SSHSkipHostKey)
	if err != nil {
		return err
	}
	if sshClient != nil {
		defer sshClient.Close()
	}

	// Get driver
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

	unsafeAllowWrite := flags.UnsafeAllowWrite || p.UnsafeAllowWrite
	result, xe := db.Query(ctx, conn, sql, db.QueryOptions{
		UnsafeAllowWrite: unsafeAllowWrite,
		DBType:           p.DB,
	})
	if xe != nil {
		return xe
	}

	return w.WriteOK(format, result)
}

// setupSSH sets up SSH proxy connection
func setupSSH(ctx context.Context, p config.Profile, allowPlaintext, skipHostKeyCheck bool) (*ssh.Client, error) {
	if p.SSHConfig == nil {
		return nil, nil
	}

	passphrase := p.SSHConfig.Passphrase
	if passphrase != "" {
		pp, xe := secret.Resolve(passphrase, secret.Options{AllowPlaintext: allowPlaintext})
		if xe != nil {
			return nil, xe
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
		SkipKnownHostsCheck: skipHostKeyCheck || p.SSHConfig.SkipHostKey,
	}

	sc, xe := ssh.Connect(ctx, sshOpts)
	if xe != nil {
		return nil, xe
	}

	return sc, nil
}
