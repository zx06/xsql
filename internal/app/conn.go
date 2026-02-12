package app

import (
	"context"
	"database/sql"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/secret"
	"github.com/zx06/xsql/internal/ssh"
)

type Connection struct {
	DB         *sql.DB
	SSHClient  *ssh.Client
	Profile    config.Profile
	CloseFuncs []func() error
}

func (c *Connection) Close() error {
	var errs []error
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.SSHClient != nil {
		if err := c.SSHClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

type ConnectionOptions struct {
	Profile          config.Profile
	AllowPlaintext   bool
	SkipHostKeyCheck bool
}

func ResolveConnection(ctx context.Context, opts ConnectionOptions) (*Connection, *errors.XError) {
	allowPlaintext := opts.AllowPlaintext || opts.Profile.AllowPlaintext

	password := opts.Profile.Password
	if password != "" {
		pw, xe := secret.Resolve(password, secret.Options{AllowPlaintext: allowPlaintext})
		if xe != nil {
			return nil, xe
		}
		password = pw
	}

	var sshClient *ssh.Client
	if opts.Profile.SSHConfig != nil {
		passphrase := opts.Profile.SSHConfig.Passphrase
		if passphrase != "" {
			pp, xe := secret.Resolve(passphrase, secret.Options{AllowPlaintext: allowPlaintext})
			if xe != nil {
				return nil, xe
			}
			passphrase = pp
		}

		sshOpts := ssh.Options{
			Host:                opts.Profile.SSHConfig.Host,
			Port:                opts.Profile.SSHConfig.Port,
			User:                opts.Profile.SSHConfig.User,
			IdentityFile:        opts.Profile.SSHConfig.IdentityFile,
			Passphrase:          passphrase,
			KnownHostsFile:      opts.Profile.SSHConfig.KnownHostsFile,
			SkipKnownHostsCheck: opts.SkipHostKeyCheck || opts.Profile.SSHConfig.SkipHostKey,
		}
		sc, xe := ssh.Connect(ctx, sshOpts)
		if xe != nil {
			return nil, xe
		}
		sshClient = sc
	}

	drv, ok := db.Get(opts.Profile.DB)
	if !ok {
		return nil, errors.New(errors.CodeDBDriverUnsupported, "unsupported db driver", map[string]any{"db": opts.Profile.DB})
	}

	connOpts := db.ConnOptions{
		DSN:      opts.Profile.DSN,
		Host:     opts.Profile.Host,
		Port:     opts.Profile.Port,
		User:     opts.Profile.User,
		Password: password,
		Database: opts.Profile.Database,
	}
	if sshClient != nil {
		connOpts.Dialer = sshClient
	}

	conn, xe := drv.Open(ctx, connOpts)
	if xe != nil {
		return nil, xe
	}

	return &Connection{
		DB:        conn,
		SSHClient: sshClient,
		Profile:   opts.Profile,
	}, nil
}

func ResolveSSH(ctx context.Context, profile config.Profile, allowPlaintext, skipHostKeyCheck bool) (*ssh.Client, *errors.XError) {
	if profile.SSHConfig == nil {
		return nil, nil
	}

	passphrase := profile.SSHConfig.Passphrase
	if passphrase != "" {
		pp, xe := secret.Resolve(passphrase, secret.Options{AllowPlaintext: allowPlaintext})
		if xe != nil {
			return nil, xe
		}
		passphrase = pp
	}

	sshOpts := ssh.Options{
		Host:                profile.SSHConfig.Host,
		Port:                profile.SSHConfig.Port,
		User:                profile.SSHConfig.User,
		IdentityFile:        profile.SSHConfig.IdentityFile,
		Passphrase:          passphrase,
		KnownHostsFile:      profile.SSHConfig.KnownHostsFile,
		SkipKnownHostsCheck: skipHostKeyCheck || profile.SSHConfig.SkipHostKey,
	}

	sc, xe := ssh.Connect(ctx, sshOpts)
	if xe != nil {
		return nil, xe
	}

	return sc, nil
}
