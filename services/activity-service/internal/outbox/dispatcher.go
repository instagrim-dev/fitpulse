// Package outbox persists and delivers domain events to Kafka.
package outbox

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

type messageWriter interface {
	WriteMessages(context.Context, string, ...kafka.Message) error
}

type schemaRegistrar interface {
	EnsureSchema(context.Context, string, string) (int, error)
}

// Dispatcher drains the outbox table and delivers events to Kafka using Schema Registry metadata.
type Dispatcher struct {
	pool             *pgxpool.Pool
	producer         messageWriter
	registry         schemaRegistrar
	dlq              *DLQWriter
	pollInterval     time.Duration
	batchSize        int
	schemaIDCache    sync.Map
	shutdownComplete chan struct{}
}

// NewDispatcher constructs a Dispatcher.
func NewDispatcher(pool *pgxpool.Pool, producer messageWriter, registry schemaRegistrar, pollInterval time.Duration, batchSize int) *Dispatcher {
	return &Dispatcher{
		pool:             pool,
		producer:         producer,
		registry:         registry,
		dlq:              NewDLQWriter(pool),
		pollInterval:     pollInterval,
		batchSize:        batchSize,
		shutdownComplete: make(chan struct{}),
	}
}

// Start launches the polling loop. It should be called in a goroutine.
func (d *Dispatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(d.pollInterval)
	defer func() {
		ticker.Stop()
		close(d.shutdownComplete)
	}()

	for {
		if err := d.processBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("outbox dispatcher error: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Wait waits until dispatcher stops.
func (d *Dispatcher) Wait() {
	<-d.shutdownComplete
}

func (d *Dispatcher) processBatch(ctx context.Context) error {
	start := time.Now()

	messages, err := d.fetchAndClaim(ctx)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}
	defer batchDuration.Observe(time.Since(start).Seconds())

	if err := d.deliver(ctx, messages); err != nil {
		log.Printf("outbox: delivery failure: %v", err)
		failedCounter.Add(float64(len(messages)))
		if dlqErr := d.moveToDLQ(ctx, messages, err.Error()); dlqErr != nil {
			return dlqErr
		}
		return d.markPublished(ctx, messages)
	}

	deliveredCounter.Add(float64(len(messages)))
	return d.markPublished(ctx, messages)
}

func (d *Dispatcher) fetchAndClaim(ctx context.Context) ([]Message, error) {
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	query := `SELECT event_id, tenant_id, aggregate_type, aggregate_id, event_type, topic, schema_subject, partition_key, payload
        FROM outbox
        WHERE published_at IS NULL
        ORDER BY event_id
        LIMIT $1
        FOR UPDATE SKIP LOCKED`

	rows, err := tx.Query(ctx, query, d.batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]Message, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.EventID, &msg.TenantID, &msg.AggregateType, &msg.AggregateID, &msg.EventType, &msg.Topic, &msg.SchemaSubject, &msg.PartitionKey, &msg.Payload); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
		ids = append(ids, msg.EventID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		tx.Rollback(ctx)
		return nil, nil
	}

	if _, err := tx.Exec(ctx, `UPDATE outbox SET claimed_at = NOW() WHERE event_id = ANY($1)`, ids); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return messages, nil
}

func (d *Dispatcher) deliver(ctx context.Context, messages []Message) error {
	type topicBatch struct {
		schemaID int
		messages []kafka.Message
	}

	batches := make(map[string]*topicBatch)

	for _, msg := range messages {
		meta, ok := schemaCatalog[msg.EventType]
		if !ok {
			return fmt.Errorf("no schema metadata for event_type=%s", msg.EventType)
		}

		cacheKey := fmt.Sprintf("%s::%s", msg.SchemaSubject, meta.Schema)
		schemaIDVal, found := d.schemaIDCache.Load(cacheKey)
		var schemaID int
		if found {
			schemaID = schemaIDVal.(int)
		} else {
			id, err := d.registry.EnsureSchema(ctx, msg.SchemaSubject, meta.Schema)
			if err != nil {
				return err
			}
			d.schemaIDCache.Store(cacheKey, id)
			schemaID = id
		}

		payload := []byte(msg.Payload)
		encoded := encodeWireFormat(schemaID, payload)
		record := kafka.Message{
			Key:   []byte(msg.PartitionKey),
			Value: encoded,
			Time:  time.Now().UTC(),
		}

		batch, exists := batches[msg.Topic]
		if !exists {
			batches[msg.Topic] = &topicBatch{schemaID: schemaID, messages: []kafka.Message{record}}
		} else {
			batch.messages = append(batch.messages, record)
		}
	}

	for topic, batch := range batches {
		if err := d.producer.WriteMessages(ctx, topic, batch.messages...); err != nil {
			return err
		}
	}

	return nil
}

func (d *Dispatcher) markPublished(ctx context.Context, messages []Message) error {
	groups := make(map[string][]int64)
	for _, msg := range messages {
		groups[msg.TenantID] = append(groups[msg.TenantID], msg.EventID)
	}

	for tenantID, ids := range groups {
		conn, err := d.pool.Acquire(ctx)
		if err != nil {
			return err
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			conn.Release()
			return err
		}

		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
			tx.Rollback(ctx)
			conn.Release()
			return err
		}

		if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at = NOW() WHERE event_id = ANY($1)`, ids); err != nil {
			tx.Rollback(ctx)
			conn.Release()
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			conn.Release()
			return err
		}
		conn.Release()
	}

	return nil
}

func (d *Dispatcher) moveToDLQ(ctx context.Context, messages []Message, reason string) error {
	for _, msg := range messages {
		entryReason := fmt.Sprintf("%s (topic=%s)", reason, msg.Topic)
		if err := d.dlq.Write(ctx, msg, entryReason); err != nil {
			return err
		}
		dlqCounter.WithLabelValues(msg.Topic).Inc()
	}
	return nil
}

// Message represents a row fetched from outbox.
type Message struct {
	EventID       int64
	TenantID      string
	AggregateType string
	AggregateID   string
	EventType     string
	Topic         string
	SchemaSubject string
	PartitionKey  string
	Payload       json.RawMessage
}

// encodeWireFormat applies Confluent framing for Schema Registry aware payloads.
func encodeWireFormat(schemaID int, payload []byte) []byte {
	frame := make([]byte, 5+len(payload))
	frame[0] = 0
	binary.BigEndian.PutUint32(frame[1:5], uint32(schemaID))
	copy(frame[5:], payload)
	return frame
}

// SchemaCatalogEntry maps event type to schema definition.
type SchemaCatalogEntry struct {
	Schema string
}

var schemaCatalog = map[string]SchemaCatalogEntry{
	"activity.created": {
		Schema: activityCreatedSchema,
	},
	"activity.state_changed": {
		Schema: activityStateChangedSchema,
	},
}
