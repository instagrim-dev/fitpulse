//go:build integration

package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestDispatcherPublishesMessages(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPostgres(t, ctx)
	defer cleanup()

	tenantID := uuid.NewString()
	aggregateID := uuid.NewString()
	require.NotZero(t, seedOutbox(t, ctx, pool, tenantID, aggregateID, "activity.created"))

	producer := &stubProducer{}
	registry := &stubRegistry{id: 42}
	dispatcher := NewDispatcher(pool, producer, registry, 10*time.Millisecond, 5)

	beforeDelivered := testutil.ToFloat64(deliveredCounter)
	beforeHistogram := histogramSampleCount(t)

	require.NoError(t, dispatcher.processBatch(ctx))

	require.Len(t, producer.writes, 1)
	require.Equal(t, "activity_events", producer.writes[0].topic)
	require.Len(t, producer.writes[0].messages, 1)

	afterDelivered := testutil.ToFloat64(deliveredCounter)
	require.InDelta(t, beforeDelivered+1, afterDelivered, 0.0001)
	afterHistogram := histogramSampleCount(t)
	require.Greater(t, afterHistogram, beforeHistogram)

	var published int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE published_at IS NOT NULL`).Scan(&published))
	require.Equal(t, 1, published)
}

func TestDispatcherRoutesMessagesToDLQOnFailure(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPostgres(t, ctx)
	defer cleanup()

	tenantID := uuid.NewString()
	aggregateID := uuid.NewString()
	require.NotZero(t, seedOutbox(t, ctx, pool, tenantID, aggregateID, "activity.state_changed"))

	producer := &stubProducer{err: errors.New("kafka write failed")}
	registry := &stubRegistry{id: 7}
	dispatcher := NewDispatcher(pool, producer, registry, 10*time.Millisecond, 5)

	beforeFailed := testutil.ToFloat64(failedCounter)
	beforeDLQ := testutil.ToFloat64(dlqCounter.WithLabelValues("activity_events"))

	require.NoError(t, dispatcher.processBatch(ctx))

	afterFailed := testutil.ToFloat64(failedCounter)
	require.InDelta(t, beforeFailed+1, afterFailed, 0.0001)
	afterDLQ := testutil.ToFloat64(dlqCounter.WithLabelValues("activity_events"))
	require.InDelta(t, beforeDLQ+1, afterDLQ, 0.0001)

	var dlqCount int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_dlq WHERE tenant_id = $1`, tenantID).Scan(&dlqCount)
	require.NoError(t, err)
	require.Equal(t, 1, dlqCount)

	var published int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE published_at IS NOT NULL`).Scan(&published)
	require.NoError(t, err)
	require.Equal(t, 1, published)
}

func TestDispatcherCachesSchemaIDsAcrossBatch(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPostgres(t, ctx)
	defer cleanup()

	tenantID := uuid.NewString()
	require.NotZero(t, seedOutbox(t, ctx, pool, tenantID, uuid.NewString(), "activity.created"))
	require.NotZero(t, seedOutbox(t, ctx, pool, tenantID, uuid.NewString(), "activity.created"))

	producer := &stubProducer{}
	registry := &stubRegistry{id: 21}
	dispatcher := NewDispatcher(pool, producer, registry, 10*time.Millisecond, 5)

	beforeDelivered := testutil.ToFloat64(deliveredCounter)
	beforeHistogram := histogramSampleCount(t)

	require.NoError(t, dispatcher.processBatch(ctx))

	require.Len(t, producer.writes, 1)
	require.Len(t, producer.writes[0].messages, 2)
	require.Len(t, registry.calls, 1, "schema registry should be invoked once due to cache")

	afterDelivered := testutil.ToFloat64(deliveredCounter)
	require.InDelta(t, beforeDelivered+2, afterDelivered, 0.0001)

	afterHistogram := histogramSampleCount(t)
	require.Greater(t, afterHistogram, beforeHistogram)
}

func TestDispatcherUnknownSchemaMovesEventsToDLQ(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPostgres(t, ctx)
	defer cleanup()

	tenantID := uuid.NewString()
	eventID := seedOutbox(t, ctx, pool, tenantID, uuid.NewString(), "activity.unknown")
	require.NotZero(t, eventID)

	producer := &stubProducer{}
	registry := &stubRegistry{id: 99}
	dispatcher := NewDispatcher(pool, producer, registry, 10*time.Millisecond, 5)

	beforeFailed := testutil.ToFloat64(failedCounter)
	beforeDLQ := testutil.ToFloat64(dlqCounter.WithLabelValues("activity_events"))

	require.NoError(t, dispatcher.processBatch(ctx))

	require.Empty(t, producer.writes, "unknown schema should skip kafka writes")
	require.Empty(t, registry.calls, "schema registry should not be invoked when metadata missing")

	var dlqCount int
	var reason string
	err := pool.QueryRow(ctx, `SELECT COUNT(*), MAX(reason) FROM outbox_dlq WHERE event_id = $1`, eventID).Scan(&dlqCount, &reason)
	require.NoError(t, err)
	require.Equal(t, 1, dlqCount)
	require.Contains(t, reason, "no schema metadata for event_type=activity.unknown")

	var publishedAt time.Time
	err = pool.QueryRow(ctx, `SELECT published_at FROM outbox WHERE event_id = $1`, eventID).Scan(&publishedAt)
	require.NoError(t, err)
	require.False(t, publishedAt.IsZero(), "event should still be marked as published")

	afterFailed := testutil.ToFloat64(failedCounter)
	require.InDelta(t, beforeFailed+1, afterFailed, 0.0001)
	afterDLQ := testutil.ToFloat64(dlqCounter.WithLabelValues("activity_events"))
	require.InDelta(t, beforeDLQ+1, afterDLQ, 0.0001)
}

type stubProducer struct {
	mu     sync.Mutex
	err    error
	writes []writtenBatch
}

type writtenBatch struct {
	topic    string
	messages []kafka.Message
}

func (s *stubProducer) WriteMessages(ctx context.Context, topic string, msgs ...kafka.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}

	copied := make([]kafka.Message, len(msgs))
	copy(copied, msgs)

	s.writes = append(s.writes, writtenBatch{
		topic:    topic,
		messages: copied,
	})
	return nil
}

type stubRegistry struct {
	mu    sync.Mutex
	id    int
	err   error
	calls []schemaCall
}

type schemaCall struct {
	subject string
	schema  string
}

func (s *stubRegistry) EnsureSchema(ctx context.Context, subject string, schema string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls = append(s.calls, schemaCall{subject: subject, schema: schema})
	if s.err != nil {
		return 0, s.err
	}
	if s.id == 0 {
		s.id = 1
	}
	return s.id, nil
}

func setupPostgres(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()

	pg, err := postgrescontainer.RunContainer(ctx,
		postgrescontainer.WithDatabase("fitness"),
		postgrescontainer.WithUsername("platform"),
		postgrescontainer.WithPassword("platform"),
	)
	require.NoError(t, err)

	connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, waitForDatabase(ctx, connStr))

	runMigrations(t, ctx, connStr)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	cleanup := func() {
		pool.Close()
		_ = pg.Terminate(ctx)
	}
	return pool, cleanup
}

func histogramSampleCount(t *testing.T) uint64 {
	t.Helper()

	metric := &dto.Metric{}
	require.NoError(t, batchDuration.Write(metric))
	hist := metric.GetHistogram()
	require.NotNil(t, hist)
	return hist.GetSampleCount()
}

func seedOutbox(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, aggregateID, eventType string) int64 {
	t.Helper()

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID)
	require.NoError(t, err)

	payloadBytes, err := json.Marshal(map[string]any{
		"activity_id": aggregateID,
		"tenant_id":   tenantID,
	})
	require.NoError(t, err)

	row := tx.QueryRow(ctx,
		`INSERT INTO outbox (tenant_id, aggregate_type, aggregate_id, event_type, topic, schema_subject, partition_key, payload)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
         RETURNING event_id`,
		tenantID,
		"activity",
		aggregateID,
		eventType,
		"activity_events",
		"activity_events-value",
		tenantID+":"+aggregateID,
		payloadBytes,
	)

	var eventID int64
	require.NoError(t, row.Scan(&eventID))
	require.NoError(t, tx.Commit(ctx))
	return eventID
}

func runMigrations(t *testing.T, ctx context.Context, connStr string) {
	t.Helper()

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	migrationsDir := resolvePath(t, "../../../../db/postgres/migrations")
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "expected at least one migration .up.sql file")

	sort.Strings(files)

	for _, file := range files {
		contents, readErr := os.ReadFile(file)
		require.NoErrorf(t, readErr, "read migration %s", file)

		if _, execErr := pool.Exec(ctx, string(contents)); execErr != nil {
			require.NoErrorf(t, execErr, "execute migration %s", file)
		}
	}
}

func resolvePath(t *testing.T, rel string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), rel)
}

func waitForDatabase(ctx context.Context, connStr string) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		pool, err := pgxpool.New(ctx, connStr)
		if err == nil {
			err = pool.Ping(ctx)
			pool.Close()
			if err == nil {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(time.Second)
	}
}
