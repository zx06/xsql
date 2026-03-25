package ssh

import "time"

// Options contains the parameters required for an SSH connection.
type Options struct {
	Host           string
	Port           int
	User           string
	IdentityFile   string // path to the private key
	Passphrase     string // private key passphrase (if any)
	KnownHostsFile string // defaults to ~/.ssh/known_hosts

	// SkipKnownHostsCheck disables known_hosts verification (strongly discouraged!).
	SkipKnownHostsCheck bool

	// KeepaliveInterval is the interval between SSH keepalive probes.
	// Zero or negative disables keepalive. Default: DefaultKeepaliveInterval.
	KeepaliveInterval time.Duration

	// KeepaliveCountMax is the maximum number of consecutive missed
	// keepalive responses before the connection is considered dead.
	// Default: DefaultKeepaliveCountMax.
	KeepaliveCountMax int
}

const (
	DefaultKeepaliveInterval = 30 * time.Second
	DefaultKeepaliveCountMax = 3
)

func DefaultKnownHostsPath() string {
	return "~/.ssh/known_hosts"
}

// keepaliveInterval returns the effective keepalive interval (applying default).
func (o Options) keepaliveInterval() time.Duration {
	if o.KeepaliveInterval > 0 {
		return o.KeepaliveInterval
	}
	return DefaultKeepaliveInterval
}

// keepaliveCountMax returns the effective max missed count (applying default).
func (o Options) keepaliveCountMax() int {
	if o.KeepaliveCountMax > 0 {
		return o.KeepaliveCountMax
	}
	return DefaultKeepaliveCountMax
}
