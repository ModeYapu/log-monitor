package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/logmonitor/collector/storage"
)

// AlertsHandler handles alert-related requests
type AlertsHandler struct {
	db *storage.DB
}

// NewAlertsHandler creates a new alerts handler
func NewAlertsHandler(db *storage.DB) *AlertsHandler {
	return &AlertsHandler{db: db}
}

// RegisterRoutes registers alert routes
func (h *AlertsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/query/alerts", h.GetAlerts)
	mux.HandleFunc("POST /api/query/alerts", h.CreateAlert)
	mux.HandleFunc("DELETE /api/query/alerts/", h.DeleteAlert)
}

// GetAlerts returns alert rules and logs for an app
func (h *AlertsHandler) GetAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	rules, err := h.db.GetAlertRules(appID)
	if err != nil {
		log.Printf("Failed to get alert rules: %v", err)
		http.Error(w, "Failed to get alert rules", http.StatusInternalServerError)
		return
	}

	logs, err := h.db.GetAlertLogs(appID, 100)
	if err != nil {
		log.Printf("Failed to get alert logs: %v", err)
	}

	response := map[string]interface{}{
		"rules": rules,
		"logs":  logs,
	}

	json.NewEncoder(w).Encode(response)
}

// CreateAlertRequest represents the request to create/update an alert rule
type CreateAlertRequest struct {
	AppID           string          `json:"app_id"`
	Name            string          `json:"name"`
	ConditionType   string          `json:"condition_type"`   // threshold|rate|new_error
	ConditionConfig json.RawMessage `json:"condition_config"` // JSON string
	NotifyType      string          `json:"notify_type"`      // webhook|feishu|email
	NotifyConfig    json.RawMessage `json:"notify_config"`    // JSON string
	Enabled         bool            `json:"enabled"`
	CooldownMinutes int             `json:"cooldown_minutes"`
}

// CreateAlert creates a new alert rule
func (h *AlertsHandler) CreateAlert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.AppID == "" || req.Name == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	conditionConfig := string(req.ConditionConfig)
	notifyConfig := string(req.NotifyConfig)

	if req.CooldownMinutes < 1 {
		req.CooldownMinutes = 30
	}

	rule := storage.AlertRule{
		AppID:           req.AppID,
		Name:            req.Name,
		ConditionType:   req.ConditionType,
		ConditionConfig: conditionConfig,
		NotifyType:      req.NotifyType,
		NotifyConfig:    notifyConfig,
		Enabled:         boolToInt(req.Enabled),
		CooldownMinutes: req.CooldownMinutes,
		CreatedAt:       time.Now().UnixMilli(),
	}

	id, err := h.db.CreateAlertRule(rule)
	if err != nil {
		log.Printf("Failed to create alert rule: %v", err)
		http.Error(w, "Failed to create alert rule", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

// DeleteAlert deletes an alert rule
func (h *AlertsHandler) DeleteAlert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	path := r.URL.Path
	idStr := ""
	if len(path) > len("/api/query/alerts/") {
		idStr = path[len("/api/query/alerts/"):]
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteAlertRule(id); err != nil {
		log.Printf("Failed to delete alert rule: %v", err)
		http.Error(w, "Failed to delete alert rule", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i == 1
}
