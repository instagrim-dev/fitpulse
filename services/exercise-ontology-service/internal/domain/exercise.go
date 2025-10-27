package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"example.com/exerciseontology/internal/cache"
	"example.com/exerciseontology/internal/observability"
)

// Exercise represents an ontology node.
type Exercise struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Difficulty        string    `json:"difficulty"`
	Targets           []string  `json:"targets"`
	Requires          []string  `json:"requires"`
	Contraindications []string  `json:"contraindicated_with"`
	ComplementaryTo   []string  `json:"complementary_to"`
	LastUpdated       time.Time `json:"last_updated"`
}

// Repository exposes persistence behaviour.
type Repository interface {
	Upsert(ctx context.Context, exercise Exercise) error
	Get(ctx context.Context, id string) (*Exercise, error)
	Search(ctx context.Context, query string, limit int) ([]Exercise, error)
}

// Service contains business logic.
type Service struct {
	repo  Repository
	cache cache.Invalidator
}

var (
	// ErrExerciseNotFound indicates the entity does not exist.
	ErrExerciseNotFound = errors.New("exercise not found")
)

// NewService constructs a new Service.
func NewService(repo Repository, invalidator cache.Invalidator) *Service {
	if invalidator == nil {
		invalidator = cache.NoopInvalidator{}
	}
	return &Service{repo: repo, cache: invalidator}
}

// UpsertExercise creates or updates entries.
func (s *Service) UpsertExercise(ctx context.Context, exercise Exercise) (Exercise, error) {
	if strings.TrimSpace(exercise.Name) == "" {
		return Exercise{}, errors.New("name is required")
	}
	if strings.TrimSpace(exercise.ID) == "" {
		exercise.ID = uuid.NewString()
	}
	exercise.LastUpdated = time.Now().UTC()
	if err := s.repo.Upsert(ctx, exercise); err != nil {
		return Exercise{}, err
	}
	if err := s.cache.Invalidate(ctx, exercise.ID); err != nil {
		return Exercise{}, fmt.Errorf("cache invalidation: %w", err)
	}
	observability.RecordOntologyUpsert(exercise.LastUpdated)
	return exercise, nil
}

// GetExercise retrieves by ID.
func (s *Service) GetExercise(ctx context.Context, id string) (*Exercise, error) {
	exercise, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if exercise == nil {
		return nil, ErrExerciseNotFound
	}
	observability.RecordOntologyRead(exercise.LastUpdated)
	return exercise, nil
}

// SearchExercises finds matches.
func (s *Service) SearchExercises(ctx context.Context, query string, limit int) ([]Exercise, error) {
	exercises, err := s.repo.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	latest := time.Time{}
	for _, ex := range exercises {
		if ex.LastUpdated.After(latest) {
			latest = ex.LastUpdated
		}
	}
	observability.RecordOntologyRead(latest)
	return exercises, nil
}
