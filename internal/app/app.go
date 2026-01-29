package app

import (
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/spec"
)

type App struct {
	Version string
}

func New(version string) App {
	return App{Version: version}
}

func (a App) BuildSpec() spec.Spec {
	return spec.Spec{
		SchemaVersion: output.SchemaVersion,
		Commands: []spec.CommandSpec{
			{
				Name:        "spec",
				Description: "Export tool spec for AI/agents",
				Flags: []spec.FlagSpec{
					{Name: "format", Shorthand: "f", Env: "XSQL_FORMAT", Default: "auto", Description: "Output format: json|yaml|table|csv|auto"},
				},
			},
			{
				Name:        "version",
				Description: "Print version information",
				Flags: []spec.FlagSpec{
					{Name: "format", Shorthand: "f", Env: "XSQL_FORMAT", Default: "auto", Description: "Output format: json|yaml|table|csv|auto"},
				},
			},
		},
		ErrorCodes: errors.AllCodes(),
	}
}

type VersionInfo struct {
	Version string `json:"version" yaml:"version"`
}

func (a App) VersionInfo() VersionInfo {
	return VersionInfo{Version: a.Version}
}
