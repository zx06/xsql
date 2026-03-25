// Package ssh provides SSH tunnel connectivity for database drivers.
package ssh

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/zx06/xsql/internal/errors"
)

// Client wraps ssh.Client and provides DialContext for use by database drivers.
type Client struct {
	client *ssh.Client
	alive  atomic.Bool
}

// Connect establishes an SSH connection.
func Connect(ctx context.Context, opts Options) (*Client, *errors.XError) {
	if opts.Host == "" {
		return nil, errors.New(errors.CodeCfgInvalid, "ssh host is required", nil)
	}
	if opts.Port == 0 {
		opts.Port = 22
	}
	if opts.User == "" {
		opts.User = os.Getenv("USER")
		if opts.User == "" {
			opts.User = os.Getenv("USERNAME")
		}
	}

	authMethods, xe := buildAuthMethods(opts)
	if xe != nil {
		return nil, xe
	}

	hostKeyCallback, xe := buildHostKeyCallback(opts)
	if xe != nil {
		return nil, xe
	}

	config := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	addr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)

	// Use net.Dialer with context so that context cancellation/timeout
	// can interrupt the TCP connection phase (ssh.Dial does not accept context).
	d := net.Dialer{}
	netConn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, errors.Wrap(errors.CodeSSHDialFailed, "failed to connect to ssh server", map[string]any{"host": opts.Host}, err)
	}

	// Perform SSH handshake over the established TCP connection.
	// Set a deadline derived from context to prevent hanging during handshake.
	if deadline, ok := ctx.Deadline(); ok {
		netConn.SetDeadline(deadline)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, config)
	if err != nil {
		netConn.Close()
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil, errors.Wrap(errors.CodeSSHAuthFailed, "ssh authentication failed", map[string]any{"host": opts.Host}, err)
		}
		return nil, errors.Wrap(errors.CodeSSHDialFailed, "failed to connect to ssh server", map[string]any{"host": opts.Host}, err)
	}
	// Clear the deadline after successful handshake so it doesn't affect later I/O.
	netConn.SetDeadline(time.Time{})

	client := ssh.NewClient(sshConn, chans, reqs)
	c := &Client{client: client}
	c.alive.Store(true)
	return c, nil
}

// DialContext establishes a connection to the target through the SSH tunnel.
func (c *Client) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if c.client == nil {
		return nil, fmt.Errorf("ssh client is not connected")
	}
	return c.client.Dial(network, addr)
}

// Close closes the SSH connection.
func (c *Client) Close() error {
	c.alive.Store(false)
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// SendKeepalive sends a single SSH keepalive request and returns any error.
// A nil error means the connection is alive.
func (c *Client) SendKeepalive() error {
	if c.client == nil {
		return fmt.Errorf("ssh client is nil")
	}
	_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
	return err
}

// Alive reports whether the SSH connection is believed to be alive.
func (c *Client) Alive() bool {
	return c.alive.Load()
}

func buildAuthMethods(opts Options) ([]ssh.AuthMethod, *errors.XError) {
	var methods []ssh.AuthMethod

	// Private key authentication
	if opts.IdentityFile != "" {
		keyPath := expandPath(opts.IdentityFile)
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, errors.Wrap(errors.CodeCfgInvalid, "failed to read ssh identity file", map[string]any{"path": keyPath}, err)
		}
		var signer ssh.Signer
		if opts.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(opts.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyData)
		}
		if err != nil {
			return nil, errors.Wrap(errors.CodeSSHAuthFailed, "failed to parse ssh private key", nil, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	// Try default private key paths
	if len(methods) == 0 {
		for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
			keyPath := expandPath("~/.ssh/" + name)
			if keyData, err := os.ReadFile(keyPath); err == nil {
				if signer, err := ssh.ParsePrivateKey(keyData); err == nil {
					methods = append(methods, ssh.PublicKeys(signer))
					break
				}
			}
		}
	}

	if len(methods) == 0 {
		return nil, errors.New(errors.CodeSSHAuthFailed, "no ssh authentication method available", nil)
	}
	return methods, nil
}

func buildHostKeyCallback(opts Options) (ssh.HostKeyCallback, *errors.XError) {
	if opts.SkipKnownHostsCheck {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	khPath := opts.KnownHostsFile
	if khPath == "" {
		khPath = "~/.ssh/known_hosts"
	}
	khPath = expandPath(khPath)
	cb, err := knownhosts.New(khPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.CodeSSHHostKeyMismatch, "known_hosts file not found; use --ssh-skip-known-hosts-check to bypass (not recommended)", map[string]any{"path": khPath})
		}
		return nil, errors.Wrap(errors.CodeSSHHostKeyMismatch, "failed to parse known_hosts", map[string]any{"path": khPath}, err)
	}
	return cb, nil
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}
