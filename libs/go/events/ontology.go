package events

import "time"

// ExerciseUpserted emitted when an exercise is created or updated.
type ExerciseUpserted struct {
	ExerciseID        string    `json:"exercise_id"`
	Name              string    `json:"name"`
	Difficulty        string    `json:"difficulty"`
	Targets           []string  `json:"targets"`
	Requires          []string  `json:"requires"`
	Contraindications []string  `json:"contraindications"`
	ComplementaryTo   []string  `json:"complementary_to"`
	UpdatedAt         time.Time `json:"updated_at"`
	TenantID          string    `json:"tenant_id,omitempty"`
}

// ExerciseDeleted emitted when an exercise is removed.
type ExerciseDeleted struct {
	ExerciseID string    `json:"exercise_id"`
	DeletedAt  time.Time `json:"deleted_at"`
}
