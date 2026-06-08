package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/logmonitor/collector/alerter"
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
	mux.HandleFunc("POST /api/alerts/test", h.TestAlert)
	mux.HandleFunc("POST /api/alerts/silence", h.SilenceAlert)
	mux.HandleFunc("POST /api/alerts/unsilence", h.UnsilenceAlert)
}

// GetAlerts returns alert rules and logs for an app
func (h *AlertsHandler) GetAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	rules, err := h.db.GetAlertRules(appID)
	if err != nil {
		slog.Error("Failed to get alert rules: %v", err)
		http.Error(w, "Failed to get alert rules", http.StatusInternalServerError)
		return
	}

	logs, err := h.db.GetAlertLogs(appID, 100)
	if err != nil {
		slog.Error("Failed to get alert logs: %v", err)
	}

	response := map[string]interface{}{
		"rules": rules,
		"logs":  logs,
	}

	json.NewEncoder(w).Encode(response)
}

// CreateAlertRequest represents the request to create/update an alert rule
type CreateAlertRequest struct {
	AppID            string          `json:"app_id"`
	Name             string          `json:"name"`
	ConditionType    string          `json:"condition_type"`   // threshold|rate|new_error
	ConditionConfig  json.RawMessage `json:"condition_config"` // JSON string
	NotifyType       string          `json:"notify_type"`      // webhook|feishu|email
	NotifyConfig     json.RawMessage `json:"notify_config"`    // JSON string
	Enabled          bool            `json:"enabled"`
	CooldownMinutes  int             `json:"cooldown_minutes"`
	MessageTemplate  string          `json:"message_template"`  // Optional message template
}

// CreateAlert creates a new alert rule
func (h *AlertsHandler) CreateAlert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

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
		AppID:            req.AppID,
		Name:             req.Name,
		ConditionType:    req.ConditionType,
		ConditionConfig:  conditionConfig,
		NotifyType:       req.NotifyType,
		NotifyConfig:     notifyConfig,
		Enabled:          boolToInt(req.Enabled),
		CooldownMinutes:  req.CooldownMinutes,
		MessageTemplate:  req.MessageTemplate,
		CreatedAt:        time.Now().UnixMilli(),
	}

	id, err := h.db.CreateAlertRule(rule)
	if err != nil {
		slog.Error("Failed to create alert rule: %v", err)
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
	w.Header().Set("Content-Type", "application/json")

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
		slog.Error("Failed to delete alert rule: %v", err)
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

// TestAlertRequest represents the request to test an alert notification
type TestAlertRequest struct {
	NotifyType   string          `json:"notify_type"`   // webhook|feishu|email|wecom|dingtalk|telegram
	NotifyConfig json.RawMessage `json:"notify_config"` // JSON string
	Message      string          `json:"message"`       // Test message
}

// TestAlert tests an alert notification
func (h *AlertsHandler) TestAlert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TestAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		req.Message = "这是一条测试告警消息"
	}

	var notifyConfig map[string]interface{}
	if err := json.Unmarshal(req.NotifyConfig, &notifyConfig); err != nil {
		slog.Error("Failed to parse notify config: %v", err)
		http.Error(w, "Invalid notify config", http.StatusBadRequest)
		return
	}

	notifier := alerter.NewNotifier()
	title := "测试告警规则"
	hasError := false

	switch req.NotifyType {
	case "feishu":
		webhookURL, _ := notifyConfig["url"].(string)
		if err := notifier.SendFeishu(webhookURL, title, req.Message); err != nil {
			slog.Error("Failed to send Feishu notification: %v", err)
			hasError = true
		}
	case "wecom":
		webhookURL, _ := notifyConfig["url"].(string)
		if err := notifier.SendWeCom(webhookURL, title, req.Message); err != nil {
			slog.Error("Failed to send WeCom notification: %v", err)
			hasError = true
		}
	case "dingtalk":
		webhookURL, _ := notifyConfig["url"].(string)
		if err := notifier.SendDingTalk(webhookURL, title, req.Message); err != nil {
			slog.Error("Failed to send DingTalk notification: %v", err)
			hasError = true
		}
	case "telegram":
		botToken, _ := notifyConfig["bot_token"].(string)
		chatID, _ := notifyConfig["chat_id"].(string)
		if err := notifier.SendTelegram(botToken, chatID, req.Message); err != nil {
			slog.Error("Failed to send Telegram notification: %v", err)
			hasError = true
		}
	case "webhook":
		webhookURL, _ := notifyConfig["url"].(string)
		if err := notifier.SendWebhook(webhookURL, title, req.Message); err != nil {
			slog.Error("Failed to send Webhook notification: %v", err)
			hasError = true
		}
	case "email":
		email, _ := notifyConfig["email"].(string)
		if err := notifier.SendEmail(email, title, req.Message); err != nil {
			slog.Error("Failed to send Email notification: %v", err)
			hasError = true
		}
	default:
		http.Error(w, "Unknown notify type", http.StatusBadRequest)
		return
	}

	if hasError {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to send notification",
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Test notification sent successfully",
		})
	}
}

// SilenceAlertRequest represents the request to silence an alert
type SilenceAlertRequest struct {
	ID             int64  `json:"id"`
	DurationMinutes int    `json:"durationMinutes"`
}

// SilenceAlert silences an alert for a specified duration
func (h *AlertsHandler) SilenceAlert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SilenceAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ID <= 0 {
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	// Default to 1 hour if not specified
	duration := req.DurationMinutes
	if duration <= 0 {
		duration = 60
	}

	silencedUntil := time.Now().Add(time.Duration(duration) * time.Minute).UnixMilli()

	if err := h.db.SilenceAlertRule(req.ID, silencedUntil); err != nil {
		slog.Error("Failed to silence alert: %v", err)
		http.Error(w, "Failed to silence alert", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"silencedUntil":  silencedUntil,
		"message":       fmt.Sprintf("Alert silenced for %d minutes", duration),
	})
}

// UnsilenceAlertRequest represents the request to unsilence an alert
type UnsilenceAlertRequest struct {
	ID int64 `json:"id"`
}

// UnsilenceAlert unsilences an alert
func (h *AlertsHandler) UnsilenceAlert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UnsilenceAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ID <= 0 {
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if err := h.db.UnsilenceAlertRule(req.ID); err != nil {
		slog.Error("Failed to unsilence alert: %v", err)
		http.Error(w, "Failed to unsilence alert", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Alert unsilenced successfully",
	})
}
