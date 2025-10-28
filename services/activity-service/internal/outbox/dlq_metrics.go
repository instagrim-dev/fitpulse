package outbox

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	dlqProcessedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "dlq",
		Name:      "messages_processed_total",
		Help:      "Number of DLQ entries successfully replayed.",
	}, []string{"topic", "event_type"})

	dlqRequeuedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "dlq",
		Name:      "messages_requeued_total",
		Help:      "Number of DLQ entries reinserted into the primary outbox.",
	}, []string{"topic", "event_type"})

	dlqQuarantinedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "dlq",
		Name:      "messages_quarantined_total",
		Help:      "Number of DLQ entries quarantined after exhausting retries.",
	}, []string{"topic", "event_type"})

	dlqRetryCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "dlq",
		Name:      "retry_scheduled_total",
		Help:      "Number of times a DLQ entry was scheduled for a future retry.",
	}, []string{"topic", "event_type"})

	dlqBacklogGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "activity_service",
		Subsystem: "dlq",
		Name:      "queued_messages",
		Help:      "Current number of entries remaining in the DLQ.",
	})
)

func init() {
	prometheus.MustRegister(dlqProcessedCounter, dlqRequeuedCounter, dlqQuarantinedCounter, dlqRetryCounter, dlqBacklogGauge)
}

func recordDLQProcessed(entry dlqEntry) {
	dlqProcessedCounter.WithLabelValues(entry.Topic, entry.EventType).Inc()
}

func recordDLQRequeued(entry dlqEntry) {
	dlqRequeuedCounter.WithLabelValues(entry.Topic, entry.EventType).Inc()
}

func recordDLQQuarantined(entry dlqEntry) {
	dlqQuarantinedCounter.WithLabelValues(entry.Topic, entry.EventType).Inc()
}

func recordDLQRetry(entry dlqEntry) {
	dlqRetryCounter.WithLabelValues(entry.Topic, entry.EventType).Inc()
}

func updateBacklogGauge(ctx context.Context, pool *pgxpool.Pool) {
	row := pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_dlq WHERE quarantined_at IS NULL`)
	var count int
	if err := row.Scan(&count); err != nil {
		return
	}
	dlqBacklogGauge.Set(float64(count))
}
