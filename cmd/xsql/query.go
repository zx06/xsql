package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/output"
)

const DefaultQueryTimeout = 30 * time.Second

// QueryFlags holds the flags for the query command
type QueryFlags struct {
	UnsafeAllowWrite bool
	AllowPlaintext   bool
	SSHSkipHostKey   bool
	QueryTimeout     int
	QueryTimeoutSet  bool
}

// NewQueryCommand creates the query command
func NewQueryCommand(w *output.Writer) *cobra.Command {
	flags := &QueryFlags{}

	cmd := &cobra.Command{
		Use:   "query [SQL]",
		Short: "Execute a SQL query (read-only by default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.QueryTimeoutSet = cmd.Flags().Changed("query-timeout")
			return runQuery(cmd, args, flags, w)
		},
	}

	cmd.Flags().BoolVar(&flags.UnsafeAllowWrite, "unsafe-allow-write", false, "Allow write operations (bypasses read-only protection)")
	cmd.Flags().BoolVar(&flags.AllowPlaintext, "allow-plaintext", false, "Allow plaintext secrets in config")
	cmd.Flags().BoolVar(&flags.SSHSkipHostKey, "ssh-skip-known-hosts-check", false, "Skip SSH known_hosts check (dangerous)")
	cmd.Flags().IntVar(&flags.QueryTimeout, "query-timeout", 0, "Query timeout in seconds (default: 30)")

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
	timeout := app.QueryTimeout(p, flags.QueryTimeout, flags.QueryTimeoutSet, DefaultQueryTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, xe := app.Query(ctx, app.QueryRequest{
		Profile:          p,
		SQL:              sql,
		AllowPlaintext:   flags.AllowPlaintext,
		SkipHostKeyCheck: flags.SSHSkipHostKey,
		UnsafeAllowWrite: flags.UnsafeAllowWrite || p.UnsafeAllowWrite,
	})
	if xe != nil {
		return xe
	}

	return w.WriteOK(format, result)
}
