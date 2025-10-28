package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"example.com/exerciseontology/internal/domain"
)

// DgraphRepository persists exercises via Dgraph's HTTP API.
type DgraphRepository struct {
	endpoint   string
	httpClient *http.Client
}

// NewDgraphRepository constructs the repository.
func NewDgraphRepository(endpoint string, timeout time.Duration) *DgraphRepository {
	return &DgraphRepository{
		endpoint: strings.TrimRight(endpoint, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Upsert creates or updates an exercise node by exercise_id.
func (r *DgraphRepository) Upsert(ctx context.Context, exercise domain.Exercise) error {
	payload := map[string]interface{}{
		"query": fmt.Sprintf(`query { exercise as var(func: eq(exercise_id, "%s")) }`, exercise.ID),
		"set":   []map[string]interface{}{buildExerciseMutation(exercise)},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint+"/mutate?commitNow=true", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("dgraph upsert failed: %s", resp.Status)
	}
	return nil
}

func buildExerciseMutation(exercise domain.Exercise) map[string]interface{} {
	node := map[string]interface{}{
		"uid":                  "uid(exercise)",
		"dgraph.type":          []string{"Exercise"},
		"exercise_id":          exercise.ID,
		"name":                 exercise.Name,
		"difficulty":           exercise.Difficulty,
		"targets":              exercise.Targets,
		"requires":             exercise.Requires,
		"contraindicated_with": exercise.Contraindications,
		"complementary_to":     exercise.ComplementaryTo,
		"session_count":        exercise.SessionCount,
	}
	if !exercise.LastUpdated.IsZero() {
		node["last_updated"] = exercise.LastUpdated.Format(time.RFC3339Nano)
	}
	if !exercise.LastSeenAt.IsZero() {
		node["last_seen_at"] = exercise.LastSeenAt.Format(time.RFC3339Nano)
	}
	return node
}

func buildSessionMutation(session domain.ActivitySession) map[string]interface{} {
	node := map[string]interface{}{
		"uid":          "uid(session)",
		"dgraph.type":  []string{"ActivitySession"},
		"exercise_id":  session.ExerciseID,
		"session_id":   session.ID,
		"activity_id":  session.ActivityID,
		"tenant_id":    session.TenantID,
		"user_id":      session.UserID,
		"source":       session.Source,
		"version":      session.Version,
		"duration_min": session.DurationMin,
		"exercise":     "uid(exercise)",
	}
	if !session.StartedAt.IsZero() {
		node["started_at"] = session.StartedAt.Format(time.RFC3339Nano)
	}
	if !session.RecordedAt.IsZero() {
		node["recorded_at"] = session.RecordedAt.Format(time.RFC3339Nano)
	}
	return node
}

// UpsertWithSession creates or updates the exercise and records an activity session edge.
func (r *DgraphRepository) UpsertWithSession(ctx context.Context, exercise domain.Exercise, session domain.ActivitySession) error {
	query := fmt.Sprintf(`query {
	  exercise as var(func: eq(exercise_id, "%s"))
	  session as var(func: eq(session_id, "%s"))
	}`, exercise.ID, session.ID)

	set := []map[string]interface{}{buildExerciseMutation(exercise), buildSessionMutation(session)}

	payload := map[string]interface{}{
		"query": query,
		"set":   set,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint+"/mutate?commitNow=true", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("dgraph upsert with session failed: %s", resp.Status)
	}
	return nil
}

// Get retrieves an exercise by ID.
func (r *DgraphRepository) Get(ctx context.Context, id string) (*domain.Exercise, error) {
	query := `query exercise($id: string) {
  exercises(func: eq(exercise_id, $id)) {
    exercise_id
    name
    difficulty
    targets
    requires
    contraindicated_with
    complementary_to
    last_updated
    last_seen_at
    session_count
  }
}`
	variables := map[string]string{"$id": id}

	result, err := r.executeQuery(ctx, query, variables)
	if err != nil {
		return nil, err
	}
	if len(result.Exercises) == 0 {
		return nil, nil
	}
	best := result.Exercises[0].toDomain()
	for _, node := range result.Exercises[1:] {
		candidate := node.toDomain()
		if candidate.LastUpdated.After(best.LastUpdated) {
			best = candidate
			continue
		}
		if candidate.LastUpdated.Equal(best.LastUpdated) && candidate.SessionCount > best.SessionCount {
			best = candidate
		}
	}
	return &best, nil
}

// Search performs a term-matching query over exercise names.
func (r *DgraphRepository) Search(ctx context.Context, queryTerm string, limit int) ([]domain.Exercise, error) {
	query := fmt.Sprintf(`query exercise($term: string) {
  exercises(func: type(Exercise), first: %d) @filter(anyofterms(name, $term)) {
    exercise_id
    name
    difficulty
    targets
    requires
    contraindicated_with
    complementary_to
    last_updated
    last_seen_at
    session_count
  }
}`, limit)
	variables := map[string]string{"$term": queryTerm}

	result, err := r.executeQuery(ctx, query, variables)
	if err != nil {
		return nil, err
	}

	exercises := make([]domain.Exercise, 0, len(result.Exercises))
	for _, item := range result.Exercises {
		exercises = append(exercises, item.toDomain())
	}
	return exercises, nil
}

// ListSessions returns sessions linked to the exercise ordered by recorded time.
func (r *DgraphRepository) ListSessions(ctx context.Context, exerciseID string, limit int) ([]domain.ActivitySession, error) {
	if limit <= 0 {
		limit = 10
	}
	query := fmt.Sprintf(`query sessions($id: string) {
	  exercises(func: eq(exercise_id, $id)) {
	    sessions: ~exercise(orderdesc: recorded_at, first: %d) {
	      session_id
	      activity_id
	      tenant_id
	      user_id
	      source
	      version
	      started_at
	      duration_min
	      recorded_at
	    }
	  }
	}`, limit)

	body := map[string]interface{}{
		"query": query,
		"variables": map[string]string{
			"$id": exerciseID,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint+"/query", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dgraph query failed: %s", resp.Status)
	}

	var wrapper struct {
		Data struct {
			Exercises []struct {
				Sessions []sessionNode `json:"sessions"`
			} `json:"exercises"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.Data.Exercises) == 0 {
		return nil, nil
	}
	sessions := make([]domain.ActivitySession, 0, len(wrapper.Data.Exercises[0].Sessions))
	for _, node := range wrapper.Data.Exercises[0].Sessions {
		sessions = append(sessions, node.toDomain(exerciseID))
	}
	return sessions, nil
}

// Delete removes the exercise and any associated sessions.
func (r *DgraphRepository) Delete(ctx context.Context, id string) error {
	query := fmt.Sprintf(`query {
	  exercise as var(func: eq(exercise_id, "%s"))
	  sessions as var(func: eq(exercise_id, "%s")) @filter(type(ActivitySession))
	}`, id, id)

	payload := map[string]interface{}{
		"query": query,
		"delete": []map[string]interface{}{
			{"uid": "uid(sessions)"},
			{"uid": "uid(exercise)"},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint+"/mutate?commitNow=true", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("dgraph delete failed: %s", resp.Status)
	}
	return nil
}

type queryResponse struct {
	Exercises []exerciseNode `json:"exercises"`
}

type exerciseNode struct {
	ExerciseID         string   `json:"exercise_id"`
	Name               string   `json:"name"`
	Difficulty         string   `json:"difficulty"`
	Targets            []string `json:"targets"`
	Requires           []string `json:"requires"`
	Contraindications  []string `json:"contraindicated_with"`
	ComplementaryTo    []string `json:"complementary_to"`
	LastUpdatedISO8601 string   `json:"last_updated"`
	LastSeenISO8601    string   `json:"last_seen_at"`
	SessionCount       int      `json:"session_count"`
}

func (node exerciseNode) toDomain() domain.Exercise {
	var lastUpdated time.Time
	if node.LastUpdatedISO8601 != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, node.LastUpdatedISO8601); err == nil {
			lastUpdated = parsed
		}
	}
	var lastSeen time.Time
	if node.LastSeenISO8601 != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, node.LastSeenISO8601); err == nil {
			lastSeen = parsed
		}
	}
	return domain.Exercise{
		ID:                node.ExerciseID,
		Name:              node.Name,
		Difficulty:        node.Difficulty,
		Targets:           node.Targets,
		Requires:          node.Requires,
		Contraindications: node.Contraindications,
		ComplementaryTo:   node.ComplementaryTo,
		LastUpdated:       lastUpdated,
		SessionCount:      node.SessionCount,
		LastSeenAt:        lastSeen,
	}
}

type sessionNode struct {
	SessionID    string `json:"session_id"`
	ExerciseID   string `json:"exercise_id"`
	ActivityID   string `json:"activity_id"`
	TenantID     string `json:"tenant_id"`
	UserID       string `json:"user_id"`
	Source       string `json:"source"`
	Version      string `json:"version"`
	StartedAtISO string `json:"started_at"`
	DurationMin  int    `json:"duration_min"`
	RecordedISO  string `json:"recorded_at"`
}

func (node sessionNode) toDomain(exerciseID string) domain.ActivitySession {
	var started, recorded time.Time
	if node.StartedAtISO != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, node.StartedAtISO); err == nil {
			started = parsed
		}
	}
	if node.RecordedISO != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, node.RecordedISO); err == nil {
			recorded = parsed
		}
	}
	return domain.ActivitySession{
		ID:          node.SessionID,
		ExerciseID:  chooseFirst(node.ExerciseID, exerciseID),
		ActivityID:  node.ActivityID,
		TenantID:    node.TenantID,
		UserID:      node.UserID,
		Source:      node.Source,
		Version:     node.Version,
		StartedAt:   started,
		DurationMin: node.DurationMin,
		RecordedAt:  recorded,
	}
}

func (r *DgraphRepository) executeQuery(ctx context.Context, query string, variables map[string]string) (queryResponse, error) {
	body := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return queryResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint+"/query", bytes.NewReader(payload))
	if err != nil {
		return queryResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return queryResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return queryResponse{}, fmt.Errorf("dgraph query failed: %s", resp.Status)
	}

	var wrapper struct {
		Data queryResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return queryResponse{}, err
	}
	return wrapper.Data, nil
}

func chooseFirst(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
