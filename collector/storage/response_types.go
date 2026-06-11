package storage

// PerformanceData represents the JSON structure stored in the events.performance column.
// Replaces map[string]interface{} for type-safe parsing.
type PerformanceData struct {
	FCP  float64 `json:"fcp,omitempty"`
	LCP  float64 `json:"lcp,omitempty"`
	CLS  float64 `json:"cls,omitempty"`
	INP  float64 `json:"inp,omitempty"`
	TTFB float64 `json:"ttfb,omitempty"`
}

// MetricValue holds a single metric value used for collection during queries.
type MetricValue struct {
	FCP  []float64
	LCP  []float64
	CLS  []float64
	INP  []float64
	TTFB []float64
}

// Add merges values from a PerformanceData into the collector.
func (mv *MetricValue) Add(pd PerformanceData) {
	if pd.FCP > 0 {
		mv.FCP = append(mv.FCP, pd.FCP)
	}
	if pd.LCP > 0 {
		mv.LCP = append(mv.LCP, pd.LCP)
	}
	if pd.CLS > 0 {
		mv.CLS = append(mv.CLS, pd.CLS)
	}
	if pd.INP > 0 {
		mv.INP = append(mv.INP, pd.INP)
	}
	if pd.TTFB > 0 {
		mv.TTFB = append(mv.TTFB, pd.TTFB)
	}
}

// GetMetric returns the value for a named metric (fcp/lcp/cls/inp/ttfb).
func (pd PerformanceData) GetMetric(name string) float64 {
	switch name {
	case "fcp":
		return pd.FCP
	case "lcp":
		return pd.LCP
	case "cls":
		return pd.CLS
	case "inp":
		return pd.INP
	case "ttfb":
		return pd.TTFB
	default:
		return 0
	}
}

// TimeSeriesPoint replaces map[string]interface{} for time series data.
type TimeSeriesPoint struct {
	Timestamp int64 `json:"timestamp"`
	Count     int64 `json:"count"`
}

// ErrorTrendPoint represents a single point in an error trend.
type ErrorTrendPoint struct {
	Timestamp int64 `json:"timestamp"`
	Count     int64 `json:"count"`
}

// LogsResponse is the typed response for QueryLogs.
type LogsResponse struct {
	Total int64                    `json:"total"`
	Page  int                      `json:"page"`
	Size  int                      `json:"size"`
	Data  []map[string]interface{} `json:"data"` // TODO: migrate to []EventResponse
}

// StatsResponse is the typed response for QueryStats.
type StatsResponse struct {
	TotalEvents int64             `json:"totalEvents"`
	ErrorCount  int64             `json:"errorCount"`
	WarnCount   int64             `json:"warnCount"`
	InfoCount   int64             `json:"infoCount"`
	TopErrors   []ErrorStat       `json:"topErrors"`
	ErrorTrend  []ErrorTrendPoint `json:"errorTrend"`
}

// TopResponse is the typed response for QueryTop.
type TopResponse struct {
	Type  string     `json:"type"`
	Data  []TopNItem `json:"data"`
	Total int64      `json:"total"`
}

// SimilarResponse is the typed response for QuerySimilar.
type SimilarResponse struct {
	Query    string         `json:"query"`
	Clusters []ErrorCluster `json:"clusters"`
}

// PerformanceTrendResponse is the typed response for QueryPerformanceTrend.
type PerformanceTrendResponse struct {
	Metric       string                 `json:"metric"`
	Granularity  string                 `json:"granularity"`
	Data         []PerformanceTrendData `json:"data"`
}

// PerformancePagesResponse is the typed response for QueryPerformancePages.
type PerformancePagesResponse struct {
	TimeRange string                `json:"time_range"`
	Data      []PagePerformanceData `json:"data"`
}

// PerformanceRegressionResponse is the typed response for QueryPerformanceRegression.
type PerformanceRegressionResponse struct {
	Regressions []PerformanceRegression `json:"regressions"`
	Count       int                     `json:"count"`
}

// NewErrorsResponse is the typed response for QueryNewErrors.
type NewErrorsResponse struct {
	Data          []NewError `json:"data"`
	Count         int        `json:"count"`
	SinceMinutes  int        `json:"since_minutes"`
}

// AlertTriggersResponse is the typed response for QueryAlertTriggers.
type AlertTriggersResponse struct {
	Data  []AlertTrigger `json:"data"`
	Count int            `json:"count"`
}

// ActiveSessionsResponse is the typed response for QueryActiveSessions.
type ActiveSessionsResponse struct {
	Data  []ActiveSession `json:"data"`
	Count int             `json:"count"`
}

// ClustersResponse is the typed response for GetClusters.
type ClustersResponse struct {
	Data   []ErrorClusterResult `json:"data"`
	Count  int                  `json:"count"`
}

// ClusterDetailResponse is the typed response for GetClusterDetail.
type ClusterDetailResponse struct {
	Cluster ClusterStats   `json:"cluster"`
	Events  []EventRecord  `json:"events"`
	Total   int64          `json:"total"`
}
