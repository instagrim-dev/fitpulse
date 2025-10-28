package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

func TestProcessorCommitsMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payload := json.RawMessage(`{"example":true}`)
	msg := kafka.Message{
		Topic:     "activity_events",
		Partition: 0,
		Offset:    12,
		Value:     payload,
		Time:      time.Now().UTC(),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("activity.created")},
		},
	}

	reader := &stubReader{msgs: []kafka.Message{msg}, errAfter: context.Canceled}
	handler := &RecordingHandler{}
	proc := NewProcessor(reader, handler)

	err := proc.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, handler.count)
	require.Equal(t, 1, reader.commitCount)
}

type stubReader struct {
	msgs        []kafka.Message
	idx         int
	commitCount int
	errAfter    error
}

func (r *stubReader) FetchMessage(context.Context) (kafka.Message, error) {
	if r.idx >= len(r.msgs) {
		return kafka.Message{}, r.errAfter
	}
	msg := r.msgs[r.idx]
	r.idx++
	return msg, nil
}

func (r *stubReader) CommitMessages(_ context.Context, _ ...kafka.Message) error {
	r.commitCount++
	return nil
}

func (r *stubReader) Close() error { return nil }

type RecordingHandler struct {
	count int
	last  Message
}

var _ Handler = (*RecordingHandler)(nil)

func (h *RecordingHandler) Handle(_ context.Context, msg Message) error {
	h.count++
	h.last = msg
	return nil
}
