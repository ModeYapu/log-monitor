package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/logmonitor/collector/alerter"
	"github.com/logmonitor/collector/storage"
)

// RuleEngineHandler handles rule engine API requests
type RuleEngineHandler struct {
	db          storage.AlertStore
	ruleEngine  *alerter.RuleEngine
}

// NewRuleEngineHandler creates a new rule engine handler
func NewRuleEngineHandler(db storage.AlertStore, ruleEngine *alerter.RuleEngine) *RuleEngineHandler {
	return &RuleEngineHandler{
		db:         db,
		ruleEngine: ruleEngine,
	}
}

// RegisterRoutes registers rule engine routes
func (h *RuleEngineHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/alerts/rules", h.ListRules)
	mux.HandleFunc("POST /api/alerts/rules", h.CreateRule)
	mux.HandleFunc("PUT /api/alerts/rules/", h.UpdateRule)
	mux.HandleFunc("DELETE /api/alerts/rules/", h.DeleteRule)
	mux.HandleFunc("GET /api/alerts/rules/", h.GetRule)
}

// ListRules returns all alert rules for an app
func (h *RuleEngineHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		// Return all rules if no app specified
		rules, err := h.db.GetAllAlertRules()
		if err != nil {
			slog.Error("Failed to get alert rules", "error", err)
			http.Error(w, "Failed to get alert rules", http.StatusInternalServerError)
			return
		}

		// Convert to response format
		response := make([]map[string]interface{}, 0, len(rules))
		for _, rule := range rules {
			response = append(response, convertAlertRuleToMap(rule))
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"rules": response,
		})
		return
	}

	rules, err := h.db.GetAlertRules(appID)
	if err != nil {
		slog.Error("Failed to get alert rules", "error", err)
		http.Error(w, "Failed to get alert rules", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	response := make([]map[string]interface{}, 0, len(rules))
	for _, rule := range rules {
		response = append(response, convertAlertRuleToMap(rule))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": response,
	})
}

// GetRule returns a single rule by ID
func (h *RuleEngineHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract ID from path
	path := r.URL.Path
	idStr := ""
	if len(path) > len("/api/alerts/rules/") {
		idStr = path[len("/api/alerts/rules/"):]
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}

	// Get all rules and find by ID
	rules, err := h.db.GetAllAlertRules()
	if err != nil {
		slog.Error("Failed to get alert rules", "error", err)
		http.Error(w, "Failed to get alert rules", http.StatusInternalServerError)
		return
	}

	for _, rule := range rules {
		if rule.ID == id {
			json.NewEncoder(w).Encode(convertAlertRuleToMap(rule))
			return
		}
	}

	http.Error(w, "Rule not found", http.StatusNotFound)
}

// CreateRuleRequest represents the request to create a rule
type CreateRuleRequest struct {
	AppID           string                   `json:"app_id"`
	Name            string                   `json:"name"`
	Condition       alerter.RuleCondition    `json:"condition"`
	Severity        string                   `json:"severity"`
	CooldownMinutes int                      `json:"cooldown_minutes"`
	Channels        []string                 `json:"channels"`
	Enabled         bool                     `json:"enabled"`
	MessageTemplate string                  `json:"message_template"`
}

// CreateRule creates a new alert rule
func (h *RuleEngineHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.AppID == "" || req.Name == "" {
		http.Error(w, "Missing required fields: app_id and name are required", http.StatusBadRequest)
		return
	}

	// Validate condition
	if req.Condition.Type == "" {
		req.Condition.Type = "threshold"
	}
	if req.Condition.WindowMin <= 0 {
		req.Condition.WindowMin = 5
	}
	if req.CooldownMinutes < 1 {
		req.CooldownMinutes = 30
	}
	if req.Severity == "" {
		req.Severity = "warning"
	}

	// Validate severity
	validSeverities := map[string]bool{"critical": true, "warning": true, "info": true}
	if !validSeverities[req.Severity] {
		req.Severity = "warning"
	}

	// Create Rule
	rule := &alerter.Rule{
		AppID:           req.AppID,
		Name:            req.Name,
		Condition:       req.Condition,
		Severity:        req.Severity,
		CooldownMinutes: req.CooldownMinutes,
		Channels:        req.Channels,
		Enabled:         req.Enabled,
		CreatedAt:       time.Now().UnixMilli(),
	}

	// Convert to storage format
	storageRule := alerter.RuleToStorage(rule)

	// Store in database
	id, err := h.db.CreateAlertRule(storageRule)
	if err != nil {
		slog.Error("Failed to create alert rule", "error", err)
		http.Error(w, "Failed to create alert rule", http.StatusInternalServerError)
		return
	}

	rule.ID = id

	// Reload rule engine rules
	if err := h.ruleEngine.LoadRules(); err != nil {
		slog.Error("Failed to reload rules", "error", err)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      id,
		"rule":    convertRuleToMap(rule),
	})
}

