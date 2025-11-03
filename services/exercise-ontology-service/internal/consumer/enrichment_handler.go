package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"example.com/exerciseontology/internal/domain"
	"example.com/platform/libs/go/events"
)

var nonAlphaNumeric = regexp.MustCompile(`[^a-z0-9]+`)

// EnrichmentHandler maps activity events to ontology exercises.
type EnrichmentHandler struct {
	service *domain.Service
}

// NewEnrichmentHandler constructs an enrichment handler backed by the provided service.
func NewEnrichmentHandler(service *domain.Service) Handler {
	return &EnrichmentHandler{service: service}
}

// Handle projects activity.created events into the ontology repository.
func (h *EnrichmentHandler) Handle(ctx context.Context, msg Message) error {
	if msg.Headers["event_type"] != "activity.created" {
		return nil
	}

	var evt events.ActivityCreated
	payload := msg.Payload
	// Handle Confluent Schema Registry wire format (magic byte + 4-byte schema id)
	if len(payload) >= 5 && payload[0] == 0x00 {
		payload = payload[5:]
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return err
	}

	exerciseID := ActivityExerciseID(evt.TenantID, evt.ActivityType)
	existing, err := h.service.GetExercise(ctx, exerciseID)
	if err != nil && !errors.Is(err, domain.ErrExerciseNotFound) {
		return err
	}

	meta := lookupMetadata(evt.ActivityType)
	complementaryIDs, contraindicatedIDs := deriveRelationshipIDs(evt.TenantID, evt.ActivityType, meta)

	eventTime := evt.StartedAt
	if eventTime.IsZero() {
		eventTime = msg.Timestamp
	}
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}

	exercise := domain.Exercise{
		ID:                exerciseID,
		Name:              evt.ActivityType,
		Difficulty:        meta.Difficulty,
		Targets:           copyIfNotEmpty(meta.Targets),
		Requires:          copyIfNotEmpty(meta.Requires),
		ComplementaryTo:   complementaryIDs,
		Contraindications: contraindicatedIDs,
		LastUpdated:       eventTime,
		LastSeenAt:        eventTime,
	}

	if existing != nil {
		exercise.SessionCount = existing.SessionCount + 1
		exercise.Targets = mergeSlices(exercise.Targets, existing.Targets)
		exercise.Requires = mergeSlices(exercise.Requires, existing.Requires)
		exercise.ComplementaryTo = mergeSlices(exercise.ComplementaryTo, existing.ComplementaryTo)
		exercise.Contraindications = mergeSlices(exercise.Contraindications, existing.Contraindications)
		exercise.Difficulty = coalesce(existing.Difficulty, exercise.Difficulty)
		if existing.LastUpdated.After(exercise.LastUpdated) {
			exercise.LastUpdated = existing.LastUpdated
		}
		if existing.LastSeenAt.After(exercise.LastSeenAt) {
			exercise.LastSeenAt = existing.LastSeenAt
		}
	} else {
		exercise.SessionCount = 1
	}

	sessionID := fmt.Sprintf("%s:%s", evt.TenantID, evt.ActivityID)
	session := domain.ActivitySession{
		ID:          sessionID,
		ExerciseID:  exercise.ID,
		ActivityID:  evt.ActivityID,
		TenantID:    evt.TenantID,
		UserID:      evt.UserID,
		Source:      evt.Source,
		Version:     evt.Version,
		StartedAt:   evt.StartedAt,
		DurationMin: evt.DurationMin,
		RecordedAt:  msg.Timestamp,
	}

	updated, err := h.service.UpsertExerciseWithSession(ctx, exercise, session)
	if err == nil {
		RecordProcessed(msg)
	} else {
		return err
	}

	if len(updated.ComplementaryTo) > 0 || len(updated.Contraindications) > 0 {
		idsToCheck := mergeSlices(updated.ComplementaryTo, updated.Contraindications)
		allExist := true
		for _, relID := range idsToCheck {
			if relID == updated.ID {
				continue
			}
			if _, relErr := h.service.GetExercise(ctx, relID); relErr != nil {
				if errors.Is(relErr, domain.ErrExerciseNotFound) {
					allExist = false
					break
				}
				return relErr
			}
		}
		if allExist {
			_, relErr := h.service.UpdateRelationships(ctx, updated.ID, domain.ExerciseRelationships{
				Targets:           updated.Targets,
				ComplementaryTo:   updated.ComplementaryTo,
				Contraindications: updated.Contraindications,
			})
			if relErr != nil {
				return relErr
			}
		}
	}

	return nil
}

// ActivityExerciseID returns the deterministic ontology identifier for an activity type.
func ActivityExerciseID(tenantID, activityType string) string {
	slug := strings.ToLower(activityType)
	slug = nonAlphaNumeric.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "activity"
	}
	return fmt.Sprintf("%s:%s", tenantID, slug)
}

func copyIfNotEmpty(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func mergeSlices(primary, secondary []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	result := make([]string, 0, len(primary)+len(secondary))

	for _, item := range primary {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	for _, item := range secondary {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func deriveRelationshipIDs(tenantID, activityType string, meta ActivityMetadata) ([]string, []string) {
	compNames := copyIfNotEmpty(meta.ComplementaryTo)
	if len(compNames) == 0 {
		compNames = fallbackComplementary(activityType, meta)
	}
	contraNames := copyIfNotEmpty(meta.ContraindicatedWith)
	if len(contraNames) == 0 {
		contraNames = fallbackContra(activityType, meta)
	}
	return namesToIDs(tenantID, compNames), namesToIDs(tenantID, contraNames)
}

func namesToIDs(tenantID string, names []string) []string {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		clean := strings.TrimSpace(name)
		if clean == "" {
			continue
		}
		id := ActivityExerciseID(tenantID, clean)
		set[id] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func fallbackComplementary(activityType string, meta ActivityMetadata) []string {
	lower := strings.ToLower(activityType)
	set := make(map[string]struct{})
	add := func(name string) {
		if strings.TrimSpace(name) == "" {
			return
		}
		set[name] = struct{}{}
	}

	if strings.Contains(lower, "run") || containsFold(meta.Targets, "cardio") {
		add("Recovery Ride")
	}
	if strings.Contains(lower, "ride") || containsFold(meta.Requires, "bike") {
		add("Yoga Flow")
	}
	if strings.Contains(lower, "strength") || containsFold(meta.Targets, "full-body") {
		add("Yoga Flow")
		add("Recovery Ride")
	}

	if len(set) == 0 && strings.EqualFold(meta.Difficulty, "beginner") {
		add("Strength Session")
	}

	if len(set) == 0 {
		return nil
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func fallbackContra(activityType string, meta ActivityMetadata) []string {
	// Default to no contraindications unless specified in the catalog.
	return nil
}

func containsFold(values []string, target string) bool {
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), target) {
			return true
		}
	}
	return false
}
