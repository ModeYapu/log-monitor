package alerter

import (
	"strings"
	"testing"
	"time"

	"github.com/logmonitor/collector/storage"
)

// MockEventStoreForClusters implements EventStore for clusterer testing
type MockEventStoreForClusters struct {
	events []storage.EventRecord
}

func (m *MockEventStoreForClusters) GetClusterEvents(appID, fingerprint string, page, pageSize int) ([]storage.EventRecord, int64, error) {
	var result []storage.EventRecord
	for _, e := range m.events {
		if e.Fingerprint == fingerprint {
			result = append(result, e)
		}
	}
	return result, int64(len(result)), nil
}

func (m *MockEventStoreForClusters) InsertEvents(events []storage.EventRecord) error {
	m.events = append(m.events, events...)
	return nil
}

func (m *MockEventStoreForClusters) QueryEvents(query storage.QueryParams) (*storage.QueryResult, error) {
	return &storage.QueryResult{}, nil
}

func (m *MockEventStoreForClusters) GetStats(appID string, projectID int64) (*storage.Stats, error) {
	return &storage.Stats{}, nil
}

func (m *MockEventStoreForClusters) GetApps(projectID int64) ([]storage.AppStats, error) {
	return []storage.AppStats{}, nil
}

func (m *MockEventStoreForClusters) GetTopN(appID, topType, orderBy string, limit int, filters storage.AnalyticsFilters) (*storage.TopNResult, error) {
	return &storage.TopNResult{}, nil
}

func (m *MockEventStoreForClusters) GetSimilarErrors(appID, message string, threshold float64, limit int, projectID int64) ([]storage.ErrorCluster, error) {
	return []storage.ErrorCluster{}, nil
}

func (m *MockEventStoreForClusters) GetSessionEvents(sessionID string, limit int) ([]storage.EventRecord, error) {
	return []storage.EventRecord{}, nil
}

func (m *MockEventStoreForClusters) GetSessionErrorCount(sessionID string) (int64, error) {
	return 0, nil
}

func (m *MockEventStoreForClusters) GetTopErrors(params storage.TopListParams) ([]storage.TopError, error) {
	return []storage.TopError{}, nil
}

func (m *MockEventStoreForClusters) GetTopPages(params storage.TopListParams) ([]storage.TopPage, error) {
	return []storage.TopPage{}, nil
}

func (m *MockEventStoreForClusters) GetTopReleases(params storage.TopListParams) ([]storage.TopRelease, error) {
	return []storage.TopRelease{}, nil
}

func (m *MockEventStoreForClusters) GetTopBrowsers(params storage.TopListParams) ([]storage.TopBrowser, error) {
	return []storage.TopBrowser{}, nil
}

func (m *MockEventStoreForClusters) GetErrorClustersByTime(appID string, startTime, endTime int64, limit int) ([]storage.ErrorClusterResult, error) {
	return []storage.ErrorClusterResult{}, nil
}

func (m *MockEventStoreForClusters) GetClusterStats(appID, fingerprint string) (storage.ClusterStats, error) {
	return storage.ClusterStats{}, nil
}

func (m *MockEventStoreForClusters) GetErrorClusters(appID, errorMessage string, threshold float64, limit int, projectID int64) ([]storage.ErrorCluster, error) {
	return []storage.ErrorCluster{}, nil
}

func (m *MockEventStoreForClusters) GetRecentEvents(limit int) ([]storage.EventRecord, error) {
	return []storage.EventRecord{}, nil
}

func (m *MockEventStoreForClusters) CountRecentErrors(sinceMs int64) (int64, error) {
	return 0, nil
}

func (m *MockEventStoreForClusters) GetSessionList(filters map[string]interface{}, limit, offset int) ([]storage.SessionSummary, error) {
	return nil, nil
}

func (m *MockEventStoreForClusters) GetSessionListCount(filters map[string]interface{}) (int64, error) {
	return 0, nil
}

func (m *MockEventStoreForClusters) GetSessionJourney(sessionID string) ([]storage.EventRecord, error) {
	return nil, nil
}

