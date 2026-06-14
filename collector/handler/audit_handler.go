package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/logmonitor/collector/model"
	"github.com/logmonitor/collector/storage"
)

// AuditHandler handles audit log requests
type AuditHandler struct {
	auditStore storage.AuditStore
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(auditStore storage.AuditStore) *AuditHandler {
	return &AuditHandler{
		auditStore: auditStore,
	}
}

// GetAuditLogs handles audit log list requests (admin only)
func (h *AuditHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	filter := model.AuditFilter{
		Page:     parseIntParam(r.URL.Query().Get("page"), 1),
		PageSize: parseIntParam(r.URL.Query().Get("page_size"), 20),
	}

	// Parse project_id filter
	if projectIDStr := r.URL.Query().Get("project_id"); projectIDStr != "" {
		if projectID, err := strconv.ParseInt(projectIDStr, 10, 64); err == nil {
			filter.ProjectID = projectID
		}
	}

	// Parse user_id filter
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if userID, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			filter.UserID = userID
		}
	}

	// Parse action filter
	filter.Action = r.URL.Query().Get("action")

	// Parse resource filter
	filter.Resource = r.URL.Query().Get("resource")

	// Parse date range filters
	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		if startDate, err := strconv.ParseInt(startDateStr, 10, 64); err == nil {
			filter.StartDate = startDate
		}
	}
	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		if endDate, err := strconv.ParseInt(endDateStr, 10, 64); err == nil {
			filter.EndDate = endDate
		}
	}

	// Default to last 7 days if no date range specified
	if filter.StartDate == 0 && filter.EndDate == 0 {
		filter.StartDate = time.Now().AddDate(0, 0, -7).UnixMilli()
		filter.EndDate = time.Now().UnixMilli()
	}

	// Query audit logs
	logs, total, err := h.auditStore.QueryAuditLogs(filter)
	if err != nil {
		http.Error(w, "Failed to query audit logs", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	response := map[string]interface{}{
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
		"data":      logs,
	}

	json.NewEncoder(w).Encode(response)
}

// RegisterRoutes registers all audit-related routes
func (h *AuditHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/audit-logs", h.GetAuditLogs)
}
