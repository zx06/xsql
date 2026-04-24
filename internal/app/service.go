package app

import (
	"context"
	"sort"
	"time"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/db"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
)

// ProfileListResult is the structured result for profile listing.
type ProfileListResult struct {
	ConfigPath string               `json:"config_path" yaml:"config_path"`
	Profiles   []config.ProfileInfo `json:"profiles" yaml:"profiles"`
}

// ToProfileListData implements output.ProfileListFormatter.
func (r *ProfileListResult) ToProfileListData() (string, []output.ProfileListItem, bool) {
	if r == nil {
		return "", nil, false
	}

	profiles := make([]output.ProfileListItem, 0, len(r.Profiles))
	for _, profile := range r.Profiles {
		profiles = append(profiles, output.ProfileListItem{
			Name:        profile.Name,
			Description: profile.Description,
			DB:          profile.DB,
			Mode:        profile.Mode,
		})
	}
	return r.ConfigPath, profiles, true
}

// QueryRequest contains options for a query operation.
type QueryRequest struct {
	Profile          config.Profile
	SQL              string
	AllowPlaintext   bool
	SkipHostKeyCheck bool
	UnsafeAllowWrite bool
}

// SchemaDumpRequest contains options for a schema dump operation.
type SchemaDumpRequest struct {
	Profile          config.Profile
	TablePattern     string
	IncludeSystem    bool
	AllowPlaintext   bool
	SkipHostKeyCheck bool
}

// TableListRequest contains options for loading the lightweight table list.
type TableListRequest struct {
	Profile          config.Profile
	TablePattern     string
	IncludeSystem    bool
	AllowPlaintext   bool
	SkipHostKeyCheck bool
}

// TableDescribeRequest contains options for loading a single table schema.
type TableDescribeRequest struct {
	Profile          config.Profile
	Schema           string
	Name             string
	AllowPlaintext   bool
	SkipHostKeyCheck bool
}

// LoadProfiles loads and summarizes the configured profiles.
func LoadProfiles(opts config.Options) (*ProfileListResult, *errors.XError) {
	cfg, cfgPath, xe := config.LoadConfig(opts)
	if xe != nil {
		return nil, xe
	}

	profiles := make([]config.ProfileInfo, 0, len(cfg.Profiles))
	for name, profile := range cfg.Profiles {
		profiles = append(profiles, config.ProfileToInfo(name, profile))
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	return &ProfileListResult{
		ConfigPath: cfgPath,
		Profiles:   profiles,
	}, nil
}

// LoadProfileDetail loads a single profile and redacts sensitive fields.
func LoadProfileDetail(opts config.Options, name string) (map[string]any, *errors.XError) {
	cfg, cfgPath, xe := config.LoadConfig(opts)
	if xe != nil {
		return nil, xe
	}

	profile, xe := ResolveProfile(cfg, name)
	if xe != nil {
		return nil, xe
	}

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
		if profile.SSHConfig != nil {
			result["ssh_host"] = profile.SSHConfig.Host
			result["ssh_port"] = profile.SSHConfig.Port
			result["ssh_user"] = profile.SSHConfig.User
			if profile.SSHConfig.IdentityFile != "" {
				result["ssh_identity_file"] = profile.SSHConfig.IdentityFile
			}
		}
	}

	return result, nil
}

// ResolveProfile returns a fully prepared profile with ssh config and default ports.
func ResolveProfile(cfg config.File, name string) (config.Profile, *errors.XError) {
	profile, ok := cfg.Profiles[name]
	if !ok {
		return config.Profile{}, errors.New(errors.CodeCfgInvalid, "profile not found", map[string]any{"name": name})
	}
	if profile.SSHProxy != "" {
		proxy, ok := cfg.SSHProxies[profile.SSHProxy]
		if !ok {
			return config.Profile{}, errors.New(errors.CodeCfgInvalid, "ssh_proxy not found", map[string]any{"profile": name, "ssh_proxy": profile.SSHProxy})
		}
		profile.SSHConfig = &proxy
	}
	if profile.Port == 0 {
		switch profile.DB {
		case "mysql":
			profile.Port = 3306
		case "pg":
			profile.Port = 5432
		}
	}
	return profile, nil
}

