package webhook

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/logmonitor/collector/storage"
)

// Mock implementations for testing

type mockResultStore struct {
	results      []storage.VerificationResult
	lastResult   *storage.VerificationResult
	shouldError bool
}

func (m *mockResultStore) StoreVerificationResult(result *storage.VerificationResult) error {
	if m.shouldError {
		return io.ErrClosedPipe
	}
	m.results = append(m.results, *result)
	m.lastResult = result
	return nil
}

func (m *mockResultStore) GetVerificationResults(site string, limit int) ([]storage.VerificationResult, error) {
	if m.shouldError {
		return nil, io.ErrClosedPipe
	}
	if limit > 0 && len(m.results) > limit {
		return m.results[:limit], nil
	}
	return m.results, nil
}

type mockAlerter struct {
	triggered       bool
	lastSite        string
	lastRelease     string
	lastScore       float64
	lastChecks      []storage.CheckResult
	shouldError     bool
}

func (m *mockAlerter) TriggerVerificationAlert(site, release string, score float64, checks []storage.CheckResult) {
	if m.shouldError {
		return
	}
	m.triggered = true
	m.lastSite = site
	m.lastRelease = release
	m.lastScore = score
	m.lastChecks = checks
}

// Helper function to create a test request
func createTestRequest(site, release, status string, score float64) *E2EVerifierRequest {
	checks := []storage.CheckResult{
		{ Name: "check1", Status: "pass", Message: "Check passed" },
		{ Name: "check2", Status: "fail", Message: "Check failed" },
	}
	return &E2EVerifierRequest{
		Site:      site,
		Release:   release,
		Status:    status,
		Score:     score,
		Checks:    checks,
		Timestamp: time.Now().UnixMilli(),
	}
}

// Test HandleVerificationResult

func TestHandleVerificationResult_ValidPassStatus(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	req := createTestRequest("test-site", "v1.0.0", "pass", 0.9)
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier", io.NopCloser(strings.NewReader(string(body))))
	r.Header.Set("X-API-Key", "test-api-key")
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if store.lastResult == nil {
		t.Fatal("Expected result to be stored")
	}

	if store.lastResult.Status != "pass" {
		t.Errorf("Expected status 'pass', got '%s'", store.lastResult.Status)
	}

	if alerter.triggered {
		t.Error("Alerter should not be triggered for pass status")
	}
}

func TestHandleVerificationResult_ValidFailStatus(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	req := createTestRequest("test-site", "v1.0.0", "fail", 0.3)
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier", io.NopCloser(strings.NewReader(string(body))))
	r.Header.Set("X-API-Key", "test-api-key")
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if !alerter.triggered {
		t.Error("Alerter should be triggered for fail status")
	}

	if alerter.lastSite != "test-site" {
		t.Errorf("Expected site 'test-site', got '%s'", alerter.lastSite)
	}
}