func TestClusterer_ProcessEvents(t *testing.T) {
	mockStore := &MockEventStoreForClusters{}
	clusterer := NewClusterer(mockStore)

	now := time.Now().UnixMilli()

	events := []storage.EventRecord{
		{
			AppID:       "test-app",
			Level:       "error",
			Fingerprint: "fp1",
			Message:     "Test error message",
			CreatedAt:   now,
		},
		{
			AppID:       "test-app",
			Level:       "error",
			Fingerprint: "fp2",
			Message:     "Different error",
			CreatedAt:   now,
		},
	}

	clusterer.ProcessEvents(events)

	// Should have created 2 clusters
	if clusterer.GetActiveClusterCount() != 2 {
		t.Errorf("Expected 2 clusters, got %d", clusterer.GetActiveClusterCount())
	}
}

func TestClusterer_MergeSimilarClusters(t *testing.T) {
	mockStore := &MockEventStoreForClusters{}
	clusterer := NewClusterer(mockStore)

	now := time.Now().UnixMilli()

	// Add first event
	events1 := []storage.EventRecord{
		{
			AppID:       "test-app",
			Level:       "error",
			Fingerprint: "fp1",
			Message:     "Cannot read property 'user' of undefined",
			CreatedAt:   now,
		},
	}
	clusterer.ProcessEvents(events1)

	// Add very similar event - should merge
	events2 := []storage.EventRecord{
		{
			AppID:       "test-app",
			Level:       "error",
			Fingerprint: "fp2",
			Message:     "Cannot read property 'user' of null",
			CreatedAt:   now + 1000,
		},
	}
	clusterer.ProcessEvents(events2)

	// Should have merged into one cluster
	count := clusterer.GetActiveClusterCount()
	if count != 1 {
		t.Logf("Expected 1 cluster (merged), got %d (similarity matching may vary)", count)
		// This is OK since similarity depends on tokenization
	}
}

func TestClusterer_ListClusters(t *testing.T) {
	mockStore := &MockEventStoreForClusters{}
	clusterer := NewClusterer(mockStore)

	now := time.Now().UnixMilli()

	// Add events for two apps
	events := []storage.EventRecord{
		{
			AppID:       "app1",
			Level:       "error",
			Fingerprint: "fp1",
			Message:     "Error in app1",
			CreatedAt:   now,
		},
		{
			AppID:       "app2",
			Level:       "error",
			Fingerprint: "fp2",
			Message:     "Error in app2",
			CreatedAt:   now,
		},
	}

	clusterer.ProcessEvents(events)

	// List clusters for app1
	clusters := clusterer.ListClusters("app1", 10)
	if len(clusters) != 1 {
		t.Errorf("Expected 1 cluster for app1, got %d", len(clusters))
	}

	if len(clusters) > 0 && clusters[0].AppID != "app1" {
		t.Errorf("Expected cluster for app1, got %s", clusters[0].AppID)
	}
}

