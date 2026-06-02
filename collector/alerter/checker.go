package alerter

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/logmonitor/collector/storage"
)

// Checker checks alert rules and triggers notifications
type Checker struct {
	db       *storage.DB
	notifier *Notifier
	stopCh   chan struct{}
}

// NewChecker creates a new alert checker
func NewChecker(db *storage.DB) *Checker {
	return &Checker{
		db:       db,
		notifier: NewNotifier(),
		stopCh:   make(chan struct{}),
	}
}

// Start begins the alert checking loop
func (c *Checker) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once on startup
	c.checkRules()

	for {
		select {
		case <-ticker.C:
			c.checkRules()
		case <-c.stopCh:
			return
		}
	}
}

// Stop stops the alert checker
func (c *Checker) Stop() {
	close(c.stopCh)
}

// checkRules checks all enabled alert rules
func (c *Checker) checkRules() {
	rules, err := c.db.GetAllAlertRules()
	if err != nil {
		log.Printf("Failed to get alert rules: %v", err)
		return
	}

	now := time.Now().UnixMilli()

	for _, rule := range rules {
		// Check cooldown
		if rule.LastTriggeredAt > 0 {
			cooldownEnd := rule.LastTriggeredAt + int64(rule.CooldownMinutes)*60*1000
			if now < cooldownEnd {
				continue
			}
		}

		triggered, message := c.checkRule(rule)
		if triggered {
			c.triggerAlert(rule, message)

			// Update last triggered time
			c.db.UpdateAlertRuleLastTriggered(rule.ID, now)
		}
	}
}

// checkRule checks a single alert rule
func (c *Checker) checkRule(rule storage.AlertRule) (bool, string) {
	switch rule.ConditionType {
	case "threshold":
		return c.checkThreshold(rule)
	case "rate":
		return c.checkRate(rule)
	case "new_error":
		return c.checkNewError(rule)
	default:
		log.Printf("Unknown condition type: %s", rule.ConditionType)
		return false, ""
	}
}

// checkThreshold checks if error count exceeds threshold
func (c *Checker) checkThreshold(rule storage.AlertRule) (bool, string) {
	var config struct {
		Level          string `json:"level"`
		Count          int    `json:"count"`
		WindowMinutes  int    `json:"windowMinutes"`
		AggregateBy    string `json:"aggregateBy"`    // none|release|page|browser
		FilterRelease  string `json:"filterRelease"`  // optional: only check specific release
		FilterPage     string `json:"filterPage"`     // optional: only check specific page
	}

	if err := json.Unmarshal([]byte(rule.ConditionConfig), &config); err != nil {
		log.Printf("Failed to parse threshold config: %v", err)
		return false, ""
	}

	if config.WindowMinutes <= 0 {
		config.WindowMinutes = 5
	}
	if config.AggregateBy == "" {
		config.AggregateBy = "none"
	}

	startTime := time.Now().Add(-time.Duration(config.WindowMinutes) * time.Minute).UnixMilli()

	// Check if aggregation is requested
	if config.AggregateBy != "none" {
		return c.checkAggregatedThreshold(rule, config, startTime)
	}

	// Simple threshold check
	query := storage.QueryParams{
		AppID:     rule.AppID,
		Release:   config.FilterRelease,
		Level:     config.Level,
		StartTime: startTime,
		Page:      1,
		PageSize:  1,
	}

	result, err := c.db.QueryEvents(query)
	if err != nil {
		log.Printf("Failed to query events: %v", err)
		return false, ""
	}

	triggered := result.Total >= int64(config.Count)
	if triggered {
		message := fmt.Sprintf("[%s] %s 级别日志在过去 %d 分钟内达到 %d 次（阈值：%d）",
			rule.AppID, config.Level, config.WindowMinutes, result.Total, config.Count)
		return true, message
	}

	return false, ""
}

// checkAggregatedThreshold checks threshold with aggregation
func (c *Checker) checkAggregatedThreshold(rule storage.AlertRule, config interface{}, startTime int64) (bool, string) {
	// For now, implement simple aggregation
	// TODO: Implement proper aggregation queries
	cfg := config.(struct {
		Level         string `json:"level"`
		Count         int    `json:"count"`
		WindowMinutes int    `json:"windowMinutes"`
		AggregateBy   string `json:"aggregateBy"`
		FilterRelease string `json:"filterRelease"`
		FilterPage    string `json:"filterPage"`
	})

	// Query events
	query := storage.QueryParams{
		AppID:     rule.AppID,
		Release:   cfg.FilterRelease,
		Level:     cfg.Level,
		StartTime: startTime,
		Page:      1,
		PageSize:  1,
	}

	result, err := c.db.QueryEvents(query)
	if err != nil {
		log.Printf("Failed to query events: %v", err)
		return false, ""
	}

	triggered := result.Total >= int64(cfg.Count)
	if triggered {
		aggregateInfo := ""
		if cfg.FilterRelease != "" {
			aggregateInfo = fmt.Sprintf(" (Release: %s)", cfg.FilterRelease)
		}
		message := fmt.Sprintf("[%s] %s 级别日志在过去 %d 分钟内达到 %d 次（阈值：%d）%s",
			rule.AppID, cfg.Level, cfg.WindowMinutes, result.Total, cfg.Count, aggregateInfo)
		return true, message
	}

	return false, ""
}

