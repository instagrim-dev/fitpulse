package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"example.com/exerciseontology/internal/cache"
	"example.com/exerciseontology/internal/domain"
	"example.com/exerciseontology/internal/knowledge"
	"example.com/platform/libs/go/events"
)

func TestActivityExerciseID(t *testing.T) {
	id := ActivityExerciseID("tenant", "High Intensity Ride!")
	require.Equal(t, "tenant:high-intensity-ride", id)
}

func TestLookupMetadataFallsBack(t *testing.T) {
	meta := lookupMetadata("unknown type")
	require.Equal(t, "unspecified", meta.Difficulty)
	require.Contains(t, meta.Requires, "activity-log")
}

func TestEnrichmentHandlerCreatesExercise(t *testing.T) {
	repo := knowledge.NewInMemoryRepository()
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := NewEnrichmentHandler(service)

	// Seed an existing exercise to ensure the handler increments session counts.
	_, err := service.UpsertExercise(context.Background(), domain.Exercise{
		ID:           ActivityExerciseID("tenant", "Tempo Ride"),
		Name:         "Tempo Ride",
		SessionCount: 1,
		LastUpdated:  time.Now().UTC().Add(-2 * time.Hour),
		LastSeenAt:   time.Now().UTC().Add(-2 * time.Hour),
	})
	require.NoError(t, err)

	startedAt := time.Date(2025, time.October, 27, 12, 0, 0, 0, time.UTC)
	evt := events.ActivityCreated{
		ActivityID:   "act-123",
		TenantID:     "tenant",
		UserID:       "user",
		ActivityType: "Tempo Ride",
		StartedAt:    startedAt,
		DurationMin:  45,
	}
	payload, err := json.Marshal(evt)
	require.NoError(t, err)

	msg := Message{
		Headers:   map[string]string{"event_type": "activity.created"},
		Payload:   payload,
		Timestamp: startedAt,
	}
	err = handler.Handle(context.Background(), msg)
	require.NoError(t, err)

	stored, err := repo.Get(context.Background(), ActivityExerciseID("tenant", "Tempo Ride"))
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, "Tempo Ride", stored.Name)
	require.Equal(t, 2, stored.SessionCount)
	require.Equal(t, "intermediate", stored.Difficulty)
	require.Contains(t, stored.Targets, "cardio")
	require.Contains(t, stored.Requires, "bike")
	require.False(t, stored.LastSeenAt.Before(startedAt), "expected last seen to be on/after the session start")

	complementaryID := ActivityExerciseID("tenant", "Recovery Ride")
	require.Contains(t, stored.ComplementaryTo, complementaryID)

	sessions, err := repo.ListSessions(context.Background(), stored.ID, 10)
	require.NoError(t, err)
	require.NotEmpty(t, sessions)
	require.Equal(t, "act-123", sessions[0].ActivityID)
	require.Equal(t, stored.ID, sessions[0].ExerciseID)
}
