package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/logmonitor/collector/storage"
)

// EventStats represents event statistics
type EventStats struct {
	TotalCount int64
	ErrorCount int64
}

// EventQuery represents event query parameters
type EventQuery struct {
	AppID     string
	Type      string
	Level     string
	StartTime int64
	EndTime   int64
}

// EventResult represents event query results
type EventResult struct {
	Count  int64
	Events []storage.EventRecord
}

// EmailConfig represents email notification configuration
type EmailConfig struct {
	Enabled   bool
	SMTPHost  string
	SMTPPort  string
	SMTPUser  string
	SMTPPass  string
	FromEmail string
	FromName  string
}

// Notifier handles alert notifications
type Notifier interface {
	SendAlert(alert storage.AlertRule, context string) error
}

// ExtendedEventStore defines the interface for event operations in alert checker
type ExtendedEventStore interface {
	GetStats(appID string) (*EventStats, error)
	QueryEvents(query EventQuery) (*EventResult, error)
}

// AlertCheckerWorker handles alert checking and notification
type AlertCheckerWorker struct {
	alertStore    AlertStore
	eventStore    interface{} // Can be EventStore or ExtendedEventStore
	checkInterval time.Duration
	emailConfig   EmailConfig
	notifier      Notifier
	stopChan      chan struct{}
}

// NewAlertCheckerWorker creates a new alert checker worker
func NewAlertCheckerWorker(alertStore AlertStore, eventStore EventStore, checkInterval time.Duration) *AlertCheckerWorker {
	return &AlertCheckerWorker{
		alertStore:    alertStore,
		eventStore:    eventStore,
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
	}
}

// SetEmailConfig sets the email configuration
func (w *AlertCheckerWorker) SetEmailConfig(config EmailConfig) {
	w.emailConfig = config
}

// SetNotifier sets the notifier implementation
func (w *AlertCheckerWorker) SetNotifier(notifier Notifier) {
	w.notifier = notifier
}

// Start begins the alert checker worker
func (w *AlertCheckerWorker) Start(ctx context.Context) error {
	slog.Info("Starting alert checker worker", "interval", w.checkInterval)

	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	// Run once on startup
	w.checkAlerts()

	for {
		select {
		case <-ticker.C:
			w.checkAlerts()
		case <-ctx.Done():
			slog.Info("Alert checker worker context cancelled")
			return w.Stop()
		case <-w.stopChan:
			slog.Info("Alert checker worker stop signal received")
			return w.Stop()
		}
	}
}

// Stop stops the alert checker worker
func (w *AlertCheckerWorker) Stop() error {
	close(w.stopChan)
	slog.Info("Alert checker worker stopped")
	return nil
}

// Name returns the worker name
func (w *AlertCheckerWorker) Name() string {
	return "AlertCheckerWorker"
}

// checkAlerts checks all alert rules and triggers notifications
func (w *AlertCheckerWorker) checkAlerts() {
	slog.Debug("Checking alerts")

	rules, err := w.alertStore.GetAllAlertRules()
	if err != nil {
		slog.Error("Failed to get alert rules", "error", err)
		return
	}

	now := time.Now().UnixMilli()
	triggeredCount := 0

	for _, rule := range rules {
		if rule.Enabled == 0 || rule.SilencedUntil > now {
			continue
		}

		// Check cooldown
		if rule.LastTriggeredAt > 0 {
			cooldownEnd := rule.LastTriggeredAt + (int64(rule.CooldownMinutes) * 60 * 1000)
			if now < cooldownEnd {
				continue
			}
		}

		// Check if alert should trigger
		shouldTrigger, alertMessage := w.evaluateRule(rule)
		if shouldTrigger {
			if err := w.triggerAlert(rule, alertMessage); err != nil {
				slog.Error("Failed to trigger alert", "rule", rule.Name, "error", err)
			} else {
				triggeredCount++
			}
		}
	}

	slog.Debug("Alert check completed", "triggered", triggeredCount, "total", len(rules))
}

// evaluateRule evaluates if an alert rule should trigger
func (w *AlertCheckerWorker) evaluateRule(rule storage.AlertRule) (bool, string) {
	// Simplified alert evaluation logic
	// In real implementation, this would parse ConditionConfig and evaluate properly
	return false, ""
}

// triggerAlert triggers an alert notification
func (w *AlertCheckerWorker) triggerAlert(rule storage.AlertRule, message string) error {
	now := time.Now().UnixMilli()

	// Update last triggered time
	if err := w.alertStore.UpdateAlertRuleLastTriggered(rule.ID, now); err != nil {
		slog.Error("Failed to update last triggered time", "rule", rule.Name, "error", err)
	}

	// Create alert log
	alertLog := storage.AlertLog{
		RuleID:    rule.ID,
		AppID:     rule.AppID,
		Message:   message,
		CreatedAt: now,
	}

	if err := w.alertStore.CreateAlertLog(alertLog); err != nil {
		slog.Error("Failed to create alert log", "rule", rule.Name, "error", err)
	}

	// Send notification
	if w.notifier != nil {
		if err := w.notifier.SendAlert(rule, message); err != nil {
			slog.Error("Failed to send notification", "rule", rule.Name, "error", err)
		}
	}

	slog.Info("Alert triggered", "rule", rule.Name, "message", message)
	return nil
}