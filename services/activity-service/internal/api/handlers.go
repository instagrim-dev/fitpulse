// Package api exposes HTTP handlers for the activity service.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/activity/internal/auth"
	"example.com/activity/internal/domain"
	"example.com/activity/internal/persistence"
)

// Handler coordinates HTTP requests with the domain service.
type Handler struct {
	service *domain.Service
}

// NewHandler builds a Handler.
func NewHandler(service *domain.Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires endpoints to the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/activities", h.activities)
	mux.HandleFunc("/v1/activities/", h.activityByID)
	mux.HandleFunc("/v1/activities/metrics", h.activityMetrics)
	mux.HandleFunc("/healthz", healthz)
}

// healthz reports a simple OK status for container health checks.
func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) activities(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createActivity(w, r)
	case http.MethodGet:
		h.listActivities(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "unsupported method")
	}
}

func (h *Handler) activityByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/activities/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "missing activity id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getActivity(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "unsupported method")
	}
}

func (h *Handler) createActivity(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	if !claims.HasScope(auth.ScopeActivitiesWrite) {
		writeError(w, http.StatusForbidden, "forbidden", "scope activities:write required")
		return
	}

	var req CreateActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "unable to parse body")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	aggregate, replay, err := h.service.CreateActivity(r.Context(), domain.CreateActivityInput{
		TenantID:       claims.TenantID,
		UserID:         req.UserID,
		ActivityType:   req.ActivityType,
		StartedAt:      req.StartedAt,
		DurationMin:    req.DurationMin,
		Source:         req.Source,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	resp := CreateActivityResponse{
		ActivityID: aggregate.ID,
		Status:     string(aggregate.State),
		Replay:     replay,
	}

	status := http.StatusAccepted
	if replay {
		status = http.StatusOK
	}
	writeJSON(w, status, resp)
}

func (h *Handler) getActivity(w http.ResponseWriter, r *http.Request, id string) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	if !claims.HasScope(auth.ScopeActivitiesRead) && !claims.HasScope(auth.ScopeActivitiesWrite) {
		writeError(w, http.StatusForbidden, "forbidden", "scope activities:read required")
		return
	}

	aggregate, err := h.service.GetActivity(r.Context(), claims.TenantID, id)
	if err != nil {
		if errors.Is(err, domain.ErrActivityNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "activity not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	resp := toActivityView(*aggregate)
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) listActivities(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	if !claims.HasScope(auth.ScopeActivitiesRead) && !claims.HasScope(auth.ScopeActivitiesWrite) {
		writeError(w, http.StatusForbidden, "forbidden", "scope activities:read required")
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "missing user_id parameter")
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	cursorToken := r.URL.Query().Get("cursor")
	cursor, err := persistence.DecodeCursor(cursorToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid cursor")
		return
	}

	aggregates, next, err := h.service.ListActivitiesByUser(r.Context(), claims.TenantID, userID, cursor, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	items := make([]ActivityView, 0, len(aggregates))
	for _, agg := range aggregates {
		items = append(items, toActivityView(agg))
	}

	resp := ListActivitiesResponse{
		Items:      items,
		NextCursor: persistence.EncodeCursor(next),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) activityMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "unsupported method")
		return
	}

	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	if !claims.HasScope(auth.ScopeActivitiesRead) && !claims.HasScope(auth.ScopeActivitiesWrite) {
		writeError(w, http.StatusForbidden, "forbidden", "scope activities:read required")
		return
	}

	userID := r.URL.Query().Get("user_id")
	if strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "missing user_id parameter")
		return
	}

	timelineLimit := 10
	if raw := r.URL.Query().Get("timeline_limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			if parsed > 50 {
				parsed = 50
			}
			timelineLimit = parsed
		}
	}

	windowHours := 24
	if raw := r.URL.Query().Get("window_hours"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			windowHours = parsed
		}
	}

	window := time.Duration(windowHours) * time.Hour
	metrics, err := h.service.GetActivityMetrics(r.Context(), claims.TenantID, userID, window, timelineLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	summary := metrics.Summary
	resp := ActivityMetricsResponse{
		Summary: ActivityMetricsSummary{
			Total:                    summary.Total,
			Pending:                  summary.Pending,
			Synced:                   summary.Synced,
			Failed:                   summary.Failed,
			AverageDurationMinutes:   summary.AverageDurationMinutes,
			AverageProcessingSeconds: summary.AverageProcessingSeconds,
			OldestPendingAgeSeconds:  summary.OldestPendingAgeSeconds,
			SuccessRate:              summary.SuccessRate,
			LastActivityAt:           summary.LastActivityAt,
		},
		WindowSeconds: metrics.WindowSeconds,
		TimelineLimit: timelineLimit,
		Timeline:      make([]ActivityView, 0, len(metrics.Timeline)),
	}

	for _, agg := range metrics.Timeline {
		resp.Timeline = append(resp.Timeline, toActivityView(agg))
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateActivityRequest is the payload for POST /v1/activities.
type CreateActivityRequest struct {
	UserID       string    `json:"user_id"`
	ActivityType string    `json:"activity_type"`
	StartedAt    time.Time `json:"started_at"`
	DurationMin  int       `json:"duration_min"`
	Source       string    `json:"source"`
}

// Validate ensures request correctness.
func (r CreateActivityRequest) Validate() error {
	if strings.TrimSpace(r.UserID) == "" {
		return errors.New("user_id is required")
	}
	if strings.TrimSpace(r.ActivityType) == "" {
		return errors.New("activity_type is required")
	}
	if r.DurationMin <= 0 {
		return errors.New("duration_min must be > 0")
	}
	if r.StartedAt.IsZero() {
		return errors.New("started_at is required")
	}
	if strings.TrimSpace(r.Source) == "" {
		return errors.New("source is required")
	}
	return nil
}

// CreateActivityResponse describes the response body for create.
type CreateActivityResponse struct {
	ActivityID string `json:"activity_id"`
	Status     string `json:"status"`
	Replay     bool   `json:"idempotent_replay"`
}

// ActivityView exposes full details about an activity.
type ActivityView struct {
	ActivityID      string     `json:"activity_id"`
	TenantID        string     `json:"tenant_id"`
	UserID          string     `json:"user_id"`
	ActivityType    string     `json:"activity_type"`
	StartedAt       time.Time  `json:"started_at"`
	DurationMin     int        `json:"duration_min"`
	Source          string     `json:"source"`
	Version         string     `json:"version"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	FailureReason   *string    `json:"failure_reason,omitempty"`
	NextRetryAt     *time.Time `json:"next_retry_at,omitempty"`
	QuarantinedAt   *time.Time `json:"quarantined_at,omitempty"`
	ReplayAvailable bool       `json:"replay_available"`
}

// ListActivitiesResponse packages list results.
type ListActivitiesResponse struct {
	Items      []ActivityView `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// ActivityMetricsSummary describes aggregate stats for a cohort of activities.
type ActivityMetricsSummary struct {
	Total                    int        `json:"total"`
	Pending                  int        `json:"pending"`
	Synced                   int        `json:"synced"`
	Failed                   int        `json:"failed"`
	AverageDurationMinutes   float64    `json:"average_duration_minutes"`
	AverageProcessingSeconds float64    `json:"average_processing_seconds"`
	OldestPendingAgeSeconds  float64    `json:"oldest_pending_age_seconds"`
	SuccessRate              float64    `json:"success_rate"`
	LastActivityAt           *time.Time `json:"last_activity_at,omitempty"`
}

// ActivityMetricsResponse merges summary metrics with recent timeline entries.
type ActivityMetricsResponse struct {
	Summary       ActivityMetricsSummary `json:"summary"`
	Timeline      []ActivityView         `json:"timeline"`
	TimelineLimit int                    `json:"timeline_limit"`
	WindowSeconds int64                  `json:"window_seconds"`
}

func writeError(w http.ResponseWriter, status int, code, detail string) {
	payload := map[string]string{
		"type":   code,
		"detail": detail,
	}
	writeJSON(w, status, payload)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func toActivityView(agg domain.ActivityAggregate) ActivityView {
	return ActivityView{
		ActivityID:      agg.ID,
		TenantID:        agg.TenantID,
		UserID:          agg.UserID,
		ActivityType:    agg.ActivityType,
		StartedAt:       agg.StartedAt,
		DurationMin:     agg.DurationMin,
		Source:          agg.Source,
		Version:         agg.Version,
		Status:          string(agg.State),
		CreatedAt:       agg.CreatedAt,
		UpdatedAt:       agg.UpdatedAt,
		FailureReason:   agg.FailureReason,
		NextRetryAt:     agg.NextRetryAt,
		QuarantinedAt:   agg.QuarantinedAt,
		ReplayAvailable: agg.ReplayAvailable,
	}
}
