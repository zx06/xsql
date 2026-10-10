package web

import (
	"net"
	"net/url"
	"strings"
)

// PublicURL converts an effective listener address to a browser URL for the local Web UI.
// The server listens with cleartext HTTP on loopback by design (see docs/rfcs/0007-web-ui.md).
func PublicURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return formatLocalWebURL(addr)
	}
	displayHost := host
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		displayHost = "127.0.0.1"
	}
	if strings.Contains(displayHost, ":") && !strings.HasPrefix(displayHost, "[") {
		displayHost = "[" + displayHost + "]"
	}
	return formatLocalWebURL(net.JoinHostPort(displayHost, port))
}

// localWebURLScheme is cleartext HTTP for the loopback-only Web UI (TLS is not supported).
const localWebURLScheme = "http"

func formatLocalWebURL(hostPort string) string {
	u := url.URL{
		Scheme: localWebURLScheme,
		Host:   hostPort,
		Path:   "/",
	}
	return u.String()
}
