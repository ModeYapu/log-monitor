package alerter

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/logmonitor/collector/storage"
)

// RuleCondition represents the condition part of a rule
type RuleCondition struct {
	Type       string  // threshold, trend, missing
	Metric     string  // error_rate, page_load, event_count, etc.
	Operator   string  // >, <, >=, <=, ==
	Value      float64
	WindowMin  int     // Time window in minutes
	TrendCount int     // Trend rule: consecutive N times
	Page       string  // Page filter
	TrendDir   string  // up, down
}

// Rule represents an alert rule with extended condition support
type Rule struct {
	ID              int64
	Name            string
	AppID           string
	Condition       RuleCondition
	Severity        string // critical, warning, info
	CooldownMinutes int
	Channels        []string // notification channel IDs
	Enabled         bool
	LastTriggeredAt int64
	CreatedAt       int64
}

// RuleEngine evaluates rules against event data
type RuleEngine struct {
	db          storage.AlertStore
	events      storage.EventStore
	notifier    *Notifier
	channelMgr  *ChannelManager
	clusterer   *Clusterer
	rules       map[int64]*Rule
	mu          sync.RWMutex
	eventBuffer chan []storage.EventRecord
	stopCh      chan struct{}
}

// NewRuleEngine creates a new rule engine
func NewRuleEngine(db storage.AlertStore, events storage.EventStore) *RuleEngine {
	return &RuleEngine{
		db:          db,
		events:      events,
		notifier:    NewNotifier(),
		rules:       make(map[int64]*Rule),
		eventBuffer: make(chan []storage.EventRecord, 100),
		stopCh:      make(chan struct{}),
	}
}

// SetChannelManager sets the channel manager for notification
func (re *RuleEngine) SetChannelManager(cm *ChannelManager) {
	re.channelMgr = cm
}

// SetClusterer sets the clusterer for anomaly clustering
func (re *RuleEngine) SetClusterer(c *Clusterer) {
	re.clusterer = c
}

// Start begins the rule engine processing loop
func (re *RuleEngine) Start() {
	go re.processEvents()
}

// Stop stops the rule engine
func (re *RuleEngine) Stop() {
	close(re.stopCh)
}

// ProcessEvents submits events for rule evaluation
func (re *RuleEngine) ProcessEvents(events []storage.EventRecord) {
	select {
	case re.eventBuffer <- events:
	case <-re.stopCh:
		return
	default:
		slog.Warn("Rule engine event buffer full, dropping events", "count", len(events))
	}
}

// processEvents processes events from the buffer
func (re *RuleEngine) processEvents() {
	for {
		select {
		case events := <-re.eventBuffer:
			re.evaluateRules(events)
		case <-re.stopCh:
			return
		}
	}
}

// LoadRules loads all enabled rules from the database
func (re *RuleEngine) LoadRules() error {
	re.mu.Lock()
	defer re.mu.Unlock()

	rules, err := re.db.GetAllAlertRules()
	if err != nil {
		return fmt.Errorf("failed to load rules: %w", err)
	}

	// Clear existing rules
	re.rules = make(map[int64]*Rule)

	// Convert storage.AlertRule to Rule
	for _, ar := range rules {
		rule, err := re.StorageRuleToRule(ar)
		if err != nil {
			slog.Error("Failed to convert rule", "id", ar.ID, "error", err)
			continue
		}
		re.rules[rule.ID] = rule
	}

	slog.Info("Rule engine loaded rules", "count", len(re.rules))
	return nil
}

