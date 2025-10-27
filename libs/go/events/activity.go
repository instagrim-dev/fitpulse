// Package events defines shared cross-service event payloads.
package events

import "time"

// ActivityCreated represents the message emitted when a new activity is accepted.
type ActivityCreated struct {
	ActivityID   string    `json:"activity_id"`
	TenantID     string    `json:"tenant_id"`
	UserID       string    `json:"user_id"`
	ActivityType string    `json:"activity_type"`
	StartedAt    time.Time `json:"started_at"`
	DurationMin  int       `json:"duration_min"`
	Source       string    `json:"source"`
	Version      string    `json:"version"`
}

// ActivityStateChanged tracks state transitions (pending, synced, failed) for optimistic UI flows.
type ActivityStateChanged struct {
	ActivityID string    `json:"activity_id"`
	TenantID   string    `json:"tenant_id"`
	UserID     string    `json:"user_id"`
	State      string    `json:"state"`
	OccurredAt time.Time `json:"occurred_at"`
	Reason     string    `json:"reason,omitempty"`
}
