package outbox

import "github.com/prometheus/client_golang/prometheus"

var (
	deliveredCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "outbox",
		Name:      "events_delivered_total",
		Help:      "Number of outbox events successfully published to Kafka.",
	})

	failedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "outbox",
		Name:      "events_failed_total",
		Help:      "Number of outbox events that failed to publish and routed to DLQ.",
	})

	batchDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "activity_service",
		Subsystem: "outbox",
		Name:      "batch_duration_seconds",
		Help:      "Time spent fetching, delivering, and marking outbox batches.",
		Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10),
	})

	dlqCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "outbox",
		Name:      "events_dlq_total",
		Help:      "Number of outbox events routed to the dead-letter queue, labeled by topic.",
	}, []string{"topic"})

	// Tracks number of activities marked synced by dispatcher after publish
	markedSyncedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "outbox",
		Name:      "activities_marked_synced_total",
		Help:      "Count of activities transitioned to synced after outbox publish.",
	})
)

func init() {
	prometheus.MustRegister(deliveredCounter, failedCounter, batchDuration, dlqCounter, markedSyncedCounter)
}
