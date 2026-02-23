package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, xe := app.ResolveConnection(ctx, app.ConnectionOptions{
		Profile:          p,
		AllowPlaintext:   flags.AllowPlaintext,
		SkipHostKeyCheck: flags.SSHSkipHostKey,
	})
	if xe != nil {
		return xe
	}
	defer func() { _ = conn.Close() }()

	unsafeAllowWrite := flags.UnsafeAllowWrite || p.UnsafeAllowWrite
	result, xe := db.Query(ctx, conn.DB, sql, db.QueryOptions{
		UnsafeAllowWrite: unsafeAllowWrite,
		DBType:           p.DB,
	})
	if xe != nil {
		return xe
	}

	return w.WriteOK(format, result)
}
