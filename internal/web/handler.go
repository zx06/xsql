package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/zx06/xsql/internal/app"
	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/output"
	"github.com/zx06/xsql/internal/stats"
	frontend "github.com/zx06/xsql/webui"
)

// HandlerOptions configures the web HTTP handlers.
type HandlerOptions struct {
	ConfigPath       string
	InitialProfile   string
	AllowPlaintext   bool
	SkipHostKeyCheck bool
	AuthRequired     bool
	AuthToken        string
	Assets           fs.FS
	Stats            stats.StatsConfig
}

type handler struct {
	configPath       string
	initialProfile   string
	allowPlaintext   bool
	skipHostKeyCheck bool
	authRequired     bool
	authToken        string
	assets           fs.FS
	stats            stats.StatsConfig
}

type queryRequest struct {
	Profile string `json:"profile"`
	SQL     string `json:"sql"`
}

// NewHandler creates the web server handler.
func NewHandler(opts HandlerOptions) http.Handler {
	assets := opts.Assets
	if assets == nil {
		assets = frontend.Dist()
	}

	h := &handler{
		configPath:       opts.ConfigPath,
		initialProfile:   opts.InitialProfile,
		allowPlaintext:   opts.AllowPlaintext,
		skipHostKeyCheck: opts.SkipHostKeyCheck,
		authRequired:     opts.AuthRequired,
		authToken:        opts.AuthToken,
		assets:           assets,
		stats:            opts.Stats,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(apiPrefix+"/health", h.handleHealth)
	mux.Handle(apiPrefix+"/profiles", h.withAuth(http.HandlerFunc(h.handleProfiles)))
	mux.Handle(apiPrefix+"/profiles/", h.withAuth(http.HandlerFunc(h.handleProfileShow)))
	mux.Handle(apiPrefix+"/config", h.withAuth(http.HandlerFunc(h.handleConfigGet)))
	mux.Handle(apiPrefix+"/config/profiles", h.withAuth(http.HandlerFunc(h.handleConfigSaveProfile)))
	mux.Handle(apiPrefix+"/config/profiles/", h.withAuth(http.HandlerFunc(h.handleConfigDeleteProfile)))
	mux.Handle(apiPrefix+"/config/ssh-proxies", h.withAuth(http.HandlerFunc(h.handleConfigSaveSSHProxy)))
	mux.Handle(apiPrefix+"/config/ssh-proxies/", h.withAuth(http.HandlerFunc(h.handleConfigDeleteSSHProxy)))
	mux.Handle(apiPrefix+"/config/ai", h.withAuth(http.HandlerFunc(h.handleConfigSaveAI)))
	mux.Handle(apiPrefix+"/config/test/profile", h.withAuth(http.HandlerFunc(h.handleConfigTestProfile)))
	mux.Handle(apiPrefix+"/config/test/ssh-proxy", h.withAuth(http.HandlerFunc(h.handleConfigTestSSHProxy)))
	mux.Handle(apiPrefix+"/config/test/ai", h.withAuth(http.HandlerFunc(h.handleConfigTestAI)))
	mux.Handle(apiPrefix+"/schema/tables/", h.withAuth(http.HandlerFunc(h.handleSchemaTable)))
	mux.Handle(apiPrefix+"/schema/tables", h.withAuth(http.HandlerFunc(h.handleSchemaTables)))
	mux.Handle(apiPrefix+"/query", h.withAuth(http.HandlerFunc(h.handleQuery)))
	mux.HandleFunc("/config.js", h.handleConfigJS)
	mux.HandleFunc("/", h.handleFrontend)
	return mux
}

func (h *handler) withAuth(next http.Handler) http.Handler {
	if !h.authRequired {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, errors.New(errors.CodeAuthRequired, "authorization token is required", nil))
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) || strings.TrimSpace(strings.TrimPrefix(authHeader, prefix)) != h.authToken {
			writeError(w, http.StatusUnauthorized, errors.New(errors.CodeAuthInvalid, "authorization token is invalid", nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"auth_required":     h.authRequired,
		"initial_profile":   h.initialProfile,
		"frontend_embedded": h.hasIndex(),
	})
}

func (h *handler) handleProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	result, xe := app.LoadProfiles(config.Options{ConfigPath: h.configPath})
	if xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) handleProfileShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, apiPrefix+"/profiles/")
	if name == "" || strings.Contains(name, "/") {
		writeError(w, http.StatusNotFound, errors.New(errors.CodeCfgInvalid, "profile not found", map[string]any{"name": name}))
		return
	}
	result, xe := app.LoadProfileDetail(config.Options{ConfigPath: h.configPath}, name)
	if xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) handleSchemaTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	profileName := strings.TrimSpace(r.URL.Query().Get("profile"))
	profile, xe := h.loadProfile(profileName)
	if xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}

	includeSystem, xe := parseIncludeSystem(r)
	if xe != nil {
		writeError(w, http.StatusBadRequest, xe)
		return
	}

	timeout := app.SchemaTimeout(profile, 0, false, 60*time.Second)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	result, xe := app.ListTables(ctx, app.TableListRequest{
		Profile:          profile,
		TablePattern:     r.URL.Query().Get("table"),
		IncludeSystem:    includeSystem,
		AllowPlaintext:   h.allowPlaintext,
		SkipHostKeyCheck: h.skipHostKeyCheck,
	})
	if xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) handleSchemaTable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	profileName := strings.TrimSpace(r.URL.Query().Get("profile"))
	profile, xe := h.loadProfile(profileName)
	if xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}

	schemaName, tableName, ok := parseSchemaTablePath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New(errors.CodeCfgInvalid, "table not found", map[string]any{"reason": "table_not_found"}))
		return
	}

	timeout := app.SchemaTimeout(profile, 0, false, 60*time.Second)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	result, xe := app.DescribeTable(ctx, app.TableDescribeRequest{
		Profile:          profile,
		Schema:           schemaName,
		Name:             tableName,
		AllowPlaintext:   h.allowPlaintext,
		SkipHostKeyCheck: h.skipHostKeyCheck,
	})
	if xe != nil {
		status := statusCodeFor(xe.Code)
		if xe.Code == errors.CodeCfgInvalid {
			if reason, _ := xe.Details["reason"].(string); reason == "table_not_found" {
				status = http.StatusNotFound
			}
		}
		writeError(w, status, xe)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	defer r.Body.Close()

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.Wrap(errors.CodeCfgInvalid, "failed to read request body", nil, err))
		return
	}

	var req queryRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errors.Wrap(errors.CodeCfgInvalid, "invalid request body", nil, err))
		return
	}
	profile, xe := h.loadProfile(req.Profile)
	if xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}

	timeout := app.QueryTimeout(profile, 0, false, 30*time.Second)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	start := time.Now()
	result, xe := app.Query(ctx, app.QueryRequest{
		Profile:          profile,
		SQL:              req.SQL,
		AllowPlaintext:   h.allowPlaintext,
		SkipHostKeyCheck: h.skipHostKeyCheck,
		UnsafeAllowWrite: false,
	})

	// Record stats
	h.recordWebStats("query", req.Profile, xe == nil, time.Since(start), xe, req.SQL)

	if xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) handleConfigJS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = fmt.Fprintf(w, "window.__XSQL_WEB_CONFIG__ = %s;\n", mustJSON(map[string]any{
		"initialProfile": h.initialProfile,
		"authRequired":   h.authRequired,
	}))
}

