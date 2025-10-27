package auth

import (
	"context"

	authlib "example.com/platform/libs/go/auth"
)

// Claims mirrors the shared auth claims type for service convenience.
type Claims = authlib.Claims

// Config mirrors the shared auth config.
type Config = authlib.Config

// ParseClaims delegates to the shared auth parser.
func ParseClaims(token string, cfg Config) (*Claims, error) {
	return authlib.Parse(token, authlib.Config(cfg))
}

// WithClaims stores the claims in the request context.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return authlib.WithClaims(ctx, claims)
}

// FromContext retrieves claims from context.
func FromContext(ctx context.Context) (*Claims, bool) {
	return authlib.FromContext(ctx)
}
