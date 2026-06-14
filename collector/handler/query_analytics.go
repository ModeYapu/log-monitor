package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/logmonitor/collector/storage"
)

// QueryHandler handles log query requests

func (h *QueryHandler) QueryPerformanceSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		http.Error(w, "Missing app_id parameter", http.StatusBadRequest)
		return
	}

	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "24h"
	}

	summary, err := h.analyticsStore.GetPerformanceSummary(appID, timeRange)
	if err != nil {
		slog.Error("Failed to get performance summary", "error", err)
		http.Error(w, "Failed to get performance summary", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(summary)
}

// QueryPerformanceTrend handles performance trend queries
func (h *QueryHandler) QueryPerformanceTrend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		http.Error(w, "Missing app_id parameter", http.StatusBadRequest)
		return
	}

	metric := r.URL.Query().Get("metric")
	if metric == "" {
		http.Error(w, "Missing metric parameter", http.StatusBadRequest)
		return
	}

	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "1h"
	}

	trend, err := h.analyticsStore.GetPerformanceTrend(appID, metric, granularity)
	if err != nil {
		slog.Error("Failed to get performance trend", "error", err)
		http.Error(w, "Failed to get performance trend", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(storage.PerformanceTrendResponse{
		Metric:      metric,
		Granularity: granularity,
		Data:        trend,
	})
}

// QueryPerformancePages handles page performance ranking queries
func (h *QueryHandler) QueryPerformancePages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		http.Error(w, "Missing app_id parameter", http.StatusBadRequest)
		return
	}

	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "7d"
	}

	pages, err := h.analyticsStore.GetPagePerformanceRanking(appID, timeRange)
	if err != nil {
		slog.Error("Failed to get page performance ranking", "error", err)
		http.Error(w, "Failed to get page performance ranking", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(storage.PerformancePagesResponse{
		TimeRange: timeRange,
		Data:      pages,
	})
}

// QueryPerformanceRegression handles performance regression queries
func (h *QueryHandler) QueryPerformanceRegression(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		http.Error(w, "Missing app_id parameter", http.StatusBadRequest)
		return
	}

	regressions, err := h.analyticsStore.GetPerformanceRegressions(appID)
	if err != nil {
		slog.Error("Failed to get performance regressions", "error", err)
		http.Error(w, "Failed to get performance regressions", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(storage.PerformanceRegressionResponse{
		Regressions: regressions,
		Count:       len(regressions),
	})
}

// QueryNewErrors handles new errors queries
func (h *QueryHandler) QueryNewErrors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		http.Error(w, "Missing app_id parameter", http.StatusBadRequest)
		return
	}

	since := parseIntParam(r.URL.Query().Get("since"), 60) // Default 60 minutes

	newErrors, err := h.analyticsStore.GetNewErrors(appID, since)
	if err != nil {
		slog.Error("Failed to get new errors", "error", err)
		http.Error(w, "Failed to get new errors", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(storage.NewErrorsResponse{
		Data:         newErrors,
		Count:        len(newErrors),
		SinceMinutes: since,
	})
}

// QueryAlertTriggers handles recent alert trigger queries
func (h *QueryHandler) QueryAlertTriggers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limit := parseIntParam(r.URL.Query().Get("limit"), 5) // Default 5 triggers

	triggers, err := h.analyticsStore.GetRecentAlertTriggers(limit)
	if err != nil {
		slog.Error("Failed to get alert triggers", "error", err)
		http.Error(w, "Failed to get alert triggers", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(storage.AlertTriggersResponse{
		Data:  triggers,
		Count: len(triggers),
	})
}

// QueryActiveSessions handles active sessions queries
func (h *QueryHandler) QueryActiveSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		http.Error(w, "Missing app_id parameter", http.StatusBadRequest)
		return
	}

	limit := parseIntParam(r.URL.Query().Get("limit"), 5) // Default 5 sessions

	sessions, err := h.analyticsStore.GetActiveSessions(appID, limit)
	if err != nil {
		slog.Error("Failed to get active sessions", "error", err)
		http.Error(w, "Failed to get active sessions", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(storage.ActiveSessionsResponse{
		Data:  sessions,
		Count: len(sessions),
	})
}

// QueryStatsComparison handles today vs yesterday statistics comparison
func (h *QueryHandler) QueryStatsComparison(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		http.Error(w, "Missing app_id parameter", http.StatusBadRequest)
		return
	}

	comparison, err := h.analyticsStore.GetStatsComparison(appID)
	if err != nil {
		slog.Error("Failed to get stats comparison", "error", err)
		http.Error(w, "Failed to get stats comparison", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(comparison)
}
