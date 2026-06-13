package alerter

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/logmonitor/collector/storage"
)

// Cluster represents an anomaly cluster of similar errors
type Cluster struct {
	ID               string
	Fingerprint      string
	Message          string // Representative message
	Count            int
	FirstSeen        int64
	LastSeen         int64
	AppID            string
	Severity         string
	SimilarClusters  []string // IDs of similar clusters
	UniqueUsers      int
	RepresentativeEvent *storage.EventRecord
}

// Clusterer performs automatic clustering of error events
type Clusterer struct {
	db        storage.EventStore
	clusters  map[string]*Cluster // fingerprint -> cluster
	mu        sync.RWMutex
	stopCh    chan struct{}
}

// similarityThreshold defines the threshold for merging clusters (0.0 - 1.0)
const similarityThreshold = 0.8

// cleanupInterval defines how often to clean up stale clusters
const cleanupInterval = 30 * time.Minute

// clusterMaxAge defines the maximum age of a cluster before cleanup
const clusterMaxAge = 24 * time.Hour

// NewClusterer creates a new clusterer
func NewClusterer(db storage.EventStore) *Clusterer {
	return &Clusterer{
		db:       db,
		clusters: make(map[string]*Cluster),
		stopCh:   make(chan struct{}),
	}
}

// Start begins the clusterer background processing
func (c *Clusterer) Start() {
	go c.cleanupLoop()
}

// Stop stops the clusterer
func (c *Clusterer) Stop() {
	close(c.stopCh)
}

// ProcessEvents processes new events and updates clusters
func (c *Clusterer) ProcessEvents(events []storage.EventRecord) {
	if len(events) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, event := range events {
		// Only cluster error events
		if event.Level != "error" || event.Fingerprint == "" {
			continue
		}

		c.processEvent(event)
	}
}

// processEvent processes a single event for clustering
func (c *Clusterer) processEvent(event storage.EventRecord) {
	fingerprint := event.Fingerprint

	// Check if cluster already exists
	if existing, ok := c.clusters[fingerprint]; ok {
		// Update existing cluster
		existing.Count++
		existing.LastSeen = event.CreatedAt
		if event.CreatedAt < existing.FirstSeen {
			existing.FirstSeen = event.CreatedAt
		}
		// Update representative message to the most common one
		if existing.Message == "" {
			existing.Message = truncateMessage(event.Message, 200)
		}
		return
	}

	// Check for similar clusters
	similarID := c.findSimilarCluster(event)
	if similarID != "" {
		// Merge with similar cluster
		similar := c.clusters[similarID]
		similar.Count++
		similar.LastSeen = event.CreatedAt
		similar.SimilarClusters = append(similar.SimilarClusters, fingerprint)
		return
	}

	// Create new cluster
	severity := c.determineSeverity(event)
	cluster := &Cluster{
		ID:                generateClusterID(event.AppID, fingerprint),
		Fingerprint:       fingerprint,
		Message:           truncateMessageCluster(event.Message, 200),
		Count:             1,
		FirstSeen:         event.CreatedAt,
		LastSeen:          event.CreatedAt,
		AppID:             event.AppID,
		Severity:          severity,
		SimilarClusters:   []string{},
		UniqueUsers:       1,
		RepresentativeEvent: &event,
	}

	c.clusters[fingerprint] = cluster
	slog.Debug("New cluster created", "fingerprint", fingerprint, "message", truncateMessage(event.Message, 50))
}

// findSimilarCluster finds an existing cluster with similar message
func (c *Clusterer) findSimilarCluster(event storage.EventRecord) string {
	eventTokens := tokenize(event.Message)

	for fp, cluster := range c.clusters {
		// Skip if different app or too old
		if cluster.AppID != event.AppID {
			continue
		}
		age := time.Now().UnixMilli() - cluster.LastSeen
		if age > int64(clusterMaxAge) {
			continue
		}

		// Calculate similarity
		clusterTokens := tokenize(cluster.Message)
		similarity := jaccardSimilarity(eventTokens, clusterTokens)

		if similarity >= similarityThreshold {
			return fp
		}
	}

	return ""
}

