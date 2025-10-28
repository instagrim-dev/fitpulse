package testhelpers

import (
    "context"
    "fmt"
    "regexp"
    "strings"
    "time"

    "github.com/segmentio/kafka-go"

    "example.com/exerciseontology/internal/cache"
    "example.com/exerciseontology/internal/consumer"
    "example.com/exerciseontology/internal/domain"
    "example.com/exerciseontology/internal/knowledge"
)

// EnrichmentConsumerHandle manages the lifecycle of a running enrichment consumer.
type EnrichmentConsumerHandle struct {
	cancel context.CancelFunc
	reader *kafka.Reader
}

// StartEnrichmentConsumer spins up the ontology enrichment processor consuming activity events from Kafka.
func StartEnrichmentConsumer(ctx context.Context, brokers []string, topic string, endpoint string) (*EnrichmentConsumerHandle, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("missing brokers")
	}

	repo := knowledge.NewDgraphRepository(endpoint, 10*time.Second)
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := consumer.NewEnrichmentHandler(service)

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        fmt.Sprintf("enrichment-integration-%d", time.Now().UnixNano()),
		Topic:          topic,
		StartOffset:    kafka.FirstOffset,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
	})

	procCtx, cancel := context.WithCancel(ctx)
	go func() {
		_ = consumer.NewProcessor(reader, handler).Run(procCtx)
	}()

	return &EnrichmentConsumerHandle{
		cancel: cancel,
		reader: reader,
	}, nil
}

// Stop terminates the running enrichment consumer.
func (h *EnrichmentConsumerHandle) Stop() error {
	h.cancel()
	return h.reader.Close()
}

// ExerciseSnapshot retrieves session count and last seen timestamp for the given exercise identifier.
func ExerciseSnapshot(ctx context.Context, endpoint string, exerciseID string) (exists bool, sessionCount int, lastSeen time.Time, err error) {
	repo := knowledge.NewDgraphRepository(endpoint, 10*time.Second)
	exercise, err := repo.Get(ctx, exerciseID)
	if err != nil {
		return false, 0, time.Time{}, err
	}
	if exercise == nil {
		return false, 0, time.Time{}, nil
	}
	return true, exercise.SessionCount, exercise.LastSeenAt, nil
}

// SessionExists checks whether a given activity session has been recorded for the exercise.
func SessionExists(ctx context.Context, endpoint string, exerciseID string, activityID string, limit int) (bool, error) {
	repo := knowledge.NewDgraphRepository(endpoint, 10*time.Second)
	sessions, err := repo.ListSessions(ctx, exerciseID, limit)
	if err != nil {
		return false, err
	}
	for _, session := range sessions {
		if session.ActivityID == activityID {
			return true, nil
		}
	}
	return false, nil
}

var nonAlphaNumeric = regexp.MustCompile(`[^a-z0-9]+`)

// ExerciseID mirrors the enrichment pipeline identifier derivation.
func ExerciseID(tenantID, activityType string) string {
    slug := strings.ToLower(activityType)
    slug = nonAlphaNumeric.ReplaceAllString(slug, "-")
    slug = strings.Trim(slug, "-")
    if slug == "" {
        slug = "activity"
    }
    return fmt.Sprintf("%s:%s", tenantID, slug)
}
