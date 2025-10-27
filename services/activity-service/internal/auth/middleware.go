package auth

import (
	"net/http"

	authlib "example.com/platform/libs/go/auth"
)

// Middleware enforces bearer-token authentication on incoming requests.
type Middleware struct {
	inner authlib.Middleware
}

// NewMiddleware constructs Middleware with validation config.
func NewMiddleware(cfg Config) Middleware {
	skipper := func(r *http.Request) bool {
		return r.URL.Path == "/healthz"
	}
	return Middleware{inner: authlib.NewMiddleware(authlib.Config(cfg), skipper)}
}

// Wrap attaches authentication handling to an http.Handler.
func (m Middleware) Wrap(next http.Handler) http.Handler {
	return m.inner.Wrap(next)
}
