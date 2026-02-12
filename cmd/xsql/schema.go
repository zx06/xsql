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

// SchemaFlags holds the flags for the schema command
type SchemaFlags struct {
	TablePattern   string
	IncludeSystem  bool
	AllowPlaintext bool
	SSHSkipHostKey bool
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
			return runSchemaDump(cmd, args, flags, w)
		},
	}

	cmd.Flags().StringVar(&flags.TablePattern, "table", "", "Table name filter (supports * and ? wildcards)")
	cmd.Flags().BoolVar(&flags.IncludeSystem, "include-system", false, "Include system tables")
	cmd.Flags().BoolVar(&flags.AllowPlaintext, "allow-plaintext", false, "Allow plaintext secrets in config")
	cmd.Flags().BoolVar(&flags.SSHSkipHostKey, "ssh-skip-known-hosts-check", false, "Skip SSH known_hosts check (dangerous)")

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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, xe := app.ResolveConnection(ctx, app.ConnectionOptions{
		Profile:          p,
		AllowPlaintext:   flags.AllowPlaintext,
		SkipHostKeyCheck: flags.SSHSkipHostKey,
	})
	if xe != nil {
		return xe
	}
	defer conn.Close()

	schemaOpts := db.SchemaOptions{
		TablePattern:  flags.TablePattern,
		IncludeSystem: flags.IncludeSystem,
	}

	result, xe := db.DumpSchema(ctx, p.DB, conn.DB, schemaOpts)
	if xe != nil {
		return xe
	}

	return w.WriteOK(format, result)
}
