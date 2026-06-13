package buffer

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/logmonitor/collector/storage"
)

// mockEventStore implements storage.EventStore for testing
type mockEventStore struct {
	mu          sync.Mutex
	records     []storage.EventRecord
	insertCalls int64
	insertErr   error
}

func (m *mockEventStore) InsertEvents(events []storage.EventRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	atomic.AddInt64(&m.insertCalls, 1)
	if m.insertErr != nil {
		return m.insertErr
	}
	m.records = append(m.records, events...)
	return nil
}

func (m *mockEventStore) getRecords() []storage.EventRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.records
}

func (m *mockEventStore) getInsertCalls() int64 {
	return atomic.LoadInt64(&m.insertCalls)
}

// Stub out remaining EventStore interface methods
func (m *mockEventStore) QueryEvents(q storage.QueryParams) (*storage.QueryResult, error) {
	return nil, nil
}
func (m *mockEventStore) GetStats(appID string) (*storage.Stats, error) { return nil, nil }
func (m *mockEventStore) GetApps() ([]storage.AppStats, error)          { return nil, nil }
func (m *mockEventStore) GetTopN(appID, topType, orderBy string, limit int, filters storage.AnalyticsFilters) (*storage.TopNResult, error) {
	return nil, nil
}
func (m *mockEventStore) GetSimilarErrors(appID, message string, threshold float64, limit int) ([]storage.ErrorCluster, error) {
	return nil, nil
}
func (m *mockEventStore) GetSessionEvents(sessionID string, limit int) ([]storage.EventRecord, error) {
	return nil, nil
}
func (m *mockEventStore) GetSessionErrorCount(sessionID string) (int64, error) { return 0, nil }
func (m *mockEventStore) GetTopErrors(params storage.TopListParams) ([]storage.TopError, error) {
	return nil, nil
}
func (m *mockEventStore) GetTopPages(params storage.TopListParams) ([]storage.TopPage, error) {
	return nil, nil
}
func (m *mockEventStore) GetTopReleases(params storage.TopListParams) ([]storage.TopRelease, error) {
	return nil, nil
}
func (m *mockEventStore) GetTopBrowsers(params storage.TopListParams) ([]storage.TopBrowser, error) {
	return nil, nil
}
func (m *mockEventStore) GetErrorClustersByTime(appID string, startTime, endTime int64, limit int) ([]storage.ErrorClusterResult, error) {
	return nil, nil
}
func (m *mockEventStore) GetClusterEvents(appID, fingerprint string, page, pageSize int) ([]storage.EventRecord, int64, error) {
	return nil, 0, nil
}
func (m *mockEventStore) GetClusterStats(appID, fingerprint string) (storage.ClusterStats, error) {
	return storage.ClusterStats{}, nil
}
func (m *mockEventStore) GetErrorClusters(appID, errorMessage string, threshold float64, limit int) ([]storage.ErrorCluster, error) {
	return nil, nil
}
func (m *mockEventStore) GetRecentEvents(limit int) ([]storage.EventRecord, error) {
	return nil, nil
}

// helper to create a test event
func makeEvent(id int) storage.EventRecord {
	return storage.EventRecord{
		AppID:     "test-app",
		Type:      "error",
		Level:     "error",
		Message:   json.Number(string(rune('A' + id%26))).String(),
		CreatedAt: time.Now().UnixMilli(),
	}
}

// ---- NewWriter tests ----

func TestNewWriter_DefaultConfig(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{})
	defer w.Close()

	if w == nil {
		t.Fatal("NewWriter returned nil")
	}
}

func TestNewWriter_CustomConfig(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    100,
		FlushInterval: 100 * time.Millisecond,
		BatchSize:     10,
	})
	defer w.Close()

	if w == nil {
		t.Fatal("NewWriter returned nil")
	}
}

// ---- Write tests ----

func TestWrite_SingleEvent(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    1000,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     100,
	})
	defer w.Close()

	err := w.Write(makeEvent(0))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
}

func TestWrite_BufferFull(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    2,
		FlushInterval: 10 * time.Second, // long interval to prevent flush
		BatchSize:     100,
	})
	defer w.Close()

	// Fill the buffer
	err := w.Write(makeEvent(0))
	if err != nil {
		t.Fatalf("Write 0 returned error: %v", err)
	}
	err = w.Write(makeEvent(1))
	if err != nil {
		t.Fatalf("Write 1 returned error: %v", err)
	}

	// Third write should fail (buffer full) since the flush loop
	// might not have drained yet with the long interval
	// We need to give the goroutine a moment to consume from the channel
	time.Sleep(50 * time.Millisecond)

	// Try to write many more - at least some should be dropped
	dropped := 0
	for i := 0; i < 100; i++ {
		if err := w.Write(makeEvent(i)); err != nil {
			dropped++
		}
	}
	if dropped == 0 {
		t.Error("expected some events to be dropped when buffer is full")
	}
}

