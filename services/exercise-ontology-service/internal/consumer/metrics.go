package consumer

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	processedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "exercise_ontology",
		Subsystem: "consumer",
		Name:      "messages_processed_total",
		Help:      "Number of Kafka messages processed by the ontology consumer.",
	}, []string{"topic", "event_type"})

	lastMessageGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "exercise_ontology",
		Subsystem: "consumer",
		Name:      "last_message_timestamp_seconds",
		Help:      "Timestamp of the most recent Kafka message processed.",
	}, []string{"topic"})
)

func init() {
	prometheus.MustRegister(processedCounter, lastMessageGauge)
}

// RecordProcessed updates counters for successfully handled messages.
func RecordProcessed(msg Message) {
	eventType := msg.Headers["event_type"]
	processedCounter.WithLabelValues(msg.Topic, eventType).Inc()
	if !msg.Timestamp.IsZero() {
		lastMessageGauge.WithLabelValues(msg.Topic).Set(float64(msg.Timestamp.Unix()))
	}
}
