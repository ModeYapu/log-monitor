package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Buffer   BufferConfig   `yaml:"buffer"`
	Alert    AlertConfig    `yaml:"alert"`
	Auth     AuthConfig     `yaml:"auth"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port           int      `yaml:"port"`
	CORS           bool     `yaml:"cors"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	AdminTokens    []string `yaml:"admin_tokens"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Path          string               `yaml:"path"`
	RetentionDays int                  `yaml:"retention_days"`
	Retention     *DataRetentionPolicy `yaml:"retention"`
}

// DataRetentionPolicy holds retention policies for different data types
type DataRetentionPolicy struct {
	EventsDays      int `yaml:"events_days"`
	RecordingsDays  int `yaml:"recordings_days"`
	ScreenshotsDays int `yaml:"screenshots_days"`
}

// GetEventRetention returns the event retention days (with fallback to legacy config)
func (c *DatabaseConfig) GetEventRetention() int {
	if c.Retention != nil && c.Retention.EventsDays > 0 {
		return c.Retention.EventsDays
	}
	return c.RetentionDays
}

// GetRecordingRetention returns the recording retention days (with fallback)
func (c *DatabaseConfig) GetRecordingRetention() int {
	if c.Retention != nil && c.Retention.RecordingsDays > 0 {
		return c.Retention.RecordingsDays
	}
	return 14 // Default to 14 days for recordings
}

// GetScreenshotRetention returns the screenshot retention days (with fallback)
func (c *DatabaseConfig) GetScreenshotRetention() int {
	if c.Retention != nil && c.Retention.ScreenshotsDays > 0 {
		return c.Retention.ScreenshotsDays
	}
	return c.RetentionDays // Default to same as general retention
}

// BufferConfig holds buffer configuration
type BufferConfig struct {
	Size           int `yaml:"size"`
	FlushInterval  int `yaml:"flush_interval_ms"`
	FlushBatchSize int `yaml:"flush_batch_size"`
}

// AlertConfig holds alert configuration
type AlertConfig struct {
	CheckInterval int         `yaml:"check_interval_ms"`
	Email         EmailConfig `yaml:"email"`
}

// EmailConfig holds email notification configuration
type EmailConfig struct {
	Enabled   bool   `yaml:"enabled"`
	SMTPHost  string `yaml:"smtp_host"`
	SMTPPort  string `yaml:"smtp_port"`
	SMTPUser  string `yaml:"smtp_user"`
	SMTPPass  string `yaml:"smtp_pass"`
	FromEmail string `yaml:"from_email"`
	FromName  string `yaml:"from_name"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled          bool   `yaml:"enabled"`
	JWTSecret        string `yaml:"jwt_secret"`
	DefaultPassword  string `yaml:"default_password"`
	TokenExpireHours int    `yaml:"token_expire_hours"`
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9200
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./data/logmonitor.db"
	}
	if cfg.Database.RetentionDays == 0 {
		cfg.Database.RetentionDays = 30
	}
	// Initialize retention policy if not set
	if cfg.Database.Retention == nil {
		cfg.Database.Retention = &DataRetentionPolicy{
			EventsDays:      30,
			RecordingsDays:  14,
			ScreenshotsDays: 30,
		}
	} else {
		// Set defaults for individual fields if not set
		if cfg.Database.Retention.EventsDays == 0 {
			cfg.Database.Retention.EventsDays = 30
		}
		if cfg.Database.Retention.RecordingsDays == 0 {
			cfg.Database.Retention.RecordingsDays = 14
		}
		if cfg.Database.Retention.ScreenshotsDays == 0 {
			cfg.Database.Retention.ScreenshotsDays = 30
		}
	}
	if cfg.Buffer.Size == 0 {
		cfg.Buffer.Size = 10000
	}
	if cfg.Buffer.FlushInterval == 0 {
		cfg.Buffer.FlushInterval = 2000
	}
	if cfg.Buffer.FlushBatchSize == 0 {
		cfg.Buffer.FlushBatchSize = 500
	}
	if cfg.Alert.CheckInterval == 0 {
		cfg.Alert.CheckInterval = 60000
	}

	return &cfg, nil
}

// Default returns default configuration
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port:           9200,
			CORS:           true,
			AllowedOrigins: nil,
		},
		Database: DatabaseConfig{
			Path:          "./data/logmonitor.db",
			RetentionDays: 30,
			Retention: &DataRetentionPolicy{
				EventsDays:      30,
				RecordingsDays:  14,
				ScreenshotsDays: 30,
			},
		},
		Buffer: BufferConfig{
			Size:           10000,
			FlushInterval:  2000,
			FlushBatchSize: 500,
		},
		Alert: AlertConfig{
			CheckInterval: 60000,
			Email: EmailConfig{
				Enabled:  false,
				SMTPHost: "smtp.gmail.com",
				SMTPPort: "587",
				FromName: "LogMonitor",
			},
		},
		Auth: AuthConfig{
			Enabled:          true,
			JWTSecret:        "",
			DefaultPassword:  "admin123",
			TokenExpireHours: 24,
		},
	}
}

// 看看当前 ServerConfig
