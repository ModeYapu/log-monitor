package alerter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Notifier sends alert notifications
type Notifier struct {
	client *http.Client
}

// NewNotifier creates a new notifier
func NewNotifier() *Notifier {
	return &Notifier{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendFeishu sends a notification to Feishu
func (n *Notifier) SendFeishu(webhookURL, title, message string) error {
	if webhookURL == "" {
		return fmt.Errorf("empty webhook URL")
	}

	payload := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": fmt.Sprintf("【%s】\n%s", title, message),
		},
	}

	return n.sendWebhook(webhookURL, payload)
}

// SendWebhook sends a notification to a generic webhook
func (n *Notifier) SendWebhook(webhookURL, title, message string) error {
	if webhookURL == "" {
		return fmt.Errorf("empty webhook URL")
	}

	payload := map[string]interface{}{
		"title":   title,
		"message": message,
		"time":    time.Now().UnixMilli(),
	}

	return n.sendWebhook(webhookURL, payload)
}

// SendEmail sends an email notification (placeholder)
func (n *Notifier) SendEmail(email, title, message string) error {
	log.Printf("Email notification to %s: [%s] %s", email, title, message)
	// Email sending requires SMTP configuration
	// This is a placeholder for future implementation
	return nil
}

// sendWebhook sends a POST request to a webhook URL
func (n *Notifier) sendWebhook(url string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	log.Printf("Notification sent successfully to %s", url)
	return nil
}