func TestHandleVerificationResult_MissingSiteField(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	req := createTestRequest("", "v1.0.0", "pass", 0.9)
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier", io.NopCloser(strings.NewReader(string(body))))
	r.Header.Set("X-API-Key", "test-api-key")
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleVerificationResult_MissingReleaseField(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	req := &E2EVerifierRequest{
		Site:      "test-site",
		Release:   "",
		Status:    "pass",
		Score:     0.9,
		Timestamp: time.Now().UnixMilli(),
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier", io.NopCloser(strings.NewReader(string(body))))
	r.Header.Set("X-API-Key", "test-api-key")
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleVerificationResult_InvalidStatus(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	req := &E2EVerifierRequest{
		Site:      "test-site",
		Release:   "v1.0.0",
		Status:    "invalid",
		Score:     0.5,
		Timestamp: time.Now().UnixMilli(),
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier", io.NopCloser(strings.NewReader(string(body))))
	r.Header.Set("X-API-Key", "test-api-key")
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleVerificationResult_WrongAPIKey(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("correct-api-key", store, alerter)

	req := createTestRequest("test-site", "v1.0.0", "pass", 0.9)
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier", io.NopCloser(strings.NewReader(string(body))))
	r.Header.Set("X-API-Key", "wrong-api-key")
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHandleVerificationResult_APIKeyInQuery(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	req := createTestRequest("test-site", "v1.0.0", "pass", 0.9)
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier?api_key=test-api-key", io.NopCloser(strings.NewReader(string(body))))
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleVerificationResult_WrongHTTPMethod(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	r := httptest.NewRequest("GET", "/api/webhooks/e2e-verifier", nil)
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleVerificationResult_InvalidJSON(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier", io.NopCloser(strings.NewReader("invalid json")))
	r.Header.Set("X-API-Key", "test-api-key")
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleVerificationResult_NoAPIKeyConfigured(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("", store, alerter) // No API key configured

	req := createTestRequest("test-site", "v1.0.0", "pass", 0.9)
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier", io.NopCloser(strings.NewReader(string(body))))
	// No API key provided
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (no auth when no API key configured), got %d", w.Code)
	}
}

func TestHandleVerificationResult_HasSignatureVerification(t *testing.T) {
	// This test verifies the signature verification logic exists
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	req := createTestRequest("test-site", "v1.0.0", "pass", 0.9)
	body, _ := json.Marshal(req)

	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier", io.NopCloser(strings.NewReader(string(body))))
	r.Header.Set("X-API-Key", "test-api-key")
	r.Header.Set("X-Signature", "invalid-signature")
	w := httptest.NewRecorder()

	handler.HandleVerificationResult(w, r)

	// Should reject invalid signature
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for invalid signature, got %d", w.Code)
	}
}

// Test GetVerificationResults

func TestGetVerificationResults_ValidQuery(t *testing.T) {
	store := &mockResultStore{
		results: []storage.VerificationResult{
			{ Site: "site1", Status: "pass", Timestamp: time.Now().UnixMilli() },
			{ Site: "site1", Status: "fail", Timestamp: time.Now().UnixMilli() },
		},
	}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	r := httptest.NewRequest("GET", "/api/webhooks/e2e-verifier/results?site=site1", nil)
	w := httptest.NewRecorder()

	handler.GetVerificationResults(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["site"] != "site1" {
		t.Errorf("Expected site 'site1', got %v", response["site"])
	}

	count := int(response["count"].(float64))
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}

func TestGetVerificationResults_LimitParameter(t *testing.T) {
	store := &mockResultStore{
		results: []storage.VerificationResult{
			{ Site: "site1", Status: "pass", Timestamp: time.Now().UnixMilli() },
			{ Site: "site1", Status: "fail", Timestamp: time.Now().UnixMilli() },
			{ Site: "site1", Status: "pass", Timestamp: time.Now().UnixMilli() },
		},
	}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	r := httptest.NewRequest("GET", "/api/webhooks/e2e-verifier/results?site=site1&limit=2", nil)
	w := httptest.NewRecorder()

	handler.GetVerificationResults(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	count := int(response["count"].(float64))
	if count != 2 {
		t.Errorf("Expected count 2 (limited), got %d", count)
	}
}

func TestGetVerificationResults_WrongHTTPMethod(t *testing.T) {
	store := &mockResultStore{}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	r := httptest.NewRequest("POST", "/api/webhooks/e2e-verifier/results", nil)
	w := httptest.NewRecorder()

	handler.GetVerificationResults(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestGetVerificationResults_InvalidLimit(t *testing.T) {
	store := &mockResultStore{
		results: []storage.VerificationResult{
			{ Site: "site1", Status: "pass", Timestamp: time.Now().UnixMilli() },
		},
	}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	// Test with invalid limit (should default to 10)
	r := httptest.NewRequest("GET", "/api/webhooks/e2e-verifier/results?site=site1&limit=invalid", nil)
	w := httptest.NewRecorder()

	handler.GetVerificationResults(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetVerificationResults_EmptyResults(t *testing.T) {
	store := &mockResultStore{results: []storage.VerificationResult{}}
	alerter := &mockAlerter{}
	handler := NewE2EVerifierHook("test-api-key", store, alerter)

	r := httptest.NewRequest("GET", "/api/webhooks/e2e-verifier/results?site=nosuchsite", nil)
	w := httptest.NewRecorder()

	handler.GetVerificationResults(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	count := int(response["count"].(float64))
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}
}

// Test verifyAPIKey

func TestVerifyAPIKey_CorrectKey(t *testing.T) {
	handler := NewE2EVerifierHook("test-key", nil, nil)
	if !handler.verifyAPIKey("test-key") {
		t.Error("Expected API key verification to succeed")
	}
}

func TestVerifyAPIKey_WrongKey(t *testing.T) {
	handler := NewE2EVerifierHook("test-key", nil, nil)
	if handler.verifyAPIKey("wrong-key") {
		t.Error("Expected API key verification to fail")
	}
}

func TestVerifyAPIKey_EmptyKey(t *testing.T) {
	handler := NewE2EVerifierHook("", nil, nil) // No key configured
	if !handler.verifyAPIKey("") {
		t.Error("Expected API key verification to succeed when no key is configured")
	}
	if !handler.verifyAPIKey("any-key") {
		t.Error("Expected any key to be accepted when no key is configured")
	}
}
