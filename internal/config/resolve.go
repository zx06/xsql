package config

import (
	"os"
	"path/filepath"

	"github.com/zx06/xsql/internal/errors"
)

// Resolve 实现第一阶段 config/profile/format 合并：CLI > ENV > Config。
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

	// 1) 读取配置文件（如有）
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

	// 2) 选择 profile：--profile > XSQL_PROFILE > profiles.default > 空
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

	// 3) 获取完整 profile
	var selectedProfile Profile
	if profile != "" {
		if p, ok := cfg.Profiles[profile]; ok {
			selectedProfile = p
			// 解析 ssh_proxy 引用
			if selectedProfile.SSHProxy != "" {
				if proxy, ok := cfg.SSHProxies[selectedProfile.SSHProxy]; ok {
					selectedProfile.SSHConfig = &proxy
				} else {
					return Resolved{}, errors.New(errors.CodeCfgInvalid, "ssh_proxy not found",
						map[string]any{"profile": profile, "ssh_proxy": selectedProfile.SSHProxy})
				}
			}
			// 设置默认端口
			if selectedProfile.Port == 0 {
				switch selectedProfile.DB {
				case "mysql":
					selectedProfile.Port = 3306
				case "pg":
					selectedProfile.Port = 5432
				}
			}
		}
	}

	// 4) 合并 format：--format > XSQL_FORMAT > profile.format > auto
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
