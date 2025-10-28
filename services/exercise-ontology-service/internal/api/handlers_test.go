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

func TestCreateExercisePersistsAndReturnsBody(t *testing.T) {
	repo := knowledge.NewInMemoryRepository()
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := NewHandler(service)

	payload := map[string]interface{}{
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

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
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

func TestCreateExerciseValidationError(t *testing.T) {
	repo := knowledge.NewInMemoryRepository()
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := NewHandler(service)

	payload := map[string]interface{}{
		"difficulty": "easy",
	}
	buf, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/v1/exercises", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	claims := &auth.Claims{
		Subject:   "coach",
		TenantID:  "tenant",
		Scopes:    scopesWith(auth.ScopeOntologyWrite),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	req = req.WithContext(auth.WithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	handler.exercises(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateExercise(t *testing.T) {
	repo := knowledge.NewInMemoryRepository()
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := NewHandler(service)

	created, err := service.UpsertExercise(context.Background(), domain.Exercise{
		ID:   "exercise-321",
		Name: "Row",
	})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	payload := map[string]interface{}{
		"name":       "Bent-over Row",
		"difficulty": "intermediate",
		"targets":    []string{"back"},
	}
	buf, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/v1/exercises/"+created.ID, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	claims := &auth.Claims{
		Subject:   "coach",
		TenantID:  "tenant",
		Scopes:    scopesWith(auth.ScopeOntologyWrite),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	req = req.WithContext(auth.WithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	handler.exerciseByID(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body struct {
		Exercise domain.Exercise `json:"exercise"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body.Exercise.Name != "Bent-over Row" {
		t.Fatalf("expected updated name, got %s", body.Exercise.Name)
	}
}

func TestDeleteExercise(t *testing.T) {
	repo := knowledge.NewInMemoryRepository()
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := NewHandler(service)

	exercise, err := service.UpsertExercise(context.Background(), domain.Exercise{
		ID:   "exercise-delete",
		Name: "Lunge",
	})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/exercises/"+exercise.ID, nil)
	claims := &auth.Claims{
		Subject:   "coach",
		TenantID:  "tenant",
		Scopes:    scopesWith(auth.ScopeOntologyWrite),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	req = req.WithContext(auth.WithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	handler.exerciseByID(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}

	retrieved, err := service.GetExercise(context.Background(), exercise.ID)
	if err == nil && retrieved != nil {
		t.Fatalf("expected exercise to be deleted")
	}
}

func TestUpdateRelationships(t *testing.T) {
	repo := knowledge.NewInMemoryRepository()
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := NewHandler(service)

	base, err := service.UpsertExercise(context.Background(), domain.Exercise{
		ID:   "exercise-base",
		Name: "Base",
	})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	other, err := service.UpsertExercise(context.Background(), domain.Exercise{
		ID:   "exercise-other",
		Name: "Other",
	})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	payload := map[string]interface{}{
		"targets":              []string{"endurance"},
		"complementary_to":     []string{other.ID},
		"contraindicated_with": []string{},
	}
	buf, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/v1/exercises/"+base.ID+"/relationships", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	claims := &auth.Claims{
		Subject:   "coach",
		TenantID:  "tenant",
		Scopes:    scopesWith(auth.ScopeOntologyWrite),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	req = req.WithContext(auth.WithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	handler.exerciseByID(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	ex, err := service.GetExercise(context.Background(), base.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if len(ex.Targets) == 0 || ex.Targets[0] != "endurance" {
		t.Fatalf("expected targets to update")
	}
	if len(ex.ComplementaryTo) != 1 || ex.ComplementaryTo[0] != other.ID {
		t.Fatalf("expected complementary to include other")
	}
	otherUpdated, err := service.GetExercise(context.Background(), other.ID)
	if err != nil {
		t.Fatalf("get other failed: %v", err)
	}
	if len(otherUpdated.ComplementaryTo) != 1 || otherUpdated.ComplementaryTo[0] != base.ID {
		t.Fatalf("expected reciprocal complementary link")
	}
}

func TestUpdateRelationshipsInvalidReference(t *testing.T) {
	repo := knowledge.NewInMemoryRepository()
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := NewHandler(service)

	base, err := service.UpsertExercise(context.Background(), domain.Exercise{
		ID:   "exercise-base",
		Name: "Base",
	})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	payload := map[string]interface{}{
		"complementary_to": []string{"missing"},
	}
	buf, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/v1/exercises/"+base.ID+"/relationships", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	claims := &auth.Claims{
		Subject:   "coach",
		TenantID:  "tenant",
		Scopes:    scopesWith(auth.ScopeOntologyWrite),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	req = req.WithContext(auth.WithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	handler.exerciseByID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func scopesWith(values ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(values))
	for _, v := range values {
		m[v] = struct{}{}
	}
	return m
}
