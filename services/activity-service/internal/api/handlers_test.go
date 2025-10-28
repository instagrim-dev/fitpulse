package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/activity/internal/auth"
	"example.com/activity/internal/domain"
)

func TestActivityMetricsSuccess(t *testing.T) {
	now := time.Date(2025, time.October, 27, 20, 0, 0, 0, time.UTC)
	repo := &mockRepo{
		summary: domain.ActivitySummary{
			Total:                    5,
			Pending:                  1,
			Synced:                   3,
			Failed:                   1,
			AverageDurationMinutes:   42.5,
			AverageProcessingSeconds: 180.0,
			OldestPendingAgeSeconds:  5400,
			LastActivityAt:           &now,
		},
		timeline: []domain.ActivityAggregate{
			{
				ID:              "act-1",
				TenantID:        "tenant-1",
				UserID:          "user-1",
				ActivityType:    "Ride",
				StartedAt:       now.Add(-30 * time.Minute),
				DurationMin:     45,
				Source:          "api",
				Version:         "v1",
				State:           domain.ActivityStateSynced,
				CreatedAt:       now.Add(-29 * time.Minute),
				UpdatedAt:       now.Add(-10 * time.Minute),
				ReplayAvailable: true,
			},
			{
				ID:           "act-2",
				TenantID:     "tenant-1",
				UserID:       "user-1",
				ActivityType: "Run",
				StartedAt:    now.Add(-2 * time.Hour),
				DurationMin:  30,
				Source:       "mobile",
				Version:      "v1",
				State:        domain.ActivityStatePending,
				CreatedAt:    now.Add(-2 * time.Hour),
				UpdatedAt:    now.Add(-90 * time.Minute),
			},
		},
	}
	service := domain.NewService(repo)
	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/v1/activities/metrics?user_id=user-1&timeline_limit=2&window_hours=0", nil)
	claims := &auth.Claims{
		Subject:  "tester",
		TenantID: "tenant-1",
		Scopes: map[string]struct{}{
			auth.ScopeActivitiesRead: {},
		},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	req = req.WithContext(auth.WithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	handler.activityMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ActivityMetricsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Summary.Total != 5 {
		t.Fatalf("expected total 5 got %d", resp.Summary.Total)
	}
	if resp.Summary.SuccessRate <= 0.59 || resp.Summary.SuccessRate >= 0.61 {
		t.Fatalf("unexpected success rate %f", resp.Summary.SuccessRate)
	}
	if resp.WindowSeconds != 0 {
		t.Fatalf("expected window_seconds 0 got %d", resp.WindowSeconds)
	}
	if len(resp.Timeline) != 2 {
		t.Fatalf("expected timeline length 2 got %d", len(resp.Timeline))
	}
	if resp.Timeline[0].ActivityID != "act-1" {
		t.Fatalf("unexpected first timeline id %s", resp.Timeline[0].ActivityID)
	}
}

func TestActivityMetricsRequiresUserID(t *testing.T) {
	service := domain.NewService(&mockRepo{})
	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/v1/activities/metrics", nil)
	req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{
		Subject:  "tester",
		TenantID: "tenant-1",
		Scopes: map[string]struct{}{
			auth.ScopeActivitiesRead: {},
		},
		ExpiresAt: time.Now().Add(time.Hour),
	}))

	rr := httptest.NewRecorder()
	handler.activityMetrics(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rr.Code)
	}
}

type mockRepo struct {
	summary  domain.ActivitySummary
	timeline []domain.ActivityAggregate
}

func (m *mockRepo) FindByIdempotency(ctx context.Context, tenantID, userID, idempotencyKey string) (*domain.ActivityAggregate, error) {
	return nil, nil
}

func (m *mockRepo) Create(ctx context.Context, aggregate domain.ActivityAggregate, idempotencyKey string) error {
	return nil
}

func (m *mockRepo) Get(ctx context.Context, tenantID, activityID string) (*domain.ActivityAggregate, error) {
	return nil, nil
}

func (m *mockRepo) ListByUser(ctx context.Context, tenantID, userID string, cursor *domain.Cursor, limit int) ([]domain.ActivityAggregate, *domain.Cursor, error) {
	if limit <= 0 || limit > len(m.timeline) {
		limit = len(m.timeline)
	}
	out := make([]domain.ActivityAggregate, limit)
	copy(out, m.timeline[:limit])
	return out, nil, nil
}

func (m *mockRepo) SummaryByUser(ctx context.Context, tenantID, userID string, window time.Duration) (domain.ActivitySummary, error) {
	return m.summary, nil
}
