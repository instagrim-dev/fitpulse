package auth

import "context"

type contextKey string

const claimsKey contextKey = "platform-auth-claims"

// WithClaims stores claims on the context.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// FromContext retrieves claims stored by WithClaims.
func FromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(*Claims)
	return claims, ok
}
