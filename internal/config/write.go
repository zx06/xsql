package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/zx06/xsql/internal/errors"
)

// InitConfig creates a template config file at the given path.
// If path is empty, uses the default path ($HOME/.config/xsql/xsql.yaml).
func InitConfig(path string) (string, *errors.XError) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.Wrap(errors.CodeInternal, "failed to get home directory", nil, err)
		}
		path = filepath.Join(home, ".config", "xsql", "xsql.yaml")
	}

	if _, err := os.Stat(path); err == nil {
		return "", errors.New(errors.CodeCfgInvalid, "config file already exists", map[string]any{"path": path})
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", errors.Wrap(errors.CodeInternal, "failed to create config directory", map[string]any{"path": dir}, err)
	}

	template := `# xsql configuration file
# Documentation: https://github.com/zx06/xsql/blob/main/docs/config.md

ssh_proxies: {}
  # example:
  #   host: bastion.example.com
  #   port: 22
  #   user: admin
  #   identity_file: ~/.ssh/id_ed25519

profiles: {}
  # example:
  #   db: mysql
  #   host: 127.0.0.1
  #   port: 3306
  #   user: root
  #   password: "keyring:dev/mysql_password"
  #   database: mydb
`

	if err := os.WriteFile(path, []byte(template), 0600); err != nil {
		return "", errors.Wrap(errors.CodeInternal, "failed to write config file", map[string]any{"path": path}, err)
	}

	return path, nil
}

// SetConfigValue sets a value in the config file using dot-notation key.
// Supported key patterns:
//   - profile.<name>.<field>
//   - ssh_proxy.<name>.<field>
func SetConfigValue(configPath, key, value string) *errors.XError {
	if configPath == "" {
		return errors.New(errors.CodeCfgNotFound, "no config file found; run 'xsql config init' first", nil)
	}

	cfg, xe := readFile(configPath)
	if xe != nil {
		if xe.Code == errors.CodeCfgNotFound {
			// Create new empty config
			cfg = File{
				SSHProxies: map[string]SSHProxy{},
				Profiles:   map[string]Profile{},
			}
		} else {
			return xe
		}
	}

	parts := strings.SplitN(key, ".", 3)
	if len(parts) != 3 {
		return errors.New(errors.CodeCfgInvalid, "invalid key format; use profile.<name>.<field> or ssh_proxy.<name>.<field>",
			map[string]any{"key": key})
	}

	section, name, field := parts[0], parts[1], parts[2]

	switch section {
	case "profile":
		if xe := setProfileField(&cfg, name, field, value); xe != nil {
			return xe
		}
	case "ssh_proxy":
		if xe := setSSHProxyField(&cfg, name, field, value); xe != nil {
			return xe
		}
	default:
		return errors.New(errors.CodeCfgInvalid, "unsupported config section; use 'profile' or 'ssh_proxy'",
			map[string]any{"section": section})
	}

	return writeFile(configPath, cfg)
}

func setProfileField(cfg *File, name, field, value string) *errors.XError {
	p := cfg.Profiles[name]

	switch field {
	case "db":
		p.DB = value
	case "host":
		p.Host = value
	case "port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return errors.New(errors.CodeCfgInvalid, "port must be a number", map[string]any{"value": value})
		}
		p.Port = port
	case "local_port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return errors.New(errors.CodeCfgInvalid, "local_port must be a number", map[string]any{"value": value})
		}
		p.LocalPort = port
	case "user":
		p.User = value
	case "password":
		p.Password = value
	case "database":
		p.Database = value
	case "dsn":
		p.DSN = value
	case "description":
		p.Description = value
	case "format":
		p.Format = value
	case "ssh_proxy":
		p.SSHProxy = value
	case "unsafe_allow_write":
		p.UnsafeAllowWrite = parseBool(value)
	case "allow_plaintext":
		p.AllowPlaintext = parseBool(value)
	default:
		return errors.New(errors.CodeCfgInvalid, fmt.Sprintf("unknown profile field: %s", field),
			map[string]any{"field": field})
	}

	cfg.Profiles[name] = p
	return nil
}

func setSSHProxyField(cfg *File, name, field, value string) *errors.XError {
	sp := cfg.SSHProxies[name]

	switch field {
	case "host":
		sp.Host = value
	case "port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return errors.New(errors.CodeCfgInvalid, "port must be a number", map[string]any{"value": value})
		}
		sp.Port = port
	case "user":
		sp.User = value
	case "identity_file":
		sp.IdentityFile = value
	case "passphrase":
		sp.Passphrase = value
	case "known_hosts_file":
		sp.KnownHostsFile = value
	case "skip_host_key":
		sp.SkipHostKey = parseBool(value)
	default:
		return errors.New(errors.CodeCfgInvalid, fmt.Sprintf("unknown ssh_proxy field: %s", field),
			map[string]any{"field": field})
	}

	cfg.SSHProxies[name] = sp
	return nil
}

func writeFile(path string, cfg File) *errors.XError {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to marshal config", nil, err)
	}

	if err := os.WriteFile(path, b, 0600); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to write config file", map[string]any{"path": path}, err)
	}

	return nil
}

func parseBool(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes"
}

// FindConfigPath returns the path to the config file being used (or default path).
func FindConfigPath(opts Options) string {
	if opts.ConfigPath != "" {
		return opts.ConfigPath
	}

	workDir := opts.WorkDir
	if workDir == "" {
		wd, _ := os.Getwd()
		workDir = wd
	}
	homeDir := opts.HomeDir
	if homeDir == "" {
		if hd, err := os.UserHomeDir(); err == nil {
			homeDir = hd
		}
	}

	for _, p := range defaultConfigPaths(workDir, homeDir) {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Return default home path if none found
	if homeDir != "" {
		return filepath.Join(homeDir, ".config", "xsql", "xsql.yaml")
	}
	return ""
}
