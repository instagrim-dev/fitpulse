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
	sessions  map[string][]domain.ActivitySession
}

// NewInMemoryRepository constructs repository populated with a seed ontology.
func NewInMemoryRepository() *InMemoryRepository {
	repo := &InMemoryRepository{
		exercises: make(map[string]domain.Exercise),
		sessions:  make(map[string][]domain.ActivitySession),
	}
	repo.seed()
	return repo
}

func (r *InMemoryRepository) seed() {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := uuid.NewString()
	r.exercises[id] = domain.Exercise{
		ID:           id,
		Name:         "Bodyweight Squat",
		Difficulty:   "beginner",
		Targets:      []string{"quadriceps", "glutes"},
		Requires:     []string{"none"},
		LastUpdated:  time.Now().UTC(),
		LastSeenAt:   time.Now().UTC(),
		SessionCount: 1,
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
	if exercise.LastSeenAt.IsZero() {
		exercise.LastSeenAt = exercise.LastUpdated
	}

	r.exercises[exercise.ID] = exercise
	return nil
}

// UpsertWithSession records the exercise and appends a session entry.
func (r *InMemoryRepository) UpsertWithSession(ctx context.Context, exercise domain.Exercise, session domain.ActivitySession) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(exercise.ID) == "" {
		exercise.ID = uuid.NewString()
	}
	if strings.TrimSpace(session.ID) == "" {
		session.ID = uuid.NewString()
	}
	if strings.TrimSpace(session.ExerciseID) == "" {
		session.ExerciseID = exercise.ID
	}
	if exercise.LastUpdated.IsZero() {
		exercise.LastUpdated = time.Now().UTC()
	}
	if session.RecordedAt.IsZero() {
		session.RecordedAt = time.Now().UTC()
	}
	if session.StartedAt.IsZero() {
		session.StartedAt = session.RecordedAt
	}
	if session.StartedAt.After(exercise.LastSeenAt) {
		exercise.LastSeenAt = session.StartedAt
	}
	if exercise.LastSeenAt.IsZero() {
		exercise.LastSeenAt = session.RecordedAt
	}

	r.exercises[exercise.ID] = exercise
	slice := append([]domain.ActivitySession{session}, r.sessions[exercise.ID]...)
	r.sessions[exercise.ID] = slice
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

// ListSessions returns in-memory sessions for the exercise.
func (r *InMemoryRepository) ListSessions(ctx context.Context, exerciseID string, limit int) ([]domain.ActivitySession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	slice := r.sessions[exerciseID]
	if len(slice) == 0 {
		return nil, nil
	}
	if limit > 0 && len(slice) > limit {
		slice = slice[:limit]
	}
	out := make([]domain.ActivitySession, len(slice))
	copy(out, slice)
	return out, nil
}

// Delete removes an exercise and its sessions.
func (r *InMemoryRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.exercises, id)
	delete(r.sessions, id)
	return nil
}
