package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/logmonitor/collector/alerter"
)

// ChannelManagerHandler handles notification channel API requests
type ChannelManagerHandler struct {
	channelMgr *alerter.ChannelManager
	mu         sync.RWMutex
}

// NewChannelManagerHandler creates a new channel manager handler
func NewChannelManagerHandler(channelMgr *alerter.ChannelManager) *ChannelManagerHandler {
	return &ChannelManagerHandler{
		channelMgr: channelMgr,
	}
}

// RegisterRoutes registers channel manager routes
func (h *ChannelManagerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/alerts/channels", h.ListChannels)
	mux.HandleFunc("POST /api/alerts/channels", h.CreateChannel)
	mux.HandleFunc("PUT /api/alerts/channels/", h.UpdateChannel)
	mux.HandleFunc("DELETE /api/alerts/channels/", h.DeleteChannel)
	mux.HandleFunc("POST /api/alerts/test", h.TestChannel)
}

// ListChannels returns all notification channels
func (h *ChannelManagerHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	channels := h.channelMgr.ListChannels()

	response := make([]map[string]interface{}, 0, len(channels))
	for _, ch := range channels {
		response = append(response, convertChannelToMap(ch))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"channels": response,
	})
}

// CreateChannelRequest represents the request to create a channel
type CreateChannelRequest struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Name    string                 `json:"name"`
	Config  map[string]interface{} `json:"config"`
	Enabled bool                   `json:"enabled"`
}

// CreateChannel creates a new notification channel
func (h *ChannelManagerHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		http.Error(w, "Missing required field: type", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		req.Name = req.Type
	}

	// Generate ID if not provided
	if req.ID == "" {
		req.ID = generateChannelID(req.Type)
	}

	if req.Config == nil {
		req.Config = make(map[string]interface{})
	}

	channel := &alerter.NotifyChannel{
		ID:      req.ID,
		Type:    req.Type,
		Name:    req.Name,
		Config:  req.Config,
		Enabled: true,
	}

	if err := h.channelMgr.AddChannel(channel); err != nil {
		slog.Error("Failed to add channel", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"channel": convertChannelToMap(channel),
	})
}

// UpdateChannelRequest represents the request to update a channel
type UpdateChannelRequest struct {
	Name    string                 `json:"name,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
	Enabled *bool                  `json:"enabled,omitempty"`
}

// UpdateChannel updates an existing channel
func (h *ChannelManagerHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	path := r.URL.Path
	idStr := ""
	if len(path) > len("/api/alerts/channels/") {
		idStr = path[len("/api/alerts/channels/"):]
	}

	var req UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get existing channel
	existing, ok := h.channelMgr.GetChannel(idStr)
	if !ok {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	updates := &alerter.NotifyChannel{
		ID:      idStr,
		Name:    existing.Name,
		Type:    existing.Type,
		Config:  existing.Config,
		Enabled: existing.Enabled,
	}

	// Apply updates
	if req.Name != "" {
		updates.Name = req.Name
	}
	if req.Config != nil {
		updates.Config = req.Config
	}
	if req.Enabled != nil {
		updates.Enabled = *req.Enabled
	}

	if err := h.channelMgr.UpdateChannel(idStr, updates); err != nil {
		slog.Error("Failed to update channel", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get updated channel
	updated, _ := h.channelMgr.GetChannel(idStr)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"channel": convertChannelToMap(updated),
	})
}

// DeleteChannel deletes a notification channel
func (h *ChannelManagerHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	path := r.URL.Path
	idStr := ""
	if len(path) > len("/api/alerts/channels/") {
		idStr = path[len("/api/alerts/channels/"):]
	}

	if err := h.channelMgr.DeleteChannel(idStr); err != nil {
		slog.Error("Failed to delete channel", "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Channel deleted successfully",
	})
}

// TestChannelRequest represents the request to test a channel
type TestChannelRequest struct {
	ChannelID string                 `json:"channel_id,omitempty"`
	Type      string                 `json:"type,omitempty"`
	Config    map[string]interface{} `json:"config,omitempty"`
}

// TestChannel sends a test notification
func (h *ChannelManagerHandler) TestChannel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TestChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// If channel_id is provided, test existing channel
	if req.ChannelID != "" {
		if err := h.channelMgr.TestChannel(req.ChannelID); err != nil {
			slog.Error("Failed to test channel", "error", err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Failed to send test: %v", err),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Test notification sent",
		})
		return
	}

	// Otherwise, test with provided config (one-time test)
	if req.Type == "" || req.Config == nil {
		http.Error(w, "Missing required fields: type and config", http.StatusBadRequest)
		return
	}

	// Create temporary channel for testing
	tempID := "temp-test-" + generateRandomID()
	tempChannel := &alerter.NotifyChannel{
		ID:      tempID,
		Type:    req.Type,
		Name:    "Temporary Test Channel",
		Config:  req.Config,
		Enabled: true,
	}

	// Add temporarily
	if err := h.channelMgr.AddChannel(tempChannel); err != nil {
		slog.Error("Failed to add temp channel", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Send test
	if err := h.channelMgr.TestChannel(tempID); err != nil {
		slog.Error("Failed to test channel", "error", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Failed to send test: %v", err),
		})
		return
	}

	// Clean up temp channel
	h.channelMgr.DeleteChannel(tempID)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Test notification sent",
	})
}

// Helper functions

func convertChannelToMap(ch *alerter.NotifyChannel) map[string]interface{} {
	// Hide sensitive config values
	safeConfig := make(map[string]interface{})
	for k, v := range ch.Config {
		if isSensitiveKey(k) {
			safeConfig[k] = "***"
		} else {
			safeConfig[k] = v
		}
	}

	return map[string]interface{}{
		"id":      ch.ID,
		"type":    ch.Type,
		"name":    ch.Name,
		"enabled": ch.Enabled,
		"config":  safeConfig,
	}
}

func isSensitiveKey(key string) bool {
	sensitiveKeys := []string{"password", "pass", "secret", "token", "key"}
	for _, sk := range sensitiveKeys {
		if key == sk || key == sk+"_" || key == "_"+sk {
			return true
		}
	}
	return false
}

func generateChannelID(channelType string) string {
	return fmt.Sprintf("%s-%s", channelType, generateRandomID())
}

func generateRandomID() string {
	// Simple random ID generation
	return fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
}