// Query executes a SQL query using a resolved profile.
func Query(ctx context.Context, req QueryRequest) (*db.QueryResult, *errors.XError) {
	if req.Profile.DB == "" {
		return nil, errors.New(errors.CodeCfgInvalid, "db type is required (mysql|pg)", nil)
	}

	conn, xe := ResolveConnection(ctx, ConnectionOptions{
		Profile:          req.Profile,
		AllowPlaintext:   req.AllowPlaintext,
		SkipHostKeyCheck: req.SkipHostKeyCheck,
	})
	if xe != nil {
		return nil, xe
	}
	defer func() { _ = conn.Close() }()

	return db.Query(ctx, conn.DB, req.SQL, db.QueryOptions{
		UnsafeAllowWrite: req.UnsafeAllowWrite,
		DBType:           req.Profile.DB,
	})
}

// DumpSchema exports the schema using a resolved profile.
func DumpSchema(ctx context.Context, req SchemaDumpRequest) (*db.SchemaInfo, *errors.XError) {
	if req.Profile.DB == "" {
		return nil, errors.New(errors.CodeCfgInvalid, "db type is required (mysql|pg)", nil)
	}

	conn, xe := ResolveConnection(ctx, ConnectionOptions{
		Profile:          req.Profile,
		AllowPlaintext:   req.AllowPlaintext,
		SkipHostKeyCheck: req.SkipHostKeyCheck,
	})
	if xe != nil {
		return nil, xe
	}
	defer func() { _ = conn.Close() }()

	return db.DumpSchema(ctx, req.Profile.DB, conn.DB, db.SchemaOptions{
		TablePattern:  req.TablePattern,
		IncludeSystem: req.IncludeSystem,
	})
}

// ListTables loads the lightweight table list using a resolved profile.
func ListTables(ctx context.Context, req TableListRequest) (*db.TableList, *errors.XError) {
	if req.Profile.DB == "" {
		return nil, errors.New(errors.CodeCfgInvalid, "db type is required (mysql|pg)", nil)
	}

	conn, xe := ResolveConnection(ctx, ConnectionOptions{
		Profile:          req.Profile,
		AllowPlaintext:   req.AllowPlaintext,
		SkipHostKeyCheck: req.SkipHostKeyCheck,
	})
	if xe != nil {
		return nil, xe
	}
	defer func() { _ = conn.Close() }()

	return db.ListTables(ctx, req.Profile.DB, conn.DB, db.SchemaOptions{
		TablePattern:  req.TablePattern,
		IncludeSystem: req.IncludeSystem,
	})
}

// DescribeTable loads the schema for a single table using a resolved profile.
func DescribeTable(ctx context.Context, req TableDescribeRequest) (*db.Table, *errors.XError) {
	if req.Profile.DB == "" {
		return nil, errors.New(errors.CodeCfgInvalid, "db type is required (mysql|pg)", nil)
	}

	conn, xe := ResolveConnection(ctx, ConnectionOptions{
		Profile:          req.Profile,
		AllowPlaintext:   req.AllowPlaintext,
		SkipHostKeyCheck: req.SkipHostKeyCheck,
	})
	if xe != nil {
		return nil, xe
	}
	defer func() { _ = conn.Close() }()

	return db.DescribeTable(ctx, req.Profile.DB, conn.DB, db.TableDescribeOptions{
		Schema: req.Schema,
		Name:   req.Name,
	})
}

// QueryTimeout resolves the effective query timeout.
func QueryTimeout(profile config.Profile, overrideSeconds int, overrideSet bool, fallback time.Duration) time.Duration {
	if overrideSet && overrideSeconds > 0 {
		return time.Duration(overrideSeconds) * time.Second
	}
	if profile.QueryTimeout > 0 {
		return time.Duration(profile.QueryTimeout) * time.Second
	}
	return fallback
}

// SchemaTimeout resolves the effective schema dump timeout.
func SchemaTimeout(profile config.Profile, overrideSeconds int, overrideSet bool, fallback time.Duration) time.Duration {
	if overrideSet && overrideSeconds > 0 {
		return time.Duration(overrideSeconds) * time.Second
	}
	if profile.SchemaTimeout > 0 {
		return time.Duration(profile.SchemaTimeout) * time.Second
	}
	return fallback
}
