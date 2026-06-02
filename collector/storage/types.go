package storage

// TopNItem represents a single item in top N results
type TopNItem struct {
	Key         string
	Count       int64
	Users       int64
	LastSeen    int64
	FirstSeen   int64
	IsNew       bool
	ImpactScore int64
}

// TopNResult represents the result of a top N query
type TopNResult struct {
	Type string
	Data []TopNItem
}

// ErrorCluster represents a cluster of similar errors
type ErrorCluster struct {
	ClusterID     string        `json:"clusterId"`
	Message       string        `json:"message"`
	Count         int64         `json:"count"`
	FirstSeen     int64         `json:"firstSeen"`
	LastSeen      int64         `json:"lastSeen"`
	AffectedUsers int64         `json:"affectedUsers"`
	SampleEvents  []EventRecord `json:"sampleEvents"`
	Pattern       string        `json:"pattern"`
	// Additional fields for new API
	ID           string
	Users        int64
	Stack        string
	AffectedURLs []string
	Releases     []string
}
