package alerter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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

// SendFeishu sends a card notification to Feishu
func (n *Notifier) SendFeishu(webhookURL, title, message string) error {
	if webhookURL == "" {
		return fmt.Errorf("empty webhook URL")
	}

	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": "⚠️ LogMonitor 告警",
				},
				"template": "red",
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**规则**: %s\n**详情**: %s\n**时间**: %s", title, message, time.Now().Format("2006-01-02 15:04:05")),
					},
				},
			},
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

// SendWeCom sends a notification to WeCom (企业微信)
func (n *Notifier) SendWeCom(webhookURL, title, message string) error {
	if webhookURL == "" {
		return fmt.Errorf("empty webhook URL")
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": fmt.Sprintf("## ⚠️ LogMonitor 告警\n> **规则**: %s\n> **详情**: %s\n> **时间**: %s", title, message, time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	return n.sendWebhook(webhookURL, payload)
}

// SendDingTalk sends a notification to DingTalk (钉钉)
func (n *Notifier) SendDingTalk(webhookURL, title, message string) error {
	if webhookURL == "" {
		return fmt.Errorf("empty webhook URL")
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": "⚠️ LogMonitor 告警",
			"text":  fmt.Sprintf("## ⚠️ LogMonitor 告警\n- **规则**: %s\n- **详情**: %s\n- **时间**: %s", title, message, time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	return n.sendWebhook(webhookURL, payload)
}

// SendTelegram sends a notification to Telegram
func (n *Notifier) SendTelegram(botToken, chatID, message string) error {
	if botToken == "" {
		return fmt.Errorf("empty bot token")
	}
	if chatID == "" {
		return fmt.Errorf("empty chat ID")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       fmt.Sprintf("⚠️ *LogMonitor 告警*\n\n%s\n*时间*: %s", escapeMarkdown(message), time.Now().Format("2006-01-02 15:04:05")),
		"parse_mode": "Markdown",
	}

	return n.sendWebhook(url, payload)
}

// escapeMarkdown escapes special characters for Telegram Markdown mode
func escapeMarkdown(text string) string {
	// Escape special characters for Telegram Markdown V2
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
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
