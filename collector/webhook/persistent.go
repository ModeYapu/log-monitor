package webhook

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/logmonitor/collector/storage"
)

// PersistentQueue stores webhook delivery attempts in SQLite for durable retry.
type PersistentQueue struct {
	db *storage.DB
}

// NewPersistentQueue creates a new persistent webhook queue.
func NewPersistentQueue(db *storage.DB) *PersistentQueue {
	return &PersistentQueue{db: db}
}

// PendingDelivery represents a queued webhook delivery.
type PendingDelivery struct {
	ID          int64  `json:"id"`
	WebhookID   int64  `json:"webhook_id"`
	Payload     string `json:"payload"`
	Status      string `json:"status"` // pending | retrying | failed | delivered
	Attempts    int    `json:"attempts"`
	MaxAttempts int    `json:"max_attempts"`
	NextRetryAt int64  `json:"next_retry_at"`
	LastError   string `json:"last_error"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// Enqueue adds a payload to the persistent delivery queue.
func (q *PersistentQueue) Enqueue(webhookID int64, payload Payload) error {
	if q.db.Closed() {
		return fmt.Errorf("database is closed")
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	now := time.Now().Unix()
	_, err = q.db.Conn().Exec(`
		INSERT INTO webhook_deliveries (webhook_id, payload, status, attempts, max_attempts, next_retry_at, last_error, created_at, updated_at)
		VALUES (?, ?, 'pending', 0, 5, ?, '', ?, ?)
	`, webhookID, string(payloadJSON), now, now, now)

	if err != nil {
		return fmt.Errorf("enqueue delivery: %w", err)
	}

	return nil
}

// GetPending returns deliveries that are ready for retry.
func (q *PersistentQueue) GetPending(limit int) ([]PendingDelivery, error) {
	if q.db.Closed() {
		return nil, fmt.Errorf("database is closed")
	}

	if limit <= 0 {
		limit = 50
	}

	now := time.Now().Unix()
	rows, err := q.db.Conn().Query(`
		SELECT id, webhook_id, payload, status, attempts, max_attempts, next_retry_at, last_error, created_at, updated_at
		FROM webhook_deliveries
		WHERE status IN ('pending', 'retrying') AND next_retry_at <= ? AND attempts < max_attempts
		ORDER BY next_retry_at ASC
		LIMIT ?
	`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("get pending deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []PendingDelivery
	for rows.Next() {
		var d PendingDelivery
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Payload, &d.Status, &d.Attempts, &d.MaxAttempts, &d.NextRetryAt, &d.LastError, &d.CreatedAt, &d.UpdatedAt); err != nil {
			continue
		}
		deliveries = append(deliveries, d)
	}

	return deliveries, nil
}

// MarkDelivered marks a delivery as successfully delivered.
func (q *PersistentQueue) MarkDelivered(id int64) error {
	if q.db.Closed() {
		return fmt.Errorf("database is closed")
	}

	_, err := q.db.Conn().Exec(`
		UPDATE webhook_deliveries SET status = 'delivered', updated_at = ? WHERE id = ?
	`, time.Now().Unix(), id)
	return err
}

// MarkFailed increments attempts and schedules retry with exponential backoff.
func (q *PersistentQueue) MarkFailed(id int64, errMsg string) error {
	if q.db.Closed() {
		return fmt.Errorf("database is closed")
	}

	now := time.Now().Unix()

	// Get current attempts
	var attempts, maxAttempts int
	err := q.db.Conn().QueryRow(`SELECT attempts, max_attempts FROM webhook_deliveries WHERE id = ?`, id).Scan(&attempts, &maxAttempts)
	if err != nil {
		return err
	}

	attempts++
	if attempts >= maxAttempts {
		// Mark as permanently failed
		_, err = q.db.Conn().Exec(`
			UPDATE webhook_deliveries SET status = 'failed', attempts = ?, last_error = ?, updated_at = ? WHERE id = ?
		`, attempts, errMsg, now, id)
		slog.Error("Webhook delivery permanently failed",
			"delivery_id", id,
			"attempts", attempts,
			"error", errMsg)
	} else {
		// Schedule retry with exponential backoff: 30s, 1m, 5m, 15m, 30m
		backoffSec := int64(30 * (1 << (attempts - 1)))
		if backoffSec > 1800 {
			backoffSec = 1800
		}
		nextRetry := now + backoffSec

		_, err = q.db.Conn().Exec(`
			UPDATE webhook_deliveries SET status = 'retrying', attempts = ?, next_retry_at = ?, last_error = ?, updated_at = ? WHERE id = ?
		`, attempts, nextRetry, errMsg, now, id)

		slog.Warn("Webhook delivery failed, scheduled retry",
			"delivery_id", id,
			"attempts", attempts,
			"next_retry_in", backoffSec,
			"error", errMsg)
	}

	return err
}

// CleanOld removes delivered/failed entries older than retentionDays.
func (q *PersistentQueue) CleanOld(retentionDays int) (int64, error) {
	if q.db.Closed() {
		return 0, fmt.Errorf("database is closed")
	}

	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	result, err := q.db.Conn().Exec(`
		DELETE FROM webhook_deliveries
		WHERE status IN ('delivered', 'failed') AND updated_at < ?
	`, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// RetryWorker processes pending webhook deliveries.
type RetryWorker struct {
	queue     *PersistentQueue
	store     WebhookStore
	deliverer *Deliverer
	stopCh    chan struct{}
}

// NewRetryWorker creates a new retry worker.
func NewRetryWorker(queue *PersistentQueue, store WebhookStore) *RetryWorker {
	return &RetryWorker{
		queue:     queue,
		store:     store,
		deliverer: NewDeliverer(),
		stopCh:    make(chan struct{}),
	}
}

// Start begins the retry loop.
func (w *RetryWorker) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processPending()
		case <-w.stopCh:
			return
		}
	}
}

// Stop stops the retry worker.
func (w *RetryWorker) Stop() {
	close(w.stopCh)
}

// processPending delivers all pending webhook payloads.
func (w *RetryWorker) processPending() {
	deliveries, err := w.queue.GetPending(50)
	if err != nil {
		slog.Error("Failed to get pending webhook deliveries", "error", err)
		return
	}

	for _, d := range deliveries {
		webhook, err := w.store.GetWebhook(d.WebhookID)
		if err != nil {
			w.queue.MarkFailed(d.ID, fmt.Sprintf("webhook not found: %v", err))
			continue
		}

		if !webhook.Enabled {
			w.queue.MarkFailed(d.ID, "webhook disabled")
			continue
		}

		var payload Payload
		if err := json.Unmarshal([]byte(d.Payload), &payload); err != nil {
			w.queue.MarkFailed(d.ID, fmt.Sprintf("invalid payload: %v", err))
			continue
		}

		if err := w.deliverer.DeliverSync(*webhook, payload); err != nil {
			w.queue.MarkFailed(d.ID, err.Error())
		} else {
			w.queue.MarkDelivered(d.ID)
			// Update webhook trigger info
			w.store.UpdateWebhookTriggerInfo(webhook.ID, time.Now().Unix(), 0)
		}
	}
}
