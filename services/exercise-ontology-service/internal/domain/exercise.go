package domain

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	SessionCount      int       `json:"session_count"`
	LastSeenAt        time.Time `json:"last_seen_at"`
}

// ActivitySession represents an individual performed session tied to an exercise.
type ActivitySession struct {
	ID          string    `json:"id"`
	ExerciseID  string    `json:"exercise_id"`
	ActivityID  string    `json:"activity_id"`
	TenantID    string    `json:"tenant_id"`
	UserID      string    `json:"user_id"`
	Source      string    `json:"source"`
	Version     string    `json:"version"`
	StartedAt   time.Time `json:"started_at"`
	DurationMin int       `json:"duration_min"`
	RecordedAt  time.Time `json:"recorded_at"`
}

// ExerciseRelationships bundles relationship updates.
type ExerciseRelationships struct {
	Targets           []string
	ComplementaryTo   []string
	Contraindications []string
}

// Repository exposes persistence behaviour.
type Repository interface {
	Upsert(ctx context.Context, exercise Exercise) error
	UpsertWithSession(ctx context.Context, exercise Exercise, session ActivitySession) error
	Get(ctx context.Context, id string) (*Exercise, error)
	Search(ctx context.Context, query string, limit int) ([]Exercise, error)
	ListSessions(ctx context.Context, exerciseID string, limit int) ([]ActivitySession, error)
	Delete(ctx context.Context, id string) error
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
	if exercise.LastUpdated.IsZero() {
		exercise.LastUpdated = time.Now().UTC()
	}
	if err := s.repo.Upsert(ctx, exercise); err != nil {
		return Exercise{}, err
	}
	if err := s.cache.Invalidate(ctx, exercise.ID); err != nil {
		return Exercise{}, fmt.Errorf("cache invalidation: %w", err)
	}
	observability.RecordOntologyUpsert(exercise.LastUpdated)
	return exercise, nil
}