// nolint:gosec // G304 - Path is from embedded filesystem only, not user filesystem
func (h *handler) handleFrontend(w http.ResponseWriter, r *http.Request) {
	// Clean path to prevent directory traversal attacks; path.Clean removes ".." and "."
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." {
		name = ""
	}
	// Only serve files from the embedded asset filesystem, not real filesystem
	// This is inherently safe because http.FileSystem is restricted to embedded assets
	if name != "" {
		if file, err := h.assets.Open(name); err == nil {
			defer func() {
				_ = file.Close()
			}()
			if info, statErr := file.Stat(); statErr == nil && !info.IsDir() {
				if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
					w.Header().Set("Content-Type", contentType)
				}
				http.ServeContent(w, r, info.Name(), info.ModTime(), file.(io.ReadSeeker))
				return
			}
		}
	}

	index, err := h.assets.Open("index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "<!doctype html><html><body><h1>xsql web assets are not built</h1><p>Run the frontend build before serving the UI.</p></body></html>")
		return
	}
	defer func() {
		_ = index.Close()
	}()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, index.(io.ReadSeeker))
}

func (h *handler) hasIndex() bool {
	file, err := h.assets.Open("index.html")
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}

func (h *handler) loadProfile(name string) (config.Profile, *errors.XError) {
	if name == "" {
		name = h.initialProfile
	}
	if name == "" {
		return config.Profile{}, errors.New(errors.CodeCfgInvalid, "profile is required", nil)
	}
	cfg, _, xe := config.LoadConfig(config.Options{ConfigPath: h.configPath})
	if xe != nil {
		return config.Profile{}, xe
	}
	return app.ResolveProfile(cfg, name)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, errors.New(errors.CodeCfgInvalid, "method not allowed", nil))
}

func parseIncludeSystem(r *http.Request) (bool, *errors.XError) {
	includeSystem := false
	if raw := r.URL.Query().Get("include_system"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return false, errors.New(errors.CodeCfgInvalid, "include_system must be a boolean", nil)
		}
		includeSystem = parsed
	}
	return includeSystem, nil
}

