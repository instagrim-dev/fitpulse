package knowledge

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"example.com/exerciseontology/internal/domain"
)

// InMemoryRepository stores exercises in memory for local development.
type InMemoryRepository struct {
	mu        sync.RWMutex
	exercises map[string]domain.Exercise
}

// NewInMemoryRepository constructs repository populated with a seed ontology.
func NewInMemoryRepository() *InMemoryRepository {
	repo := &InMemoryRepository{
		exercises: make(map[string]domain.Exercise),
	}
	repo.seed()
	return repo
}

func (r *InMemoryRepository) seed() {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := uuid.NewString()
	r.exercises[id] = domain.Exercise{
		ID:          id,
		Name:        "Bodyweight Squat",
		Difficulty:  "beginner",
		Targets:     []string{"quadriceps", "glutes"},
		Requires:    []string{"none"},
		LastUpdated: time.Now().UTC(),
	}
}

// Upsert implements domain.Repository.
func (r *InMemoryRepository) Upsert(ctx context.Context, exercise domain.Exercise) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(exercise.ID) == "" {
		exercise.ID = uuid.NewString()
	}
	if exercise.LastUpdated.IsZero() {
		exercise.LastUpdated = time.Now().UTC()
	}

	r.exercises[exercise.ID] = exercise
	return nil
}

// Get returns entity by ID.
func (r *InMemoryRepository) Get(ctx context.Context, id string) (*domain.Exercise, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	exercise, ok := r.exercises[id]
	if !ok {
		return nil, nil
	}
	return &exercise, nil
}

// Search performs naive substring search.
func (r *InMemoryRepository) Search(ctx context.Context, query string, limit int) ([]domain.Exercise, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSpace(query))
	results := make([]domain.Exercise, 0)
	for _, exercise := range r.exercises {
		if normalized == "" || strings.Contains(strings.ToLower(exercise.Name), normalized) {
			results = append(results, exercise)
		}
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}