// UpsertExerciseWithSession persists exercise metadata and records a linked activity session.
func (s *Service) UpsertExerciseWithSession(ctx context.Context, exercise Exercise, session ActivitySession) (Exercise, error) {
	if strings.TrimSpace(exercise.Name) == "" {
		return Exercise{}, errors.New("name is required")
	}
	if strings.TrimSpace(exercise.ID) == "" {
		exercise.ID = uuid.NewString()
	}
	if exercise.LastUpdated.IsZero() {
		exercise.LastUpdated = time.Now().UTC()
	}
	if strings.TrimSpace(session.ID) == "" {
		session.ID = uuid.NewString()
	}
	if strings.TrimSpace(session.ExerciseID) == "" {
		session.ExerciseID = exercise.ID
	}
	if session.RecordedAt.IsZero() {
		session.RecordedAt = time.Now().UTC()
	}
	candidateSeen := session.StartedAt
	if candidateSeen.IsZero() {
		candidateSeen = session.RecordedAt
	}
	if candidateSeen.IsZero() {
		candidateSeen = time.Now().UTC()
	}
	if candidateSeen.After(exercise.LastSeenAt) {
		exercise.LastSeenAt = candidateSeen
	}
	if err := s.repo.UpsertWithSession(ctx, exercise, session); err != nil {
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

// ListSessions returns recent sessions for an exercise.
func (s *Service) ListSessions(ctx context.Context, exerciseID string, limit int) ([]ActivitySession, error) {
	if strings.TrimSpace(exerciseID) == "" {
		return nil, errors.New("exercise_id is required")
	}
	return s.repo.ListSessions(ctx, exerciseID, limit)
}

// DeleteExercise removes an exercise and associated sessions.
func (s *Service) DeleteExercise(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("exercise_id is required")
	}
	exercise, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if exercise == nil {
		return ErrExerciseNotFound
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return fmt.Errorf("cache invalidation: %w", err)
	}
	return nil
}

// UpdateRelationships replaces relationship collections for the exercise and manages symmetric edges.
func (s *Service) UpdateRelationships(ctx context.Context, id string, rel ExerciseRelationships) (Exercise, error) {
	if strings.TrimSpace(id) == "" {
		return Exercise{}, errors.New("exercise_id is required")
	}

	base, err := s.repo.Get(ctx, id)
	if err != nil {
		return Exercise{}, err
	}
	if base == nil {
		return Exercise{}, ErrExerciseNotFound
	}

	now := time.Now().UTC()
	base.LastUpdated = now
	base.Targets = normalizeStrings(rel.Targets)

	newComplementary := normalizeStrings(rel.ComplementaryTo)
	newContra := normalizeStrings(rel.Contraindications)

	if err := ensureNotSelf(id, newComplementary); err != nil {
		return Exercise{}, err
	}
	if err := ensureNotSelf(id, newContra); err != nil {
		return Exercise{}, err
	}

	complementarySet := toSet(newComplementary)
	contraSet := toSet(newContra)

	oldComplementary := toSet(base.ComplementaryTo)
	oldContra := toSet(base.Contraindications)

	// Validate references exist and build cache of retrieved exercises.
	related := make(map[string]*Exercise)
	for ref := range complementarySet {
		ex, err := s.ensureExercise(ctx, ref, related)
		if err != nil {
			return Exercise{}, err
		}
		ex.ComplementaryTo = addToSet(ex.ComplementaryTo, id)
	}
	for ref := range contraSet {
		ex, err := s.ensureExercise(ctx, ref, related)
		if err != nil {
			return Exercise{}, err
		}
		ex.Contraindications = addToSet(ex.Contraindications, id)
	}

	// Remove stale symmetric links.
	for ref := range difference(oldComplementary, complementarySet) {
		ex, err := s.ensureExercise(ctx, ref, related)
		if err != nil {
			return Exercise{}, err
		}
		ex.ComplementaryTo = removeFromSet(ex.ComplementaryTo, id)
	}
	for ref := range difference(oldContra, contraSet) {
		ex, err := s.ensureExercise(ctx, ref, related)
		if err != nil {
			return Exercise{}, err
		}
		ex.Contraindications = removeFromSet(ex.Contraindications, id)
	}

	base.ComplementaryTo = setToSlice(complementarySet)
	base.Contraindications = setToSlice(contraSet)

	if err := s.repo.Upsert(ctx, *base); err != nil {
		return Exercise{}, err
	}
	if err := s.cache.Invalidate(ctx, base.ID); err != nil {
		return Exercise{}, fmt.Errorf("cache invalidation: %w", err)
	}

	for _, ex := range related {
		ex.LastUpdated = now
		if err := s.repo.Upsert(ctx, *ex); err != nil {
			return Exercise{}, err
		}
		if err := s.cache.Invalidate(ctx, ex.ID); err != nil {
			return Exercise{}, fmt.Errorf("cache invalidation: %w", err)
		}
	}

	observability.RecordOntologyUpsert(now)
	return *base, nil
}

func ensureNotSelf(id string, refs []string) error {
	for _, ref := range refs {
		if ref == id {
			return fmt.Errorf("exercise %s cannot reference itself", id)
		}
	}
	return nil
}

func normalizeStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		clean := strings.TrimSpace(v)
		if clean == "" {
			continue
		}
		set[clean] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	slices.Sort(out)
	return out
}

func toSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		set[v] = struct{}{}
	}
	return set
}

func difference(a, b map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for k := range a {
		if _, ok := b[k]; !ok {
			result[k] = struct{}{}
		}
	}
	return result
}

func setToSlice(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func addToSet(values []string, item string) []string {
	set := toSet(values)
	set[item] = struct{}{}
	return setToSlice(set)
}

func removeFromSet(values []string, item string) []string {
	set := toSet(values)
	delete(set, item)
	return setToSlice(set)
}

func (s *Service) ensureExercise(ctx context.Context, id string, cache map[string]*Exercise) (*Exercise, error) {
	if ex, ok := cache[id]; ok {
		return ex, nil
	}
	ex, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if ex == nil {
		return nil, fmt.Errorf("exercise %s not found", id)
	}
	clone := *ex
	cache[id] = &clone
	return &clone, nil
}
