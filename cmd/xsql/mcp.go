package main

import (
	"context"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	mcp_pkg "github.com/zx06/xsql/internal/mcp"
	"github.com/zx06/xsql/internal/secret"
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
	opts := &mcpServerOptions{}
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start MCP server for AI assistant integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.transportSet = cmd.Flags().Changed("transport")
			opts.httpAddrSet = cmd.Flags().Changed("http-addr")
			opts.httpAuthTokenSet = cmd.Flags().Changed("http-auth-token")
			return runMCPServer(opts)
		},
	}
	cmd.Flags().StringVar(&opts.transport, "transport", mcp_pkg.TransportStdio, "MCP transport: stdio|streamable_http")
	cmd.Flags().StringVar(&opts.httpAddr, "http-addr", "127.0.0.1:8787", "Streamable HTTP listen address")
	cmd.Flags().StringVar(&opts.httpAuthToken, "http-auth-token", "", "Streamable HTTP auth token (required for streamable_http)")
	return cmd
}

// runMCPServer runs the MCP server
func runMCPServer(opts *mcpServerOptions) error {
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

	resolved, xe := resolveMCPServerOptions(opts, cfg)
	if xe != nil {
		return xe
	}

	switch resolved.transport {
	case mcp_pkg.TransportStdio:
		ctx := context.Background()
		return server.Run(ctx, &mcp.StdioTransport{})
	case mcp_pkg.TransportStreamableHTTP:
		handler, err := mcp_pkg.NewStreamableHTTPHandler(server, resolved.httpAuthToken)
		if err != nil {
			if xe, ok := err.(*errors.XError); ok {
				return xe
			}
			return errors.Wrap(errors.CodeInternal, "failed to create streamable http handler", nil, err)
		}
		httpServer := &http.Server{
			Addr:    resolved.httpAddr,
			Handler: handler,
		}
		return httpServer.ListenAndServe()
	default:
		return errors.New(errors.CodeCfgInvalid, "unsupported mcp transport", map[string]any{"transport": resolved.transport})
	}
}

type mcpServerOptions struct {
	transport        string
	transportSet     bool
	httpAddr         string
	httpAddrSet      bool
	httpAuthToken    string
	httpAuthTokenSet bool
}

type mcpServerResolved struct {
	transport     string
	httpAddr      string
	httpAuthToken string
}

func resolveMCPServerOptions(opts *mcpServerOptions, cfg config.File) (mcpServerResolved, *errors.XError) {
	if opts == nil {
		opts = &mcpServerOptions{}
	}

	transport := firstNonEmpty(
		valueIfSet(opts.transportSet, opts.transport),
		os.Getenv("XSQL_MCP_TRANSPORT"),
		cfg.MCP.Transport,
	)
	if transport == "" {
		transport = mcp_pkg.TransportStdio
	}
	if transport != mcp_pkg.TransportStdio && transport != mcp_pkg.TransportStreamableHTTP {
		return mcpServerResolved{}, errors.New(errors.CodeCfgInvalid, "invalid mcp transport", map[string]any{"transport": transport})
	}

	httpAddr := firstNonEmpty(
		valueIfSet(opts.httpAddrSet, opts.httpAddr),
		os.Getenv("XSQL_MCP_HTTP_ADDR"),
		cfg.MCP.HTTP.Addr,
	)
	if httpAddr == "" {
		httpAddr = "127.0.0.1:8787"
	}

	authToken := firstNonEmpty(
		valueIfSet(opts.httpAuthTokenSet, opts.httpAuthToken),
		os.Getenv("XSQL_MCP_HTTP_AUTH_TOKEN"),
	)
	if authToken == "" && cfg.MCP.HTTP.AuthToken != "" {
		secretValue, xe := secret.Resolve(cfg.MCP.HTTP.AuthToken, secret.Options{
			AllowPlaintext: cfg.MCP.HTTP.AllowPlaintextToken,
		})
		if xe != nil {
			return mcpServerResolved{}, xe
		}
		authToken = secretValue
	}

	if transport == mcp_pkg.TransportStreamableHTTP && authToken == "" {
		return mcpServerResolved{}, errors.New(errors.CodeCfgInvalid, "streamable http transport requires auth token", nil)
	}

	return mcpServerResolved{
		transport:     transport,
		httpAddr:      httpAddr,
		httpAuthToken: authToken,
	}, nil
}

func valueIfSet(set bool, value string) string {
	if !set {
		return ""
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
