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

	// Resolve all profiles in cfg.Profiles
	resolvedProfiles := make(map[string]Profile, len(cfg.Profiles))
	for name, p := range cfg.Profiles {
		pCopy := p
		if pCopy.SSHProxy != "" {
			if proxy, ok := cfg.SSHProxies[pCopy.SSHProxy]; ok {
				pCopy.SSHConfig = &proxy
			}
		}
		if pCopy.Port == 0 {
			switch pCopy.DB {
			case "mysql":
				pCopy.Port = 3306
			case "pg":
				pCopy.Port = 5432
			}
		}
		resolvedProfiles[name] = pCopy
	}

	// 3) Retrieve full profile
	var selectedProfile Profile
	if profile != "" {
		p, ok := resolvedProfiles[profile]
		if !ok {
			return Resolved{}, errors.New(errors.CodeCfgInvalid, "profile not found",
				map[string]any{"profile": profile})
		}
		if p.SSHProxy != "" && p.SSHConfig == nil {
			return Resolved{}, errors.New(errors.CodeCfgInvalid, "ssh_proxy not found",
				map[string]any{"profile": profile, "ssh_proxy": p.SSHProxy})
		}
		selectedProfile = p
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

	return Resolved{
		ConfigPath:  cfgPath,
		ProfileName: profile,
		Format:      format,
		Profile:     selectedProfile,
		AllProfiles: resolvedProfiles,
	}, nil
}
