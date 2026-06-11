package storage

import (
	"fmt"
)

// AlertRule represents an alert rule
type AlertRule struct {
	ID              int64
	AppID           string
	Name            string
	ConditionType   string
	ConditionConfig string
	NotifyType      string
	NotifyConfig    string
	Enabled         int
	LastTriggeredAt int64
	CooldownMinutes int
	SilencedUntil   int64
	Fingerprint     string
	MessageTemplate string
	CreatedAt       int64
}

// AlertLog represents an alert log entry
type AlertLog struct {
	ID        int64
	RuleID    int64
	AppID     string
	Message   string
	CreatedAt int64
}

// CreateAlertRule creates a new alert rule
func (db *DB) CreateAlertRule(rule AlertRule) (int64, error) {

	if db.closed.Load() {
		return 0, fmt.Errorf("database is closed")
	}

	result, err := db.conn.Exec(`
		INSERT INTO alert_rules (app_id, name, condition_type, condition_config, notify_type, notify_config, enabled, cooldown_minutes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rule.AppID, rule.Name, rule.ConditionType, rule.ConditionConfig, rule.NotifyType, rule.NotifyConfig, rule.Enabled, rule.CooldownMinutes, rule.CreatedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to create alert rule: %w", err)
	}

	return result.LastInsertId()
}

// GetAlertRules retrieves alert rules for an app
func (db *DB) GetAlertRules(appID string) ([]AlertRule, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	rows, err := db.conn.Query(`
		SELECT id, app_id, name, condition_type, condition_config, notify_type, notify_config, enabled, last_triggered_at, cooldown_minutes, silenced_until, fingerprint, created_at
		FROM alert_rules
		WHERE app_id = ?
		ORDER BY created_at DESC
	`, appID)

	if err != nil {
		return nil, fmt.Errorf("failed to get alert rules: %w", err)
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		var rule AlertRule
		err := rows.Scan(
			&rule.ID, &rule.AppID, &rule.Name, &rule.ConditionType, &rule.ConditionConfig,
			&rule.NotifyType, &rule.NotifyConfig, &rule.Enabled, &rule.LastTriggeredAt,
			&rule.CooldownMinutes, &rule.SilencedUntil, &rule.Fingerprint, &rule.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// GetAllAlertRules retrieves all enabled alert rules
func (db *DB) GetAllAlertRules() ([]AlertRule, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	rows, err := db.conn.Query(`
		SELECT id, app_id, name, condition_type, condition_config, notify_type, notify_config, enabled, last_triggered_at, cooldown_minutes, silenced_until, fingerprint, created_at
		FROM alert_rules
		WHERE enabled = 1
		ORDER BY created_at DESC
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to get all alert rules: %w", err)
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		var rule AlertRule
		err := rows.Scan(
			&rule.ID, &rule.AppID, &rule.Name, &rule.ConditionType, &rule.ConditionConfig,
			&rule.NotifyType, &rule.NotifyConfig, &rule.Enabled, &rule.LastTriggeredAt,
			&rule.CooldownMinutes, &rule.SilencedUntil, &rule.Fingerprint, &rule.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// UpdateAlertRuleLastTriggered updates the last triggered timestamp
func (db *DB) UpdateAlertRuleLastTriggered(id int64, timestamp int64) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		UPDATE alert_rules SET last_triggered_at = ? WHERE id = ?
	`, timestamp, id)

	if err != nil {
		return fmt.Errorf("failed to update alert rule: %w", err)
	}

	return nil
}

// DeleteAlertRule deletes an alert rule
func (db *DB) DeleteAlertRule(id int64) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec("DELETE FROM alert_rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete alert rule: %w", err)
	}

	return nil
}

// SilenceAlertRule silences an alert rule until a specified time
func (db *DB) SilenceAlertRule(id int64, until int64) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		UPDATE alert_rules SET silenced_until = ? WHERE id = ?
	`, until, id)

	if err != nil {
		return fmt.Errorf("failed to silence alert rule: %w", err)
	}

	return nil
}

// UnsilenceAlertRule unsilences an alert rule
func (db *DB) UnsilenceAlertRule(id int64) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec("UPDATE alert_rules SET silenced_until = 0 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to unsilence alert rule: %w", err)
	}

	return nil
}

// CreateAlertLog creates an alert log entry
func (db *DB) CreateAlertLog(log AlertLog) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		INSERT INTO alert_logs (rule_id, app_id, message, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, log.RuleID, log.AppID, log.Message, log.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create alert log: %w", err)
	}

	return nil
}

// GetAlertLogs retrieves alert logs for an app
func (db *DB) GetAlertLogs(appID string, limit int) ([]AlertLog, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	rows, err := db.conn.Query(`
		SELECT id, rule_id, app_id, message, created_at
		FROM alert_logs
		WHERE app_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, appID, limit)

	if err != nil {
		return nil, fmt.Errorf("failed to get alert logs: %w", err)
	}
	defer rows.Close()

	var logs []AlertLog
	for rows.Next() {
		var log AlertLog
		err := rows.Scan(&log.ID, &log.RuleID, &log.AppID, &log.Message, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}