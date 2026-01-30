package main

import (
	"github.com/spf13/cobra"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
)

// NewProfileCommand creates the profile command group
func NewProfileCommand(w *output.Writer) *cobra.Command {
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles",
	}

	profileCmd.AddCommand(newProfileListCommand(w))
	profileCmd.AddCommand(newProfileShowCommand(w))

	return profileCmd
}

// newProfileListCommand creates the profile list command
func newProfileListCommand(w *output.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(GlobalConfig.FormatStr)
			if err != nil {
				return err
			}

			cfg, cfgPath, xe := config.LoadConfig(config.Options{
				ConfigPath: GlobalConfig.ConfigStr,
			})
			if xe != nil {
				return xe
			}

			type profileInfo struct {
				Name        string `json:"name"`
				Description string `json:"description,omitempty"`
				DB          string `json:"db"`
				Mode        string `json:"mode"` // "read-only" or "read-write"
			}

			profiles := make([]profileInfo, 0, len(cfg.Profiles))
			for name, p := range cfg.Profiles {
				mode := "read-only"
				if p.UnsafeAllowWrite {
					mode = "read-write"
				}
				profiles = append(profiles, profileInfo{
					Name:        name,
					Description: p.Description,
					DB:          p.DB,
					Mode:        mode,
				})
			}

			result := map[string]any{
				"config_path": cfgPath,
				"profiles":    profiles,
			}

			return w.WriteOK(format, result)
		},
	}
}

// newProfileShowCommand creates the profile show command
func newProfileShowCommand(w *output.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show profile details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			format, err := parseOutputFormat(GlobalConfig.FormatStr)
			if err != nil {
				return err
			}

			cfg, cfgPath, xe := config.LoadConfig(config.Options{
				ConfigPath: GlobalConfig.ConfigStr,
			})
			if xe != nil {
				return xe
			}

			profile, ok := cfg.Profiles[name]
			if !ok {
				return errors.New(errors.CodeCfgInvalid, "profile not found", map[string]any{"name": name})
			}

			// Redact sensitive information: hide password
			result := map[string]any{
				"config_path":        cfgPath,
				"name":               name,
				"description":        profile.Description,
				"db":                 profile.DB,
				"host":               profile.Host,
				"port":               profile.Port,
				"user":               profile.User,
				"database":           profile.Database,
				"unsafe_allow_write": profile.UnsafeAllowWrite,
				"allow_plaintext":    profile.AllowPlaintext,
			}

			if profile.DSN != "" {
				result["dsn"] = "***"
			}
			if profile.Password != "" {
				result["password"] = "***"
			}
			if profile.SSHProxy != "" {
				result["ssh_proxy"] = profile.SSHProxy
				if proxy, ok := cfg.SSHProxies[profile.SSHProxy]; ok {
					result["ssh_host"] = proxy.Host
					result["ssh_port"] = proxy.Port
					result["ssh_user"] = proxy.User
					if proxy.IdentityFile != "" {
						result["ssh_identity_file"] = proxy.IdentityFile
					}
				}
			}

			return w.WriteOK(format, result)
		},
	}
}