func parseSchemaTablePath(rawPath string) (string, string, bool) {
	trimmed := strings.TrimPrefix(rawPath, apiPrefix+"/schema/tables/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	schemaName, err := url.PathUnescape(parts[0])
	if err != nil {
		return "", "", false
	}
	tableName, err := url.PathUnescape(parts[1])
	if err != nil {
		return "", "", false
	}
	return schemaName, tableName, true
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(output.Envelope{
		OK:            true,
		SchemaVersion: output.SchemaVersion,
		Data:          data,
	})
}

func writeError(w http.ResponseWriter, status int, xe *errors.XError) {
	if xe == nil {
		xe = errors.New(errors.CodeInternal, "internal error", nil)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(output.Envelope{
		OK:            false,
		SchemaVersion: output.SchemaVersion,
		Error: &output.ErrorObject{
			Code:    xe.Code,
			Message: xe.Message,
			Details: xe.Details,
		},
	})
}

func statusCodeFor(code errors.Code) int {
	switch code {
	case errors.CodeCfgInvalid, errors.CodeCfgNotFound, errors.CodeSecretNotFound:
		return http.StatusBadRequest
	case errors.CodeAuthRequired, errors.CodeAuthInvalid:
		return http.StatusUnauthorized
	case errors.CodeROBlocked:
		return http.StatusForbidden
	case errors.CodeDBConnectFailed, errors.CodeDBAuthFailed, errors.CodeSSHDialFailed, errors.CodeSSHAuthFailed, errors.CodeSSHHostKeyMismatch:
		return http.StatusBadGateway
	case errors.CodeDBDriverUnsupported:
		return http.StatusBadRequest
	case errors.CodeDBExecFailed:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// recordWebStats records a web query to the stats store.
func (h *handler) recordWebStats(cmd, profile string, ok bool, duration time.Duration, xe *errors.XError, sql string) {
	if !h.stats.Enabled {
		return
	}
	store := stats.NewStore(h.stats.FilePath)
	r := &stats.Record{
		Timestamp:  time.Now(),
		Cmd:        cmd,
		Profile:    profile,
		OK:         ok,
		DurationMs: duration.Milliseconds(),
	}
	if xe != nil {
		r.ErrorCode = string(xe.Code)
	}
	if h.stats.LogSQL && sql != "" {
		r.SQL = sql
	}
	_ = store.Append(r)
}

// IsLoopbackAddr reports whether the listen address is bound only to loopback.
func IsLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// PublicURL converts an effective listener address to a browser URL.
func PublicURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr + "/"
	}
	displayHost := host
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		displayHost = "127.0.0.1"
	}
	if strings.Contains(displayHost, ":") && !strings.HasPrefix(displayHost, "[") {
		displayHost = "[" + displayHost + "]"
	}
	return "http://" + net.JoinHostPort(displayHost, port) + "/"
}

func (h *handler) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	cfg, configPath, xe := config.LoadConfig(config.Options{ConfigPath: h.configPath})
	if xe != nil && xe.Code != errors.CodeCfgNotFound {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	if configPath == "" {
		configPath = config.FindConfigPath(config.Options{ConfigPath: h.configPath})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"config_path": configPath,
		"profiles":    cfg.Profiles,
		"ssh_proxies": cfg.SSHProxies,
		"mcp":         cfg.MCP,
		"web":         cfg.Web,
		"ai":          cfg.AI,
	})
}

type configProfileRequest struct {
	Name    string         `json:"name"`
	Profile config.Profile `json:"profile"`
}

func (h *handler) getResolvedConfigPath() string {
	if h.configPath != "" {
		return h.configPath
	}
	return config.FindConfigPath(config.Options{})
}

func (h *handler) parseConfigDeleteName(w http.ResponseWriter, r *http.Request, pathPrefix, nameReqMsg string) (string, bool) {
	if r.Method != http.MethodDelete {
		writeMethodNotAllowed(w)
		return "", false
	}
	name := strings.TrimPrefix(r.URL.Path, apiPrefix+pathPrefix)
	name = strings.TrimSpace(name)
	if name == "" {
		writeError(w, http.StatusBadRequest, errors.New(errors.CodeCfgInvalid, nameReqMsg, nil))
		return "", false
	}
	return name, true
}

func (h *handler) handleConfigSaveProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeMethodNotAllowed(w)
		return
	}
	var req configProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.Wrap(errors.CodeCfgInvalid, "invalid JSON payload", nil, err))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, errors.New(errors.CodeCfgInvalid, "profile name is required", nil))
		return
	}
	configPath := h.getResolvedConfigPath()
	if xe := config.SaveProfile(configPath, name, req.Profile); xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "config_path": configPath})
}

