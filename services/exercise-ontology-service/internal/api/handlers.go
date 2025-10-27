// Package api exposes HTTP handlers for the exercise ontology service.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"example.com/exerciseontology/internal/auth"
	"example.com/exerciseontology/internal/domain"
)

// Handler handles HTTP interactions.
type Handler struct {
	service *domain.Service
}

// NewHandler constructs Handler.
func NewHandler(service *domain.Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes sets up routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/exercises", h.exercises)
	mux.HandleFunc("/v1/exercises/", h.exerciseByID)
	mux.HandleFunc("/healthz", healthz)
}

// healthz returns an OK response for readiness probes.
func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) exercises(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.searchExercises(w, r)
	case http.MethodPost:
		h.upsertExercise(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "unsupported method")
	}
}

func (h *Handler) exerciseByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	if !claims.HasScope(auth.ScopeOntologyRead) && !claims.HasScope(auth.ScopeOntologyWrite) {
		writeError(w, http.StatusForbidden, "forbidden", "scope ontology:read required")
		return
	}

	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "unsupported method")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/exercises/")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "missing exercise id")
		return
	}

	exercise, err := h.service.GetExercise(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrExerciseNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "exercise not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exercise)
}

func (h *Handler) searchExercises(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	if !claims.HasScope(auth.ScopeOntologyRead) && !claims.HasScope(auth.ScopeOntologyWrite) {
		writeError(w, http.StatusForbidden, "forbidden", "scope ontology:read required")
		return
	}

	query := r.URL.Query().Get("query")
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	exercises, err := h.service.SearchExercises(r.Context(), query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": exercises})
}

func (h *Handler) upsertExercise(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	if !claims.HasScope(auth.ScopeOntologyWrite) {
		writeError(w, http.StatusForbidden, "forbidden", "scope ontology:write required")
		return
	}

	var req UpsertExerciseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "unable to parse body")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	exercise := domain.Exercise{
		ID:                req.ID,
		Name:              req.Name,
		Difficulty:        req.Difficulty,
		Targets:           req.Targets,
		Requires:          req.Requires,
		Contraindications: req.Contraindications,
		ComplementaryTo:   req.ComplementaryTo,
	}

    updated, err := h.service.UpsertExercise(r.Context(), exercise)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "server_error", err.Error())
        return
    }

    writeJSON(w, http.StatusOK, map[string]any{"exercise": updated})
}

// UpsertExerciseRequest represents the request payload.
type UpsertExerciseRequest struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Difficulty        string   `json:"difficulty"`
	Targets           []string `json:"targets"`
	Requires          []string `json:"requires"`
	Contraindications []string `json:"contraindicated_with"`
	ComplementaryTo   []string `json:"complementary_to"`
}

// Validate ensures request integrity.
func (r UpsertExerciseRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("name is required")
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, code, detail string) {
	writeJSON(w, status, map[string]string{"type": code, "detail": detail})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
