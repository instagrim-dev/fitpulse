package auth

import (
	"net/http"
	"strings"
)

// Skipper allows callers to bypass authentication for specific requests.
type Skipper func(r *http.Request) bool

// Middleware provides HTTP middleware for bearer-token validation.
type Middleware struct {
	Config  Config
	Skipper Skipper
}

// NewMiddleware constructs a middleware with optional skipper.
func NewMiddleware(cfg Config, skipper Skipper) Middleware {
	return Middleware{Config: cfg, Skipper: skipper}
}

// Wrap wraps an http.Handler with authentication.
func (m Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.Skipper != nil && m.Skipper(r) {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := m.parseRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		ctx := WithClaims(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m Middleware) parseRequest(r *http.Request) (*Claims, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, ErrMissingToken
	}
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return nil, ErrInvalidToken
	}
	token := strings.TrimSpace(header[len("Bearer "):])
	return Parse(token, m.Config)
}
