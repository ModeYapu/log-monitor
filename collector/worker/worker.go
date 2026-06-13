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

// WorkerStatus represents the status of a single worker
type WorkerStatus struct {
	Name      string `json:"name"`
	Running   bool   `json:"running"`
	LastRunAt int64  `json:"last_run_at"`
}

// StatusableWorker is an optional interface that workers can implement
// to provide detailed status information
type StatusableWorker interface {
	Worker
	Status() WorkerStatus
}

// Manager manages all background workers
type Manager struct {
	workers   []Worker
	stopChan  chan struct{}
	wg        sync.WaitGroup
	startTime time.Time
}

// NewManager creates a new worker manager
func NewManager() *Manager {
	return &Manager{
		workers:   make([]Worker, 0),
		stopChan:  make(chan struct{}),
		startTime: time.Now(),
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

// Status returns the status of all registered workers
func (m *Manager) Status() []WorkerStatus {
	statuses := make([]WorkerStatus, 0, len(m.workers))
	for _, worker := range m.workers {
		// If worker implements StatusableWorker, use its Status method
		if statusable, ok := worker.(StatusableWorker); ok {
			statuses = append(statuses, statusable.Status())
		} else {
			// Otherwise, provide basic status
			statuses = append(statuses, WorkerStatus{
				Name:      worker.Name(),
				Running:   true, // Assume running if registered
				LastRunAt: m.startTime.Unix(),
			})
		}
	}
	return statuses
}
