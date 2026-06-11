package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/logmonitor/collector/storage"
	"github.com/logmonitor/collector/webhook"
)

// WebhooksHandler handles webhook-related requests
type WebhooksHandler struct {
	webhookManager *webhook.Manager
	db            *storage.DB // Keep for legacy methods
}

// NewWebhooksHandler creates a new webhooks handler
func NewWebhooksHandler(db *storage.DB, webhookManager *webhook.Manager) *WebhooksHandler {
	return &WebhooksHandler{
		webhookManager: webhookManager,
		db:             db,
	}
}

// GetWebhooks handles webhook list requests
func (h *WebhooksHandler) GetWebhooks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse project_id parameter (default to 0 for global webhooks)
	projectID := parseIntParam(r.URL.Query().Get("project_id"), 0)

	// Get webhooks
	webhooks, err := h.webhookManager.GetWebhooks(int64(projectID))
	if err != nil {
		slog.Error("Failed to get webhooks", "error", err)
		http.Error(w, "Failed to get webhooks", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(webhooks)
}

// CreateWebhook creates a new webhook
func (h *WebhooksHandler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		ProjectID int64    `json:"project_id"`
		Name      string   `json:"name"`
		URL       string   `json:"url"`
		Secret    string   `json:"secret"`
		Events    []string `json:"events"`
		Enabled   bool     `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate inputs
	if req.Name == "" {
		http.Error(w, "Webhook name is required", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "Webhook URL is required", http.StatusBadRequest)
		return
	}

	if len(req.Events) == 0 {
		http.Error(w, "At least one event type is required", http.StatusBadRequest)
		return
	}

	// Validate event types
	validEvents := map[string]bool{
		"issue.created":    true,
		"issue.resolved":   true,
		"issue.reopened":   true,
		"alert.triggered":  true,
	}
	for _, event := range req.Events {
		if !validEvents[event] {
			http.Error(w, "Invalid event type: "+event, http.StatusBadRequest)
			return
		}
	}

	// Set default values
	if req.ProjectID == 0 {
		req.ProjectID = 0 // Global webhook
	}
	if req.Secret == "" {
		// Generate a default secret
		req.Secret = generateWebhookSecret()
	}

	// Create webhook
	webhookObj := &storage.Webhook{
		ProjectID: req.ProjectID,
		Name:      req.Name,
		URL:       req.URL,
		Secret:    req.Secret,
		Events:    req.Events,
		Enabled:   req.Enabled,
	}

	if err := h.webhookManager.CreateWebhook(webhookObj); err != nil {
		slog.Error("Failed to create webhook", "error", err)
		http.Error(w, "Failed to create webhook", http.StatusInternalServerError)
		return
	}

	slog.Info("Webhook created", "webhook_id", webhookObj.ID, "name", webhookObj.Name)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(webhookObj)
}

// UpdateWebhook updates an existing webhook
func (h *WebhooksHandler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse webhook ID from URL
	id, err := parseIDFromPath(r.URL.Path, "/api/admin/webhooks/")
	if err != nil {
		http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
		return
	}

	// Get existing webhook
	webhookObj, err := h.webhookManager.GetWebhook(id)
	if err != nil {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	var req struct {
		Name    *string   `json:"name"`
		URL     *string   `json:"url"`
		Secret  *string   `json:"secret"`
		Events  []string  `json:"events"`
		Enabled *bool     `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update fields if provided
	if req.Name != nil {
		if *req.Name == "" {
			http.Error(w, "Webhook name cannot be empty", http.StatusBadRequest)
			return
		}
		webhookObj.Name = *req.Name
	}

	if req.URL != nil {
		if *req.URL == "" {
			http.Error(w, "Webhook URL cannot be empty", http.StatusBadRequest)
			return
		}
		webhookObj.URL = *req.URL
	}

	if req.Secret != nil {
		webhookObj.Secret = *req.Secret
	}

	if req.Events != nil {
		if len(req.Events) == 0 {
			http.Error(w, "At least one event type is required", http.StatusBadRequest)
			return
		}

		// Validate event types
		validEvents := map[string]bool{
			"issue.created":    true,
			"issue.resolved":   true,
			"issue.reopened":   true,
			"alert.triggered":  true,
		}
		for _, event := range req.Events {
			if !validEvents[event] {
				http.Error(w, "Invalid event type: "+event, http.StatusBadRequest)
				return
			}
		}

		webhookObj.Events = req.Events
	}

	if req.Enabled != nil {
		webhookObj.Enabled = *req.Enabled
	}

	// Update webhook
	if err := h.webhookManager.UpdateWebhook(webhookObj); err != nil {
		slog.Error("Failed to update webhook", "webhook_id", id, "error", err)
		http.Error(w, "Failed to update webhook", http.StatusInternalServerError)
		return
	}

	slog.Info("Webhook updated", "webhook_id", id)

	json.NewEncoder(w).Encode(webhookObj)
}

// DeleteWebhook deletes a webhook
func (h *WebhooksHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse webhook ID from URL
	id, err := parseIDFromPath(r.URL.Path, "/api/admin/webhooks/")
	if err != nil {
		http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
		return
	}

	// Delete webhook
	if err := h.webhookManager.DeleteWebhook(id); err != nil {
		slog.Error("Failed to delete webhook", "webhook_id", id, "error", err)
		http.Error(w, "Failed to delete webhook", http.StatusInternalServerError)
		return
	}

	slog.Info("Webhook deleted", "webhook_id", id)

	w.WriteHeader(http.StatusNoContent)
}

// TestWebhook sends a test payload to a webhook
func (h *WebhooksHandler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse webhook ID from URL
	id, err := parseIDFromPath(r.URL.Path, "/api/admin/webhooks/")
	if err != nil {
		http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
		return
	}

	// Send test webhook
	if err := h.webhookManager.TestWebhook(id); err != nil {
		slog.Error("Failed to send test webhook", "webhook_id", id, "error", err)
		http.Error(w, "Failed to send test webhook: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("Test webhook sent", "webhook_id", id)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Test webhook sent successfully",
	})
}

// Helper functions

// parseIDFromPath extracts ID from URL path like "/api/admin/webhooks/123"
func parseIDFromPath(path, prefix string) (int64, error) {
	// Remove prefix to get ID
	idStr := path
	if len(path) > len(prefix) {
		idStr = path[len(prefix):]
	}

	// Parse trailing parts for the action
	// Handle paths like "/api/admin/webhooks/123/test"
	if idx := findSeparator(idStr, "/"); idx != -1 {
		idStr = idStr[:idx]
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// findSeparator finds the separator character in string
func findSeparator(s, sep string) int {
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}

// stringSliceToEventType converts string slice to EventType slice
func stringSliceToEventType(events []string) []webhook.EventType {
	result := make([]webhook.EventType, len(events))
	for i, event := range events {
		result[i] = webhook.EventType(event)
	}
	return result
}

// generateWebhookSecret generates a random webhook secret
func generateWebhookSecret() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}