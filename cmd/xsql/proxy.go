package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/proxy"
	"github.com/zx06/xsql/internal/ssh"
)

// ProxyFlags holds the flags for the proxy command
type ProxyFlags struct {
	LocalPort      int
	LocalHost      string
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

// resolveProxyPort determines the port to use with the following priority:
// CLI --local-port > profile.local_port > 0 (auto)
// Returns the port and whether it came from config (for conflict handling).
func resolveProxyPort(cmd *cobra.Command, flags *ProxyFlags, profileLocalPort int) (port int, fromConfig bool) {
	if cmd != nil && cmd.Flags().Changed("local-port") {
		return flags.LocalPort, false
	}
	if profileLocalPort > 0 {
		return profileLocalPort, true
	}
	return 0, false
}

// handlePortConflict handles a port conflict when the port comes from config.
// In TTY mode, prompts the user to choose random port or quit.
// In non-TTY mode, returns an error.
func handlePortConflict(port int, host string) (int, *errors.XError) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return 0, errors.New(errors.CodePortInUse, "configured port is already in use",
			map[string]any{"port": port, "host": host})
	}

	fmt.Fprintf(os.Stderr, "⚠ Port %d is already in use.\n", port)
	fmt.Fprintf(os.Stderr, "  [R] Use a random port\n")
	fmt.Fprintf(os.Stderr, "  [Q] Quit\n")
	fmt.Fprintf(os.Stderr, "Choice [R/Q]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "r", "":
		return 0, nil // 0 means auto-assign
	case "q":
		return 0, errors.New(errors.CodePortInUse, "user chose to quit due to port conflict",
			map[string]any{"port": port})
	default:
		return 0, errors.New(errors.CodePortInUse, "user chose to quit due to port conflict",
			map[string]any{"port": port})
	}
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

	if p.SSHConfig == nil {
		return errors.New(errors.CodeCfgInvalid, "profile must have ssh_proxy configured for port forwarding", nil)
	}

	// Resolve port: CLI > config local_port > 0 (auto)
	localPort, fromConfig := resolveProxyPort(cmd, flags, p.LocalPort)

	// Check for port conflict if a specific port is configured
	if localPort > 0 && !proxy.IsPortAvailable(flags.LocalHost, localPort) {
		if fromConfig {
			// Port from config: offer interactive choice
			newPort, xe := handlePortConflict(localPort, flags.LocalHost)
			if xe != nil {
				return xe
			}
			localPort = newPort
		}
		// If port from CLI flag, let proxy.Start handle the error naturally
	}

	allowPlaintext := flags.AllowPlaintext || p.AllowPlaintext

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use ReconnectDialer for automatic SSH reconnection
	onStatus := func(event ssh.StatusEvent) {
		switch event.Type {
		case ssh.StatusDisconnected:
			log.Printf("[proxy] SSH connection lost: %v", event.Error)
		case ssh.StatusReconnecting:
			log.Printf("[proxy] reconnecting to SSH server...")
		case ssh.StatusReconnected:
			log.Printf("[proxy] SSH reconnected successfully")
		case ssh.StatusReconnectFailed:
			log.Printf("[proxy] SSH reconnection failed: %v", event.Error)
		}
	}

	reconnDialer, xe := app.ResolveReconnectableSSH(ctx, p, allowPlaintext, flags.SSHSkipHostKey, onStatus)
	if xe != nil {
		return xe
	}
	defer func() {
		if err := reconnDialer.Close(); err != nil {
			log.Printf("[proxy] failed to close ssh dialer: %v", err)
		}
	}()

	proxyOpts := proxy.Options{
		LocalHost:  flags.LocalHost,
		LocalPort:  localPort,
		RemoteHost: p.Host,
		RemotePort: p.Port,
		Dialer:     reconnDialer,
	}

	px, result, xe := proxy.Start(ctx, proxyOpts)
	if xe != nil {
		return xe
	}
	defer func() { _ = px.Stop() }()

	if format == output.FormatTable {
		fmt.Fprintf(os.Stderr, "✓ Proxy started successfully\n")
		fmt.Fprintf(os.Stderr, "  Local:   %s\n", result.LocalAddress)
		fmt.Fprintf(os.Stderr, "  Remote:  %s (via %s)\n", result.RemoteAddress, p.SSHConfig.Host)
		fmt.Fprintf(os.Stderr, "  Profile: %s\n", profileName)
		fmt.Fprintf(os.Stderr, "\nPress Ctrl+C to stop\n")
	} else {
		data := map[string]any{
			"local_address":  result.LocalAddress,
			"remote_address": result.RemoteAddress,
			"ssh_proxy":      fmt.Sprintf("%s:%d", p.SSHConfig.Host, p.SSHConfig.Port),
			"profile":        profileName,
		}
		_ = w.WriteOK(format, data)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	fmt.Fprintf(os.Stderr, "\nShutting down proxy...\n")

	return nil
}
