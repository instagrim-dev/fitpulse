package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	activityPersistGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "activity_service",
		Subsystem: "persistence",
		Name:      "last_activity_persisted_timestamp_seconds",
		Help:      "Unix timestamp of the most recent activity persisted to Postgres.",
	})
)

func init() {
	prometheus.MustRegister(activityPersistGauge)
}

// RecordActivityPersisted updates the persistence watermark gauge.
func RecordActivityPersisted(ts time.Time) {
	if ts.IsZero() {
		return
	}
	activityPersistGauge.Set(float64(ts.Unix()))
}
