package outbox

import (
	"context"
	"sync"

	"github.com/segmentio/kafka-go"
)

// KafkaProducer lazily manages writers per topic.
type KafkaProducer struct {
	brokers []string
	mu      sync.Mutex
	writers map[string]*kafka.Writer
}

// NewKafkaProducer creates a KafkaProducer.
func NewKafkaProducer(brokers []string) *KafkaProducer {
	return &KafkaProducer{
		brokers: brokers,
		writers: make(map[string]*kafka.Writer),
	}
}

// WriteMessages writes messages to the given topic, creating a writer if necessary.
func (p *KafkaProducer) WriteMessages(ctx context.Context, topic string, msgs ...kafka.Message) error {
	writer := p.writerForTopic(topic)
	return writer.WriteMessages(ctx, msgs...)
}

func (p *KafkaProducer) writerForTopic(topic string) *kafka.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()

	if writer, ok := p.writers[topic]; ok {
		return writer
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(p.brokers...),
		Topic:        topic,
		RequiredAcks: kafka.RequireAll,
		Compression:  kafka.Snappy,
		Async:        false,
	}
	p.writers[topic] = writer
	return writer
}

// Close releases all writers.
func (p *KafkaProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	for topic, writer := range p.writers {
		if err := writer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(p.writers, topic)
	}
	return firstErr
}