// StorageRuleToRule converts storage.AlertRule to Rule
func (re *RuleEngine) StorageRuleToRule(ar storage.AlertRule) (*Rule, error) {
	// Parse condition config
	var condition RuleCondition
	if ar.ConditionConfig != "" {
		if err := json.Unmarshal([]byte(ar.ConditionConfig), &condition); err != nil {
			// Try legacy format
			return re.parseLegacyRule(ar)
		}
	}

	// Determine severity from condition or default to warning
	severity := "warning"
	if condition.Metric == "error_rate" || condition.Type == "threshold" {
		severity = "critical"
	}

	return &Rule{
		ID:              ar.ID,
		Name:            ar.Name,
		AppID:           ar.AppID,
		Condition:       condition,
		Severity:        severity,
		CooldownMinutes: ar.CooldownMinutes,
		Channels:        []string{}, // Will be populated from notify_config
		Enabled:         ar.Enabled == 1,
		LastTriggeredAt: ar.LastTriggeredAt,
		CreatedAt:       ar.CreatedAt,
	}, nil
}

// parseLegacyRule handles legacy alert rule format
func (re *RuleEngine) parseLegacyRule(ar storage.AlertRule) (*Rule, error) {
	condition := RuleCondition{
		Type:      ar.ConditionType,
		WindowMin: 5,
		Value:     0,
		Operator:  ">",
	}

	// Parse legacy config for threshold/rate
	switch ar.ConditionType {
	case "threshold":
		var cfg struct {
			Count         int    `json:"count"`
			WindowMinutes int    `json:"windowMinutes"`
		}
		if err := json.Unmarshal([]byte(ar.ConditionConfig), &cfg); err == nil {
			condition.Metric = "event_count"
			condition.Value = float64(cfg.Count)
			condition.WindowMin = cfg.WindowMinutes
		}
	case "rate":
		var cfg struct {
			Rate          float64 `json:"rate"`
			WindowMinutes int     `json:"windowMinutes"`
		}
		if err := json.Unmarshal([]byte(ar.ConditionConfig), &cfg); err == nil {
			condition.Metric = "error_rate"
			condition.Value = cfg.Rate
			condition.WindowMin = cfg.WindowMinutes
		}
	}

	return &Rule{
		ID:              ar.ID,
		Name:            ar.Name,
		AppID:           ar.AppID,
		Condition:       condition,
		Severity:        "warning",
		CooldownMinutes: ar.CooldownMinutes,
		Channels:        []string{},
		Enabled:         ar.Enabled == 1,
		LastTriggeredAt: ar.LastTriggeredAt,
		CreatedAt:       ar.CreatedAt,
	}, nil
}

// evaluateRules evaluates all rules against the given events
func (re *RuleEngine) evaluateRules(events []storage.EventRecord) {
	if len(events) == 0 {
		return
	}

	re.mu.RLock()
	defer re.mu.RUnlock()

	now := time.Now().UnixMilli()

	for _, rule := range re.rules {
		if !rule.Enabled {
			continue
		}

		// Check cooldown
		if rule.LastTriggeredAt > 0 {
			cooldownEnd := rule.LastTriggeredAt + int64(rule.CooldownMinutes)*60*1000
			if now < cooldownEnd {
				continue
			}
		}

		// Filter events for this rule's app
		var appEvents []storage.EventRecord
		for _, e := range events {
			if e.AppID == rule.AppID {
				appEvents = append(appEvents, e)
			}
		}

		if len(appEvents) == 0 {
			continue
		}

		// Evaluate rule
		triggered, message := re.evaluateRule(rule, appEvents)
		if triggered {
			re.triggerRule(rule, message)
		}
	}
}

// evaluateRule evaluates a single rule against events
func (re *RuleEngine) evaluateRule(rule *Rule, events []storage.EventRecord) (bool, string) {
	switch rule.Condition.Type {
	case "threshold":
		return re.evaluateThreshold(rule, events)
	case "trend":
		return re.evaluateTrend(rule, events)
	case "missing":
		return re.evaluateMissing(rule)
	default:
		return false, ""
	}
}

