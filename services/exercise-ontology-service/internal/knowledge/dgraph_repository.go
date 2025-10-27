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
		"set": []map[string]interface{}{
			{
				"uid":                  "uid(exercise)",
				"dgraph.type":          "Exercise",
				"exercise_id":          exercise.ID,
				"name":                 exercise.Name,
				"difficulty":           exercise.Difficulty,
				"targets":              exercise.Targets,
				"requires":             exercise.Requires,
				"contraindicated_with": exercise.Contraindications,
				"complementary_to":     exercise.ComplementaryTo,
				"last_updated":         exercise.LastUpdated.Format(time.RFC3339Nano),
			},
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
		return fmt.Errorf("dgraph upsert failed: %s", resp.Status)
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
	ex := result.Exercises[0].toDomain()
	return &ex, nil
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
}

func (node exerciseNode) toDomain() domain.Exercise {
	var lastUpdated time.Time
	if node.LastUpdatedISO8601 != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, node.LastUpdatedISO8601); err == nil {
			lastUpdated = parsed
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