func (h *handler) handleConfigDeleteProfile(w http.ResponseWriter, r *http.Request) {
	name, ok := h.parseConfigDeleteName(w, r, "/config/profiles/", "profile name is required")
	if !ok {
		return
	}
	configPath := h.getResolvedConfigPath()
	if xe := config.DeleteProfile(configPath, name); xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

type configSSHProxyRequest struct {
	Name     string          `json:"name"`
	SSHProxy config.SSHProxy `json:"ssh_proxy"`
}

func (h *handler) handleConfigSaveSSHProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeMethodNotAllowed(w)
		return
	}
	var req configSSHProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.Wrap(errors.CodeCfgInvalid, "invalid JSON payload", nil, err))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, errors.New(errors.CodeCfgInvalid, "proxy name is required", nil))
		return
	}
	configPath := h.getResolvedConfigPath()
	if xe := config.SaveSSHProxy(configPath, name, req.SSHProxy); xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "config_path": configPath})
}

func (h *handler) handleConfigDeleteSSHProxy(w http.ResponseWriter, r *http.Request) {
	name, ok := h.parseConfigDeleteName(w, r, "/config/ssh-proxies/", "proxy name is required")
	if !ok {
		return
	}
	configPath := h.getResolvedConfigPath()
	if xe := config.DeleteSSHProxy(configPath, name); xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

type configAIRequest struct {
	AI config.AIConfig `json:"ai"`
}

func (h *handler) handleConfigSaveAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeMethodNotAllowed(w)
		return
	}
	var req configAIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.Wrap(errors.CodeCfgInvalid, "invalid JSON payload", nil, err))
		return
	}
	configPath := h.getResolvedConfigPath()
	if xe := config.SaveAI(configPath, req.AI); xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "config_path": configPath})
}

func (h *handler) handleConfigTestProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var req struct {
		Name    string         `json:"name"`
		Profile config.Profile `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.Wrap(errors.CodeCfgInvalid, "invalid JSON payload", nil, err))
		return
	}

	profile := req.Profile
	cfg, _, _ := config.LoadConfig(config.Options{ConfigPath: h.configPath})

	// If password is blank but profile already exists in config, preserve saved password
	if profile.Password == "" && req.Name != "" {
		if existing, ok := cfg.Profiles[req.Name]; ok {
			profile.Password = existing.Password
		}
	}

	// Resolve SSH proxy if specified
	if profile.SSHConfig == nil && profile.SSHProxy != "" {
		if proxy, ok := cfg.SSHProxies[profile.SSHProxy]; ok {
			profile.SSHConfig = &proxy
		} else {
			writeError(w, http.StatusBadRequest, errors.New(errors.CodeCfgInvalid, "ssh_proxy not found in configuration", map[string]any{"ssh_proxy": profile.SSHProxy}))
			return
		}
	}

	// Default ports
	if profile.Port == 0 {
		switch profile.DB {
		case "mysql":
			profile.Port = 3306
		case "pg":
			profile.Port = 5432
		}
	}

	result, xe := app.TestProfileConnection(r.Context(), profile, h.allowPlaintext, h.skipHostKeyCheck)
	if xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) handleConfigTestSSHProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var req struct {
		Name     string          `json:"name"`
		SSHProxy config.SSHProxy `json:"ssh_proxy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.Wrap(errors.CodeCfgInvalid, "invalid JSON payload", nil, err))
		return
	}

	proxy := req.SSHProxy
	cfg, _, _ := config.LoadConfig(config.Options{ConfigPath: h.configPath})
	if proxy.Passphrase == "" && req.Name != "" {
		if existing, ok := cfg.SSHProxies[req.Name]; ok {
			proxy.Passphrase = existing.Passphrase
		}
	}
	if proxy.Port == 0 {
		proxy.Port = 22
	}

	result, xe := app.TestSSHProxyConnection(r.Context(), proxy, h.allowPlaintext, h.skipHostKeyCheck)
	if xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) handleConfigTestAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var req struct {
		AI config.AIConfig `json:"ai"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.Wrap(errors.CodeCfgInvalid, "invalid JSON payload", nil, err))
		return
	}

	aiCfg := req.AI
	cfg, _, _ := config.LoadConfig(config.Options{ConfigPath: h.configPath})
	if aiCfg.APIKey == "" {
		aiCfg.APIKey = cfg.AI.APIKey
	}
	if aiCfg.BaseURL == "" {
		aiCfg.BaseURL = cfg.AI.BaseURL
	}
	if aiCfg.Model == "" {
		aiCfg.Model = cfg.AI.Model
	}

	result, xe := app.TestAIConnection(r.Context(), aiCfg)
	if xe != nil {
		writeError(w, statusCodeFor(xe.Code), xe)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
