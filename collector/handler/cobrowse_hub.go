package handler

import (
	"context"
	"github.com/logmonitor/collector/middleware"
	"github.com/logmonitor/collector/model"
	"github.com/logmonitor/collector/storage"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type CoBrowseHub struct {
	sessions       map[string]*SessionHub
	mu             sync.RWMutex
	db             CoBrowseDB
	auth           *middleware.AuthConfig
	allowedOrigins []string
	maxSessions    int // Maximum sessions to prevent resource exhaustion
}

type CoBrowseDB interface {
	CreateRecording(recording storage.RecordingInfo) (int64, error)
	AddRecordingEvent(sessionID string, seq int, timestamp int64, eventData []byte) error
	GetRecording(sessionID string) (*storage.RecordingInfo, error)
	GetRecordings(limit, offset int, filters map[string]interface{}) ([]storage.RecordingInfo, error)
	GetRecordingEvents(sessionID string, limit, offset int) ([]storage.RecordingEventData, error)
	GetRecordingStats(sessionID string) (interface{}, error)
	GetRecordingTimeline(sessionID string) ([]storage.TimelineEvent, error)
	DeleteRecording(sessionID string) error
	UpdateRecording(sessionID string, endTime int64, durationMs int64, eventCount int, status string) error
	GetSessionEvents(sessionID string, limit int) ([]storage.EventRecord, error)
	GetSessionErrorCount(sessionID string) (int64, error)
}

func NewCoBrowseHub(db CoBrowseDB) *CoBrowseHub {
	return &CoBrowseHub{
		sessions:    make(map[string]*SessionHub),
		maxSessions: 1000, // Limit concurrent sessions to prevent resource exhaustion
		db:          db,
		auth:        middleware.NewAuthConfig(),
	}
}

// SetAuthConfig sets the authentication configuration
func (h *CoBrowseHub) SetAuthConfig(auth *middleware.AuthConfig) {
	h.auth = auth
}

// SetAllowedOrigins configures allowed browser origins for admin viewer connections.
func (h *CoBrowseHub) SetAllowedOrigins(origins []string) {
	h.allowedOrigins = append([]string(nil), origins...)
}

// Close closes all active sessions
func (h *CoBrowseHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, hub := range h.sessions {
		hub.close()
	}
	h.sessions = make(map[string]*SessionHub)
}

// GetLiveSessions returns all currently active sessions
func (h *CoBrowseHub) GetLiveSessions() []model.LiveSession {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var sessions []model.LiveSession
	for _, hub := range h.sessions {
		hub.mu.RLock()
		if !hub.closed {
			sessions = append(sessions, model.LiveSession{
				SessionID:    hub.sessionID,
				AppID:        hub.appID,
				URL:          hub.url,
				UA:           hub.ua,
				ConnectedAt:  hub.startTime,
				ViewerCount:  len(hub.viewerConns),
				IsControlled: false,
			})
		}
		hub.mu.RUnlock()
	}
	return sessions
}

func (h *CoBrowseHub) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ws/cobrowse/", func(w http.ResponseWriter, r *http.Request) {
		// Extract session ID from path
		// Path format: /ws/cobrowse/{sessionId} or /ws/cobrowse/{sessionId}/view
		path := r.URL.Path

		// Normalize path
		path = strings.TrimPrefix(path, "/ws/cobrowse/")
		if path == "" {
			http.Error(w, "Session ID required", http.StatusBadRequest)
			return
		}

		// Check if viewer connection
		if strings.HasSuffix(path, "/view") {
			sessionID := strings.TrimSuffix(path, "/view")
			r = r.WithContext(context.WithValue(r.Context(), "sessionId", sessionID))
			h.HandleViewerConnection(w, r)
			return
		}

		// User connection
		sessionID := path
		r = r.WithContext(context.WithValue(r.Context(), "sessionId", sessionID))
		h.HandleUserConnection(w, r)
	})
}

func (h *CoBrowseHub) isAllowedViewerOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	if len(h.allowedOrigins) == 0 {
		parsed, err := url.Parse(origin)
		return err == nil && strings.EqualFold(parsed.Host, r.Host)
	}

	for _, allowedOrigin := range h.allowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
	}

	return false
}

// getSessionIDFromRequest extracts session ID from request
func getSessionIDFromRequest(r *http.Request) string {
	// Try context first
	if sessionID := r.Context().Value("sessionId"); sessionID != nil {
		if s, ok := sessionID.(string); ok {
			return s
		}
	}
	// Try PathValue (Go 1.22+)
	if sessionID := r.PathValue("sessionId"); sessionID != "" {
		return sessionID
	}
	// Fallback to query param
	return r.URL.Query().Get("sessionId")
}
