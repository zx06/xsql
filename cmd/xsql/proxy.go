package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/proxy"
	"github.com/zx06/xsql/internal/secret"
	"github.com/zx06/xsql/internal/ssh"
)

// ProxyFlags holds the flags for the proxy command
type ProxyFlags struct {
	LocalPort     int
	LocalHost     string
	AllowPlaintext bool
	SSHSkipHostKey bool
}

// NewProxyCommand creates the proxy command
func NewProxyCommand(w *output.Writer) *cobra.Command {
	flags := &ProxyFlags{}

	cmd := &cobra.Command{
		Use:   "proxy [flags]",
		Short: "Start a port forwarding proxy (replaces ssh -L)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProxy(cmd, flags, w)
		},
	}

	cmd.Flags().IntVar(&flags.LocalPort, "local-port", 0, "Local port to listen on (0 for auto-assign)")
	cmd.Flags().StringVar(&flags.LocalHost, "local-host", "127.0.0.1", "Local host to bind to")
	cmd.Flags().BoolVar(&flags.AllowPlaintext, "allow-plaintext", false, "Allow plaintext secrets in config")
	cmd.Flags().BoolVar(&flags.SSHSkipHostKey, "ssh-skip-known-hosts-check", false, "Skip SSH known_hosts check (dangerous)")

	return cmd
}

// runProxy executes the proxy command
func runProxy(cmd *cobra.Command, flags *ProxyFlags, w *output.Writer) error {
	if GlobalConfig.ProfileStr == "" {
		return errors.New(errors.CodeCfgInvalid, "profile is required (use global -p/--profile flag)", nil)
	}
	profileName := GlobalConfig.ProfileStr
	format, err := parseOutputFormat(GlobalConfig.FormatStr)
	if err != nil {
		return err
	}

	p := GlobalConfig.Resolved.Profile
	if p.DB == "" {
		return errors.New(errors.CodeCfgInvalid, "db type is required (mysql|pg)", nil)
	}

	// Check if SSH proxy is configured
	if p.SSHConfig == nil {
		return errors.New(errors.CodeCfgInvalid, "profile must have ssh_proxy configured for port forwarding", nil)
	}

	// Allow plaintext passwords (CLI > Config)
	allowPlaintext := flags.AllowPlaintext || p.AllowPlaintext

	// Resolve SSH passphrase
	passphrase := p.SSHConfig.Passphrase
	if passphrase != "" {
		pp, xe := secret.Resolve(passphrase, secret.Options{AllowPlaintext: allowPlaintext})
		if xe != nil {
			return xe
		}
		passphrase = pp
	}

	// Setup SSH connection
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sshOpts := ssh.Options{
		Host:                p.SSHConfig.Host,
		Port:                p.SSHConfig.Port,
		User:                p.SSHConfig.User,
		IdentityFile:        p.SSHConfig.IdentityFile,
		Passphrase:          passphrase,
		KnownHostsFile:      p.SSHConfig.KnownHostsFile,
		SkipKnownHostsCheck: flags.SSHSkipHostKey || p.SSHConfig.SkipHostKey,
	}

	sshClient, xe := ssh.Connect(ctx, sshOpts)
	if xe != nil {
		return xe
	}
	defer func() { _ = sshClient.Close() }()

	// Start proxy
	proxyOpts := proxy.Options{
		LocalHost:  flags.LocalHost,
		LocalPort:  flags.LocalPort,
		RemoteHost: p.Host,
		RemotePort: p.Port,
		Dialer:     sshClient,
	}

	px, result, xe := proxy.Start(ctx, proxyOpts)
	if xe != nil {
		return xe
	}
	defer func() { _ = px.Stop() }()

	// Print result based on format
	if format == output.FormatTable {
		// Custom table output for proxy
		fmt.Fprintf(os.Stderr, "✓ Proxy started successfully\n")
		fmt.Fprintf(os.Stderr, "  Local:   %s\n", result.LocalAddress)
		fmt.Fprintf(os.Stderr, "  Remote:  %s (via %s)\n", result.RemoteAddress, p.SSHConfig.Host)
		fmt.Fprintf(os.Stderr, "  Profile: %s\n", profileName)
		fmt.Fprintf(os.Stderr, "\nPress Ctrl+C to stop\n")
	} else {
		// JSON/YAML output
		data := map[string]any{
			"local_address":  result.LocalAddress,
			"remote_address": result.RemoteAddress,
			"ssh_proxy":      fmt.Sprintf("%s:%d", p.SSHConfig.Host, p.SSHConfig.Port),
			"profile":        profileName,
		}
		_ = w.WriteOK(format, data)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-sigChan
	fmt.Fprintf(os.Stderr, "\nShutting down proxy...\n")

	return nil
}
