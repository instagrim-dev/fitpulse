//go:build integration

package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	kafkaContainer "github.com/testcontainers/testcontainers-go/modules/kafka"

	testhelpers "example.com/exerciseontology/pkg/testhelpers"
	"testing"
)

func TestDLQReplayTriggersEnrichmentPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	// Postgres setup for activity service outbox/DLQ tables.
	pool, cleanup := setupPostgres(t, ctx)
	defer cleanup()

	tenantID := uuid.NewString()
	accountID := uuid.NewString()
	activityID := uuid.NewString()

	payload := map[string]any{
		"activity_id":   activityID,
		"tenant_id":     tenantID,
		"user_id":       accountID,
		"activity_type": "Recovery Ride",
		"started_at":    time.Now().UTC().Truncate(time.Second),
		"duration_min":  35,
		"source":        "integration-test",
		"version":       "v1",
	}
	insertOutboxPayload(t, ctx, pool, tenantID, activityID, payload)

	registry := &stubRegistry{id: 100}

	// 1. Initial dispatch fails and moves the message to DLQ.
	failingProducer := &stubProducer{err: errors.New("upstream kafka unavailable")}
	dispatcher := NewDispatcher(pool, failingProducer, registry, 5*time.Millisecond, 10)
	require.NoError(t, dispatcher.processBatch(ctx))

	var dlqCount int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_dlq`).Scan(&dlqCount)
	require.NoError(t, err)
	require.Equal(t, 1, dlqCount, "expected message routed to DLQ on failure")

	// 2. Requeue the DLQ entry.
	manager := NewDLQManager(pool, 5, time.Second)
	replayed, err := manager.RunOnce(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, replayed)

	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_dlq`).Scan(&dlqCount)
	require.NoError(t, err)
	require.Equal(t, 0, dlqCount, "expected DLQ cleared after requeue")

	// 3. Start Kafka and Dgraph containers plus enrichment consumer.
	kContainer, err := kafkaContainer.RunContainer(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = kContainer.Terminate(context.Background()) })

	brokers, err := kContainer.Brokers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, brokers)
	broker := brokers[0]

	conn, err := kafka.Dial("tcp", broker)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.CreateTopics(kafka.TopicConfig{
		Topic:             "activity_events",
		NumPartitions:     1,
		ReplicationFactor: 1,
	}))

	dContainer, endpoint, err := testhelpers.StartDgraph(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = dContainer.Terminate(context.Background()) })

	consumerHandle, err := testhelpers.StartEnrichmentConsumer(ctx, brokers, "activity_events", endpoint)
	require.NoError(t, err)
	t.Cleanup(func() { _ = consumerHandle.Stop() })

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(broker),
		Topic:                  "activity_events",
		AllowAutoTopicCreation: false,
		BatchTimeout:           50 * time.Millisecond,
	}
	defer writer.Close()

	producer := &jsonKafkaProducer{writer: writer}
	dispatcher = NewDispatcher(pool, producer, registry, 5*time.Millisecond, 10)
	require.NoError(t, dispatcher.processBatch(ctx))

	expectExerciseID := testhelpers.ExerciseID(tenantID, payload["activity_type"].(string))

	require.Eventually(t, func() bool {
		exists, count, _, err := testhelpers.ExerciseSnapshot(ctx, endpoint, expectExerciseID)
		if err != nil || !exists {
			return false
		}
		return count >= 1
	}, 45*time.Second, time.Second, "expected enrichment pipeline to project activity into Dgraph")

	sessionExists, err := testhelpers.SessionExists(ctx, endpoint, expectExerciseID, activityID, 10)
	require.NoError(t, err)
	require.True(t, sessionExists, "expected activity session to be persisted in Dgraph")
}

func insertOutboxPayload(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, aggregateID string, payload map[string]any) {
	t.Helper()

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID)
	require.NoError(t, err)

	_, err = tx.Exec(ctx,
		`INSERT INTO outbox (tenant_id, aggregate_type, aggregate_id, event_type, topic, schema_subject, partition_key, payload)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		tenantID,
		"activity",
		aggregateID,
		"activity.created",
		"activity_events",
		"activity_events-value",
		fmt.Sprintf("%s:%s", tenantID, aggregateID),
		payloadBytes,
	)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
}

type jsonKafkaProducer struct {
	writer *kafka.Writer
}

// WriteMessages satisfies the dispatcher producer interface while stripping the Confluent framing.
func (p *jsonKafkaProducer) WriteMessages(ctx context.Context, topic string, msgs ...kafka.Message) error {
	trimmed := make([]kafka.Message, len(msgs))
	for i, msg := range msgs {
		value := msg.Value
		if len(value) > 5 {
			value = value[5:]
		}
		trimmed[i] = kafka.Message{
			Key:     msg.Key,
			Value:   append([]byte(nil), value...),
			Time:    msg.Time,
			Headers: msg.Headers,
		}
	}
	return p.writer.WriteMessages(ctx, trimmed...)
}