// determineSeverity determines the severity of a cluster based on event properties
func (c *Clusterer) determineSeverity(event storage.EventRecord) string {
	// Check for critical indicators in message
	msg := strings.ToLower(event.Message)
	if strings.Contains(msg, "critical") || strings.Contains(msg, "fatal") || strings.Contains(msg, "panic") {
		return "critical"
	}

	// Check URL patterns for high-value pages
	if strings.Contains(event.URL, "/checkout") || strings.Contains(event.URL, "/payment") {
		return "critical"
	}

	// Default to warning
	return "warning"
}

// GetCluster retrieves a cluster by fingerprint
func (c *Clusterer) GetCluster(fingerprint string) (*Cluster, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cluster, ok := c.clusters[fingerprint]
	return cluster, ok
}

// ListClusters returns all active clusters for an app
func (c *Clusterer) ListClusters(appID string, limit int) []*Cluster {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	// Filter by app and convert to slice
	var result []*Cluster
	for _, cluster := range c.clusters {
		if cluster.AppID == appID {
			result = append(result, cluster)
		}
	}

	// Sort by count (descending) and limit
	// Simple sort since result size is typically small
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Count > result[i].Count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if len(result) > limit {
		result = result[:limit]
	}

	return result
}

// GetClusterEvents retrieves events for a specific cluster
func (c *Clusterer) GetClusterEvents(appID, fingerprint string, page, pageSize int) ([]storage.EventRecord, int64, error) {
	return c.db.GetClusterEvents(appID, fingerprint, page, pageSize)
}

// cleanupLoop periodically cleans up stale clusters
func (c *Clusterer) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup removes stale clusters
func (c *Clusterer) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixMilli()
	removed := 0

	for fp, cluster := range c.clusters {
		age := now - cluster.LastSeen
		if age > int64(clusterMaxAge) {
			delete(c.clusters, fp)
			removed++
		}
	}

	if removed > 0 {
		slog.Info("Cluster cleanup completed", "removed", removed, "remaining", len(c.clusters))
	}
}

// GetActiveClusterCount returns the number of active clusters
func (c *Clusterer) GetActiveClusterCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.clusters)
}

// GetActiveClustersByApp returns cluster count by app
func (c *Clusterer) GetActiveClustersByApp(appID string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, cluster := range c.clusters {
		if cluster.AppID == appID {
			count++
		}
	}
	return count
}

// tokenize splits a message into tokens for similarity comparison
func tokenize(message string) map[string]bool {
	tokens := make(map[string]bool)
	words := strings.Fields(strings.ToLower(message))

	for _, word := range words {
		// Skip very short words and common words
		if len(word) < 3 {
			continue
		}
		// Simple stopword filter
		if word == "the" || word == "and" || word == "for" || word == "are" || word == "but" {
			continue
		}
		tokens[word] = true
	}

	return tokens
}

// jaccardSimilarity calculates Jaccard similarity between two token sets
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}

	// Calculate intersection
	intersection := 0
	for token := range a {
		if b[token] {
			intersection++
		}
	}

	// Calculate union
	union := len(a) + len(b) - intersection

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Use smaller string for row optimization
	if len(a) > len(b) {
		a, b = b, a
	}

	previousRow := make([]int, len(b)+1)
	currentRow := make([]int, len(b)+1)

	for i := 0; i <= len(b); i++ {
		previousRow[i] = i
	}

	for i := 0; i < len(a); i++ {
		currentRow[0] = i + 1
		for j := 0; j < len(b); j++ {
			cost := 1
			if a[i] == b[j] {
				cost = 0
			}

			currentRow[j+1] = min(
				previousRow[j+1]+1,     // deletion
				currentRow[j]+1,        // insertion
				previousRow[j]+cost,    // substitution
			)
		}
		previousRow, currentRow = currentRow, previousRow
	}

	return previousRow[len(b)]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// generateClusterID generates a unique cluster ID
func generateClusterID(appID, fingerprint string) string {
	fp := fingerprint
	if len(fp) > 8 {
		fp = fp[:8]
	}
	return fmt.Sprintf("%s-%s", appID, fp)
}

// truncateMessageCluster truncates a message to a maximum length
func truncateMessageCluster(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen] + "..."
}
