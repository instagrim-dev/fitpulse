package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	ontologyUpsertGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "exercise_ontology_service",
		Subsystem: "knowledge",
		Name:      "last_ontology_upsert_timestamp_seconds",
		Help:      "Unix timestamp of the most recent exercise upsert applied to Dgraph.",
	})

	ontologyReadGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "exercise_ontology_service",
		Subsystem: "knowledge",
		Name:      "last_ontology_read_timestamp_seconds",
		Help:      "Unix timestamp of the most recent exercise read operation.",
	})
)

func init() {
	prometheus.MustRegister(ontologyUpsertGauge, ontologyReadGauge)
}

// RecordOntologyUpsert updates the upsert watermark.
func RecordOntologyUpsert(ts time.Time) {
	if ts.IsZero() {
		return
	}
	ontologyUpsertGauge.Set(float64(ts.Unix()))
}

// RecordOntologyRead updates the read watermark.
func RecordOntologyRead(ts time.Time) {
	if ts.IsZero() {
		return
	}
	ontologyReadGauge.Set(float64(ts.Unix()))
}
