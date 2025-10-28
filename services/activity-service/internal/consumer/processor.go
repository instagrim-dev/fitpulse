// Package consumer provides Kafka consumer utilities for downstream event processing.
package consumer

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// Reader exposes the minimal kafka.Reader interface needed by the processor.
type Reader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
	Close() error
}

// Handler receives decoded messages from Kafka.
type Handler interface {
	Handle(context.Context, Message) error
}

// Message is the decoded representation of a Kafka record emitted by the outbox dispatcher.
type Message struct {
	Topic         string
	Partition     int
	Offset        int64
	Timestamp     time.Time
	EventType     string
	TenantID      string
	SchemaSubject string
	SchemaID      int
	Payload       json.RawMessage
}

// Option configures optional behaviour for the Processor.
type Option func(*Processor)

// WithLogger overrides the logger used to report errors.
func WithLogger(logger *log.Logger) Option {
	return func(p *Processor) {
		p.logger = logger
	}
}

// Processor pulls messages from Kafka, decodes them, and dispatches to a Handler.
type Processor struct {
	reader  Reader
	handler Handler
	logger  *log.Logger
}

// NewProcessor constructs a Processor with the provided reader and handler.
func NewProcessor(reader Reader, handler Handler, opts ...Option) *Processor {
	p := &Processor{
		reader:  reader,
		handler: handler,
		logger:  log.New(log.Writer(), "[consumer] ", log.LstdFlags|log.Lshortfile),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Run starts a blocking loop that processes Kafka messages until the context is cancelled.
func (p *Processor) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		msg, err := p.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			p.logger.Printf("fetch error: %v", err)
			continue
		}

		event, decodeErr := decodeMessage(msg)
		if decodeErr != nil {
			p.logger.Printf("decode error (topic=%s, partition=%d, offset=%d): %v", msg.Topic, msg.Partition, msg.Offset, decodeErr)
			recordDecodeError(msg.Topic)
			// Commit malformed messages to avoid poison-pill loops.
			if commitErr := p.reader.CommitMessages(ctx, msg); commitErr != nil {
				p.logger.Printf("commit error after decode failure: %v", commitErr)
			}
			continue
		}

		if handleErr := p.handler.Handle(ctx, event); handleErr != nil {
			p.logger.Printf("handler error (event_type=%s, tenant=%s): %v", event.EventType, event.TenantID, handleErr)
			recordHandlerError(event)
			continue
		}

		if commitErr := p.reader.CommitMessages(ctx, msg); commitErr != nil {
			p.logger.Printf("commit error: %v", commitErr)
		} else {
			recordProcessed(event)
		}
	}
}

func decodeMessage(msg kafka.Message) (Message, error) {
	if len(msg.Value) < 5 {
		return Message{}, fmt.Errorf("invalid payload length: %d", len(msg.Value))
	}

	eventType, ok := headerValue(msg, "event_type")
	if !ok {
		return Message{}, errors.New("missing event_type header")
	}
	tenantID, _ := headerValue(msg, "tenant_id")
	schemaSubject, _ := headerValue(msg, "schema_subject")

	schemaID := int(binary.BigEndian.Uint32(msg.Value[1:5]))
	payload := json.RawMessage(append([]byte(nil), msg.Value[5:]...))

	return Message{
		Topic:         msg.Topic,
		Partition:     msg.Partition,
		Offset:        msg.Offset,
		Timestamp:     msg.Time,
		EventType:     string(eventType),
		TenantID:      string(tenantID),
		SchemaSubject: string(schemaSubject),
		SchemaID:      schemaID,
		Payload:       payload,
	}, nil
}

func headerValue(msg kafka.Message, key string) ([]byte, bool) {
	for _, header := range msg.Headers {
		if header.Key == key {
			return header.Value, true
		}
	}
	return nil, false
}
