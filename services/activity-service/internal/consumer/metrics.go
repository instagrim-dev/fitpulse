package consumer

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	processedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "consumer",
		Name:      "messages_processed_total",
		Help:      "Number of Kafka messages successfully handled.",
	}, []string{"topic", "event_type"})

	handlerErrorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "consumer",
		Name:      "handler_errors_total",
		Help:      "Number of handler errors grouped by topic and event type.",
	}, []string{"topic", "event_type"})

	decodeErrorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "activity_service",
		Subsystem: "consumer",
		Name:      "decode_errors_total",
		Help:      "Number of decode failures per topic.",
	}, []string{"topic"})

	lastMessageGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "activity_service",
		Subsystem: "consumer",
		Name:      "last_message_timestamp_seconds",
		Help:      "Unix timestamp of the most recent successfully processed message per topic.",
	}, []string{"topic"})
)

func init() {
	prometheus.MustRegister(processedCounter, handlerErrorCounter, decodeErrorCounter, lastMessageGauge)
}

func recordProcessed(msg Message) {
	processedCounter.WithLabelValues(msg.Topic, msg.EventType).Inc()
	if !msg.Timestamp.IsZero() {
		lastMessageGauge.WithLabelValues(msg.Topic).Set(float64(msg.Timestamp.Unix()))
	}
}

func recordHandlerError(msg Message) {
	handlerErrorCounter.WithLabelValues(msg.Topic, msg.EventType).Inc()
}

func recordDecodeError(topic string) {
	decodeErrorCounter.WithLabelValues(topic).Inc()
}

// RecordLag allows external callers (e.g. tests) to set the last timestamp gauge directly.
func RecordLag(topic string, ts time.Time) {
	if ts.IsZero() {
		return
	}
	lastMessageGauge.WithLabelValues(topic).Set(float64(ts.Unix()))
}
