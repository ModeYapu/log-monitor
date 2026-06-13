package alerter

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// NotifyChannel represents a notification channel configuration
type NotifyChannel struct {
	ID      string
	Type    string // feishu, webhook, email, wecom, dingtalk, telegram
	Name    string
	Config  map[string]interface{}
	Enabled bool
}

// ChannelTemplate represents notification template for severity levels
type ChannelTemplate struct {
	Icon       string
	Color      string // for Feishu card color
	TitleColor string
}

// SeverityTemplates holds templates for different severity levels
var SeverityTemplates = map[string]ChannelTemplate{
	"critical": {
		Icon:       "🔴",
		Color:      "red",
		TitleColor: "#FF0000",
	},
	"warning": {
		Icon:       "🟡",
		Color:      "orange",
		TitleColor: "#FFA500",
	},
	"info": {
		Icon:       "🔵",
		Color:      "blue",
		TitleColor: "#0000FF",
	},
}

// ChannelManager manages notification channels
type ChannelManager struct {
	channels map[string]*NotifyChannel
	notifier *Notifier
	mu       sync.RWMutex
}

// NewChannelManager creates a new channel manager
func NewChannelManager() *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]*NotifyChannel),
		notifier: NewNotifier(),
	}
}

// SetNotifier sets the notifier instance
func (cm *ChannelManager) SetNotifier(n *Notifier) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.notifier = n
}

// AddChannel adds a new notification channel
func (cm *ChannelManager) AddChannel(channel *NotifyChannel) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if channel.ID == "" {
		return fmt.Errorf("channel ID cannot be empty")
	}

	if channel.Type == "" {
		return fmt.Errorf("channel type cannot be empty")
	}

	// Validate channel type
	validTypes := map[string]bool{
		"feishu": true, "webhook": true, "email": true,
		"wecom": true, "dingtalk": true, "telegram": true,
	}
	if !validTypes[channel.Type] {
		return fmt.Errorf("invalid channel type: %s", channel.Type)
	}

	cm.channels[channel.ID] = channel
	slog.Info("Channel added", "id", channel.ID, "type", channel.Type, "name", channel.Name)
	return nil
}

// GetChannel retrieves a channel by ID
func (cm *ChannelManager) GetChannel(id string) (*NotifyChannel, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	channel, ok := cm.channels[id]
	return channel, ok
}

// ListChannels returns all channels
func (cm *ChannelManager) ListChannels() []*NotifyChannel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*NotifyChannel, 0, len(cm.channels))
	for _, ch := range cm.channels {
		result = append(result, ch)
	}
	return result
}

// UpdateChannel updates an existing channel
func (cm *ChannelManager) UpdateChannel(id string, updates *NotifyChannel) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	existing, ok := cm.channels[id]
	if !ok {
		return fmt.Errorf("channel not found: %s", id)
	}

	if updates.Name != "" {
		existing.Name = updates.Name
	}
	if updates.Type != "" {
		existing.Type = updates.Type
	}
	if updates.Config != nil {
		existing.Config = updates.Config
	}
	existing.Enabled = updates.Enabled

	cm.channels[id] = existing
	slog.Info("Channel updated", "id", id)
	return nil
}

// DeleteChannel removes a channel
func (cm *ChannelManager) DeleteChannel(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, ok := cm.channels[id]; !ok {
		return fmt.Errorf("channel not found: %s", id)
	}

	delete(cm.channels, id)
	slog.Info("Channel deleted", "id", id)
	return nil
}

// Send sends a notification to multiple channels with severity-based formatting
func (cm *ChannelManager) Send(channelIDs []string, severity, title, message string) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if len(channelIDs) == 0 {
		return nil
	}

	template, ok := SeverityTemplates[severity]
	if !ok {
		template = SeverityTemplates["info"]
	}

	// Enhance title with severity icon
	enhancedTitle := fmt.Sprintf("%s %s", template.Icon, title)

	// Send to each channel
	for _, channelID := range channelIDs {
		channel, ok := cm.channels[channelID]
		if !ok || !channel.Enabled {
			slog.Warn("Channel not found or disabled", "id", channelID)
			continue
		}

		if err := cm.sendToChannel(channel, severity, enhancedTitle, message, template); err != nil {
			slog.Error("Failed to send to channel", "id", channelID, "error", err)
		}
	}

	return nil
}

