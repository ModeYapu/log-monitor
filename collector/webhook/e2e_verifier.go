package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/logmonitor/collector/storage"
)

// E2EVerifierHook handles incoming webhooks from E2E Verifier
type E2EVerifierHook struct {
	apiKey  string // Simple API key for authentication
	store   VerificationResultStore
	alerter VerificationAlerter
}

// VerificationResultStore defines the interface for storing verification results
type VerificationResultStore interface {
	StoreVerificationResult(result *storage.VerificationResult) error
	GetVerificationResults(site string, limit int) ([]storage.VerificationResult, error)
}

// VerificationAlerter defines the interface for alerting on failed verifications
type VerificationAlerter interface {
	TriggerVerificationAlert(site, release string, score float64, checks []storage.CheckResult)
}

// E2EVerifierRequest represents the incoming webhook payload
type E2EVerifierRequest struct {
	Site      string                  `json:"site"`
	Release   string                  `json:"release"`
	Status    string                  `json:"status"`
	Score     float64                 `json:"score"`
	Checks    []storage.CheckResult   `json:"checks"`
	Timestamp int64                   `json:"timestamp"`
}

// NewE2EVerifierHook creates a new E2E verifier webhook handler
func NewE2EVerifierHook(apiKey string, store VerificationResultStore, alerter VerificationAlerter) *E2EVerifierHook {
	return &E2EVerifierHook{
		apiKey:  apiKey,
		store:   store,
		alerter: alerter,
	}
}

// HandleVerificationResult processes incoming E2E verification results
// POST /api/webhooks/e2e-verifier
func (h *E2EVerifierHook) HandleVerificationResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Verify API key authentication
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		// Also try query parameter
		apiKey = r.URL.Query().Get("api_key")
	}
	if !h.verifyAPIKey(apiKey) {
		slog.Warn("E2E verifier webhook: invalid API key", "remote_addr", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Optional: Verify HMAC signature if provided
	if signature := r.Header.Get("X-Signature"); signature != "" {
		if !h.verifySignature(r, signature) {
			slog.Warn("E2E verifier webhook: invalid signature", "remote_addr", r.RemoteAddr)
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Failed to read E2E verifier webhook body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse request
	var req E2EVerifierRequest
	if err := json.Unmarshal(body, &req); err != nil {
		slog.Error("Failed to parse E2E verifier webhook", "error", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Site == "" {
		http.Error(w, "Missing site field", http.StatusBadRequest)
		return
	}
	if req.Release == "" {
		http.Error(w, "Missing release field", http.StatusBadRequest)
		return
	}
	if req.Status != "pass" && req.Status != "fail" {
		http.Error(w, "Invalid status (must be 'pass' or 'fail')", http.StatusBadRequest)
		return
	}

	// Set timestamp if not provided
	if req.Timestamp == 0 {
		req.Timestamp = time.Now().UnixMilli()
	}

	// Create verification result
	result := &storage.VerificationResult{
		Site:      req.Site,
		Release:   req.Release,
		Status:    req.Status,
		Score:     req.Score,
		Checks:    req.Checks,
		Timestamp: req.Timestamp,
		CreatedAt: time.Now().UnixMilli(),
	}

	// Store the result
	if err := h.store.StoreVerificationResult(result); err != nil {
		slog.Error("Failed to store verification result", "error", err)
		http.Error(w, "Failed to store verification result", http.StatusInternalServerError)
		return
	}

	slog.Info("E2E verification result received",
		"site", req.Site,
		"release", req.Release,
		"status", req.Status,
		"score", req.Score)

	// Trigger alert if verification failed
	if req.Status == "fail" && h.alerter != nil {
		h.alerter.TriggerVerificationAlert(req.Site, req.Release, req.Score, req.Checks)
	}

	// Respond with success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Verification result stored",
	})
}

// GetVerificationResults returns verification results for a site
// GET /api/webhooks/e2e-verifier/results?site=travel-planner&limit=10
func (h *E2EVerifierHook) GetVerificationResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	site := r.URL.Query().Get("site")
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	results, err := h.store.GetVerificationResults(site, limit)
	if err != nil {
		slog.Error("Failed to get verification results", "error", err)
		http.Error(w, "Failed to get verification results", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"site":    site,
		"count":   len(results),
		"results": results,
	})
}

// verifyAPIKey verifies the provided API key
func (h *E2EVerifierHook) verifyAPIKey(apiKey string) bool {
	if h.apiKey == "" {
		// If no API key configured, allow all requests (for development)
		return true
	}
	// Constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(apiKey), []byte(h.apiKey))
}

// verifySignature verifies the HMAC signature of the request
func (h *E2EVerifierHook) verifySignature(r *http.Request, signature string) bool {
	// Read body for signature verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	// Restore body for subsequent reads
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	// Calculate expected signature
	expected := hmacSHA256(h.apiKey, string(body))

	// Secure comparison
	return hmac.Equal([]byte(signature), []byte(expected))
}

// hmacSHA256 calculates HMAC-SHA256
func hmacSHA256(key, data string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
