package alerter

import (
	"testing"
)

func TestChannelManager_AddChannel(t *testing.T) {
	cm := NewChannelManager()

	channel := &NotifyChannel{
		ID:      "test-channel",
		Type:    "feishu",
		Name:    "Test Channel",
		Config: map[string]interface{}{
			"url": "https://example.com/webhook",
		},
		Enabled: true,
	}

	err := cm.AddChannel(channel)
	if err != nil {
		t.Fatalf("AddChannel failed: %v", err)
	}

	// Verify channel was added
	retrieved, ok := cm.GetChannel("test-channel")
	if !ok {
		t.Fatal("Channel not found after adding")
	}

	if retrieved.ID != "test-channel" {
		t.Errorf("Expected ID test-channel, got %s", retrieved.ID)
	}
	if retrieved.Type != "feishu" {
		t.Errorf("Expected Type feishu, got %s", retrieved.Type)
	}
}

func TestChannelManager_AddChannelValidation(t *testing.T) {
	cm := NewChannelManager()

	// Test empty ID
	channel := &NotifyChannel{
		ID:   "",
		Type: "feishu",
		Name: "Test",
	}
	err := cm.AddChannel(channel)
	if err == nil {
		t.Error("Expected error for empty ID")
	}

	// Test invalid type
	channel.ID = "test-id"
	channel.Type = "invalid-type"
	err = cm.AddChannel(channel)
	if err == nil {
		t.Error("Expected error for invalid type")
	}
}

func TestChannelManager_ListChannels(t *testing.T) {
	cm := NewChannelManager()

	// Add multiple channels
	channels := []*NotifyChannel{
		{ID: "ch1", Type: "feishu", Name: "Channel 1"},
		{ID: "ch2", Type: "webhook", Name: "Channel 2"},
		{ID: "ch3", Type: "email", Name: "Channel 3"},
	}

	for _, ch := range channels {
		cm.AddChannel(ch)
	}

	list := cm.ListChannels()

	if len(list) != 3 {
		t.Errorf("Expected 3 channels, got %d", len(list))
	}
}

func TestChannelManager_UpdateChannel(t *testing.T) {
	cm := NewChannelManager()

	// Add a channel
	original := &NotifyChannel{
		ID:      "test-channel",
		Type:    "feishu",
		Name:    "Original Name",
		Config:  map[string]interface{}{"url": "https://old-url.com"},
		Enabled: true,
	}
	cm.AddChannel(original)

	// Update it
	updates := &NotifyChannel{
		Name:   "Updated Name",
		Type:   "feishu",
		Config: map[string]interface{}{"url": "https://new-url.com"},
		Enabled: false,
	}

	err := cm.UpdateChannel("test-channel", updates)
	if err != nil {
		t.Fatalf("UpdateChannel failed: %v", err)
	}

	// Verify updates
	updated, ok := cm.GetChannel("test-channel")
	if !ok {
		t.Fatal("Channel not found after update")
	}

	if updated.Name != "Updated Name" {
		t.Errorf("Expected Name 'Updated Name', got %s", updated.Name)
	}
	if updated.Enabled {
		t.Error("Expected Enabled to be false")
	}
}

func TestChannelManager_DeleteChannel(t *testing.T) {
	cm := NewChannelManager()

	// Add a channel
	channel := &NotifyChannel{
		ID:      "test-channel",
		Type:    "feishu",
		Name:    "Test",
		Enabled: true,
	}
	cm.AddChannel(channel)

	// Delete it
	err := cm.DeleteChannel("test-channel")
	if err != nil {
		t.Fatalf("DeleteChannel failed: %v", err)
	}

	// Verify it's gone
	_, ok := cm.GetChannel("test-channel")
	if ok {
		t.Error("Channel should not exist after deletion")
	}
}

func TestChannelManager_DeleteNonExistent(t *testing.T) {
	cm := NewChannelManager()

	err := cm.DeleteChannel("non-existent")
	if err == nil {
		t.Error("Expected error when deleting non-existent channel")
	}
}

func TestSeverityTemplates(t *testing.T) {
	tests := []struct {
		severity   string
		hasIcon    bool
		hasColor   bool
		expectIcon string
	}{
		{"critical", true, true, "🔴"},
		{"warning", true, true, "🟡"},
		{"info", true, true, "🔵"},
		{"unknown", true, true, "🔵"}, // Defaults to info
	}

	for _, tt := range tests {
		template, ok := SeverityTemplates[tt.severity]
		if !ok {
			template = SeverityTemplates["info"]
		}

		if template.Icon == "" && tt.hasIcon {
			t.Errorf("Severity %s: expected icon", tt.severity)
		}
		if tt.expectIcon != "" && template.Icon != tt.expectIcon {
			t.Errorf("Severity %s: expected icon %s, got %s", tt.severity, tt.expectIcon, template.Icon)
		}
		if template.Color == "" && tt.hasColor {
			t.Errorf("Severity %s: expected color", tt.severity)
		}
	}
}

func TestChannelToJSON(t *testing.T) {
	cm := NewChannelManager()

	channel := &NotifyChannel{
		ID:      "test-channel",
		Type:    "feishu",
		Name:    "Test Channel",
		Config:  map[string]interface{}{"url": "https://example.com"},
		Enabled: true,
	}

	jsonStr, err := cm.ChannelToJSON(channel)
	if err != nil {
		t.Fatalf("ChannelToJSON failed: %v", err)
	}

	if jsonStr == "" {
		t.Error("Expected non-empty JSON string")
	}

	// Try to parse it back
	parsed, err := cm.JSONToChannel(jsonStr)
	if err != nil {
		t.Fatalf("JSONToChannel failed: %v", err)
	}

	if parsed.ID != channel.ID {
		t.Errorf("Expected ID %s, got %s", channel.ID, parsed.ID)
	}
	if parsed.Type != channel.Type {
		t.Errorf("Expected Type %s, got %s", channel.Type, parsed.Type)
	}
}

func TestChannelToJSONInvalid(t *testing.T) {
	cm := NewChannelManager()

	_, err := cm.JSONToChannel("invalid json")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}
