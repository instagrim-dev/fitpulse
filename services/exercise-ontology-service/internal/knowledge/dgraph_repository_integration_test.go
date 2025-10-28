//go:build integration

package knowledge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"example.com/exerciseontology/internal/cache"
	"example.com/exerciseontology/internal/domain"
	"example.com/exerciseontology/internal/testsupport"
)

func TestDgraphRepositoryUpsertWithSession(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, endpoint := testsupport.StartDgraph(ctx, t)
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	repo := NewDgraphRepository(endpoint, 10*time.Second)
	service := domain.NewService(repo, cache.NoopInvalidator{})

	exercise := domain.Exercise{
		ID:           "tenant:tempo-ride",
		Name:         "Tempo Ride",
		Difficulty:   "intermediate",
		Targets:      []string{"cardio"},
		SessionCount: 1,
		LastUpdated:  time.Now().UTC(),
	}

	session := domain.ActivitySession{
		ID:          "session-1",
		ExerciseID:  exercise.ID,
		ActivityID:  "activity-1",
		TenantID:    "tenant",
		UserID:      "user",
		Source:      "integration-test",
		Version:     "v1",
		StartedAt:   time.Now().UTC(),
		DurationMin: 30,
		RecordedAt:  time.Now().UTC(),
	}

	_, err := service.UpsertExerciseWithSession(ctx, exercise, session)
	require.NoError(t, err)

	debugQueryDgraph(t, endpoint, exercise.ID)

	var (
		stored   *domain.Exercise
		sessions []domain.ActivitySession
		errGet   error
		errList  error
	)
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		stored, errGet = repo.Get(ctx, exercise.ID)
		if errGet != nil || stored == nil || stored.SessionCount != 1 {
			time.Sleep(time.Second)
			continue
		}
		sessions, errList = repo.ListSessions(ctx, exercise.ID, 5)
		if errList != nil || len(sessions) == 0 {
			time.Sleep(time.Second)
			continue
		}
		if sessions[0].ActivityID == session.ActivityID {
			break
		}
		time.Sleep(time.Second)
	}
	require.NoError(t, errGet, "get exercise from dgraph")
	require.NotNil(t, stored, "exercise should exist")
	require.Equal(t, 1, stored.SessionCount, "session count should update")
	require.WithinDuration(t, session.StartedAt, stored.LastSeenAt, time.Second, "last seen should reflect session start")
	require.NoError(t, errList, "list sessions from dgraph")
	require.NotEmpty(t, sessions, "expected at least one session")
	require.Equal(t, session.ActivityID, sessions[0].ActivityID, "session activity id should match")
}

func TestDgraphRepositoryCRUDAndRelationships(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, endpoint := testsupport.StartDgraph(ctx, t)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	repo := NewDgraphRepository(endpoint, 10*time.Second)
	service := domain.NewService(repo, cache.NoopInvalidator{})

	now := time.Now().UTC().Truncate(time.Second)
	tempo := domain.Exercise{
		ID:           "tenant:tempo-ride",
		Name:         "Tempo Ride",
		Difficulty:   "intermediate",
		Targets:      []string{"cardio"},
		Requires:     []string{"bike"},
		LastUpdated:  now,
		LastSeenAt:   now,
		SessionCount: 0,
	}
	recovery := domain.Exercise{
		ID:           "tenant:recovery-ride",
		Name:         "Recovery Ride",
		Difficulty:   "beginner",
		Targets:      []string{"cardio"},
		Requires:     []string{"bike"},
		LastUpdated:  now,
		LastSeenAt:   now,
		SessionCount: 0,
	}

	_, err := service.UpsertExercise(ctx, tempo)
	require.NoError(t, err)
	_, err = service.UpsertExercise(ctx, recovery)
	require.NoError(t, err)

	fetched, err := repo.Get(ctx, tempo.ID)
	require.NoError(t, err)
	require.Equal(t, tempo.ID, fetched.ID)

	results, err := repo.Search(ctx, "Tempo", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	_, err = service.UpdateRelationships(ctx, tempo.ID, domain.ExerciseRelationships{
		Targets:           []string{"endurance"},
		ComplementaryTo:   []string{recovery.ID},
		Contraindications: []string{},
	})
	require.NoError(t, err)

	updatedTempo, err := repo.Get(ctx, tempo.ID)
	require.NoError(t, err)
	require.Contains(t, updatedTempo.ComplementaryTo, recovery.ID)
	require.Contains(t, updatedTempo.Targets, "endurance")

	updatedRecovery, err := repo.Get(ctx, recovery.ID)
	require.NoError(t, err)
	require.Contains(t, updatedRecovery.ComplementaryTo, tempo.ID)

	err = service.DeleteExercise(ctx, tempo.ID)
	require.NoError(t, err)

	deleted, err := repo.Get(ctx, tempo.ID)
	require.NoError(t, err)
	require.Nil(t, deleted)
}

func debugQueryDgraph(t *testing.T, endpoint, exerciseID string) {
	t.Helper()

	query := fmt.Sprintf(`{"query":"{ exercises(func: eq(exercise_id, \"%s\")) { exercise_id session_count name } }"}`, exerciseID)
	resp, err := http.Post(endpoint+"/query", "application/json", bytes.NewBufferString(query))
	if err != nil {
		t.Logf("dgraph debug query error: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Logf("dgraph debug read error: %v", err)
		return
	}
	t.Logf("dgraph debug response (%d): %s", resp.StatusCode, string(body))
}