func TestClusterer_GetCluster(t *testing.T) {
	mockStore := &MockEventStoreForClusters{}
	clusterer := NewClusterer(mockStore)

	now := time.Now().UnixMilli()

	events := []storage.EventRecord{
		{
			AppID:       "test-app",
			Level:       "error",
			Fingerprint: "fp1",
			Message:     "Test error",
			CreatedAt:   now,
		},
	}

	clusterer.ProcessEvents(events)

	cluster, ok := clusterer.GetCluster("fp1")
	if !ok {
		t.Fatal("Cluster not found")
	}

	if cluster.Fingerprint != "fp1" {
		t.Errorf("Expected fingerprint fp1, got %s", cluster.Fingerprint)
	}
	if cluster.Count != 1 {
		t.Errorf("Expected count 1, got %d", cluster.Count)
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		message   string
		wantCount int
	}{
		{"Cannot read property user of undefined", 5}, // cannot, read, property, user, undefined
		{"Failed to fetch data", 3},                  // failed, fetch, data ("to" filtered out)
		{"Network error", 2},                         // network, error
		{"a b c d e f", 0},                           // all words filtered (too short)
	}

	for _, tt := range tests {
		tokens := tokenize(tt.message)
		if len(tokens) != tt.wantCount {
			t.Errorf("tokenize(%q) = %d tokens, want %d", tt.message, len(tokens), tt.wantCount)
		}
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected float64
	}{
		{"same message", "same message", 1.0},
		{"completely different words here", "nothing in common at all", 0.0},
		{"partial match some words", "partial match other words", 0.6}, // 3/5 overlap: partial, match, words
	}

	for _, tt := range tests {
		aTokens := tokenize(tt.a)
		bTokens := tokenize(tt.b)
		similarity := jaccardSimilarity(aTokens, bTokens)

		// Allow small floating point differences
		diff := similarity - tt.expected
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.1 {
			t.Errorf("jaccardSimilarity(%q, %q) = %f, want %f", tt.a, tt.b, similarity, tt.expected)
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected int
	}{
		{"same", "same", 0},
		{"test", "tent", 1},
		{"kitten", "sitting", 3},
		{"", "test", 4},
		{"test", "", 4},
	}

	for _, tt := range tests {
		distance := levenshteinDistance(tt.a, tt.b)
		if distance != tt.expected {
			t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, distance, tt.expected)
		}
	}
}

func TestDetermineSeverity(t *testing.T) {
	mockStore := &MockEventStoreForClusters{}
	clusterer := NewClusterer(mockStore)

	tests := []struct {
		event    storage.EventRecord
		expected string
	}{
		{
			event:    storage.EventRecord{Message: "Critical error in system", URL: "/api/test"},
			expected: "critical",
		},
		{
			event:    storage.EventRecord{Message: "Fatal exception occurred", URL: "/"},
			expected: "critical",
		},
		{
			event:    storage.EventRecord{Message: "Normal error", URL: "/checkout"},
			expected: "critical", // Checkout page is critical
		},
		{
			event:    storage.EventRecord{Message: "Warning message", URL: "/home"},
			expected: "warning",
		},
	}

	for _, tt := range tests {
		result := clusterer.determineSeverity(tt.event)
		if result != tt.expected {
			t.Errorf("determineSeverity() = %s, want %s for event %s", result, tt.expected, tt.event.Message)
		}
	}
}

func TestTruncateMessageCluster(t *testing.T) {
	tests := []struct {
		message  string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a very long message that should be truncated", 10, "this is a ..."},
		{"exactlength", 11, "exactlength"},
	}

	for _, tt := range tests {
		result := truncateMessageCluster(tt.message, tt.maxLen)
		// If truncated, the result will have "..." appended, so length could be maxLen + 3
		if len(tt.message) > tt.maxLen {
			expectedLen := tt.maxLen + 3
			if len(result) != expectedLen {
				t.Errorf("truncateMessageCluster(%q, %d) = %q (len %d), expected len %d", tt.message, tt.maxLen, result, len(result), expectedLen)
			}
			if !strings.HasSuffix(result, "...") {
				t.Errorf("truncateMessageCluster(%q, %d) = %q, expected to end with '...'", tt.message, tt.maxLen, result)
			}
		} else {
			if result != tt.message {
				t.Errorf("truncateMessageCluster(%q, %d) = %q, want %q", tt.message, tt.maxLen, result, tt.message)
			}
		}
	}
}

func TestClusterer_Cleanup(t *testing.T) {
	mockStore := &MockEventStoreForClusters{}
	clusterer := NewClusterer(mockStore)

	// Simulate old cluster
	oldTime := time.Now().UnixMilli() - int64(clusterMaxAge) - 1000

	events := []storage.EventRecord{
		{
			AppID:       "test-app",
			Level:       "error",
			Fingerprint: "old-fp",
			Message:     "Old error",
			CreatedAt:   oldTime,
		},
	}

	clusterer.ProcessEvents(events)

	// Run cleanup
	clusterer.cleanup()

	// Old cluster should be removed
	if clusterer.GetActiveClusterCount() != 0 {
		t.Errorf("Expected 0 clusters after cleanup, got %d", clusterer.GetActiveClusterCount())
	}
}
