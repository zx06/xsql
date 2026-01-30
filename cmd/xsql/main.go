package main

import (
	"os"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
)

func main() {
	exit := run()
	os.Exit(exit)
}

// run is the main entry point
func run() int {
	// Initialize application
	a := app.New(version, commit, date)
	w := output.New(os.Stdout, os.Stderr)

	// Create root command
	root := NewRootCommand()

	// Add subcommands
	root.AddCommand(NewSpecCommand(&a, &w))
	root.AddCommand(NewVersionCommand(&a, &w))
	root.AddCommand(NewQueryCommand(&w))
	root.AddCommand(NewProfileCommand(&w))
	root.AddCommand(NewMCPCommand())

	// Execute and handle errors
	if err := root.Execute(); err != nil {
		xe := normalizeErr(err)
		format := resolveFormatForError(GlobalConfig.FormatStr)
		_ = w.WriteError(format, xe)
		return int(errors.ExitCodeFor(xe.Code))
	}

	return int(errors.ExitOK)
}
