package config

import (
	"os"
	"path/filepath"

	"github.com/zx06/xsql/internal/errors"
)

// Resolve performs phase-1 config/profile/format merging: CLI > ENV > Config.
func Resolve(opts Options) (Resolved, *errors.XError) {
	workDir := opts.WorkDir
	if workDir == "" {
		wd, _ := os.Getwd()
		workDir = wd
	}
	if opts.HomeDir == "" {
		if hd, err := os.UserHomeDir(); err == nil {
			opts.HomeDir = hd
		}
	}

	// 1) Read config file (if any)
	var cfg File
	var cfgPath string
	if opts.ConfigPath != "" {
		abs := opts.ConfigPath
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workDir, abs)
		}
		f, xe := readFile(abs)
		if xe != nil {
			return Resolved{}, xe
		}
		cfg = f
		cfgPath = abs
	} else {
		for _, p := range defaultConfigPaths(workDir, opts.HomeDir) {
			f, xe := readFile(p)
			if xe != nil {
				if xe.Code == errors.CodeCfgNotFound {
					continue
				}
				return Resolved{}, xe
			}
			cfg = f
			cfgPath = p
			break
		}
	}

	// 2) Select profile: --profile > XSQL_PROFILE > profiles.default > empty
	profile := ""
	if opts.CLIProfileSet {
		profile = opts.CLIProfile
	} else if opts.EnvProfile != "" {
		profile = opts.EnvProfile
	} else {
		if _, ok := cfg.Profiles["default"]; ok {
			profile = "default"
		}
	}

	// 3) Retrieve full profile
	var selectedProfile Profile
	if profile != "" {
		p, ok := cfg.Profiles[profile]
		if !ok {
			return Resolved{}, errors.New(errors.CodeCfgInvalid, "profile not found",
				map[string]any{"profile": profile})
		}
		selectedProfile = p
		// Resolve ssh_proxy reference
		if selectedProfile.SSHProxy != "" {
			if proxy, ok := cfg.SSHProxies[selectedProfile.SSHProxy]; ok {
				selectedProfile.SSHConfig = &proxy
			} else {
				return Resolved{}, errors.New(errors.CodeCfgInvalid, "ssh_proxy not found",
					map[string]any{"profile": profile, "ssh_proxy": selectedProfile.SSHProxy})
			}
		}
		// Set default port
		if selectedProfile.Port == 0 {
			switch selectedProfile.DB {
			case "mysql":
				selectedProfile.Port = 3306
			case "pg":
				selectedProfile.Port = 5432
			}
		}
	}

	// 4) Merge format: --format > XSQL_FORMAT > profile.format > auto
	format := "auto"
	if selectedProfile.Format != "" {
		format = selectedProfile.Format
	}
	if opts.EnvFormat != "" {
		format = opts.EnvFormat
	}
	if opts.CLIFormatSet {
		format = opts.CLIFormat
	}

	return Resolved{ConfigPath: cfgPath, ProfileName: profile, Format: format, Profile: selectedProfile}, nil
}
