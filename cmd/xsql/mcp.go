package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	mcp_pkg "github.com/zx06/xsql/internal/mcp"
)

// NewMCPCommand creates the MCP command group
func NewMCPCommand() *cobra.Command {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP (Model Context Protocol) server commands",
	}

	mcpCmd.AddCommand(newMCPServerCommand())

	return mcpCmd
}

// newMCPServerCommand creates the MCP server command
func newMCPServerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start MCP server for AI assistant integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServer()
		},
	}
}

// runMCPServer runs the MCP server
func runMCPServer() error {
	// Load config
	cfg, _, xe := config.LoadConfig(config.Options{
		ConfigPath: GlobalConfig.ConfigStr,
	})
	if xe != nil {
		return xe
	}

	// Create MCP server using official SDK
	server, err := mcp_pkg.CreateServer(version, &cfg)
	if err != nil {
		// Convert SDK error to XError if needed
		if xe, ok := err.(*errors.XError); ok {
			return xe
		}
		return errors.Wrap(errors.CodeInternal, "failed to create MCP server", nil, err)
	}

	// Run server over stdin/stdout
	ctx := context.Background()
	return server.Run(ctx, &mcp.StdioTransport{})
}
