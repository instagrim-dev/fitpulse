// Package consumer streams Kafka activity events into the ontology service.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// Reader describes the kafka.Reader functions the processor interacts with.
type Reader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
	Close() error
}

// Handler processes decoded Kafka messages.
type Handler interface {
	Handle(context.Context, Message) error
}

// Message represents a decoded Kafka record.
type Message struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Payload   json.RawMessage
	Timestamp time.Time
	Headers   map[string]string
}

// Option configures processor behaviour.
type Option func(*Processor)

// WithLogger sets a custom logger.
func WithLogger(l *log.Logger) Option {
	return func(p *Processor) { p.logger = l }
}

// Processor coordinates the consumer loop.
type Processor struct {
	reader  Reader
	handler Handler
	logger  *log.Logger
}

// NewProcessor constructs a processor from a reader/handler pair.
func NewProcessor(reader Reader, handler Handler, opts ...Option) *Processor {
	p := &Processor{reader: reader, handler: handler, logger: log.Default()}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Run consumes messages until ctx cancellation.
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

		decoded := Message{
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
			Key:       msg.Key,
			Payload:   append(json.RawMessage{}, msg.Value...),
			Timestamp: msg.Time,
			Headers:   make(map[string]string, len(msg.Headers)),
		}
		for _, header := range msg.Headers {
			decoded.Headers[header.Key] = string(header.Value)
		}

		if err := p.handler.Handle(ctx, decoded); err != nil {
			p.logger.Printf("handler error (topic=%s offset=%d): %v", msg.Topic, msg.Offset, err)
		} else {
			p.logger.Printf("processed (topic=%s offset=%d)", msg.Topic, msg.Offset)
		}

		if err := p.reader.CommitMessages(ctx, msg); err != nil {
			p.logger.Printf("commit error: %v", err)
		}
	}
}
