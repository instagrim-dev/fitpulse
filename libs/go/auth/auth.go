package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config holds signer verification parameters shared by backend services.
type Config struct {
	Secret string
	Issuer string
}

// Claims represents the payload extracted from a JWT.
type Claims struct {
	Subject   string
	TenantID  string
	Scopes    map[string]struct{}
	ExpiresAt time.Time
}

// ErrMissingToken is returned when the Authorization header is absent.
var ErrMissingToken = errors.New("missing bearer token")

// ErrInvalidToken wraps parsing/validation errors.
var ErrInvalidToken = errors.New("invalid bearer token")

// Parse validates a JWT and returns normalized claims.
func Parse(token string, cfg Config) (*Claims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrMissingToken
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	}, jwt.WithIssuer(cfg.Issuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}

	subject, _ := claims["sub"].(string)
	tenantID, _ := claims["tenant_id"].(string)
	if subject == "" || tenantID == "" {
		return nil, ErrInvalidToken
	}

	scopes := normalizeScopes(claims["scopes"])
	exp, err := claims.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	return &Claims{
		Subject:   subject,
		TenantID:  tenantID,
		Scopes:    scopes,
		ExpiresAt: exp.Time,
	}, nil
}

func normalizeScopes(value interface{}) map[string]struct{} {
	out := make(map[string]struct{})
	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				out[str] = struct{}{}
			}
		}
	case []string:
		for _, str := range v {
			if str != "" {
				out[str] = struct{}{}
			}
		}
	case string:
		for _, str := range strings.Split(v, " ") {
			str = strings.TrimSpace(str)
			if str != "" {
				out[str] = struct{}{}
			}
		}
	}
	return out
}

// HasScope reports whether the claim set includes the provided scope.
func (c *Claims) HasScope(scope string) bool {
	if c == nil {
		return false
	}
	_, ok := c.Scopes[scope]
	return ok
}
