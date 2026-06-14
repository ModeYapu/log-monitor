package buffer

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/logmonitor/collector/storage"
)

// Retry policy for a failed flush attempt: bounded retries with exponential
// backoff avoid losing a batch to a momentary database error while keeping the
// flush loop from blocking indefinitely.
const (
	maxFlushAttempts    = 3
	initialFlushBackoff = 100 * time.Millisecond
)

// Writer manages buffered batch writing to database
type Writer struct {
	db            storage.EventStore
	buffer        chan storage.EventRecord
	bufferSize    int32
	flushInterval time.Duration
	batchSize     int
	stopCh        chan struct{}
	wg            sync.WaitGroup
	flushCount    atomic.Int64
	droppedCount  atomic.Int64
	closeOnce     sync.Once
}

// Config holds writer configuration
type Config struct {
	BufferSize    int           // Channel buffer capacity
	FlushInterval time.Duration // Flush interval
	BatchSize     int           // Max batch size per flush
}

// NewWriter creates a new buffered writer
func NewWriter(db storage.EventStore, cfg Config) *Writer {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 10000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 2 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}

	w := &Writer{
		db:            db,
		buffer:        make(chan storage.EventRecord, cfg.BufferSize),
		flushInterval: cfg.FlushInterval,
		batchSize:     cfg.BatchSize,
		stopCh:        make(chan struct{}),
	}

	w.wg.Add(1)
	go w.flushLoop()

	return w
}

// Write adds an event to the buffer
func (w *Writer) Write(event storage.EventRecord) error {
	select {
	case w.buffer <- event:
		atomic.AddInt32(&w.bufferSize, 1)
		return nil
	default:
		// Buffer is full, drop the event
		w.droppedCount.Add(1)
		return fmt.Errorf("buffer full, event dropped")
	}
}

// WriteBatch adds multiple events to the buffer
func (w *Writer) WriteBatch(events []storage.EventRecord) error {
	for _, e := range events {
		if err := w.Write(e); err != nil {
			return err
		}
	}
	return nil
}

// flushLoop periodically flushes buffered events to database
func (w *Writer) flushLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	batch := make([]storage.EventRecord, 0, w.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}

		// Retry transient database failures with exponential backoff (up to 3
		// attempts) before giving up. Only when every attempt fails do we drop
		// the batch; on success the flushed count is credited.
		var lastErr error
		backoff := initialFlushBackoff
		for attempt := 1; attempt <= maxFlushAttempts; attempt++ {
			if err := w.db.InsertEvents(batch); err != nil {
				lastErr = err
				if attempt < maxFlushAttempts {
					slog.Warn("Failed to insert batch, retrying",
						"attempt", attempt, "events", len(batch), "backoff", backoff, "error", err)
					time.Sleep(backoff)
					backoff *= 2
					continue
				}
			} else {
				lastErr = nil
				w.flushCount.Add(int64(len(batch)))
			}
			break
		}
		if lastErr != nil {
			slog.Error("Failed to insert batch after retries, dropping batch",
				"attempts", maxFlushAttempts, "events", len(batch), "error", lastErr)
		}

		// Clear batch
		atomic.AddInt32(&w.bufferSize, -int32(len(batch)))
		batch = batch[:0]
	}

	for {
		select {
		case event := <-w.buffer:
			batch = append(batch, event)
			if len(batch) >= w.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-w.stopCh:
			// Final flush before stopping
			flush()
			// Drain remaining buffer
			for {
				select {
				case event := <-w.buffer:
					batch = append(batch, event)
				default:
					flush()
					return
				}
			}
		}
	}
}

// Stats returns current writer statistics
func (w *Writer) Stats() map[string]interface{} {
	return map[string]interface{}{
		"buffer_size":   atomic.LoadInt32(&w.bufferSize),
		"flushed_count": w.flushCount.Load(),
		"dropped_count": w.droppedCount.Load(),
	}
}

// Close gracefully stops the writer. Guarded with sync.Once so a second close
// (e.g. deferred + explicit) does not panic on the already-closed stopCh.
func (w *Writer) Close() error {
	w.closeOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	return nil
}

// GetBufferSize returns the current buffer size
func (w *Writer) GetBufferSize() int {
	return int(atomic.LoadInt32(&w.bufferSize))
}

// GetFlushedCount returns the total number of flushed events
func (w *Writer) GetFlushedCount() int64 {
	return w.flushCount.Load()
}

// GetDroppedCount returns the total number of dropped events
func (w *Writer) GetDroppedCount() int64 {
	return w.droppedCount.Load()
}
