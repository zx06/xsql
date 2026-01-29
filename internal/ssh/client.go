package ssh

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/zx06/xsql/internal/errors"
)

// Client 包装 ssh.Client，提供 DialContext 供 driver 使用。
type Client struct {
	client *ssh.Client
}

// Connect 建立 SSH 连接。
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
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil, errors.Wrap(errors.CodeSSHAuthFailed, "ssh authentication failed", map[string]any{"host": opts.Host}, err)
		}
		return nil, errors.Wrap(errors.CodeSSHDialFailed, "failed to connect to ssh server", map[string]any{"host": opts.Host}, err)
	}
	return &Client{client: client}, nil
}

// DialContext 通过 SSH 通道建立到 target 的连接。
func (c *Client) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return c.client.Dial(network, addr)
}

// Close 关闭 SSH 连接。
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func buildAuthMethods(opts Options) ([]ssh.AuthMethod, *errors.XError) {
	var methods []ssh.AuthMethod

	// 私钥认证
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

	// 尝试默认私钥路径
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
