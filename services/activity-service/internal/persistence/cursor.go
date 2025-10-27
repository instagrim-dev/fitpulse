// Package persistence contains helpers shared by repository implementations.
package persistence

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"example.com/activity/internal/domain"
)

// EncodeCursor serialises the cursor to a string token.
func EncodeCursor(c *domain.Cursor) string {
	if c == nil {
		return ""
	}
	raw := fmt.Sprintf("%s|%s", c.StartedAt.UTC().Format(time.RFC3339Nano), c.ID)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses the encoded cursor token.
func DecodeCursor(token string) (*domain.Cursor, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cursor format")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, err
	}
	return &domain.Cursor{StartedAt: ts, ID: parts[1]}, nil
}
