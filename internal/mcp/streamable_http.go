package mcp

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/zx06/xsql/internal/errors"
)

const (
	TransportStdio          = "stdio"
	TransportStreamableHTTP = "streamable_http"
)

const (
	authHeader    = "Authorization"
	bearerPrefix  = "Bearer "
	unauthorized  = "unauthorized"
	headerMissing = "authorization header is required"
)

// NewStreamableHTTPHandler creates a streamable HTTP handler with required auth.
func NewStreamableHTTPHandler(server *mcp.Server, authToken string) (http.Handler, error) {
	if server == nil {
		return nil, errors.New(errors.CodeInternal, "mcp server is nil", nil)
	}
	if authToken == "" {
		return nil, errors.New(errors.CodeCfgInvalid, "mcp streamable http auth token is required", nil)
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
	return requireAuth(handler, authToken), nil
}

func requireAuth(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		auth := strings.TrimSpace(req.Header.Get(authHeader))
		if auth == "" {
			http.Error(w, headerMissing, http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(auth, bearerPrefix) {
			http.Error(w, unauthorized, http.StatusUnauthorized)
			return
		}
		received := strings.TrimPrefix(auth, bearerPrefix)
		if subtle.ConstantTimeCompare([]byte(received), []byte(token)) != 1 {
			http.Error(w, unauthorized, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}
