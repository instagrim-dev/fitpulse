package auth

import (
	"context"
	"net/http"

	authlib "example.com/platform/libs/go/auth"
)

// Scopes used by the ontology service.
const (
	ScopeOntologyRead  = "ontology:read"
	ScopeOntologyWrite = "ontology:write"
)

// Claims provides convenient aliasing.
type Claims = authlib.Claims

// Config mirrors shared auth config.
type Config = authlib.Config

// WithClaims stores claims in context.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return authlib.WithClaims(ctx, claims)
}

// FromContext retrieves claims from context.
func FromContext(ctx context.Context) (*Claims, bool) {
	return authlib.FromContext(ctx)
}

// ParseClaims delegates to shared auth.
func ParseClaims(token string, cfg Config) (*Claims, error) {
	return authlib.Parse(token, authlib.Config(cfg))
}

// Middleware wraps handlers with JWT auth enforcement.
type Middleware struct {
	inner authlib.Middleware
}

// NewMiddleware constructs middleware with optional health skip.
func NewMiddleware(cfg Config) Middleware {
	skipper := func(r *http.Request) bool {
		return r.URL.Path == "/healthz"
	}
	return Middleware{inner: authlib.NewMiddleware(authlib.Config(cfg), skipper)}
}

// Wrap applies authentication around the provided handler.
func (m Middleware) Wrap(next http.Handler) http.Handler {
	return m.inner.Wrap(next)
}
