package consumer

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PersistenceHandler writes consumed events into Postgres for downstream auditing.
type PersistenceHandler struct {
	pool *pgxpool.Pool
}

// NewPersistenceHandler constructs a handler backed by the provided pool.
func NewPersistenceHandler(pool *pgxpool.Pool) *PersistenceHandler {
	return &PersistenceHandler{pool: pool}
}

// Handle stores the event payload in the activity_event_log table.
func (h *PersistenceHandler) Handle(ctx context.Context, msg Message) error {
	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(ctx,
		`INSERT INTO activity_event_log (event_type, tenant_id, schema_id, schema_subject, topic, partition, record_offset, payload, received_at)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		msg.EventType,
		msg.TenantID,
		msg.SchemaID,
		msg.SchemaSubject,
		msg.Topic,
		msg.Partition,
		msg.Offset,
		msg.Payload,
		msg.Timestamp,
	)
	return err
}
