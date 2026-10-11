package config

import (
	"github.com/zx06/xsql/internal/secret"
)

// MaskedSecret is the placeholder returned by Web/CLI APIs for stored plaintext secrets.
const MaskedSecret = "***"

// MaskSecretForDisplay returns a safe value for API responses. Keyring references are shown as-is.
func MaskSecretForDisplay(raw string) string {
	if raw == "" {
		return ""
	}
	if secret.IsKeyringRef(raw) {
		return raw
	}
	return MaskedSecret
}

// IsMaskedSecret reports whether s is the API mask placeholder (not a real secret).
func IsMaskedSecret(s string) bool {
	return s == MaskedSecret
}

// CoalesceSecret chooses the stored secret when the client omitted it or sent the mask placeholder.
func CoalesceSecret(incoming, existing string) string {
	if incoming == "" || IsMaskedSecret(incoming) {
		return existing
	}
	return incoming
}

// ProfileForAPI returns a copy of p with sensitive fields redacted for HTTP responses.
func ProfileForAPI(p Profile) Profile {
	out := p
	out.Password = MaskSecretForDisplay(p.Password)
	if p.DSN != "" {
		out.DSN = MaskedSecret
	}
	out.SSHConfig = nil
	return out
}

// SSHProxyForAPI returns a copy of sp with sensitive fields redacted for HTTP responses.
func SSHProxyForAPI(sp SSHProxy) SSHProxy {
	out := sp
	out.Passphrase = MaskSecretForDisplay(sp.Passphrase)
	return out
}

// MCPHTTPConfigForAPI redacts auth tokens for HTTP responses.
func MCPHTTPConfigForAPI(c MCPHTTPConfig) MCPHTTPConfig {
	out := c
	out.AuthToken = MaskSecretForDisplay(c.AuthToken)
	return out
}

// WebHTTPConfigForAPI redacts auth tokens for HTTP responses.
func WebHTTPConfigForAPI(c WebHTTPConfig) WebHTTPConfig {
	out := c
	out.AuthToken = MaskSecretForDisplay(c.AuthToken)
	return out
}

// FileForAPI returns a copy of the config file with secrets redacted for HTTP responses.
func FileForAPI(f File) File {
	out := f
	if len(f.Profiles) > 0 {
		out.Profiles = make(map[string]Profile, len(f.Profiles))
		for name, p := range f.Profiles {
			out.Profiles[name] = ProfileForAPI(p)
		}
	}
	if len(f.SSHProxies) > 0 {
		out.SSHProxies = make(map[string]SSHProxy, len(f.SSHProxies))
		for name, sp := range f.SSHProxies {
			out.SSHProxies[name] = SSHProxyForAPI(sp)
		}
	}
	out.MCP = f.MCP
	out.MCP.HTTP = MCPHTTPConfigForAPI(f.MCP.HTTP)
	out.Web = f.Web
	out.Web.HTTP = WebHTTPConfigForAPI(f.Web.HTTP)
	return out
}

// MergeProfileSecrets copies stored secrets from existing when incoming values are omitted or masked.
func MergeProfileSecrets(incoming, existing Profile) Profile {
	out := incoming
	out.Password = CoalesceSecret(incoming.Password, existing.Password)
	out.DSN = CoalesceSecret(incoming.DSN, existing.DSN)
	return out
}

// MergeSSHProxySecrets copies stored secrets from existing when incoming values are omitted or masked.
func MergeSSHProxySecrets(incoming, existing SSHProxy) SSHProxy {
	out := incoming
	out.Passphrase = CoalesceSecret(incoming.Passphrase, existing.Passphrase)
	return out
}