// UpdateRuleRequest represents the request to update a rule
type UpdateRuleRequest struct {
	Name            *string                `json:"name,omitempty"`
	Condition       *alerter.RuleCondition `json:"condition,omitempty"`
	Severity        *string                `json:"severity,omitempty"`
	CooldownMinutes *int                   `json:"cooldown_minutes,omitempty"`
	Channels        []string               `json:"channels,omitempty"`
	Enabled         *bool                  `json:"enabled,omitempty"`
	MessageTemplate string                 `json:"message_template,omitempty"`
}

// UpdateRule updates an existing rule
func (h *RuleEngineHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	path := r.URL.Path
	idStr := ""
	if len(path) > len("/api/alerts/rules/") {
		idStr = path[len("/api/alerts/rules/"):]
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}

	var req UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get existing rule
	rules, err := h.db.GetAllAlertRules()
	if err != nil {
		slog.Error("Failed to get alert rules", "error", err)
		http.Error(w, "Failed to get alert rules", http.StatusInternalServerError)
		return
	}

	var existing *storage.AlertRule
	for _, r := range rules {
		if r.ID == id {
			existing = &r
			break
		}
	}

	if existing == nil {
		http.Error(w, "Rule not found", http.StatusNotFound)
		return
	}

	// Convert to Rule for easier manipulation
	rule, err := h.ruleEngine.StorageRuleToRule(*existing)
	if err != nil {
		slog.Error("Failed to convert rule", "error", err)
		http.Error(w, "Failed to update rule", http.StatusInternalServerError)
		return
	}

	// Apply updates
	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Condition != nil {
		rule.Condition = *req.Condition
	}
	if req.Severity != nil {
		rule.Severity = *req.Severity
	}
	if req.CooldownMinutes != nil {
		rule.CooldownMinutes = *req.CooldownMinutes
	}
	if req.Channels != nil {
		rule.Channels = req.Channels
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	// Note: Full UpdateAlertRule method would be needed in storage
	// For now, we update through Delete + Create pattern
	if err := h.db.DeleteAlertRule(id); err != nil {
		slog.Error("Failed to delete old rule", "error", err)
		http.Error(w, "Failed to update rule", http.StatusInternalServerError)
		return
	}

	storageRule := alerter.RuleToStorage(rule)
	storageRule.ID = id // Preserve ID

	newID, err := h.db.CreateAlertRule(storageRule)
	if err != nil {
		slog.Error("Failed to create updated rule", "error", err)
		http.Error(w, "Failed to update rule", http.StatusInternalServerError)
		return
	}

	// Reload rule engine rules
	if err := h.ruleEngine.LoadRules(); err != nil {
		slog.Error("Failed to reload rules", "error", err)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      newID,
		"rule":    convertRuleToMap(rule),
	})
}

// DeleteRule deletes an alert rule
func (h *RuleEngineHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract ID from path
	path := r.URL.Path
	idStr := ""
	if len(path) > len("/api/alerts/rules/") {
		idStr = path[len("/api/alerts/rules/"):]
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteAlertRule(id); err != nil {
		slog.Error("Failed to delete alert rule", "error", err)
		http.Error(w, "Failed to delete alert rule", http.StatusInternalServerError)
		return
	}

	// Reload rule engine rules
	if err := h.ruleEngine.LoadRules(); err != nil {
		slog.Error("Failed to reload rules", "error", err)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Rule deleted successfully",
	})
}

// Helper functions to convert between storage and API formats

func convertAlertRuleToMap(rule storage.AlertRule) map[string]interface{} {
	condition := alerter.RuleCondition{}
	if rule.ConditionConfig != "" {
		json.Unmarshal([]byte(rule.ConditionConfig), &condition)
	}

	return map[string]interface{}{
		"id":               rule.ID,
		"app_id":           rule.AppID,
		"name":             rule.Name,
		"condition":        condition,
		"condition_type":   rule.ConditionType,
		"condition_config": rule.ConditionConfig,
		"severity":         "warning", // Default for legacy rules
		"cooldown_minutes": rule.CooldownMinutes,
		"enabled":          rule.Enabled == 1,
		"last_triggered_at": rule.LastTriggeredAt,
		"created_at":       rule.CreatedAt,
		"channels":         []string{},
	}
}

func convertRuleToMap(rule *alerter.Rule) map[string]interface{} {
	return map[string]interface{}{
		"id":               rule.ID,
		"app_id":           rule.AppID,
		"name":             rule.Name,
		"condition":        rule.Condition,
		"severity":         rule.Severity,
		"cooldown_minutes": rule.CooldownMinutes,
		"enabled":          rule.Enabled,
		"last_triggered_at": rule.LastTriggeredAt,
		"created_at":       rule.CreatedAt,
		"channels":         rule.Channels,
	}
}
