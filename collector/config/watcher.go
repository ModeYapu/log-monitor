package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches for config file changes
type Watcher struct {
	configPath string
	onReload   func(*Config, *Config) error
	stopChan   chan struct{}
	doneChan   chan struct{}
	currentCfg *Config
}

// NewWatcher creates a new config watcher
func NewWatcher(configPath string, onReload func(*Config, *Config) error) *Watcher {
	return &Watcher{
		configPath: configPath,
		onReload:   onReload,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}
}

// Start begins watching the config file
func (w *Watcher) Start(initialCfg *Config) error {
	w.currentCfg = initialCfg

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Watch the directory containing the config file
	configDir := filepath.Dir(w.configPath)
	if err := watcher.Add(configDir); err != nil {
		watcher.Close()
		return err
	}

	slog.Info("Config watcher started", "path", w.configPath)

	// Start watching goroutine
	go func() {
		defer watcher.Close()
		defer close(w.doneChan)

		// Debounce timer
		var debounceTimer *time.Timer
		debounceDuration := 500 * time.Millisecond

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Check if the event is for our config file
				if event.Name != w.configPath {
					continue
				}

				// Only process write and create events
				if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}

				slog.Debug("Config file change detected", "event", event.Op)

				// Debounce rapid file changes
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDuration, func() {
					w.handleConfigChange()
				})

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error("Config watcher error", "error", err)

			case <-w.stopChan:
				slog.Info("Config watcher stopping")
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				return
			}
		}
	}()

	return nil
}

// Stop stops the config watcher
func (w *Watcher) Stop() {
	close(w.stopChan)
	<-w.doneChan
	slog.Info("Config watcher stopped")
}

// handleConfigChange handles config file changes
func (w *Watcher) handleConfigChange() {
	slog.Info("Config file changed, reloading", "path", w.configPath)

	// Load new config
	newCfg, err := Load(w.configPath)
	if err != nil {
		slog.Error("Failed to reload config", "error", err)
		return
	}

	// Call reload callback
	if w.onReload != nil {
		if err := w.onReload(w.currentCfg, newCfg); err != nil {
			slog.Error("Failed to apply new config", "error", err)
			return
		}
	}

	// Update current config
	w.currentCfg = newCfg
	slog.Info("Config reloaded successfully",
		"port", newCfg.Server.Port,
		"retentionDays", newCfg.Database.RetentionDays,
		"alertInterval", newCfg.Alert.CheckInterval)
}

// GetCurrentConfig returns the current config
func (w *Watcher) GetCurrentConfig() *Config {
	return w.currentCfg
}

// PollingWatcher is a fallback watcher that polls the config file
type PollingWatcher struct {
	configPath   string
	onReload     func(*Config, *Config) error
	pollInterval time.Duration
	stopChan     chan struct{}
	doneChan     chan struct{}
	currentCfg   *Config
}

// NewPollingWatcher creates a new polling config watcher
func NewPollingWatcher(configPath string, onReload func(*Config, *Config) error, pollInterval time.Duration) *PollingWatcher {
	return &PollingWatcher{
		configPath:   configPath,
		onReload:     onReload,
		pollInterval: pollInterval,
		stopChan:     make(chan struct{}),
		doneChan:     make(chan struct{}),
	}
}

// Start begins polling the config file
func (w *PollingWatcher) Start(initialCfg *Config) error {
	w.currentCfg = initialCfg

	// Get initial file info
	initialInfo, err := os.Stat(w.configPath)
	if err != nil {
		return err
	}

	lastModTime := initialInfo.ModTime()

	slog.Info("Config polling watcher started", "path", w.configPath, "interval", w.pollInterval)

	// Start polling goroutine
	go func() {
		defer close(w.doneChan)

		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check file modification time
				info, err := os.Stat(w.configPath)
				if err != nil {
					slog.Error("Failed to stat config file", "error", err)
					continue
				}

				// Check if file was modified
				if info.ModTime().After(lastModTime) {
					slog.Info("Config file modification detected", "path", w.configPath)
					lastModTime = info.ModTime()

					// Reload config
					newCfg, err := Load(w.configPath)
					if err != nil {
						slog.Error("Failed to reload config", "error", err)
						continue
					}

					// Call reload callback
					if w.onReload != nil {
						if err := w.onReload(w.currentCfg, newCfg); err != nil {
							slog.Error("Failed to apply new config", "error", err)
							continue
						}
					}

					// Update current config
					w.currentCfg = newCfg
					slog.Info("Config reloaded successfully")
				}

			case <-w.stopChan:
				slog.Info("Config polling watcher stopping")
				return
			}
		}
	}()

	return nil
}

// Stop stops the polling watcher
func (w *PollingWatcher) Stop() {
	close(w.stopChan)
	<-w.doneChan
	slog.Info("Config polling watcher stopped")
}

// GetCurrentConfig returns the current config
func (w *PollingWatcher) GetCurrentConfig() *Config {
	return w.currentCfg
}