// evaluateThreshold evaluates a threshold condition
func (re *RuleEngine) evaluateThreshold(rule *Rule, events []storage.EventRecord) (bool, string) {
	now := time.Now().UnixMilli()
	windowMin := rule.Condition.WindowMin
	if windowMin <= 0 {
		windowMin = 5
	}
	startTime := now - int64(windowMin)*60*1000

	var count int64
	var metricValue float64

	switch rule.Condition.Metric {
	case "event_count", "error_count":
		level := ""
		if rule.Condition.Metric == "error_count" {
			level = "error"
		}
		for _, e := range events {
			if e.CreatedAt >= startTime && (level == "" || e.Level == level) {
				count++
			}
		}
		metricValue = float64(count)

	case "error_rate":
		// Query total events in window
		totalQuery := storage.QueryParams{
			AppID:     rule.AppID,
			StartTime: startTime,
			Page:      1,
			PageSize:  1,
		}
		if rule.Condition.Page != "" {
			// Filter by URL (page)
			totalQuery.Keyword = rule.Condition.Page
		}
		totalResult, err := re.events.QueryEvents(totalQuery)
		if err != nil {
			return false, ""
		}

		// Count error events
		errorCount := int64(0)
		for _, e := range events {
			if e.CreatedAt >= startTime && e.Level == "error" {
				if rule.Condition.Page == "" || e.URL == rule.Condition.Page {
					errorCount++
				}
			}
		}

		if totalResult.Total > 0 {
			metricValue = (float64(errorCount) / float64(totalResult.Total)) * 100
		}

	default:
		// Generic metric count
		for _, e := range events {
			if e.CreatedAt >= startTime {
				count++
			}
		}
		metricValue = float64(count)
	}

	// Check threshold
	triggered := re.compareValues(metricValue, rule.Condition.Operator, rule.Condition.Value)
	if triggered {
		message := fmt.Sprintf("[%s] %s 规则触发: %s = %.2f (阈值: %.2f %s)",
			rule.AppID, rule.Name, rule.Condition.Metric, metricValue, rule.Condition.Value, rule.Condition.Operator)
		return true, message
	}

	return false, ""
}

// evaluateTrend evaluates a trend condition
func (re *RuleEngine) evaluateTrend(rule *Rule, events []storage.EventRecord) (bool, string) {
	now := time.Now().UnixMilli()
	windowMin := rule.Condition.WindowMin
	if windowMin <= 0 {
		windowMin = 5
	}

	trendCount := rule.Condition.TrendCount
	if trendCount <= 0 {
		trendCount = 3
	}

	// Collect data points for trend analysis
	dataPoints := re.collectDataPoints(rule, now, windowMin, trendCount)

	if len(dataPoints) < trendCount {
		return false, ""
	}

	// Check trend direction
	consecutiveCount := 0
	expectedDir := rule.Condition.TrendDir
	if expectedDir == "" {
		expectedDir = "up"
	}

	for i := 1; i < len(dataPoints); i++ {
		isTrend := false
		if expectedDir == "up" && dataPoints[i] > dataPoints[i-1] {
			isTrend = true
		} else if expectedDir == "down" && dataPoints[i] < dataPoints[i-1] {
			isTrend = true
		}

		if isTrend {
			consecutiveCount++
		} else {
			consecutiveCount = 0
		}

		if consecutiveCount >= trendCount-1 {
			metricValue := dataPoints[len(dataPoints)-1]
			message := fmt.Sprintf("[%s] %s 趋势规则触发: %s 连续 %d 次%s (当前值: %.2f)",
				rule.AppID, rule.Name, rule.Condition.Metric, trendCount, expectedDir, metricValue)
			return true, message
		}
	}

	return false, ""
}

// evaluateMissing evaluates a missing condition (no events for N minutes)
func (re *RuleEngine) evaluateMissing(rule *Rule) (bool, string) {
	now := time.Now().UnixMilli()
	windowMin := rule.Condition.WindowMin
	if windowMin <= 0 {
		windowMin = 5
	}
	startTime := now - int64(windowMin)*60*1000

	query := storage.QueryParams{
		AppID:     rule.AppID,
		StartTime: startTime,
		Page:      1,
		PageSize:  1,
	}

	if rule.Condition.Page != "" {
		// For page-level missing, we need to check if there are events for that specific page
		query.Keyword = rule.Condition.Page
	}

	result, err := re.events.QueryEvents(query)
	if err != nil {
		return false, ""
	}

	// Check if no events found
	triggered := result.Total == 0
	if triggered {
		message := fmt.Sprintf("[%s] %s 缺失规则触发: %s 在过去 %d 分钟内无事件",
			rule.AppID, rule.Name, rule.Condition.Metric, windowMin)
		return true, message
	}

	return false, ""
}

