package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/zx06/xsql/internal/errors"
)

func defaultConfigPaths(workDir, homeDir string) []string {
	paths := make([]string, 0, 2)
	if workDir != "" {
		paths = append(paths, filepath.Join(workDir, "xsql.yaml"))
	}
	if homeDir != "" {
		paths = append(paths, filepath.Join(homeDir, ".config", "xsql", "xsql.yaml"))
	}
	return paths
}

func readFile(path string) (File, *errors.XError) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, errors.New(errors.CodeCfgNotFound, "config file not found", map[string]any{"path": path})
		}
		return File{}, errors.Wrap(errors.CodeCfgInvalid, "failed to read config file", map[string]any{"path": path}, err)
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return File{}, errors.Wrap(errors.CodeCfgInvalid, "invalid config file", map[string]any{"path": path}, err)
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	if f.SSHProxies == nil {
		f.SSHProxies = map[string]SSHProxy{}
	}
	return f, nil
}

// LoadConfig 加载配置文件，返回完整配置和配置文件路径。
func LoadConfig(opts Options) (File, string, *errors.XError) {
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

	if opts.ConfigPath != "" {
		abs := opts.ConfigPath
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workDir, abs)
		}
		f, xe := readFile(abs)
		if xe != nil {
			return File{}, "", xe
		}
		return f, abs, nil
	}

	for _, p := range defaultConfigPaths(workDir, opts.HomeDir) {
		f, xe := readFile(p)
		if xe != nil {
			if xe.Code == errors.CodeCfgNotFound {
				continue
			}
			return File{}, "", xe
		}
		return f, p, nil
	}

	return File{Profiles: map[string]Profile{}, SSHProxies: map[string]SSHProxy{}}, "", nil
}
