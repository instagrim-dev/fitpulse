package consumer

import "strings"

// ActivityMetadata captures enrichment hints for common activity types.
type ActivityMetadata struct {
	Difficulty          string
	Targets             []string
	Requires            []string
	ComplementaryTo     []string
	ContraindicatedWith []string
}

var defaultMetadata = ActivityMetadata{
	Difficulty: "unspecified",
	Requires:   []string{"activity-log"},
}

var activityCatalog = map[string]ActivityMetadata{
	"tempo ride": {
		Difficulty:      "intermediate",
		Targets:         []string{"cardio", "legs"},
		Requires:        []string{"bike"},
		ComplementaryTo: []string{"Recovery Ride", "Strength Session"},
	},
	"recovery ride": {
		Difficulty:      "beginner",
		Targets:         []string{"cardio"},
		Requires:        []string{"bike"},
		ComplementaryTo: []string{"Tempo Ride", "Yoga Flow"},
	},
	"long run": {
		Difficulty:          "advanced",
		Targets:             []string{"cardio", "legs"},
		Requires:            []string{"running-shoes"},
		ComplementaryTo:     []string{"Recovery Ride", "Yoga Flow"},
		ContraindicatedWith: []string{"Strength Session"},
	},
	"easy run": {
		Difficulty:      "beginner",
		Targets:         []string{"cardio"},
		Requires:        []string{"running-shoes"},
		ComplementaryTo: []string{"Strength Session", "Yoga Flow"},
	},
	"strength session": {
		Difficulty:          "intermediate",
		Targets:             []string{"full-body"},
		Requires:            []string{"weights"},
		ComplementaryTo:     []string{"Recovery Ride", "Yoga Flow"},
		ContraindicatedWith: []string{"Long Run"},
	},
	"yoga flow": {
		Difficulty:      "beginner",
		Targets:         []string{"flexibility", "balance"},
		Requires:        []string{"mat"},
		ComplementaryTo: []string{"Strength Session", "Easy Run"},
	},
}

// lookupMetadata returns enrichment metadata for an activity type.
func lookupMetadata(activityType string) ActivityMetadata {
	key := strings.ToLower(strings.TrimSpace(activityType))
	if meta, ok := activityCatalog[key]; ok {
		return meta
	}
	return defaultMetadata
}
