package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/activity/internal/domain"
	"example.com/activity/internal/observability"
	platformevents "example.com/platform/libs/go/events"
)

// Repository provides Postgres-backed persistence for activities and outbox events.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// FindByIdempotency checks if an activity already exists for the supplied idempotency key.
func (r *Repository) FindByIdempotency(ctx context.Context, tenantID, userID, idempotencyKey string) (*domain.ActivityAggregate, error) {
	if idempotencyKey == "" {
		return nil, nil
	}

	const query = `SELECT activity_id, tenant_id, user_id, activity_type, started_at, duration_min, source, version, processing_state, created_at, updated_at
        FROM activities WHERE tenant_id=$1::uuid AND user_id=$2::uuid AND idempotency_key=$3`

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, err
	}

	row := tx.QueryRow(ctx, query, tenantID, userID, idempotencyKey)
	var agg domain.ActivityAggregate
	if err := row.Scan(&agg.ID, &agg.TenantID, &agg.UserID, &agg.ActivityType, &agg.StartedAt, &agg.DurationMin, &agg.Source, &agg.Version, &agg.State, &agg.CreatedAt, &agg.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, tx.Commit(ctx)
		}
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &agg, nil
}

// Create persists the aggregate and records outbox events inside a single transaction.
func (r *Repository) Create(ctx context.Context, aggregate domain.ActivityAggregate, idempotencyKey string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	if _, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", aggregate.TenantID); err != nil {
		return err
	}

	insertActivity := `INSERT INTO activities (activity_id, tenant_id, user_id, activity_type, started_at, duration_min, source, idempotency_key, version, processing_state, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

	_, err = tx.Exec(ctx, insertActivity,
		aggregate.ID,
		aggregate.TenantID,
		aggregate.UserID,
		aggregate.ActivityType,
		aggregate.StartedAt,
		aggregate.DurationMin,
		aggregate.Source,
		nullIfEmpty(idempotencyKey),
		aggregate.Version,
		aggregate.State,
		aggregate.CreatedAt,
		aggregate.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if err = r.insertOutbox(ctx, tx, aggregate, "activity.created", platformevents.ActivityCreated{
		ActivityID:   aggregate.ID,
		TenantID:     aggregate.TenantID,
		UserID:       aggregate.UserID,
		ActivityType: aggregate.ActivityType,
		StartedAt:    aggregate.StartedAt,
		DurationMin:  aggregate.DurationMin,
		Source:       aggregate.Source,
		Version:      aggregate.Version,
	}); err != nil {
		return err
	}

	if err = r.insertOutbox(ctx, tx, aggregate, "activity.state_changed", platformevents.ActivityStateChanged{
		ActivityID: aggregate.ID,
		TenantID:   aggregate.TenantID,
		UserID:     aggregate.UserID,
		State:      string(aggregate.State),
		OccurredAt: aggregate.UpdatedAt,
	}); err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	observability.RecordActivityPersisted(aggregate.UpdatedAt)
	return nil
}

func (r *Repository) insertOutbox(ctx context.Context, tx pgx.Tx, aggregate domain.ActivityAggregate, eventType string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	meta := eventCatalog[eventType]
	if meta.Topic == "" {
		return fmt.Errorf("unknown event type: %s", eventType)
	}

	partitionKey := meta.PartitionKeyFn(aggregate)
	dedupeKey := fmt.Sprintf("%s:%s", aggregate.ID, eventType)

	const stmt = `INSERT INTO outbox (tenant_id, aggregate_type, aggregate_id, event_type, topic, schema_subject, partition_key, payload, dedupe_key)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`

	_, err = tx.Exec(ctx, stmt,
		aggregate.TenantID,
		"activity",
		aggregate.ID,
		eventType,
		meta.Topic,
		meta.SchemaSubject,
		partitionKey,
		body,
		dedupeKey,
	)
	return err
}

// Get retrieves an activity by ID.
func (r *Repository) Get(ctx context.Context, tenantID, activityID string) (*domain.ActivityAggregate, error) {
	const query = `SELECT a.activity_id,
	                     a.tenant_id,
	                     a.user_id,
	                     a.activity_type,
	                     a.started_at,
	                     a.duration_min,
	                     a.source,
	                     a.version,
	                     a.processing_state,
	                     a.created_at,
	                     a.updated_at,
	                     dlq.reason,
	                     dlq.next_retry_at,
	                     dlq.quarantined_at,
	                     COALESCE(dlq.replay_available, FALSE)
	                FROM activities AS a
	                LEFT JOIN LATERAL (
	                    SELECT reason,
	                           next_retry_at,
	                           quarantined_at,
	                           (quarantined_at IS NULL) AS replay_available
	                      FROM outbox_dlq
	                     WHERE aggregate_type = 'activity'
	                       AND aggregate_id = a.activity_id::text
	                     ORDER BY created_at DESC
	                     LIMIT 1
	                ) AS dlq ON TRUE
	               WHERE a.tenant_id = $1::uuid AND a.activity_id = $2::uuid`

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, err
	}

	row := tx.QueryRow(ctx, query, tenantID, activityID)
	var (
		agg             domain.ActivityAggregate
		reason          sql.NullString
		nextRetryRaw    pgtype.Timestamptz
		quarantinedRaw  pgtype.Timestamptz
		replayAvailable bool
	)
	if err := row.Scan(&agg.ID, &agg.TenantID, &agg.UserID, &agg.ActivityType, &agg.StartedAt, &agg.DurationMin, &agg.Source, &agg.Version, &agg.State, &agg.CreatedAt, &agg.UpdatedAt, &reason, &nextRetryRaw, &quarantinedRaw, &replayAvailable); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return nil, err
	}
	if reason.Valid {
		value := reason.String
		agg.FailureReason = &value
	}
	if nextRetryRaw.Valid {
		t := nextRetryRaw.Time
		agg.NextRetryAt = &t
	}
	if quarantinedRaw.Valid {
		t := quarantinedRaw.Time
		agg.QuarantinedAt = &t
	}
	agg.ReplayAvailable = replayAvailable
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &agg, nil
}

// SummaryByUser aggregates activity statistics for the specified user within the optional window.
func (r *Repository) SummaryByUser(ctx context.Context, tenantID, userID string, window time.Duration) (domain.ActivitySummary, error) {
	var summary domain.ActivitySummary

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return summary, err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return summary, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return summary, err
	}

	windowSeconds := int64(window / time.Second)
	if windowSeconds < 0 {
		windowSeconds = 0
	}

	const summaryQuery = `SELECT
	    COUNT(*) AS total,
	    COUNT(*) FILTER (WHERE processing_state = 'pending') AS pending,
	    COUNT(*) FILTER (WHERE processing_state = 'synced') AS synced,
	    COUNT(*) FILTER (WHERE processing_state = 'failed') AS failed,
	    AVG(duration_min)::float AS avg_duration_minutes,
	    AVG(EXTRACT(EPOCH FROM (a.updated_at - a.started_at))) FILTER (WHERE processing_state = 'synced') AS avg_processing_seconds,
	    MAX(EXTRACT(EPOCH FROM (NOW() - a.started_at))) FILTER (WHERE processing_state = 'pending') AS oldest_pending_seconds,
	    MAX(a.started_at) AS last_activity_at
	  FROM activities AS a
	  WHERE a.tenant_id = $1::uuid
	    AND a.user_id = $2::uuid
	    AND ($3 = 0 OR a.started_at >= NOW() - ($3::double precision * INTERVAL '1 second'))`

	var (
		total, pending, synced, failed int64
		avgDuration                    sql.NullFloat64
		avgProcessing                  sql.NullFloat64
		oldestPending                  sql.NullFloat64
		lastActivity                   pgtype.Timestamptz
	)

	if err := tx.QueryRow(ctx, summaryQuery, tenantID, userID, windowSeconds).Scan(
		&total,
		&pending,
		&synced,
		&failed,
		&avgDuration,
		&avgProcessing,
		&oldestPending,
		&lastActivity,
	); err != nil {
		return summary, err
	}

	summary.Total = int(total)
	summary.Pending = int(pending)
	summary.Synced = int(synced)
	summary.Failed = int(failed)
	if avgDuration.Valid {
		summary.AverageDurationMinutes = avgDuration.Float64
	}
	if avgProcessing.Valid {
		summary.AverageProcessingSeconds = avgProcessing.Float64
	}
	if oldestPending.Valid {
		summary.OldestPendingAgeSeconds = oldestPending.Float64
	}
	if lastActivity.Valid {
		t := lastActivity.Time.UTC()
		summary.LastActivityAt = &t
	}

	if err := tx.Commit(ctx); err != nil {
		return summary, err
	}
	return summary, nil
}

// ListByUser returns activities for a user ordered by time.
func (r *Repository) ListByUser(ctx context.Context, tenantID, userID string, cursor *domain.Cursor, limit int) ([]domain.ActivityAggregate, *domain.Cursor, error) {
	args := []interface{}{tenantID, userID, limit}
	query := `SELECT a.activity_id,
	                 a.tenant_id,
	                 a.user_id,
	                 a.activity_type,
	                 a.started_at,
	                 a.duration_min,
	                 a.source,
	                 a.version,
	                 a.processing_state,
	                 a.created_at,
	                 a.updated_at,
	                 dlq.reason,
	                 dlq.next_retry_at,
	                 dlq.quarantined_at,
	                 COALESCE(dlq.replay_available, FALSE)
	          FROM activities AS a
	          LEFT JOIN LATERAL (
	              SELECT reason,
	                     next_retry_at,
	                     quarantined_at,
	                     (quarantined_at IS NULL) AS replay_available
	                FROM outbox_dlq
	               WHERE aggregate_type = 'activity'
	                 AND aggregate_id = a.activity_id::text
	               ORDER BY created_at DESC
	               LIMIT 1
	          ) AS dlq ON TRUE
	         WHERE a.tenant_id = $1 AND a.user_id = $2`

	if cursor != nil {
		query += ` AND (a.started_at, a.activity_id) < ($4, $5)`
		args = append(args, cursor.StartedAt, cursor.ID)
	}

	query += ` ORDER BY a.started_at DESC, a.activity_id DESC LIMIT $3`

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, nil, err
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	results := make([]domain.ActivityAggregate, 0, limit)
	for rows.Next() {
		var (
			agg             domain.ActivityAggregate
			reason          sql.NullString
			nextRetryRaw    pgtype.Timestamptz
			quarantinedRaw  pgtype.Timestamptz
			replayAvailable bool
		)
		if err := rows.Scan(
			&agg.ID,
			&agg.TenantID,
			&agg.UserID,
			&agg.ActivityType,
			&agg.StartedAt,
			&agg.DurationMin,
			&agg.Source,
			&agg.Version,
			&agg.State,
			&agg.CreatedAt,
			&agg.UpdatedAt,
			&reason,
			&nextRetryRaw,
			&quarantinedRaw,
			&replayAvailable,
		); err != nil {
			return nil, nil, err
		}
		if reason.Valid {
			value := reason.String
			agg.FailureReason = &value
		}
		if nextRetryRaw.Valid {
			t := nextRetryRaw.Time
			agg.NextRetryAt = &t
		}
		if quarantinedRaw.Valid {
			t := quarantinedRaw.Time
			agg.QuarantinedAt = &t
		}
		agg.ReplayAvailable = replayAvailable
		results = append(results, agg)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	var nextCursor *domain.Cursor
	if len(results) == limit {
		last := results[len(results)-1]
		nextCursor = &domain.Cursor{StartedAt: last.StartedAt, ID: last.ID}
	}

	return results, nextCursor, nil
}

func nullIfEmpty(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

// EventMetadata describes how to route an outbox event.
type EventMetadata struct {
	Topic          string
	SchemaSubject  string
	PartitionKeyFn func(domain.ActivityAggregate) string
}

var eventCatalog = map[string]EventMetadata{
	"activity.created": {
		Topic:         "activity_events",
		SchemaSubject: "activity_events-value",
		PartitionKeyFn: func(a domain.ActivityAggregate) string {
			return fmt.Sprintf("%s:%s", a.TenantID, a.UserID)
		},
	},
	"activity.state_changed": {
		Topic:         "activity_state_changed",
		SchemaSubject: "activity_state_changed-value",
		PartitionKeyFn: func(a domain.ActivityAggregate) string {
			return a.ID
		},
	},
}
