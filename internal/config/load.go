package config

import (
	"os"
	"path/filepath"

	"github.com/zx06/xsql/internal/errors"
	"gopkg.in/yaml.v3"
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
	return f, nil
}
