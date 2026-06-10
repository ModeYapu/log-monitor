package webhook

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Deliverer handles webhook delivery with retry logic
type Deliverer struct {
	client    *http.Client
	mu        sync.Mutex
	attempts  map[int64]int // Track delivery attempts per webhook
}

// NewDeliverer creates a new webhook deliverer
func NewDeliverer() *Deliverer {
	return &Deliverer{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false, // Don't skip TLS verification
				},
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		attempts: make(map[int64]int),
	}
}

// Deliver delivers a webhook payload asynchronously
func (d *Deliverer) Deliver(webhook Webhook, payload Payload) {
	// Track delivery attempts
	d.mu.Lock()
	d.attempts[webhook.ID]++
	attempt := d.attempts[webhook.ID]
	d.mu.Unlock()

	// Deliver with retry
	err := d.deliverWithRetry(webhook, payload, attempt)
	if err != nil {
		slog.Error("Webhook delivery failed after all retries",
			"webhook_id", webhook.ID,
			"url", webhook.URL,
			"event", payload.Event,
			"error", err)
	}
}

// DeliverSync delivers a webhook payload synchronously (for testing)
func (d *Deliverer) DeliverSync(webhook Webhook, payload Payload) error {
	return d.deliverWithRetry(webhook, payload, 1)
}

// deliverWithRetry delivers a webhook with exponential backoff retry
func (d *Deliverer) deliverWithRetry(webhook Webhook, payload Payload, attempt int) error {
	maxRetries := 3
	baseDelay := 1 * time.Second

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := d.deliverOnce(ctx, webhook, payload)
		cancel()

		if err == nil {
			// Success - clear attempt counter
			d.mu.Lock()
			delete(d.attempts, webhook.ID)
			d.mu.Unlock()

			slog.Info("Webhook delivered successfully",
				"webhook_id", webhook.ID,
				"url", webhook.URL,
				"event", payload.Event,
				"attempt", i+1)

			return nil
		}

		lastErr = err

		// Exponential backoff
		delay := baseDelay * time.Duration(1<<uint(i))
		slog.Warn("Webhook delivery failed, retrying",
			"webhook_id", webhook.ID,
			"url", webhook.URL,
			"event", payload.Event,
			"attempt", i+1,
			"delay", delay,
			"error", err)

		time.Sleep(delay)
	}

	return lastErr
}

// deliverOnce delivers a webhook payload a single time
func (d *Deliverer) deliverOnce(ctx context.Context, webhook Webhook, payload Payload) error {
	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LogMonitor-Webhook/1.0")

	// Add signature if secret is configured
	if webhook.Secret != "" {
		signature, err := GenerateSignature(payloadBytes, webhook.Secret)
		if err != nil {
			return err
		}
		req.Header.Set("X-Webhook-Signature", signature)
		req.Header.Set("X-Webhook-Timestamp", string(rune(payload.Timestamp)))
	}

	// Send request
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &DeliveryError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	return nil
}

// DeliveryError represents a webhook delivery error
type DeliveryError struct {
	StatusCode int
	Status     string
}

func (e *DeliveryError) Error() string {
	return e.Status
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if deliveryErr, ok := err.(*DeliveryError); ok {
		// Retry on server errors (5xx) and client errors (429) for rate limiting
		return deliveryErr.StatusCode >= 500 || deliveryErr.StatusCode == 429
	}

	// Retry on network errors
	return true
}