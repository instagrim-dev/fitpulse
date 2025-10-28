package consumer

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

func TestProcessorCommitsOnSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payload := []byte(`{"activity_id":"abc"}`)
	value := make([]byte, 5+len(payload))
	value[0] = 0
	binary.BigEndian.PutUint32(value[1:5], uint32(42))
	copy(value[5:], payload)

	msg := kafka.Message{
		Topic:     "activity_events",
		Partition: 0,
		Offset:    10,
		Time:      time.Now().UTC(),
		Value:     value,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("activity.created")},
			{Key: "tenant_id", Value: []byte("tenant-1")},
			{Key: "schema_subject", Value: []byte("activity_events-value")},
		},
	}

	reader := &stubReader{
		messages: []kafka.Message{msg},
		after:    contextCanceled,
	}
	handler := &stubHandler{}

	processor := NewProcessor(reader, handler, WithLogger(log.New(testWriter{t}, "", 0)))

	err := processor.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)

	require.Equal(t, 1, handler.calls)
	require.Equal(t, 1, reader.commitCalls)
	require.Equal(t, "activity.created", handler.last.EventType)
	require.Equal(t, "tenant-1", handler.last.TenantID)
	require.Equal(t, 42, handler.last.SchemaID)
	require.JSONEq(t, string(payload), string(handler.last.Payload))
}

func TestProcessorSkipsCommitOnHandlerError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payload := []byte(`{"activity_id":"def"}`)
	value := make([]byte, 5+len(payload))
	value[0] = 0
	binary.BigEndian.PutUint32(value[1:5], uint32(99))
	copy(value[5:], payload)

	msg := kafka.Message{
		Topic:     "activity_events",
		Partition: 0,
		Offset:    20,
		Time:      time.Now().UTC(),
		Value:     value,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("activity.state_changed")},
			{Key: "tenant_id", Value: []byte("tenant-2")},
			{Key: "schema_subject", Value: []byte("activity_state_changed-value")},
		},
	}

	reader := &stubReader{
		messages: []kafka.Message{msg},
		after:    contextCanceled,
	}
	handler := &stubHandler{err: errors.New("boom")}

	processor := NewProcessor(reader, handler, WithLogger(log.New(testWriter{t}, "", 0)))

	err := processor.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)

	require.Equal(t, 1, handler.calls)
	require.Equal(t, 0, reader.commitCalls)
}

type stubReader struct {
	messages    []kafka.Message
	index       int
	commitCalls int
	after       func() error
}

func (r *stubReader) FetchMessage(context.Context) (kafka.Message, error) {
	if r.index >= len(r.messages) {
		if r.after != nil {
			return kafka.Message{}, r.after()
		}
		return kafka.Message{}, context.Canceled
	}
	msg := r.messages[r.index]
	r.index++
	return msg, nil
}

func (r *stubReader) CommitMessages(_ context.Context, _ ...kafka.Message) error {
	r.commitCalls++
	return nil
}

func (r *stubReader) Close() error { return nil }

func contextCanceled() error { return context.Canceled }

type stubHandler struct {
	calls int
	err   error
	last  Message
}

func (h *stubHandler) Handle(_ context.Context, msg Message) error {
	h.calls++
	h.last = msg
	return h.err
}

type testWriter struct {
	t *testing.T
}

func (tw testWriter) Write(p []byte) (int, error) {
	tw.t.Log(string(p))
	return len(p), nil
}
