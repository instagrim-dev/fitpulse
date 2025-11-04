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
	activitySyncedGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "activity_service",
		Subsystem: "persistence",
		Name:      "last_activity_synced_timestamp_seconds",
		Help:      "Unix timestamp of the most recent activity transitioned to synced.",
	})
)

func init() {
	prometheus.MustRegister(activityPersistGauge, activitySyncedGauge)
}

// RecordActivityPersisted updates the persistence watermark gauge.
func RecordActivityPersisted(ts time.Time) {
	if ts.IsZero() {
		return
	}
	activityPersistGauge.Set(float64(ts.Unix()))
}

// RecordActivitySynced updates the synced watermark gauge.
func RecordActivitySynced(ts time.Time) {
	if ts.IsZero() {
		return
	}
	activitySyncedGauge.Set(float64(ts.Unix()))
}
