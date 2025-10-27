// Package domain defines the business logic for the activity service.
package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrIdempotentReplay indicates an existing activity was found for the provided idempotency key.
	ErrIdempotentReplay = errors.New("activity already exists for idempotency key")
	// ErrActivityNotFound is returned when an activity cannot be located.
	ErrActivityNotFound = errors.New("activity not found")
)

// ActivityState represents the processing status of an activity.
type ActivityState string

const (
	ActivityStatePending ActivityState = "pending"
	ActivityStateSynced  ActivityState = "synced"
	ActivityStateFailed  ActivityState = "failed"
)

// ActivityAggregate is the domain object stored in Postgres and replayed to downstream stores.
type ActivityAggregate struct {
	ID           string
	TenantID     string
	UserID       string
	ActivityType string
	StartedAt    time.Time
	DurationMin  int
	Source       string
	Version      string
	State        ActivityState
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ActivityRepository captures persistence operations.
type ActivityRepository interface {
	FindByIdempotency(ctx context.Context, tenantID, userID, idempotencyKey string) (*ActivityAggregate, error)
	Create(ctx context.Context, aggregate ActivityAggregate, idempotencyKey string) error
	Get(ctx context.Context, tenantID, activityID string) (*ActivityAggregate, error)
	ListByUser(ctx context.Context, tenantID, userID string, cursor *Cursor, limit int) ([]ActivityAggregate, *Cursor, error)
}

// Service orchestrates activity workflows.
type Service struct {
	repo ActivityRepository
}

// NewService constructs a Service.
func NewService(repo ActivityRepository) *Service {
	return &Service{repo: repo}
}

// CreateActivityInput captures the payload from the API layer.
type CreateActivityInput struct {
	TenantID       string
	UserID         string
	ActivityType   string
	StartedAt      time.Time
	DurationMin    int
	Source         string
	IdempotencyKey string
}

// Cursor models the pagination token.
type Cursor struct {
	StartedAt time.Time
	ID        string
}

// CreateActivity handles idempotent create semantics and outbox recording.
func (s *Service) CreateActivity(ctx context.Context, input CreateActivityInput) (*ActivityAggregate, bool, error) {
	if existing, err := s.repo.FindByIdempotency(ctx, input.TenantID, input.UserID, input.IdempotencyKey); err == nil && existing != nil {
		return existing, true, nil
	}

	now := time.Now().UTC()
	aggregate := ActivityAggregate{
		ID:           uuid.NewString(),
		TenantID:     input.TenantID,
		UserID:       input.UserID,
		ActivityType: input.ActivityType,
		StartedAt:    input.StartedAt.UTC(),
		DurationMin:  input.DurationMin,
		Source:       input.Source,
		Version:      "v1",
		State:        ActivityStatePending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, aggregate, input.IdempotencyKey); err != nil {
		return nil, false, err
	}

	return &aggregate, false, nil
}

// GetActivity fetches by ID.
func (s *Service) GetActivity(ctx context.Context, tenantID, activityID string) (*ActivityAggregate, error) {
	agg, err := s.repo.Get(ctx, tenantID, activityID)
	if err != nil {
		return nil, err
	}
	if agg == nil {
		return nil, ErrActivityNotFound
	}
	return agg, nil
}

// ListActivitiesByUser fetches activities with cursor pagination.
func (s *Service) ListActivitiesByUser(ctx context.Context, tenantID, userID string, cursor *Cursor, limit int) ([]ActivityAggregate, *Cursor, error) {
	return s.repo.ListByUser(ctx, tenantID, userID, cursor, limit)
}
