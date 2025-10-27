package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DLQManager handles retrying failed outbox messages and quarantining exhausted entries.
type DLQManager struct {
    pool       *pgxpool.Pool
    maxRetries int
    baseDelay  time.Duration
}

// NewDLQManager constructs a DLQManager with the provided pool and retry configuration.
func NewDLQManager(pool *pgxpool.Pool, maxRetries int, baseDelay time.Duration) *DLQManager {
	if maxRetries <= 0 {
		maxRetries = 5
	}
	if baseDelay <= 0 {
		baseDelay = time.Minute
	}
	return &DLQManager{pool: pool, maxRetries: maxRetries, baseDelay: baseDelay}
}

// RunOnce processes a batch of DLQ entries and returns the count of successfully
// re-queued messages.
func (m *DLQManager) RunOnce(ctx context.Context, batchSize int) (int, error) {
	const query = `SELECT dlq_id, tenant_id, event_id, event_type, topic, payload, reason, aggregate_type, aggregate_id, schema_subject, partition_key, retry_count
                    FROM outbox_dlq
                   WHERE quarantined_at IS NULL AND (next_retry_at IS NULL OR next_retry_at <= NOW())
                   ORDER BY created_at
                   LIMIT $1`

	rows, err := m.pool.Query(ctx, query, batchSize)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	processed := 0
	for rows.Next() {
		entry, scanErr := scanDLQEntry(rows)
		if scanErr != nil {
			err = errors.Join(err, scanErr)
			continue
		}
		if procErr := m.handleEntry(ctx, entry); procErr != nil {
			err = errors.Join(err, procErr)
		} else {
			processed++
		}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		err = errors.Join(err, rowsErr)
	}
	return processed, err
}

// handleEntry applies retry/quarantine logic for a single DLQ entry.
func (m *DLQManager) handleEntry(ctx context.Context, entry dlqEntry) error {
	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", entry.TenantID); err != nil {
		return err
	}

	if entry.RetryCount >= m.maxRetries {
		if _, err := tx.Exec(ctx, `UPDATE outbox_dlq SET quarantined_at = NOW(), quarantine_reason = $1 WHERE dlq_id = $2`, "retry limit reached", entry.ID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	insertErr := requeueOutbox(ctx, tx, entry)
	if insertErr != nil {
        delay := m.backoffDelay(entry.RetryCount + 1)
        if _, err := tx.Exec(ctx,
            `UPDATE outbox_dlq
               SET retry_count = retry_count + 1,
                   last_attempt_at = NOW(),
                   next_retry_at = NOW() + $1::interval,
                   reason = $2
             WHERE dlq_id = $3`,
            delay, insertErr.Error(), entry.ID,
        ); err != nil {
            return err
        }
		return tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM outbox_dlq WHERE dlq_id = $1`, entry.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// backoffDelay calculates exponential backoff capped at one hour.
func (m *DLQManager) backoffDelay(attempt int) time.Duration {
    delay := time.Duration(1<<uint(attempt-1)) * m.baseDelay
    if delay > time.Hour {
        delay = time.Hour
    }
    return delay
}

// requeueOutbox reinserts the payload into the primary outbox table for replay.
func requeueOutbox(ctx context.Context, tx pgx.Tx, entry dlqEntry) error {
	if entry.SchemaSubject == "" {
		return fmt.Errorf("missing schema_subject for dlq entry %d", entry.ID)
	}

	const stmt = `INSERT INTO outbox (tenant_id, aggregate_type, aggregate_id, event_type, topic, schema_subject, partition_key, payload)
                   VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`

	_, err := tx.Exec(ctx, stmt,
		entry.TenantID,
		entry.AggregateType,
		entry.AggregateID,
		entry.EventType,
		entry.Topic,
		entry.SchemaSubject,
		entry.PartitionKey,
		entry.Payload,
	)
	return err
}

// dlqEntry represents an outbox_dlq row selected for processing.
type dlqEntry struct {
	ID            int64
	TenantID      string
	EventID       int64
	EventType     string
	Topic         string
	Payload       []byte
	Reason        string
	AggregateType string
	AggregateID   string
	SchemaSubject string
	PartitionKey  string
	RetryCount    int
}

func scanDLQEntry(rows pgx.Rows) (dlqEntry, error) {
	var entry dlqEntry
	if err := rows.Scan(&entry.ID, &entry.TenantID, &entry.EventID, &entry.EventType, &entry.Topic, &entry.Payload, &entry.Reason, &entry.AggregateType, &entry.AggregateID, &entry.SchemaSubject, &entry.PartitionKey, &entry.RetryCount); err != nil {
		return dlqEntry{}, err
	}
	return entry, nil
}
