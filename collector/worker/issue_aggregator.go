package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/logmonitor/collector/storage"
)

// IssueAggregatorWorker handles automatic issue generation from events
type IssueAggregatorWorker struct {
	issueStore    IssueStore
	eventStore    EventStore
	checkInterval time.Duration
	stopChan      chan struct{}
}

// NewIssueAggregatorWorker creates a new issue aggregator worker
func NewIssueAggregatorWorker(issueStore IssueStore, eventStore EventStore, checkInterval time.Duration) *IssueAggregatorWorker {
	return &IssueAggregatorWorker{
		issueStore:    issueStore,
		eventStore:    eventStore,
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
	}
}

// Start begins the issue aggregator worker
func (w *IssueAggregatorWorker) Start(ctx context.Context) error {
	slog.Info("Starting issue aggregator worker", "interval", w.checkInterval)

	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processIssues()
		case <-ctx.Done():
			slog.Info("Issue aggregator worker context cancelled")
			return w.Stop()
		case <-w.stopChan:
			slog.Info("Issue aggregator worker stop signal received")
			return w.Stop()
		}
	}
}

// Stop stops the issue aggregator worker
func (w *IssueAggregatorWorker) Stop() error {
	close(w.stopChan)
	slog.Info("Issue aggregator worker stopped")
	return nil
}

// Name returns the worker name
func (w *IssueAggregatorWorker) Name() string {
	return "IssueAggregatorWorker"
}

// processIssues processes recent events and creates/updates issues
func (w *IssueAggregatorWorker) processIssues() {
	slog.Debug("Processing issues from recent events")

	// Get recent events that need issue processing
	events, err := w.eventStore.GetRecentEvents(1000)
	if err != nil {
		slog.Error("Failed to get recent events", "error", err)
		return
	}

	if len(events) == 0 {
		slog.Debug("No recent events to process")
		return
	}

	// Create or update issues from events
	err = w.issueStore.CreateOrUpdateIssues(events)
	if err != nil {
		slog.Error("Failed to create/update issues", "error", err)
		return
	}

	slog.Debug("Issues processed successfully", "events", len(events))
}

// ProcessEvents processes a batch of events immediately (called from report handler)
func (w *IssueAggregatorWorker) ProcessEvents(events []storage.EventRecord) error {
	slog.Debug("Processing events immediately", "events", len(events))
	return w.issueStore.CreateOrUpdateIssues(events)
}
