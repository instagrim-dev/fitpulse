package domain

import "time"

// Activity represents the canonical workout record stored in PostgreSQL.
type Activity struct {
	ID           string
	TenantID     string
	UserID       string
	ActivityType string
	StartedAt    time.Time
	DurationMin  int
	Source       string
	Version      string
}
