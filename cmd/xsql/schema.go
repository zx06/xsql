package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/db"
	_ "github.com/zx06/xsql/internal/db/mysql"
	_ "github.com/zx06/xsql/internal/db/pg"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/secret"
)

const DefaultSchemaTimeout = 60 * time.Second

// SchemaFlags holds the flags for the schema command
type SchemaFlags struct {
	TablePattern     string
	IncludeSystem    bool
	AllowPlaintext   bool
	SSHSkipHostKey   bool
	SchemaTimeout    int
	SchemaTimeoutSet bool
}

// NewSchemaCommand creates the schema command
func NewSchemaCommand(w *output.Writer) *cobra.Command {
	flags := &SchemaFlags{}

	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Database schema operations",
	}

	// Add subcommands
	cmd.AddCommand(NewSchemaDumpCommand(w, flags))

	return cmd
}

// NewSchemaDumpCommand creates the schema dump subcommand
func NewSchemaDumpCommand(w *output.Writer, flags *SchemaFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump database schema (tables, columns, indexes, foreign keys)",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.SchemaTimeoutSet = cmd.Flags().Changed("schema-timeout")
			return runSchemaDump(cmd, args, flags, w)
		},
	}

	cmd.Flags().StringVar(&flags.TablePattern, "table", "", "Table name filter (supports * and ? wildcards)")
	cmd.Flags().BoolVar(&flags.IncludeSystem, "include-system", false, "Include system tables")
	cmd.Flags().BoolVar(&flags.AllowPlaintext, "allow-plaintext", false, "Allow plaintext secrets in config")
	cmd.Flags().BoolVar(&flags.SSHSkipHostKey, "ssh-skip-known-hosts-check", false, "Skip SSH known_hosts check (dangerous)")
	cmd.Flags().IntVar(&flags.SchemaTimeout, "schema-timeout", 0, "Schema dump timeout in seconds (default: 60)")

	return cmd
}

// runSchemaDump executes the schema dump command
func runSchemaDump(cmd *cobra.Command, args []string, flags *SchemaFlags, w *output.Writer) error {
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

	// Schema timeout: CLI flag > profile config > default (60s)
	timeout := DefaultSchemaTimeout
	if flags.SchemaTimeoutSet && flags.SchemaTimeout > 0 {
		timeout = time.Duration(flags.SchemaTimeout) * time.Second
	} else if p.SchemaTimeout > 0 {
		timeout = time.Duration(p.SchemaTimeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

	// Dump schema
	schemaOpts := db.SchemaOptions{
		TablePattern:  flags.TablePattern,
		IncludeSystem: flags.IncludeSystem,
	}

	result, xe := db.DumpSchema(ctx, p.DB, conn, schemaOpts)
	if xe != nil {
		return xe
	}

	return w.WriteOK(format, result)
}
