package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/output"
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
	timeout := app.SchemaTimeout(p, flags.SchemaTimeout, flags.SchemaTimeoutSet, DefaultSchemaTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, xe := app.DumpSchema(ctx, app.SchemaDumpRequest{
		Profile:          p,
		TablePattern:     flags.TablePattern,
		IncludeSystem:    flags.IncludeSystem,
		AllowPlaintext:   flags.AllowPlaintext,
		SkipHostKeyCheck: flags.SSHSkipHostKey,
	})
	if xe != nil {
		return xe
	}

	return w.WriteOK(format, result)
}
