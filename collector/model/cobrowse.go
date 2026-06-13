package model

import (
	"encoding/json"
	"time"
)

// CoBrowseMessage represents a WebSocket message for cobrowsing
type CoBrowseMessage struct {
	Type     string          `json:"type"`     // rrweb-event, rrweb-full-snapshot, control
	Data     json.RawMessage `json:"data"`     // rrweb event data or control payload
	Action   string          `json:"action"`   // For control: click, input, scroll, keydown, navigate
	X        int             `json:"x"`        // For click action
	Y        int             `json:"y"`        // For click action
	Selector string          `json:"selector"` // For input action
	Value    string          `json:"value"`    // For input action
	Key      string          `json:"key"`      // For keydown action
	URL      string          `json:"url"`      // For navigate action
}

// RRWebEvent represents an rrweb event
type RRWebEvent struct {
	Seq       int64           `json:"-"`
	Type      int             `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// Recording represents a cobrowsing session recording
type Recording struct {
	ID           int64  `json:"id"`
	SessionID    string `json:"sessionId"`
	AppID        string `json:"appId"`
	StartTime    int64  `json:"startTime"`
	EndTime      int64  `json:"endTime"`
	DurationMs   int64  `json:"durationMs"`
	EventCount   int    `json:"eventCount"`
	FullSnapshot string `json:"fullSnapshot"` // JSON string
	URL          string `json:"url"`
	UA           string `json:"ua"`
	Status       string `json:"status"` // recording, completed, error
	CreatedAt    int64  `json:"createdAt"`
}

// RecordingEvent represents a single rrweb event in a recording
type RecordingEvent struct {
	ID        int64           `json:"id"`
	SessionID string          `json:"sessionId"`
	Seq       int             `json:"seq"`
	Timestamp int64           `json:"timestamp"`
	EventData json.RawMessage `json:"eventData"`
	CreatedAt int64           `json:"createdAt"`
}

// LiveSession represents a currently active cobrowsing session
type LiveSession struct {
	SessionID    string `json:"sessionId"`
	AppID        string `json:"appId"`
	URL          string `json:"url"`
	UA           string `json:"ua"`
	ConnectedAt  int64  `json:"connectedAt"`
	ViewerCount  int    `json:"viewerCount"`
	IsControlled bool   `json:"isControlled"`
}

// NewCoBrowseMessage creates a new cobrowse message
func NewCoBrowseMessage(msgType string, data interface{}) (*CoBrowseMessage, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &CoBrowseMessage{
		Type: msgType,
		Data: dataBytes,
	}, nil
}

// NewControlMessage creates a new control message
func NewControlMessage(action string, params map[string]interface{}) *CoBrowseMessage {
	msg := &CoBrowseMessage{
		Type:   "control",
		Action: action,
	}
	if x, ok := params["x"].(int); ok {
		msg.X = x
	}
	if y, ok := params["y"].(int); ok {
		msg.Y = y
	}
	if selector, ok := params["selector"].(string); ok {
		msg.Selector = selector
	}
	if value, ok := params["value"].(string); ok {
		msg.Value = value
	}
	if key, ok := params["key"].(string); ok {
		msg.Key = key
	}
	if url, ok := params["url"].(string); ok {
		msg.URL = url
	}
	return msg
}

// MarshalEvents marshals multiple rrweb events
func MarshalEvents(events []RRWebEvent) (json.RawMessage, error) {
	return json.Marshal(events)
}

// UnmarshalEvents unmarshals rrweb events
func UnmarshalEvents(data json.RawMessage) ([]RRWebEvent, error) {
	var events []RRWebEvent
	err := json.Unmarshal(data, &events)
	return events, err
}

// SessionStatus returns whether a session is currently active
func (s *LiveSession) SessionStatus() string {
	if s.IsControlled {
		return "controlling"
	}
	return "viewing"
}

// Duration returns the session duration in milliseconds
func (r *Recording) Duration() int64 {
	if r.EndTime > 0 {
		return r.EndTime - r.StartTime
	}
	return time.Now().UnixMilli() - r.StartTime
}
