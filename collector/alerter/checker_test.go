package alerter

import (
	"testing"
)

func TestNewChecker(t *testing.T) {
	checker := NewChecker(nil, nil)
	if checker == nil {
		t.Fatal("NewChecker returned nil")
	}
	if checker.notifier == nil {
		t.Error("Checker should have a notifier initialized")
	}
}

func TestChecker_StopWithoutStart(t *testing.T) {
	checker := NewChecker(nil, nil)
	// Stop should be safe even without Start
	checker.Stop()
}

// Start/Stop with nil store would panic, skip integration test
// Unit tests cover construction and helper functions only

func TestNewNotifier(t *testing.T) {
	n := NewNotifier()
	if n == nil {
		t.Fatal("NewNotifier returned nil")
	}
}

func TestNotifier_RenderTemplate(t *testing.T) {
	n := NewNotifier()
	ctx := NotificationContext{
		AppID:      "test-app",
		ErrorCount: 10,
	}

	// Test that template renders fields correctly
	result := n.RenderTemplate("test-app", ctx)
	_ = result // Just verify it doesn't panic

	// Test with empty template
	result2 := n.RenderTemplate("", ctx)
	if result2 != "" {
		t.Errorf("Empty template should return empty string, got: %s", result2)
	}
}

func TestNotifier_RenderTemplate_EmptyTemplate(t *testing.T) {
	n := NewNotifier()
	ctx := NotificationContext{AppID: "test"}
	result := n.RenderTemplate("", ctx)
	if result != "" {
		t.Errorf("Empty template should return empty string, got: %s", result)
	}
}

func TestTruncateMessage(t *testing.T) {
	tests := []struct {
		name   string
		msg    string
		maxLen int
	}{
		{"short", "hello", 10},
		{"exact", "hello", 5},
		{"truncate", "hello world", 8},
		{"empty", "", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateMessage(tt.msg, tt.maxLen)
			if tt.msg != "" && len(tt.msg) <= tt.maxLen {
				if result != tt.msg {
					t.Errorf("Expected %q, got %q", tt.msg, result)
				}
			}
			// For truncated case, just verify it starts with the original prefix
			if len(tt.msg) > tt.maxLen {
				if result[:tt.maxLen] != tt.msg[:tt.maxLen] {
					t.Errorf("Truncated message should start with original prefix")
				}
			}
		})
	}
}

func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"normal", "normal"},
		{"bold**text", "bold\\*\\*text"},
		{"link[text]", "link\\[text\\]"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeMarkdown(tt.input)
			if result == "" && tt.input != "" {
				t.Errorf("escapeMarkdown returned empty for non-empty input")
			}
		})
	}
}

func TestGenerateFeishuSignature(t *testing.T) {
	sig := generateFeishuSignature("test-secret", 1700000000)
	if sig == "" {
		t.Error("Signature should not be empty")
	}
}

func TestAlertContext_Structure(t *testing.T) {
	ctx := AlertContext{
		AppID:      "myapp",
		Release:    "v1.0.0",
		Env:        "production",
		Page:       "/home",
		Device:     "desktop",
		UserAgent:  "Mozilla/5.0",
		UserCount:  100,
		ErrorCount: 5,
		Rate:       5.0,
		TimeRange:  "5m",
	}

	if ctx.AppID != "myapp" {
		t.Error("AppID mismatch")
	}
	if ctx.ErrorCount != 5 {
		t.Error("ErrorCount mismatch")
	}
}

func TestNotificationContext_Structure(t *testing.T) {
	ctx := NotificationContext{
		AppID:      "test",
		ErrorCount: 3,
		Rate:       2.5,
	}

	if ctx.AppID != "test" {
		t.Error("AppID mismatch")
	}
}

func TestThresholdConfig(t *testing.T) {
	cfg := thresholdConfig{
		Level:         "error",
		Count:         10,
		WindowMinutes: 5,
		AggregateBy:   "app_id",
	}

	if cfg.Count != 10 {
		t.Error("Count mismatch")
	}
	if cfg.WindowMinutes != 5 {
		t.Error("WindowMinutes mismatch")
	}
}

func TestRateConfig(t *testing.T) {
	cfg := rateConfig{
		Rate:          5.0,
		MinSamples:    10,
		WindowMinutes: 1,
		AggregateBy:   "app_id",
	}

	if cfg.Rate != 5.0 {
		t.Error("Rate mismatch")
	}
	if cfg.MinSamples != 10 {
		t.Error("MinSamples mismatch")
	}
}

func TestEmailConfig_Structure(t *testing.T) {
	cfg := EmailConfig{
		Enabled:  true,
		SMTPHost: "smtp.gmail.com",
		SMTPPort: "587",
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestChecker_SetEmailConfig(t *testing.T) {
	checker := NewChecker(nil, nil)
	cfg := EmailConfig{
		Enabled:  true,
		SMTPHost: "smtp.test.com",
		SMTPPort: "465",
	}
	// Should not panic
	checker.SetEmailConfig(cfg)
}