// checkRate checks if error rate exceeds threshold
func (c *Checker) checkRate(rule storage.AlertRule) (bool, string) {
	var config struct {
		Rate          float64 `json:"rate"`
		MinSamples    int     `json:"minSamples"`
		WindowMinutes int     `json:"windowMinutes"`
		AggregateBy   string  `json:"aggregateBy"`   // none|release|page|browser
		FilterRelease string  `json:"filterRelease"` // optional: only check specific release
		FilterPage    string  `json:"filterPage"`    // optional: only check specific page
	}

	if err := json.Unmarshal([]byte(rule.ConditionConfig), &config); err != nil {
		log.Printf("Failed to parse rate config: %v", err)
		return false, ""
	}

	if config.WindowMinutes <= 0 {
		config.WindowMinutes = 5
	}
	if config.MinSamples <= 0 {
		config.MinSamples = 100
	}
	if config.AggregateBy == "" {
		config.AggregateBy = "none"
	}

	startTime := time.Now().Add(-time.Duration(config.WindowMinutes) * time.Minute).UnixMilli()

	// Get total events
	totalQuery := storage.QueryParams{
		AppID:     rule.AppID,
		Release:   config.FilterRelease,
		StartTime: startTime,
		Page:      1,
		PageSize:  1,
	}
	totalResult, err := c.db.QueryEvents(totalQuery)
	if err != nil {
		log.Printf("Failed to query total events: %v", err)
		return false, ""
	}

	if totalResult.Total < int64(config.MinSamples) {
		return false, ""
	}

	// Get error events
	errorQuery := storage.QueryParams{
		AppID:     rule.AppID,
		Release:   config.FilterRelease,
		Level:     "error",
		StartTime: startTime,
		Page:      1,
		PageSize:  1,
	}
	errorResult, err := c.db.QueryEvents(errorQuery)
	if err != nil {
		log.Printf("Failed to query error events: %v", err)
		return false, ""
	}

	rate := float64(errorResult.Total) / float64(totalResult.Total) * 100
	triggered := rate >= config.Rate

	if triggered {
		aggregateInfo := ""
		if config.FilterRelease != "" {
			aggregateInfo = fmt.Sprintf(" (Release: %s)", config.FilterRelease)
		}
		message := fmt.Sprintf("[%s] 错误率在过去 %d 分钟内达到 %.2f%%（阈值：%.2f%%），总样本：%d%s",
			rule.AppID, config.WindowMinutes, rate, config.Rate, totalResult.Total, aggregateInfo)
		return true, message
	}

	return false, ""
}

// checkNewError checks for new error messages
func (c *Checker) checkNewError(rule storage.AlertRule) (bool, string) {
	// Get recent errors
	startTime := time.Now().Add(-24 * time.Hour).UnixMilli()

	query := storage.QueryParams{
		AppID:     rule.AppID,
		Level:     "error",
		StartTime: startTime,
		Page:      1,
		PageSize:  1000,
	}

	result, err := c.db.QueryEvents(query)
	if err != nil {
		log.Printf("Failed to query events: %v", err)
		return false, ""
	}

	// Check if there are any new error messages
	// For simplicity, we trigger if we have any errors in the last hour
	recentStartTime := time.Now().Add(-time.Hour).UnixMilli()

	for _, event := range result.Data {
		if event.CreatedAt > recentStartTime {
			message := fmt.Sprintf("[%s] 检测到新错误: %s", rule.AppID, truncateMessage(event.Message, 100))
			return true, message
		}
	}

	return false, ""
}

// triggerAlert sends a notification for a triggered alert
func (c *Checker) triggerAlert(rule storage.AlertRule, message string) {
	log.Printf("Alert triggered: %s - %s", rule.Name, message)

	// Create alert log
	alertLog := storage.AlertLog{
		RuleID:    rule.ID,
		AppID:     rule.AppID,
		Message:   message,
		CreatedAt: time.Now().UnixMilli(),
	}

	if err := c.db.CreateAlertLog(alertLog); err != nil {
		log.Printf("Failed to create alert log: %v", err)
	}

	// Send notification
	var notifyConfig map[string]interface{}
	if err := json.Unmarshal([]byte(rule.NotifyConfig), &notifyConfig); err != nil {
		log.Printf("Failed to parse notify config: %v", err)
		return
	}

	switch rule.NotifyType {
	case "feishu":
		webhookURL, _ := notifyConfig["url"].(string)
		c.notifier.SendFeishu(webhookURL, rule.Name, message)
	case "wecom":
		webhookURL, _ := notifyConfig["url"].(string)
		c.notifier.SendWeCom(webhookURL, rule.Name, message)
	case "dingtalk":
		webhookURL, _ := notifyConfig["url"].(string)
		c.notifier.SendDingTalk(webhookURL, rule.Name, message)
	case "telegram":
		botToken, _ := notifyConfig["bot_token"].(string)
		chatID, _ := notifyConfig["chat_id"].(string)
		c.notifier.SendTelegram(botToken, chatID, fmt.Sprintf("%s\n%s", rule.Name, message))
	case "webhook":
		webhookURL, _ := notifyConfig["url"].(string)
		c.notifier.SendWebhook(webhookURL, rule.Name, message)
	case "email":
		email, _ := notifyConfig["email"].(string)
		c.notifier.SendEmail(email, rule.Name, message)
	}
}

func truncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen] + "..."
}
