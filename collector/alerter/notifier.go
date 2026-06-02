package alerter

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

// EmailConfig holds SMTP configuration
type EmailConfig struct {
	Enabled  bool
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	FromEmail string
	FromName string
}

// Notifier sends alert notifications
type Notifier struct {
	client      *http.Client
	emailConfig EmailConfig
}

// NotificationContext holds context for template rendering
type NotificationContext struct {
	AppID       string
	Release     string
	Env         string
	Page        string
	Device      string
	UserAgent   string
	UserCount   int
	ErrorCount  int
	Rate        float64
	TimeRange   string
	TriggerTime time.Time
}

// NewNotifier creates a new notifier
func NewNotifier() *Notifier {
	return &Notifier{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
				},
			},
		},
		emailConfig: EmailConfig{},
	}
}

// SetEmailConfig sets the email configuration
func (n *Notifier) SetEmailConfig(cfg EmailConfig) {
	n.emailConfig = cfg
}

// SendFeishu sends a card notification to Feishu with optional signature
func (n *Notifier) SendFeishu(webhookURL, title, message string) error {
	if webhookURL == "" {
		return fmt.Errorf("empty webhook URL")
	}

	// Parse webhook URL to extract sign key if present
	signKey := ""
	if u, err := url.Parse(webhookURL); err == nil {
		// Check if timestamp and sign parameters exist (signed webhook)
		// For signed webhooks, we need to extract the key
		// The base URL without params should have the key embedded
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

	return n.sendWebhookWithRetry(webhookURL, payload, 3)
}

// SendFeishuWithSignature sends to Feishu with signature support
func (n *Notifier) SendFeishuWithSignature(webhookURL, signKey, title, message string) error {
	if webhookURL == "" {
		return fmt.Errorf("empty webhook URL")
	}

	timestamp := time.Now().Unix()
	sign := generateFeishuSignature(signKey, timestamp)

	// Build URL with timestamp and sign
	u, _ := url.Parse(webhookURL)
	q := u.Query()
	q.Set("timestamp", fmt.Sprintf("%d", timestamp))
	q.Set("sign", sign)
	u.RawQuery = q.Encode()
	signedURL := u.String()

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

	return n.sendWebhookWithRetry(signedURL, payload, 3)
}

// generateFeishuSignature generates signature for Feishu webhook
func generateFeishuSignature(signKey string, timestamp int64) string {
	stringToSign := fmt.Sprintf("%d", timestamp) + "\n" + signKey
	h := hmac.New(sha256.New, []byte(signKey))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
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

	return n.sendWebhookWithRetry(webhookURL, payload, 3)
}

// SendWebhookWithContext sends a webhook with template context
func (n *Notifier) SendWebhookWithContext(webhookURL, title, message string, ctx NotificationContext) error {
	if webhookURL == "" {
		return fmt.Errorf("empty webhook URL")
	}

	renderedMsg := n.renderTemplate(message, ctx)
	renderedTitle := n.renderTemplate(title, ctx)

	payload := map[string]interface{}{
		"title":       renderedTitle,
		"message":     renderedMsg,
		"time":        time.Now().UnixMilli(),
		"context":     ctx,
		"triggeredAt": time.Now().Format(time.RFC3339),
	}

	return n.sendWebhookWithRetry(webhookURL, payload, 3)
}

// renderTemplate renders a message template with context variables
func (n *Notifier) renderTemplate(template string, ctx NotificationContext) string {
	result := template
	result = strings.ReplaceAll(result, "{{appId}}", ctx.AppID)
	result = strings.ReplaceAll(result, "{{release}}", ctx.Release)
	result = strings.ReplaceAll(result, "{{env}}", ctx.Env)
	result = strings.ReplaceAll(result, "{{page}}", ctx.Page)
	result = strings.ReplaceAll(result, "{{device}}", ctx.Device)
	result = strings.ReplaceAll(result, "{{userAgent}}", ctx.UserAgent)
	result = strings.ReplaceAll(result, "{{userCount}}", fmt.Sprintf("%d", ctx.UserCount))
	result = strings.ReplaceAll(result, "{{errorCount}}", fmt.Sprintf("%d", ctx.ErrorCount))
	result = strings.ReplaceAll(result, "{{rate}}", fmt.Sprintf("%.2f%%", ctx.Rate))
	result = strings.ReplaceAll(result, "{{timeRange}}", ctx.TimeRange)
	result = strings.ReplaceAll(result, "{{timestamp}}", ctx.TriggerTime.Format("2006-01-02 15:04:05"))
	return result
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

	return n.sendWebhookWithRetry(webhookURL, payload, 3)
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

	return n.sendWebhookWithRetry(webhookURL, payload, 3)
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

	return n.sendWebhookWithRetry(url, payload, 3)
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

// SendEmail sends an email notification
func (n *Notifier) SendEmail(to, title, message string) error {
	if to == "" {
		return fmt.Errorf("empty email address")
	}

	// Parse email address
	addr, err := mail.ParseAddress(to)
	if err != nil {
		return fmt.Errorf("invalid email address: %w", err)
	}

	// Build email message
	fromName := n.emailConfig.FromName
	if fromName == "" {
		fromName = "LogMonitor"
	}
	fromEmail := n.emailConfig.FromEmail
	if fromEmail == "" {
		fromEmail = "noreply@logmonitor.local"
	}

	subject := fmt.Sprintf("[LogMonitor] %s", title)
	body := fmt.Sprintf("LogMonitor Alert Notification\n\n%s\n\nTime: %s\n\n---\nThis is an automated message from LogMonitor",
		message, time.Now().Format("2006-01-02 15:04:05"))

	// If email is enabled and SMTP config is available, send the email
	if n.emailConfig.Enabled && n.emailConfig.SMTPHost != "" && n.emailConfig.SMTPUser != "" {
		return n.SendEmailWithSMTP(to, title, message, n.emailConfig.SMTPHost, n.emailConfig.SMTPPort, n.emailConfig.SMTPUser, n.emailConfig.SMTPPass)
	}

	// Otherwise, just log the email (for testing/debugging)
	log.Printf("[Email] To: %s | Subject: %s | Body (first 200 chars): %.200s", addr.Address, subject, body)
	return nil
}

// SendEmailWithSMTP sends email using SMTP
func (n *Notifier) SendEmailWithSMTP(to, title, message, smtpHost, smtpPort, smtpUser, smtpPass string) error {
	if to == "" {
		return fmt.Errorf("empty email address")
	}
	if smtpHost == "" {
		return fmt.Errorf("empty SMTP host")
	}

	fromName := n.emailConfig.FromName
	if fromName == "" {
		fromName = "LogMonitor"
	}
	fromEmail := n.emailConfig.FromEmail
	if fromEmail == "" {
		fromEmail = smtpUser
	}

	from := mail.Address{Name: fromName, Address: fromEmail}
	toAddr := mail.Address{Name: "", Address: to}

	subject := fmt.Sprintf("[LogMonitor] %s", title)
	body := fmt.Sprintf("LogMonitor Alert\n\n%s\n\nTime: %s", message, time.Now().Format("2006-01-02 15:04:05"))

	// Compose email
	headers := make(map[string]string)
	headers["From"] = from.String()
	headers["To"] = toAddr.String()
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=\"utf-8\""

	// Build message
	msg := ""
	for k, v := range headers {
		msg += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	msg += "\r\n" + body

	// Send email
	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	err := smtp.SendMail(addr, auth, from.Address, []string{to}, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Email sent successfully to %s", to)
	return nil
}

// sendWebhook sends a POST request to a webhook URL
func (n *Notifier) sendWebhook(url string, payload interface{}) error {
	return n.sendWebhookWithRetry(url, payload, 1)
}

// sendWebhookWithRetry sends a POST request with retry logic
func (n *Notifier) sendWebhookWithRetry(webhookURL string, payload interface{}, maxRetries int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			log.Printf("Webhook retry %d/%d for %s after %v", attempt+1, maxRetries, webhookURL, backoff)
			time.Sleep(backoff)
		}

		req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(body))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := n.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to send request: %w", err)
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(bodyBytes))
			continue
		}

		log.Printf("Notification sent successfully to %s", webhookURL)
		return nil
	}

	return lastErr
}
