package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/exerciseontology/internal/auth"
	"example.com/exerciseontology/internal/cache"
	"example.com/exerciseontology/internal/domain"
	"example.com/exerciseontology/internal/knowledge"
)

func TestSearchExercisesReturnsResults(t *testing.T) {
	repo := knowledge.NewInMemoryRepository()
	service := domain.NewService(repo, cache.NoopInvalidator{})

	_, err := service.UpsertExercise(context.Background(), domain.Exercise{
		Name:            "Kettlebell Swing",
		Targets:         []string{"glutes", "hamstrings"},
		Difficulty:      "intermediate",
		Requires:        []string{"kettlebell"},
		ComplementaryTo: []string{"Deadlift"},
	})
	if err != nil {
		t.Fatalf("seed upsert failed: %v", err)
	}

	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises?query=Kettlebell", nil)
	claims := &auth.Claims{
		Subject:   "user",
		TenantID:  "tenant",
		Scopes:    scopesWith(auth.ScopeOntologyRead),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	req = req.WithContext(auth.WithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	handler.exercises(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body struct {
		Items []domain.Exercise `json:"items"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(body.Items) == 0 {
		t.Fatalf("expected at least one search result")
	}
}

func TestSearchExercisesRequiresAuth(t *testing.T) {
	repo := knowledge.NewInMemoryRepository()
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises", nil)
	rr := httptest.NewRecorder()
	handler.exercises(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestUpsertExercisePersistsAndReturnsBody(t *testing.T) {
	repo := knowledge.NewInMemoryRepository()
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := NewHandler(service)

	payload := map[string]interface{}{
		"id":         "exercise-123",
		"name":       "Turkish Get-Up",
		"difficulty": "advanced",
		"targets":    []string{"core", "shoulders"},
	}
	buf, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/v1/exercises", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	claims := &auth.Claims{
		Subject:   "coach",
		TenantID:  "tenant",
		Scopes:    scopesWith(auth.ScopeOntologyWrite, auth.ScopeOntologyRead),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	req = req.WithContext(auth.WithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	handler.exercises(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body struct {
		Exercise domain.Exercise `json:"exercise"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body.Exercise.ID == "" {
		t.Fatalf("expected exercise ID")
	}
	if body.Exercise.Name != "Turkish Get-Up" {
		t.Fatalf("expected name \"Turkish Get-Up\" got %s", body.Exercise.Name)
	}
}

func scopesWith(values ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(values))
	for _, v := range values {
		m[v] = struct{}{}
	}
	return m
}