// collectDataPoints collects data points for trend analysis
func (re *RuleEngine) collectDataPoints(rule *Rule, now int64, windowMin int, count int) []float64 {
	dataPoints := make([]float64, 0, count)

	// Divide window into smaller buckets for trend analysis
	bucketSize := int64(windowMin * 60 * 1000 / count)

	for i := 0; i < count; i++ {
		bucketStart := now - int64((count-i)*windowMin*60*1000/count)
		bucketEnd := bucketStart + bucketSize

		query := storage.QueryParams{
			AppID:     rule.AppID,
			StartTime: bucketStart,
			EndTime:   bucketEnd,
			Page:      1,
			PageSize:  1,
		}

		if rule.Condition.Metric == "error_count" || rule.Condition.Metric == "error_rate" {
			query.Level = "error"
		}

		result, err := re.events.QueryEvents(query)
		if err != nil {
			continue
		}

		var value float64
		if rule.Condition.Metric == "error_rate" {
			// Need total for rate calculation
			totalQuery := storage.QueryParams{
				AppID:     rule.AppID,
				StartTime: bucketStart,
				EndTime:   bucketEnd,
				Page:      1,
				PageSize:  1,
			}
			totalResult, totalErr := re.events.QueryEvents(totalQuery)
			if totalErr != nil || totalResult.Total == 0 {
				value = 0
			} else {
				value = (float64(result.Total) / float64(totalResult.Total)) * 100
			}
		} else {
			value = float64(result.Total)
		}

		dataPoints = append(dataPoints, value)
	}

	return dataPoints
}

// compareValues compares two values based on operator
func (re *RuleEngine) compareValues(left float64, operator string, right float64) bool {
	switch operator {
	case ">":
		return left > right
	case ">=":
		return left >= right
	case "<":
		return left < right
	case "<=":
		return left <= right
	case "==":
		return left == right
	default:
		return left > right
	}
}

// triggerRule triggers a rule and sends notifications
func (re *RuleEngine) triggerRule(rule *Rule, message string) {
	slog.Error("Rule triggered", "rule", rule.Name, "message", message)

	// Update last triggered time
	now := time.Now().UnixMilli()
	rule.LastTriggeredAt = now
	re.db.UpdateAlertRuleLastTriggered(rule.ID, now)

	// Create alert log
	alertLog := storage.AlertLog{
		RuleID:    rule.ID,
		AppID:     rule.AppID,
		Message:   message,
		CreatedAt: now,
	}
	if err := re.db.CreateAlertLog(alertLog); err != nil {
		slog.Error("Failed to create alert log", "error", err)
	}

	// Send notifications through channel manager if available
	if re.channelMgr != nil && len(rule.Channels) > 0 {
		title := fmt.Sprintf("[%s] %s", rule.Severity, rule.Name)
		re.channelMgr.Send(rule.Channels, rule.Severity, title, message)
	}
}

// RuleToStorage converts Rule to storage.AlertRule
func RuleToStorage(rule *Rule) storage.AlertRule {
	conditionConfig, _ := json.Marshal(rule.Condition)

	return storage.AlertRule{
		ID:              rule.ID,
		AppID:           rule.AppID,
		Name:            rule.Name,
		ConditionType:   rule.Condition.Type,
		ConditionConfig: string(conditionConfig),
		NotifyType:      "channel", // Using channel manager
		NotifyConfig:    "", // Channels stored separately
		Enabled:         boolToInt(rule.Enabled),
		CooldownMinutes: rule.CooldownMinutes,
		LastTriggeredAt: rule.LastTriggeredAt,
		CreatedAt:       rule.CreatedAt,
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
