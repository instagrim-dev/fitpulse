package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
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
        FROM activities WHERE tenant_id=$1 AND user_id=$2 AND idempotency_key=$3`

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
	const query = `SELECT activity_id, tenant_id, user_id, activity_type, started_at, duration_min, source, version, processing_state, created_at, updated_at
        FROM activities WHERE tenant_id=$1 AND activity_id=$2`

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
	var agg domain.ActivityAggregate
	if err := row.Scan(&agg.ID, &agg.TenantID, &agg.UserID, &agg.ActivityType, &agg.StartedAt, &agg.DurationMin, &agg.Source, &agg.Version, &agg.State, &agg.CreatedAt, &agg.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &agg, nil
}

// ListByUser returns activities for a user ordered by time.
func (r *Repository) ListByUser(ctx context.Context, tenantID, userID string, cursor *domain.Cursor, limit int) ([]domain.ActivityAggregate, *domain.Cursor, error) {
	args := []interface{}{tenantID, userID, limit}
	query := `SELECT activity_id, tenant_id, user_id, activity_type, started_at, duration_min, source, version, processing_state, created_at, updated_at
        FROM activities WHERE tenant_id=$1 AND user_id=$2`

	if cursor != nil {
		query += ` AND (started_at, activity_id) < ($4, $5)`
		args = append(args, cursor.StartedAt, cursor.ID)
	}

	query += ` ORDER BY started_at DESC, activity_id DESC LIMIT $3`

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
		var agg domain.ActivityAggregate
		if err := rows.Scan(&agg.ID, &agg.TenantID, &agg.UserID, &agg.ActivityType, &agg.StartedAt, &agg.DurationMin, &agg.Source, &agg.Version, &agg.State, &agg.CreatedAt, &agg.UpdatedAt); err != nil {
			return nil, nil, err
		}
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