// sendToChannel sends notification to a specific channel
func (cm *ChannelManager) sendToChannel(channel *NotifyChannel, severity, title, message string, template ChannelTemplate) error {
	switch channel.Type {
	case "feishu":
		return cm.sendFeishu(channel, title, message, template)
	case "wecom":
		return cm.sendWeCom(channel, title, message, template)
	case "dingtalk":
		return cm.sendDingTalk(channel, title, message, template)
	case "telegram":
		return cm.sendTelegram(channel, title, message)
	case "webhook":
		return cm.sendWebhook(channel, title, message)
	case "email":
		return cm.sendEmail(channel, title, message)
	default:
		return fmt.Errorf("unsupported channel type: %s", channel.Type)
	}
}

// sendFeishu sends notification to Feishu with severity-based card
func (cm *ChannelManager) sendFeishu(channel *NotifyChannel, title, message string, template ChannelTemplate) error {
	webhookURL, _ := channel.Config["url"].(string)

	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": title,
				},
				"template": template.Color,
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**消息**: %s\n**时间**: %s", message, currentTimeStr()),
					},
				},
			},
		},
	}

	return cm.notifier.sendWebhookWithRetry(webhookURL, payload, 3)
}

// sendWeCom sends notification to WeCom
func (cm *ChannelManager) sendWeCom(channel *NotifyChannel, title, message string, template ChannelTemplate) error {
	webhookURL, _ := channel.Config["url"].(string)

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": fmt.Sprintf("%s\n> **消息**: %s\n> **时间**: %s", title, message, currentTimeStr()),
		},
	}

	return cm.notifier.sendWebhookWithRetry(webhookURL, payload, 3)
}

// sendDingTalk sends notification to DingTalk
func (cm *ChannelManager) sendDingTalk(channel *NotifyChannel, title, message string, template ChannelTemplate) error {
	webhookURL, _ := channel.Config["url"].(string)

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": title,
			"text":  fmt.Sprintf("%s\n\n**消息**: %s\n**时间**: %s", title, message, currentTimeStr()),
		},
	}

	return cm.notifier.sendWebhookWithRetry(webhookURL, payload, 3)
}

// sendTelegram sends notification to Telegram
func (cm *ChannelManager) sendTelegram(channel *NotifyChannel, title, message string) error {
	botToken, _ := channel.Config["bot_token"].(string)
	chatID, _ := channel.Config["chat_id"].(string)

	text := fmt.Sprintf("%s\n\n%s\n时间: %s", title, message, currentTimeStr())
	return cm.notifier.SendTelegram(botToken, chatID, text)
}

// sendWebhook sends notification to generic webhook
func (cm *ChannelManager) sendWebhook(channel *NotifyChannel, title, message string) error {
	webhookURL, _ := channel.Config["url"].(string)

	payload := map[string]interface{}{
		"title":       title,
		"message":     message,
		"time":        currentTimeStr(),
		"severity":    "",
		"triggeredAt": currentTimeStr(),
	}

	return cm.notifier.sendWebhookWithRetry(webhookURL, payload, 3)
}

// sendEmail sends notification via email
func (cm *ChannelManager) sendEmail(channel *NotifyChannel, title, message string) error {
	email, _ := channel.Config["email"].(string)

	// Set email config if available
	if smtpHost, ok := channel.Config["smtp_host"].(string); ok {
		smtpPort := "587"
		if p, ok := channel.Config["smtp_port"].(string); ok {
			smtpPort = p
		}
		smtpUser, _ := channel.Config["smtp_user"].(string)
		smtpPass, _ := channel.Config["smtp_pass"].(string)

		cm.notifier.SetEmailConfig(EmailConfig{
			Enabled:   true,
			SMTPHost:  smtpHost,
			SMTPPort:  smtpPort,
			SMTPUser:  smtpUser,
			SMTPPass:  smtpPass,
		})
	}

	return cm.notifier.SendEmail(email, title, message)
}

// TestChannel sends a test notification to a channel
func (cm *ChannelManager) TestChannel(channelID string) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	channel, ok := cm.channels[channelID]
	if !ok {
		return fmt.Errorf("channel not found: %s", channelID)
	}

	template := SeverityTemplates["info"]
	title := fmt.Sprintf("🧪 测试通知 - %s", channel.Name)
	message := "这是一条测试消息，用于验证通知渠道配置是否正确。"

	return cm.sendToChannel(channel, "info", title, message, template)
}

// ChannelToJSON converts a channel to JSON for storage
func (cm *ChannelManager) ChannelToJSON(channel *NotifyChannel) (string, error) {
	data, err := json.Marshal(channel)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// JSONToChannel converts JSON to a channel
func (cm *ChannelManager) JSONToChannel(jsonStr string) (*NotifyChannel, error) {
	var channel NotifyChannel
	if err := json.Unmarshal([]byte(jsonStr), &channel); err != nil {
		return nil, err
	}
	return &channel, nil
}

// currentTimeStr returns current time as formatted string
func currentTimeStr() string {
	t := time.Now()
	return fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}
