package cache

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// Invalidator defines a cache invalidation contract.
type Invalidator interface {
	Invalidate(ctx context.Context, exerciseID string) error
}

// NoopInvalidator is a no-op implementation.
type NoopInvalidator struct{}

// Invalidate performs no action.
func (NoopInvalidator) Invalidate(context.Context, string) error { return nil }

// HTTPInvalidator calls an upstream edge cache invalidation endpoint.
type HTTPInvalidator struct {
	client *http.Client
	url    string
	token  string
}

// NewHTTPInvalidator constructs an HTTPInvalidator.
func NewHTTPInvalidator(endpoint, token string, timeout time.Duration) *HTTPInvalidator {
	return &HTTPInvalidator{
		client: &http.Client{Timeout: timeout},
		url:    strings.TrimRight(endpoint, "/"),
		token:  token,
	}
}

// Invalidate triggers an HTTP POST containing the exercise identifier.
func (h *HTTPInvalidator) Invalidate(ctx context.Context, exerciseID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, strings.NewReader(exerciseID))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &InvalidationError{Status: resp.StatusCode}
	}
	return nil
}

// InvalidationError represents a non-successful invalidation response.
type InvalidationError struct {
	Status int
}

func (e *InvalidationError) Error() string {
	return "cache invalidation failed with status " + http.StatusText(e.Status)
}
