package webhook

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/logmonitor/collector/storage"
)

// EventType represents the type of event that can trigger webhooks
type EventType string

const (
	EventIssueCreated   EventType = "issue.created"
	EventIssueResolved  EventType = "issue.resolved"
	EventIssueReopened  EventType = "issue.reopened"
	EventAlertTriggered EventType = "alert.triggered"
)

// Payload represents a webhook payload
type Payload struct {
	Event     string      `json:"event"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
	ProjectID int64       `json:"project_id"`
}

// Webhook represents a webhook configuration
type Webhook struct {
	ID              int64       `json:"id"`
	ProjectID       int64       `json:"project_id"`
	Name            string      `json:"name"`
	URL             string      `json:"url"`
	Secret          string      `json:"secret"`
	Events          []EventType `json:"events"`
	Enabled         bool        `json:"enabled"`
	LastTriggeredAt int64       `json:"last_triggered_at"`
	FailureCount    int         `json:"failure_count"`
	CreatedAt       int64       `json:"created_at"`
	UpdatedAt       int64       `json:"updated_at"`
}

// WebhookStore defines the interface for webhook storage
type WebhookStore interface {
	GetWebhooks(projectID int64) ([]storage.Webhook, error)
	GetWebhook(id int64) (*storage.Webhook, error)
	CreateWebhook(webhook *storage.Webhook) error
	UpdateWebhook(webhook *storage.Webhook) error
	DeleteWebhook(id int64) error
	UpdateWebhookTriggerInfo(id int64, lastTriggeredAt int64, failureCount int) error
}

// Manager manages webhook delivery
type Manager struct {
	store      WebhookStore
	deliverer  *Deliverer
	mu         sync.RWMutex
	bufferSize int
	buffer     []Payload
	flushTimer *time.Timer
	stopCh     chan struct{}
}

// ManagerConfig holds configuration for the webhook manager
type ManagerConfig struct {
	BufferSize    int           // Number of payloads to buffer before flushing
	FlushInterval time.Duration // Maximum time to wait before flushing buffer
}

// NewManager creates a new webhook manager
func NewManager(store WebhookStore, config ManagerConfig) *Manager {
	if config.BufferSize <= 0 {
		config.BufferSize = 100
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 5 * time.Second
	}

	m := &Manager{
		store:      store,
		deliverer:  NewDeliverer(),
		bufferSize: config.BufferSize,
		buffer:     make([]Payload, 0, config.BufferSize),
		stopCh:     make(chan struct{}),
	}

	m.flushTimer = time.AfterFunc(config.FlushInterval, func() {
		m.flushBuffer()
	})

	return m
}

// Trigger sends a webhook event
func (m *Manager) Trigger(eventType EventType, data interface{}, projectID int64) {
	payload := Payload{
		Event:     string(eventType),
		Timestamp: time.Now().Unix(),
		Data:      data,
		ProjectID: projectID,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.buffer = append(m.buffer, payload)

	if len(m.buffer) >= m.bufferSize {
		m.flushBufferLocked()
	}
}

// flushBuffer flushes the current buffer of payloads
func (m *Manager) flushBuffer() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushBufferLocked()
}

// flushBufferLocked flushes the buffer (must be called with lock held)
func (m *Manager) flushBufferLocked() {
	if len(m.buffer) == 0 {
		return
	}

	payloads := make([]Payload, len(m.buffer))
	copy(payloads, m.buffer)
	m.buffer = m.buffer[:0]

	// Reset flush timer
	m.flushTimer.Reset(5 * time.Second)

	// Deliver payloads asynchronously
	go m.deliverPayloads(payloads)
}

// deliverPayloads delivers payloads to matching webhooks
func (m *Manager) deliverPayloads(payloads []Payload) {
	for _, payload := range payloads {
		// Get webhooks for this project (including global webhooks with project_id = 0)
		webhooks, err := m.store.GetWebhooks(0) // Get all webhooks
		if err != nil {
			slog.Error("Failed to get webhooks", "error", err)
			continue
		}

		for _, webhook := range webhooks {
			if !webhook.Enabled {
				continue
			}

			// Check if webhook matches project
			if webhook.ProjectID != 0 && webhook.ProjectID != payload.ProjectID {
				continue
			}

			// Check if webhook is interested in this event type
			eventType := EventType(payload.Event)
			if !m.webhookSubscribesTo(webhook, eventType) {
				continue
			}

			// Deliver the webhook
			go m.deliverer.Deliver(webhook, payload)
		}
	}
}

// webhookSubscribesTo checks if a webhook subscribes to a specific event type
func (m *Manager) webhookSubscribesTo(webhook storage.Webhook, eventType EventType) bool {
	for _, e := range webhook.Events {
		if EventType(e) == eventType {
			return true
		}
	}
	return false
}

// Stop stops the webhook manager
func (m *Manager) Stop() {
	close(m.stopCh)
	m.flushTimer.Stop()
	m.flushBuffer()
}

// GetWebhooks returns all webhooks for a project
func (m *Manager) GetWebhooks(projectID int64) ([]storage.Webhook, error) {
	return m.store.GetWebhooks(projectID)
}

// GetWebhook returns a single webhook by ID
func (m *Manager) GetWebhook(id int64) (*storage.Webhook, error) {
	return m.store.GetWebhook(id)
}

// CreateWebhook creates a new webhook
func (m *Manager) CreateWebhook(webhook *storage.Webhook) error {
	webhook.CreatedAt = time.Now().Unix()
	webhook.UpdatedAt = time.Now().Unix()
	return m.store.CreateWebhook(webhook)
}

// UpdateWebhook updates an existing webhook
func (m *Manager) UpdateWebhook(webhook *storage.Webhook) error {
	webhook.UpdatedAt = time.Now().Unix()
	return m.store.UpdateWebhook(webhook)
}

// DeleteWebhook deletes a webhook
func (m *Manager) DeleteWebhook(id int64) error {
	return m.store.DeleteWebhook(id)
}

// TestWebhook sends a test payload to a webhook
func (m *Manager) TestWebhook(id int64) error {
	webhook, err := m.store.GetWebhook(id)
	if err != nil {
		return err
	}

	testPayload := Payload{
		Event:     "test",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"message": "This is a test webhook payload",
			"test":    true,
		},
		ProjectID: webhook.ProjectID,
	}

	return m.deliverer.DeliverSync(*webhook, testPayload)
}

// MarshalJSON marshals the webhook events to JSON
func (w *Webhook) MarshalJSON() ([]byte, error) {
	type Alias Webhook
	return json.Marshal(struct {
		*Alias
		Events []string `json:"events"`
	}{
		Alias:  (*Alias)(w),
		Events: eventTypeToStringSlice(w.Events),
	})
}

// UnmarshalJSON unmarshals the webhook events from JSON
func (w *Webhook) UnmarshalJSON(data []byte) error {
	type Alias Webhook
	aux := &struct {
		Events []string `json:"events"`
		*Alias
	}{
		Alias: (*Alias)(w),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	w.Events = stringSliceToEventType(aux.Events)
	return nil
}

// Helper functions for event type conversion
func eventTypeToStringSlice(events []EventType) []string {
	result := make([]string, len(events))
	for i, e := range events {
		result[i] = string(e)
	}
	return result
}

func stringSliceToEventType(events []string) []EventType {
	result := make([]EventType, len(events))
	for i, e := range events {
		result[i] = EventType(e)
	}
	return result
}
