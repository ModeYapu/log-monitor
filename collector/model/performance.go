package model

// PerformanceMetric represents a single Web Vitals measurement
type PerformanceMetric struct {
	ID         int64   `json:"id"`
	ProjectID  int64   `json:"project_id"`
	AppID      string  `json:"app_id"`
	PageURL    string  `json:"page_url"`
	MetricName string  `json:"metric_name"` // fcp/lcp/cls/inp/ttfb
	Value      float64 `json:"value"`
	Rating     string  `json:"rating"` // good/needs-improvement/poor
	Release    string  `json:"release"`
	UserID     string  `json:"user_id"`
	SessionID  string  `json:"session_id"`
	UA         string  `json:"ua"`
	CreatedAt  int64   `json:"created_at"`
}

// PagePerformanceSummary represents aggregated performance metrics for a page
type PagePerformanceSummary struct {
	PageURL string  `json:"page_url"`
	P50     float64 `json:"p50"`
	P75     float64 `json:"p75"`
	P95     float64 `json:"p95"`
	Count   int64   `json:"count"`
}

// DailyMetric represents a single day's metric value
type DailyMetric struct {
	Date        string  `json:"date"`        // YYYY-MM-DD
	P75         float64 `json:"p75"`
	Count       int64   `json:"count"`
	AvgRating   string  `json:"avg_rating"`
}

// ReleaseComparison compares metrics between two releases
type ReleaseComparison struct {
	MetricName string  `json:"metric_name"`
	ReleaseA   string  `json:"release_a"`
	ReleaseB   string  `json:"release_b"`
	ValueA     float64 `json:"value_a"`
	ValueB     float64 `json:"value_b"`
	CountA     int64   `json:"count_a"`
	CountB     int64   `json:"count_b"`
	Change     float64 `json:"change"`     // percentage change
	Improved   bool    `json:"improved"`   // true if value decreased (better)
}

// PerformanceRegression represents a performance regression detected between releases
type PerformanceRegression struct {
	MetricName  string  `json:"metric_name"`
	PageURL     string  `json:"page_url"`
	PreviousP75 float64 `json:"previous_p75"`
	CurrentP75  float64 `json:"current_p75"`
	Change      float64 `json:"change"`   // percentage change (positive = worse)
	Severity    string  `json:"severity"` // minor (20-50%) / major (50-100%) / critical (>100%)
}

// Web Vitals rating thresholds (from web-vitals library)
// FCP: good<=1800ms, needs-improvement<=3000ms, poor>3000ms
// LCP: good<=2500ms, needs-improvement<=4000ms, poor>4000ms
// CLS: good<=0.1, needs-improvement<=0.25, poor>0.25
// INP: good<=200ms, needs-improvement<=500ms, poor>500ms
// TTFB: good<=800ms, needs-improvement<=1800ms, poor>1800ms

// GetRating returns the rating for a metric value based on Web Vitals thresholds
func GetRating(metricName string, value float64) string {
	switch metricName {
	case "fcp":
		if value <= 1800 {
			return "good"
		} else if value <= 3000 {
			return "needs-improvement"
		}
		return "poor"
	case "lcp":
		if value <= 2500 {
			return "good"
		} else if value <= 4000 {
			return "needs-improvement"
		}
		return "poor"
	case "cls":
		if value <= 0.1 {
			return "good"
		} else if value <= 0.25 {
			return "needs-improvement"
		}
		return "poor"
	case "inp":
		if value <= 200 {
			return "good"
		} else if value <= 500 {
			return "needs-improvement"
		}
		return "poor"
	case "ttfb":
		if value <= 800 {
			return "good"
		} else if value <= 1800 {
			return "needs-improvement"
		}
		return "poor"
	default:
		return "unknown"
	}
}

// IsLowerBetter returns true if lower values are better for the metric
func IsLowerBetter(metricName string) bool {
	switch metricName {
	case "fcp", "lcp", "inp", "ttfb":
		return true
	case "cls":
		return true
	default:
		return true
	}
}
