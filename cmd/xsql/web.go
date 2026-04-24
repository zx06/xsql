package main

import (
	"context"
	stderrors "errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/secret"
	webpkg "github.com/zx06/xsql/internal/web"
)

var openBrowser = openBrowserDefault

type webCommandOptions struct {
	addr           string
	addrSet        bool
	authToken      string
	authTokenSet   bool
	allowPlaintext bool
	skipHostKey    bool
	openBrowser    bool
}

type resolvedWebOptions struct {
	addr         string
	authToken    string
	authRequired bool
}

// NewServeCommand starts the embedded web server.
func NewServeCommand(w *output.Writer) *cobra.Command {
	return newWebCommand("serve", "Start the local web management server", false, w)
}

// NewWebCommand starts the embedded web server and opens a browser.
func NewWebCommand(w *output.Writer) *cobra.Command {
	return newWebCommand("web", "Start the local web management server and open a browser", true, w)
}

func newWebCommand(use, short string, shouldOpenBrowser bool, w *output.Writer) *cobra.Command {
	opts := &webCommandOptions{openBrowser: shouldOpenBrowser}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.addrSet = cmd.Flags().Changed("addr")
			opts.authTokenSet = cmd.Flags().Changed("auth-token")
			return runWebCommand(opts, w)
		},
	}
	cmd.Flags().StringVar(&opts.addr, "addr", webpkg.DefaultAddr, "Web HTTP listen address")
	cmd.Flags().StringVar(&opts.authToken, "auth-token", "", "Bearer auth token (required for non-loopback addresses)")
	cmd.Flags().BoolVar(&opts.allowPlaintext, "allow-plaintext", false, "Allow plaintext secrets in config")
	cmd.Flags().BoolVar(&opts.skipHostKey, "ssh-skip-known-hosts-check", false, "Skip SSH known_hosts check (dangerous)")
	return cmd
}

// runWebCommand initializes and runs the web server with signal handling.
func runWebCommand(opts *webCommandOptions, w *output.Writer) error {
	cfg, _, xe := config.LoadConfig(config.Options{
		ConfigPath: GlobalConfig.ConfigStr,
	})
	if xe != nil {
		return xe
	}

	resolved, xe := resolveWebOptions(opts, cfg)
	if xe != nil {
		return xe
	}

	listener, err := net.Listen("tcp", resolved.addr)
	if err != nil {
		if opErr := (*net.OpError)(nil); stderrors.As(err, &opErr) {
			return errors.Wrap(errors.CodePortInUse, "failed to listen on web address", map[string]any{"addr": resolved.addr}, err)
		}
		return errors.Wrap(errors.CodeInternal, "failed to listen on web address", map[string]any{"addr": resolved.addr}, err)
	}
	defer func() {
		_ = listener.Close()
	}()

	handler := webpkg.NewHandler(webpkg.HandlerOptions{
		ConfigPath:       GlobalConfig.ConfigStr,
		InitialProfile:   GlobalConfig.ProfileStr,
		AllowPlaintext:   opts.allowPlaintext,
		SkipHostKeyCheck: opts.skipHostKey,
		AuthRequired:     resolved.authRequired,
		AuthToken:        resolved.authToken,
	})
	server := webpkg.NewServer(listener, handler)
	url := webpkg.PublicURL(server.Addr())

	format, err := parseOutputFormat(GlobalConfig.FormatStr)
	if err != nil {
		return err
	}
	if err := w.WriteOK(format, map[string]any{
		"addr":          server.Addr(),
		"url":           url,
		"auth_required": resolved.authRequired,
		"mode":          modeForWebCommand(opts.openBrowser),
	}); err != nil {
		return err
	}

	if opts.openBrowser {
		if err := openBrowser(url); err != nil {
			log.Printf("[web] failed to open browser: %v", err)
		}
	}

	return runServerWithSignalHandling(server)
}

// runServerWithSignalHandling manages the web server lifecycle with OS signal handling.
// This function is extracted for testing and reusability.
func runServerWithSignalHandling(server *webpkg.Server) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	select {
	case sig := <-sigChan:
		_ = sig
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if shutdownErr := server.Shutdown(ctx); shutdownErr != nil {
			return errors.Wrap(errors.CodeInternal, "failed to shutdown web server", nil, shutdownErr)
		}
		return nil
	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return errors.Wrap(errors.CodeInternal, "web server stopped unexpectedly", nil, err)
	}
}

func resolveWebOptions(opts *webCommandOptions, cfg config.File) (resolvedWebOptions, *errors.XError) {
	if opts == nil {
		opts = &webCommandOptions{}
	}

	addr := firstNonEmpty(
		valueIfSet(opts.addrSet, opts.addr),
		os.Getenv("XSQL_WEB_HTTP_ADDR"),
		cfg.Web.HTTP.Addr,
	)
	if addr == "" {
		addr = webpkg.DefaultAddr
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return resolvedWebOptions{}, errors.New(errors.CodeCfgInvalid, "invalid web listen address", map[string]any{"addr": addr})
	}

	authToken := firstNonEmpty(
		valueIfSet(opts.authTokenSet, opts.authToken),
		os.Getenv("XSQL_WEB_HTTP_AUTH_TOKEN"),
	)
	if authToken == "" && cfg.Web.HTTP.AuthToken != "" {
		secretValue, xe := secret.Resolve(cfg.Web.HTTP.AuthToken, secret.Options{
			AllowPlaintext: cfg.Web.HTTP.AllowPlaintextToken,
		})
		if xe != nil {
			return resolvedWebOptions{}, xe
		}
		authToken = secretValue
	}

	authRequired := !webpkg.IsLoopbackAddr(addr)
	if authRequired && authToken == "" {
		return resolvedWebOptions{}, errors.New(errors.CodeCfgInvalid, "web auth token is required for non-loopback addresses", map[string]any{"addr": addr})
	}

	return resolvedWebOptions{
		addr:         addr,
		authToken:    authToken,
		authRequired: authRequired,
	}, nil
}

func modeForWebCommand(open bool) string {
	if open {
		return "web"
	}
	return "serve"
}

func openBrowserDefault(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
