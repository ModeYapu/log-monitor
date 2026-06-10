package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Worker interface defines the contract for all background workers
type Worker interface {
	Start(ctx context.Context) error
	Stop() error
	Name() string
}

// Manager manages all background workers
type Manager struct {
	workers  []Worker
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewManager creates a new worker manager
func NewManager() *Manager {
	return &Manager{
		workers:  make([]Worker, 0),
		stopChan: make(chan struct{}),
	}
}

// RegisterWorker registers a new worker
func (m *Manager) RegisterWorker(worker Worker) {
	m.workers = append(m.workers, worker)
	slog.Info("Worker registered", "worker", worker.Name())
}

// Start starts all registered workers
func (m *Manager) Start(ctx context.Context) error {
	slog.Info("Starting worker manager", "workers", len(m.workers))

	for _, worker := range m.workers {
		m.wg.Add(1)
		go func(w Worker) {
			defer m.wg.Done()
			if err := w.Start(ctx); err != nil {
				slog.Error("Worker failed", "worker", w.Name(), "error", err)
			}
		}(worker)
	}

	return nil
}

// Stop stops all workers gracefully
func (m *Manager) Stop() error {
	slog.Info("Stopping worker manager")

	// Stop all workers
	for _, worker := range m.workers {
		if err := worker.Stop(); err != nil {
			slog.Error("Worker stop failed", "worker", worker.Name(), "error", err)
		}
	}

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("All workers stopped successfully")
		return nil
	case <-time.After(30 * time.Second):
		slog.Warn("Timeout waiting for workers to stop")
		return nil
	}
}

// WorkerCount returns the number of registered workers
func (m *Manager) WorkerCount() int {
	return len(m.workers)
}