// ---- WriteBatch tests ----

func TestWriteBatch(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    1000,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     100,
	})
	defer w.Close()

	events := make([]storage.EventRecord, 5)
	for i := range events {
		events[i] = makeEvent(i)
	}

	err := w.WriteBatch(events)
	if err != nil {
		t.Fatalf("WriteBatch returned error: %v", err)
	}
}

// ---- Close / flush tests ----

func TestClose_FlushesRemaining(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    1000,
		FlushInterval: 10 * time.Second, // long interval
		BatchSize:     100,
	})

	// Write some events
	for i := 0; i < 5; i++ {
		w.Write(makeEvent(i))
	}

	// Close should flush remaining
	if err := w.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	records := mock.getRecords()
	if len(records) < 5 {
		t.Errorf("expected at least 5 records after close, got %d", len(records))
	}
}

// ---- Flush by interval ----

func TestFlush_ByInterval(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    1000,
		FlushInterval: 100 * time.Millisecond,
		BatchSize:     100,
	})
	defer w.Close()

	for i := 0; i < 3; i++ {
		w.Write(makeEvent(i))
	}

	// Wait for interval-based flush
	time.Sleep(300 * time.Millisecond)

	records := mock.getRecords()
	if len(records) < 3 {
		t.Errorf("expected at least 3 records flushed by interval, got %d", len(records))
	}
}

// ---- Flush by batch size ----

func TestFlush_ByBatchSize(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    10000,
		FlushInterval: 10 * time.Second, // long interval
		BatchSize:     5,
	})
	defer w.Close()

	// Write more than batch size events
	for i := 0; i < 7; i++ {
		w.Write(makeEvent(i))
	}

	// Give the flush loop time to process
	time.Sleep(200 * time.Millisecond)

	records := mock.getRecords()
	if len(records) < 5 {
		t.Errorf("expected at least 5 records flushed by batch size, got %d", len(records))
	}
}

// ---- Stats tests ----

func TestStats(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    10000,
		FlushInterval: 100 * time.Millisecond,
		BatchSize:     100,
	})
	defer w.Close()

	stats := w.Stats()
	if stats == nil {
		t.Fatal("Stats returned nil")
	}

	// Should have flushed_count and dropped_count
	if _, ok := stats["flushed_count"]; !ok {
		t.Error("Stats missing flushed_count")
	}
	if _, ok := stats["dropped_count"]; !ok {
		t.Error("Stats missing dropped_count")
	}
	if _, ok := stats["buffer_size"]; !ok {
		t.Error("Stats missing buffer_size")
	}
}

func TestGetFlushedCount(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    10000,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     100,
	})

	for i := 0; i < 5; i++ {
		w.Write(makeEvent(i))
	}

	// Wait for flush
	time.Sleep(200 * time.Millisecond)
	w.Close()

	flushed := w.GetFlushedCount()
	if flushed < 5 {
		t.Errorf("GetFlushedCount = %d, want >= 5", flushed)
	}
}

func TestGetDroppedCount(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    2,
		FlushInterval: 10 * time.Second,
		BatchSize:     100,
	})
	defer w.Close()

	// Fill buffer then write more
	w.Write(makeEvent(0))
	w.Write(makeEvent(1))
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 50; i++ {
		w.Write(makeEvent(i + 2))
	}

	dropped := w.GetDroppedCount()
	if dropped == 0 {
		t.Error("expected some dropped events")
	}
}

// ---- Concurrent writes ----

func TestConcurrentWrites(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    10000,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     100,
	})

	var wg sync.WaitGroup
	const goroutines = 10
	const eventsPer = 50

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < eventsPer; i++ {
				w.Write(makeEvent(goroutineID*100 + i))
			}
		}(g)
	}

	wg.Wait()

	// Wait for all events to flush
	time.Sleep(500 * time.Millisecond)
	w.Close()

	totalWritten := goroutines * eventsPer
	flushed := w.GetFlushedCount()
	dropped := w.GetDroppedCount()

	if int(flushed)+int(dropped) != totalWritten {
		t.Errorf("flushed(%d) + dropped(%d) = %d, want %d",
			flushed, dropped, int(flushed)+int(dropped), totalWritten)
	}
}

// ---- GetBufferSize test ----

func TestGetBufferSize(t *testing.T) {
	mock := &mockEventStore{}
	w := NewWriter(mock, Config{
		BufferSize:    10000,
		FlushInterval: 10 * time.Second,
		BatchSize:     100,
	})
	defer w.Close()

	// Initially buffer size should be 0 or close to it
	size := w.GetBufferSize()
	if size < 0 {
		t.Errorf("GetBufferSize = %d, want >= 0", size)
	}
}
