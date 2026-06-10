package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/logmonitor/collector/storage"
)

// IssuesHandler handles issue-related requests
type IssuesHandler struct {
	issueStore storage.IssueStore
	db        *storage.DB // Keep for legacy methods
}

// NewIssuesHandler creates a new issues handler
func NewIssuesHandler(db *storage.DB) *IssuesHandler {
	return &IssuesHandler{
		issueStore: db,
		db:         db,
	}
}

// NewIssuesHandlerWithStore creates a new issues handler with explicit store
func NewIssuesHandlerWithStore(issueStore storage.IssueStore, db *storage.DB) *IssuesHandler {
	return &IssuesHandler{
		issueStore: issueStore,
		db:         db,
	}
}

// GetIssues handles issue list requests
func (h *IssuesHandler) GetIssues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse query parameters
	filter := storage.IssueFilter{
		AppID:    r.URL.Query().Get("app_id"),
		Status:   r.URL.Query().Get("status"),
		Priority: r.URL.Query().Get("priority"),
		Search:   r.URL.Query().Get("search"),
		SortBy:   r.URL.Query().Get("sort"),
		Page:     parseIntParam(r.URL.Query().Get("page"), 1),
		PageSize: parseIntParam(r.URL.Query().Get("page_size"), 20),
	}

	// Validate required params
	if filter.AppID == "" {
		http.Error(w, "Missing app_id parameter", http.StatusBadRequest)
		return
	}

	// Default sort
	if filter.SortBy == "" {
		filter.SortBy = "last_seen"
	}

	// Query issues
	issues, total, err := h.db.GetIssues(filter)
	if err != nil {
		slog.Error("Failed to get issues", "error", err)
		http.Error(w, "Failed to get issues", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	response := map[string]interface{}{
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
		"data":      issues,
	}

	json.NewEncoder(w).Encode(response)
}

// GetIssue handles single issue requests
func (h *IssuesHandler) GetIssue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse issue ID from URL
	idStr := r.URL.Path[len("/api/query/issues/"):]
	if idStr == "" {
		http.Error(w, "Missing issue ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid issue ID", http.StatusBadRequest)
		return
	}

	// Get issue
	issue, err := h.db.GetIssue(id)
	if err != nil {
		slog.Error("Failed to get issue", "error", err)
		http.Error(w, "Failed to get issue", http.StatusInternalServerError)
		return
	}

	// Get recent events for this issue
	events, total, err := h.db.GetIssueEvents(id, 1, 20)
	if err != nil {
		slog.Error("Failed to get issue events", "error", err)
		http.Error(w, "Failed to get issue events", http.StatusInternalServerError)
		return
	}

	// Convert events to response format
	eventsData := make([]map[string]interface{}, 0, len(events))
	for _, event := range events {
		eventsData = append(eventsData, map[string]interface{}{
			"id":          event.ID,
			"app_id":      event.AppID,
			"type":        event.Type,
			"level":       event.Level,
			"message":     event.Message,
			"stack":       event.Stack,
			"url":         event.URL,
			"user_id":     event.UserID,
			"session_id":  event.SessionID,
			"timestamp":   event.CreatedAt,
			"fingerprint": event.Fingerprint,
		})
	}

	// Build response
	response := map[string]interface{}{
		"issue":         issue,
		"recent_events": eventsData,
		"event_count":   total,
	}

	json.NewEncoder(w).Encode(response)
}

// UpdateIssue handles issue update requests
func (h *IssuesHandler) UpdateIssue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow PUT method
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse issue ID from URL
	idStr := r.URL.Path[len("/api/query/issues/"):]
	if idStr == "" {
		http.Error(w, "Missing issue ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid issue ID", http.StatusBadRequest)
		return
	}

	// Parse request body
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Update issue
	if err := h.db.UpdateIssue(id, updates); err != nil {
		slog.Error("Failed to update issue", "error", err)
		http.Error(w, "Failed to update issue", http.StatusInternalServerError)
		return
	}

	// Get updated issue
	issue, err := h.db.GetIssue(id)
	if err != nil {
		slog.Error("Failed to get updated issue", "error", err)
		http.Error(w, "Failed to get updated issue", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"issue":   issue,
	}

	json.NewEncoder(w).Encode(response)
}

// ResolveIssue handles issue resolve requests
func (h *IssuesHandler) ResolveIssue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse issue ID from URL
	idStr := r.URL.Path[len("/api/query/issues/"):]
	if idStr == "" {
		http.Error(w, "Missing issue ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid issue ID", http.StatusBadRequest)
		return
	}

	// Update issue status to resolved
	updates := map[string]interface{}{
		"status": storage.IssueStatusResolved,
	}

	if err := h.db.UpdateIssue(id, updates); err != nil {
		slog.Error("Failed to resolve issue", "error", err)
		http.Error(w, "Failed to resolve issue", http.StatusInternalServerError)
		return
	}

	// Get updated issue
	issue, err := h.db.GetIssue(id)
	if err != nil {
		slog.Error("Failed to get resolved issue", "error", err)
		http.Error(w, "Failed to get resolved issue", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"issue":   issue,
	}

	json.NewEncoder(w).Encode(response)
}

// IgnoreIssue handles issue ignore requests
func (h *IssuesHandler) IgnoreIssue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse issue ID from URL
	idStr := r.URL.Path[len("/api/query/issues/"):]
	if idStr == "" {
		http.Error(w, "Missing issue ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid issue ID", http.StatusBadRequest)
		return
	}

	// Update issue status to ignored
	updates := map[string]interface{}{
		"status": storage.IssueStatusIgnored,
	}

	if err := h.db.UpdateIssue(id, updates); err != nil {
		slog.Error("Failed to ignore issue", "error", err)
		http.Error(w, "Failed to ignore issue", http.StatusInternalServerError)
		return
	}

	// Get updated issue
	issue, err := h.db.GetIssue(id)
	if err != nil {
		slog.Error("Failed to get ignored issue", "error", err)
		http.Error(w, "Failed to get ignored issue", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"issue":   issue,
	}

	json.NewEncoder(w).Encode(response)
}

// GetIssueStats handles issue statistics requests
func (h *IssuesHandler) GetIssueStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		http.Error(w, "Missing app_id parameter", http.StatusBadRequest)
		return
	}

	stats, err := h.db.GetIssueStats(appID)
	if err != nil {
		slog.Error("Failed to get issue stats", "error", err)
		http.Error(w, "Failed to get issue stats", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(stats)
}

// RegisterRoutes registers all issue-related routes
func (h *IssuesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/query/issues", h.GetIssues)
	mux.HandleFunc("GET /api/query/issues/", h.GetIssue)
	mux.HandleFunc("PUT /api/query/issues/", h.UpdateIssue)
	mux.HandleFunc("POST /api/query/issues/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a resolve or ignore request
		path := r.URL.Path
		if len(path) > len("/api/query/issues/") {
			action := r.URL.Query().Get("action")
			switch action {
			case "resolve":
				h.ResolveIssue(w, r)
			case "ignore":
				h.IgnoreIssue(w, r)
			default:
				http.Error(w, "Invalid action", http.StatusBadRequest)
			}
		} else {
			http.Error(w, "Invalid request", http.StatusBadRequest)
		}
	})
	mux.HandleFunc("GET /api/query/issues/stats", h.GetIssueStats)
}