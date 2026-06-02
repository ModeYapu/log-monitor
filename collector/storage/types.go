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
	ID           string
	Message      string
	Stack        string
	Count        int64
	Users        int64
	FirstSeen    int64
	LastSeen     int64
	AffectedURLs []string
	Releases     []string
	Pattern      string
}
