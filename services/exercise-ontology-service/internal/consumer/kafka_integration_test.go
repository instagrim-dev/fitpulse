//go:build integration

package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	kafkaContainer "github.com/testcontainers/testcontainers-go/modules/kafka"

	"example.com/exerciseontology/internal/cache"
	"example.com/exerciseontology/internal/domain"
	"example.com/exerciseontology/internal/knowledge"
	"example.com/exerciseontology/internal/testsupport"
	"example.com/platform/libs/go/events"
)

func TestKafkaActivityEventCreatesExercise(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	kafkaC, err := kafkaContainer.RunContainer(ctx, testcontainers.WithEnv(map[string]string{
		"KAFKA_AUTO_CREATE_TOPICS_ENABLE": "true",
	}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = kafkaC.Terminate(context.Background()) })

	brokers, err := kafkaC.Brokers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, brokers)
	broker := brokers[0]

	topic := "activity_events"

	dgraphContainer, endpoint := testsupport.StartDgraph(ctx, t)
	t.Cleanup(func() { _ = dgraphContainer.Terminate(context.Background()) })

	repo := knowledge.NewDgraphRepository(endpoint, 10*time.Second)
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := NewEnrichmentHandler(service)

	// Seed complementary exercise so relationship updates can be evaluated after enrichment.
	seed := domain.Exercise{
		ID:          ActivityExerciseID("tenant", "Tempo Ride"),
		Name:        "Tempo Ride",
		Difficulty:  "intermediate",
		Targets:     []string{"cardio"},
		Requires:    []string{"bike"},
		LastUpdated: time.Now().UTC(),
	}
	_, err = service.UpsertExercise(ctx, seed)
	require.NoError(t, err)

	_, err = service.UpsertExercise(ctx, domain.Exercise{
		ID:          ActivityExerciseID("tenant", "Yoga Flow"),
		Name:        "Yoga Flow",
		Difficulty:  "beginner",
		Targets:     []string{"flexibility"},
		Requires:    []string{"mat"},
		LastUpdated: time.Now().UTC(),
	})
	require.NoError(t, err)

	conn, err := kafka.Dial("tcp", broker)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}))

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{broker},
		GroupID:     "ontology-integration",
		Topic:       topic,
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.FirstOffset,
	})
	defer reader.Close()

	consumerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	proc := NewProcessor(reader, handler)
	go func() {
		_ = proc.Run(consumerCtx)
	}()

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(broker),
		Topic:                  topic,
		BatchTimeout:           10 * time.Millisecond,
		AllowAutoTopicCreation: true,
	}
	defer writer.Close()

	evt := events.ActivityCreated{
		ActivityID:   "act-int",
		TenantID:     "tenant",
		UserID:       "user",
		ActivityType: "Recovery Ride",
		StartedAt:    time.Now().UTC(),
		DurationMin:  30,
		Source:       "integration-test",
		Version:      "v1",
	}
	payload, err := json.Marshal(evt)
	require.NoError(t, err)

	err = writer.WriteMessages(context.Background(), kafka.Message{
		Key:     []byte(evt.ActivityID),
		Value:   payload,
		Headers: []kafka.Header{{Key: "event_type", Value: []byte("activity.created")}},
	})
	require.NoError(t, err)

	expectID := ActivityExerciseID(evt.TenantID, evt.ActivityType)
	require.Eventually(t, func() bool {
		ex, err := repo.Get(ctx, expectID)
		if err != nil || ex == nil {
			return false
		}
		return ex.SessionCount >= 1
	}, 30*time.Second, 500*time.Millisecond)

	require.Eventually(t, func() bool {
		sessions, err := repo.ListSessions(ctx, expectID, 5)
		if err != nil || len(sessions) == 0 {
			return false
		}
		return sessions[0].ActivityID == evt.ActivityID
	}, 30*time.Second, 500*time.Millisecond)

	stored, err := repo.Get(ctx, expectID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.WithinDuration(t, evt.StartedAt, stored.LastSeenAt, time.Second)
	tempoID := ActivityExerciseID(evt.TenantID, "Tempo Ride")
	require.Contains(t, stored.ComplementaryTo, tempoID)

	tempoExercise, err := repo.Get(ctx, tempoID)
	require.NoError(t, err)
	require.NotNil(t, tempoExercise)
	require.Contains(t, tempoExercise.ComplementaryTo, expectID)
}
