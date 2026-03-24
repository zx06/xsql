package ssh

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
}

func DefaultKnownHostsPath() string {
	return "~/.ssh/known_hosts"
}
