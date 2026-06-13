package alerter

import (
	"testing"
	"time"

	"github.com/logmonitor/collector/storage"
)

// MockAlertStore is a mock implementation of AlertStore for testing
type MockAlertStore struct {
	rules      []storage.AlertRule
	alertLogs   []storage.AlertLog
	createRule func(rule storage.AlertRule) (int64, error)
	getRules   func(appID string) ([]storage.AlertRule, error)
	getAllRules func() ([]storage.AlertRule, error)
	updateRule func(id int64, timestamp int64) error
	deleteRule func(id int64) error
	createLog  func(log storage.AlertLog) error
	getLogs    func(appID string, limit int) ([]storage.AlertLog, error)
}

func (m *MockAlertStore) CreateAlertRule(rule storage.AlertRule) (int64, error) {
	if m.createRule != nil {
		return m.createRule(rule)
	}
	id := int64(len(m.rules) + 1)
	rule.ID = id
	m.rules = append(m.rules, rule)
	return id, nil
}

func (m *MockAlertStore) GetAlertRules(appID string) ([]storage.AlertRule, error) {
	if m.getRules != nil {
		return m.getRules(appID)
	}
	var result []storage.AlertRule
	for _, r := range m.rules {
		if r.AppID == appID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *MockAlertStore) GetAllAlertRules() ([]storage.AlertRule, error) {
	if m.getAllRules != nil {
		return m.getAllRules()
	}
	return m.rules, nil
}

func (m *MockAlertStore) UpdateAlertRuleLastTriggered(id int64, timestamp int64) error {
	if m.updateRule != nil {
		return m.updateRule(id, timestamp)
	}
	for i, r := range m.rules {
		if r.ID == id {
			m.rules[i].LastTriggeredAt = timestamp
			return nil
		}
	}
	return nil
}

func (m *MockAlertStore) DeleteAlertRule(id int64) error {
	if m.deleteRule != nil {
		return m.deleteRule(id)
	}
	for i, r := range m.rules {
		if r.ID == id {
			m.rules = append(m.rules[:i], m.rules[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *MockAlertStore) SilenceAlertRule(id int64, until int64) error {
	return nil
}

func (m *MockAlertStore) UnsilenceAlertRule(id int64) error {
	return nil
}

func (m *MockAlertStore) CreateAlertLog(log storage.AlertLog) error {
	if m.createLog != nil {
		return m.createLog(log)
	}
	log.ID = int64(len(m.alertLogs) + 1)
	m.alertLogs = append(m.alertLogs, log)
	return nil
}

func (m *MockAlertStore) GetAlertLogs(appID string, limit int) ([]storage.AlertLog, error) {
	if m.getLogs != nil {
		return m.getLogs(appID, limit)
	}
	return m.alertLogs, nil
}

// MockEventStore is a mock implementation of EventStore for testing
type MockEventStore struct {
	queryResult *storage.QueryResult
	queryCount  int
}

func (m *MockEventStore) InsertEvents(events []storage.EventRecord) error {
	return nil
}

func (m *MockEventStore) QueryEvents(query storage.QueryParams) (*storage.QueryResult, error) {
	m.queryCount++
	if m.queryResult != nil {
		return m.queryResult, nil
	}
	return &storage.QueryResult{Total: 0, Data: []storage.EventRecord{}}, nil
}

func (m *MockEventStore) GetStats(appID string, projectID int64) (*storage.Stats, error) {
	return &storage.Stats{}, nil
}

func (m *MockEventStore) GetApps(projectID int64) ([]storage.AppStats, error) {
	return []storage.AppStats{}, nil
}

func (m *MockEventStore) GetTopN(appID, topType, orderBy string, limit int, filters storage.AnalyticsFilters) (*storage.TopNResult, error) {
	return &storage.TopNResult{}, nil
}

func (m *MockEventStore) GetSimilarErrors(appID, message string, threshold float64, limit int, projectID int64) ([]storage.ErrorCluster, error) {
	return []storage.ErrorCluster{}, nil
}

func (m *MockEventStore) GetSessionEvents(sessionID string, limit int) ([]storage.EventRecord, error) {
	return []storage.EventRecord{}, nil
}

func (m *MockEventStore) GetSessionErrorCount(sessionID string) (int64, error) {
	return 0, nil
}

func (m *MockEventStore) GetTopErrors(params storage.TopListParams) ([]storage.TopError, error) {
	return []storage.TopError{}, nil
}

func (m *MockEventStore) GetTopPages(params storage.TopListParams) ([]storage.TopPage, error) {
	return []storage.TopPage{}, nil
}

func (m *MockEventStore) GetTopReleases(params storage.TopListParams) ([]storage.TopRelease, error) {
	return []storage.TopRelease{}, nil
}

func (m *MockEventStore) GetTopBrowsers(params storage.TopListParams) ([]storage.TopBrowser, error) {
	return []storage.TopBrowser{}, nil
}

func (m *MockEventStore) GetErrorClustersByTime(appID string, startTime, endTime int64, limit int) ([]storage.ErrorClusterResult, error) {
	return []storage.ErrorClusterResult{}, nil
}

func (m *MockEventStore) GetClusterEvents(appID, fingerprint string, page, pageSize int) ([]storage.EventRecord, int64, error) {
	return []storage.EventRecord{}, 0, nil
}

func (m *MockEventStore) GetClusterStats(appID, fingerprint string) (storage.ClusterStats, error) {
	return storage.ClusterStats{}, nil
}

func (m *MockEventStore) GetErrorClusters(appID, errorMessage string, threshold float64, limit int, projectID int64) ([]storage.ErrorCluster, error) {
	return []storage.ErrorCluster{}, nil
}

func (m *MockEventStore) GetRecentEvents(limit int) ([]storage.EventRecord, error) {
	return []storage.EventRecord{}, nil
}

func TestRuleEngine_LoadRules(t *testing.T) {
	mockStore := &MockAlertStore{
		rules: []storage.AlertRule{
			{
				ID:              1,
				AppID:           "test-app",
				Name:            "Test Rule",
				ConditionType:   "threshold",
				ConditionConfig: `{"metric":"error_count","value":100,"operator":">=","windowMin":5}`,
				Enabled:         1,
				CooldownMinutes: 30,
			},
		},
	}

	mockEvents := &MockEventStore{}

	engine := NewRuleEngine(mockStore, mockEvents)
	err := engine.LoadRules()
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	if len(engine.rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(engine.rules))
	}
}

func TestRuleEngine_EvaluateThreshold(t *testing.T) {
	mockStore := &MockAlertStore{}
	mockEvents := &MockEventStore{
		queryResult: &storage.QueryResult{
			Total: 150,
			Data:  []storage.EventRecord{},
		},
	}

	engine := NewRuleEngine(mockStore, mockEvents)

	rule := &Rule{
		ID:       1,
		AppID:    "test-app",
		Name:     "Test Threshold",
		Condition: RuleCondition{
			Type:      "threshold",
			Metric:    "error_count",
			Operator:  ">=",
			Value:     100,
			WindowMin: 5,
		},
		Severity:        "critical",
		CooldownMinutes: 30,
		Enabled:         true,
	}

	events := []storage.EventRecord{}
	now := time.Now().UnixMilli()
	// Create 150 events to exceed the threshold of 100
	for i := 0; i < 150; i++ {
		events = append(events, storage.EventRecord{
			AppID:     "test-app",
			Level:     "error",
			CreatedAt: now,
		})
	}

	triggered, message := engine.evaluateThreshold(rule, events)
	if !triggered {
		t.Error("Expected threshold rule to trigger")
	}
	if message == "" {
		t.Error("Expected non-empty message")
	}
}

func TestRuleEngine_EvaluateTrend(t *testing.T) {
	mockStore := &MockAlertStore{}
	mockEvents := &MockEventStore{}

	engine := NewRuleEngine(mockStore, mockEvents)

	rule := &Rule{
		ID:       1,
		AppID:    "test-app",
		Name:     "Test Trend",
		Condition: RuleCondition{
			Type:       "trend",
			Metric:     "error_count",
			TrendDir:   "up",
			TrendCount: 3,
			WindowMin:  5,
		},
		Severity:        "warning",
		CooldownMinutes: 30,
		Enabled:         true,
	}

	events := []storage.EventRecord{}

	triggered, _ := engine.evaluateTrend(rule, events)
	// Should not trigger without enough data points
	if triggered {
		t.Error("Expected trend rule not to trigger without sufficient data")
	}
}

func TestRuleEngine_CompareValues(t *testing.T) {
	engine := &RuleEngine{}

	tests := []struct {
		left     float64
		operator string
		right    float64
		expected bool
	}{
		{10, ">", 5, true},
		{5, ">", 10, false},
		{10, ">=", 10, true},
		{10, "<", 20, true},
		{20, "<", 10, false},
		{10, "<=", 10, true},
		{10, "==", 10, true},
		{10, "==", 5, false},
	}

	for _, tt := range tests {
		result := engine.compareValues(tt.left, tt.operator, tt.right)
		if result != tt.expected {
			t.Errorf("compareValues(%.2f %s %.2f) = %v, want %v",
				tt.left, tt.operator, tt.right, result, tt.expected)
		}
	}
}

func TestRuleToStorage(t *testing.T) {
	rule := &Rule{
		ID:              1,
		AppID:           "test-app",
		Name:            "Test Rule",
		Condition: RuleCondition{
			Type:    "threshold",
			Metric:  "error_count",
			Operator: ">=",
			Value:   100,
		},
		Severity:        "critical",
		CooldownMinutes: 30,
		Enabled:         true,
		LastTriggeredAt: 1234567890,
		CreatedAt:       1234567890,
		Channels:        []string{"channel1"},
	}

	storageRule := RuleToStorage(rule)

	if storageRule.ID != rule.ID {
		t.Errorf("Expected ID %d, got %d", rule.ID, storageRule.ID)
	}
	if storageRule.AppID != rule.AppID {
		t.Errorf("Expected AppID %s, got %s", rule.AppID, storageRule.AppID)
	}
	if storageRule.Enabled != 1 {
		t.Errorf("Expected Enabled 1, got %d", storageRule.Enabled)
	}
}

func TestBoolToInt(t *testing.T) {
	if boolToInt(true) != 1 {
		t.Error("boolToInt(true) should return 1")
	}
	if boolToInt(false) != 0 {
		t.Error("boolToInt(false) should return 0")
	}
}
