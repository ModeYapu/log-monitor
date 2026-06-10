package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Webhook represents a webhook configuration
type Webhook struct {
	ID             int64       `json:"id"`
	ProjectID      int64       `json:"project_id"`
	Name           string      `json:"name"`
	URL            string      `json:"url"`
	Secret         string      `json:"secret"`
	Events         []string    `json:"events"`
	Enabled        bool        `json:"enabled"`
	LastTriggeredAt int64      `json:"last_triggered_at"`
	FailureCount   int         `json:"failure_count"`
	CreatedAt      int64       `json:"created_at"`
	UpdatedAt      int64       `json:"updated_at"`
}

// WebhookStore defines the interface for webhook storage operations
type WebhookStore interface {
	GetWebhooks(projectID int64) ([]Webhook, error)
	GetWebhook(id int64) (*Webhook, error)
	CreateWebhook(webhook *Webhook) error
	UpdateWebhook(webhook *Webhook) error
	DeleteWebhook(id int64) error
	UpdateWebhookTriggerInfo(id int64, lastTriggeredAt int64, failureCount int) error
}

// GetWebhooks retrieves all webhooks for a project
func (db *DB) GetWebhooks(projectID int64) ([]Webhook, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	var rows *sql.Rows
	var err error

	if projectID == 0 {
		// Get all webhooks
		rows, err = db.conn.Query(`
			SELECT id, project_id, name, url, secret, events, enabled, last_triggered_at, failure_count, created_at, updated_at
			FROM webhooks
			ORDER BY created_at DESC
		`)
	} else {
		// Get webhooks for specific project
		rows, err = db.conn.Query(`
			SELECT id, project_id, name, url, secret, events, enabled, last_triggered_at, failure_count, created_at, updated_at
			FROM webhooks
			WHERE project_id = ? OR project_id = 0
			ORDER BY created_at DESC
		`, projectID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var webhook Webhook
		var eventsJSON string

		err := rows.Scan(
			&webhook.ID,
			&webhook.ProjectID,
			&webhook.Name,
			&webhook.URL,
			&webhook.Secret,
			&eventsJSON,
			&webhook.Enabled,
			&webhook.LastTriggeredAt,
			&webhook.FailureCount,
			&webhook.CreatedAt,
			&webhook.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook: %w", err)
		}

		// Parse events JSON
		if err := json.Unmarshal([]byte(eventsJSON), &webhook.Events); err != nil {
			slog.Error("Failed to parse webhook events", "webhook_id", webhook.ID, "error", err)
			webhook.Events = []string{}
		}

		webhooks = append(webhooks, webhook)
	}

	return webhooks, nil
}

// GetWebhook retrieves a single webhook by ID
func (db *DB) GetWebhook(id int64) (*Webhook, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	var webhook Webhook
	var eventsJSON string

	err := db.conn.QueryRow(`
		SELECT id, project_id, name, url, secret, events, enabled, last_triggered_at, failure_count, created_at, updated_at
		FROM webhooks
		WHERE id = ?
	`, id).Scan(
		&webhook.ID,
		&webhook.ProjectID,
		&webhook.Name,
		&webhook.URL,
		&webhook.Secret,
		&eventsJSON,
		&webhook.Enabled,
		&webhook.LastTriggeredAt,
		&webhook.FailureCount,
		&webhook.CreatedAt,
		&webhook.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("webhook not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook: %w", err)
	}

	// Parse events JSON
	if err := json.Unmarshal([]byte(eventsJSON), &webhook.Events); err != nil {
		slog.Error("Failed to parse webhook events", "webhook_id", webhook.ID, "error", err)
		webhook.Events = []string{}
	}

	return &webhook, nil
}

// CreateWebhook creates a new webhook
func (db *DB) CreateWebhook(webhook *Webhook) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	webhook.CreatedAt = time.Now().Unix()
	webhook.UpdatedAt = time.Now().Unix()
	webhook.LastTriggeredAt = 0
	webhook.FailureCount = 0

	// Marshal events to JSON
	eventsJSON, err := json.Marshal(webhook.Events)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	result, err := db.conn.Exec(`
		INSERT INTO webhooks (project_id, name, url, secret, events, enabled, last_triggered_at, failure_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, webhook.ProjectID, webhook.Name, webhook.URL, webhook.Secret, eventsJSON, webhook.Enabled, webhook.LastTriggeredAt, webhook.FailureCount, webhook.CreatedAt, webhook.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	webhook.ID = id
	return nil
}

// UpdateWebhook updates an existing webhook
func (db *DB) UpdateWebhook(webhook *Webhook) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	webhook.UpdatedAt = time.Now().Unix()

	// Marshal events to JSON
	eventsJSON, err := json.Marshal(webhook.Events)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	_, err = db.conn.Exec(`
		UPDATE webhooks
		SET project_id = ?, name = ?, url = ?, secret = ?, events = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, webhook.ProjectID, webhook.Name, webhook.URL, webhook.Secret, eventsJSON, webhook.Enabled, webhook.UpdatedAt, webhook.ID)

	if err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

// DeleteWebhook deletes a webhook
func (db *DB) DeleteWebhook(id int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	return nil
}

// UpdateWebhookTriggerInfo updates the last triggered time and failure count for a webhook
func (db *DB) UpdateWebhookTriggerInfo(id int64, lastTriggeredAt int64, failureCount int) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		UPDATE webhooks
		SET last_triggered_at = ?, failure_count = ?, updated_at = ?
		WHERE id = ?
	`, lastTriggeredAt, failureCount, time.Now().Unix(), id)

	if err != nil {
		return fmt.Errorf("failed to update webhook trigger info: %w", err)
	}

	return nil
}