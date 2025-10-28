package outbox

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DLQWriter persists failed events for investigation.
type DLQWriter struct {
	pool *pgxpool.Pool
}

// NewDLQWriter initialises a writer backed by the provided connection pool.
func NewDLQWriter(pool *pgxpool.Pool) *DLQWriter {
	return &DLQWriter{pool: pool}
}

// Write records a failed outbox message in the DLQ alongside the supplied reason.
func (w *DLQWriter) Write(ctx context.Context, msg Message, reason string) error {
	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", msg.TenantID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO outbox_dlq (tenant_id, event_id, event_type, topic, payload, reason, aggregate_type, aggregate_id, schema_subject, partition_key, next_retry_at)
	         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, NOW())`,
		msg.TenantID, msg.EventID, msg.EventType, msg.Topic, msg.Payload, reason, msg.AggregateType, msg.AggregateID, msg.SchemaSubject, msg.PartitionKey,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
