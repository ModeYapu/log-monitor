package storage

import (
	"testing"
	"time"

	"github.com/logmonitor/collector/model"
)

// ---- Test InsertPerformanceMetric ----

func TestInsertPerformanceMetric(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Ensure the table exists
	if err := db.EnsurePerformanceMetricsTable(); err != nil {
		t.Fatalf("Failed to ensure performance_metrics table: %v", err)
	}

	metric := &model.PerformanceMetric{
		ProjectID:  1,
		AppID:      "test-app",
		PageURL:    "/test-page",
		MetricName: "lcp",
		Value:      2500,
		Rating:     "good",
		Release:    "v1.0.0",
		UserID:     "user1",
		SessionID:  "sess1",
		UA:         "Mozilla/5.0",
		CreatedAt:  time.Now().UnixMilli(),
	}

	err := db.InsertPerformanceMetric(metric)
	if err != nil {
		t.Fatalf("Failed to insert performance metric: %v", err)
	}
}

func TestInsertPerformanceMetric_ClosedDB(t *testing.T) {
	db := setupTestDB(t)
	if err := db.EnsurePerformanceMetricsTable(); err != nil {
		t.Fatalf("Failed to ensure performance_metrics table: %v", err)
	}
	db.Close()

	metric := &model.PerformanceMetric{
		ProjectID:  1,
		AppID:      "test-app",
		PageURL:    "/test-page",
		MetricName: "lcp",
		Value:      2500,
		Rating:     "good",
		CreatedAt:  time.Now().UnixMilli(),
	}

	err := db.InsertPerformanceMetric(metric)
	if err == nil {
		t.Error("expected error for closed database")
	}
}

// ---- Test GetPerformanceSummaryByPage ----

func TestGetPerformanceSummaryByPage_SinglePage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if err := db.EnsurePerformanceMetricsTable(); err != nil {
		t.Fatalf("Failed to ensure performance_metrics table: %v", err)
	}

	// Insert multiple metrics for the same page
	now := time.Now()
	metrics := []float64{1200, 1800, 2400, 3000, 3500}
	for i, val := range metrics {
		metric := &model.PerformanceMetric{
			ProjectID:  1,
			AppID:      "test-app",
			PageURL:    "/page1",
			MetricName: "fcp",
			Value:      val,
			Rating:     model.GetRating("fcp", val),
			CreatedAt:  now.Add(-time.Duration(i) * time.Minute).UnixMilli(),
		}
		if err := db.InsertPerformanceMetric(metric); err != nil {
			t.Fatalf("Failed to insert metric: %v", err)
		}
	}

	summary, err := db.GetPerformanceSummaryByPage(1, "fcp", "1d")
	if err != nil {
		t.Fatalf("GetPerformanceSummary returned error: %v", err)
	}

	if len(summary) == 0 {
		t.Fatal("expected at least one summary entry")
	}

	// Should have page1
	found := false
	for _, s := range summary {
		if s.PageURL == "/page1" {
			found = true
			if s.Count != int64(len(metrics)) {
				t.Errorf("Count = %d, want %d", s.Count, len(metrics))
			}
			// P75 should be around 4th value (3000)
			if s.P75 == 0 {
				t.Error("P75 should not be zero")
			}
		}
	}
	if !found {
		t.Error("expected to find /page1 in summary")
	}
}

func TestGetPerformanceSummaryByPage_MultiplePages(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if err := db.EnsurePerformanceMetricsTable(); err != nil {
		t.Fatalf("Failed to ensure performance_metrics table: %v", err)
	}

	now := time.Now()
	pages := []struct {
		url         string
		metricValues []float64
	}{
		{"/fast", []float64{800, 1000, 1200, 1500}},
		{"/slow", []float64{3000, 3500, 4000, 4500}},
		{"/medium", []float64{1800, 2200, 2600, 3000}},
	}

	for _, page := range pages {
		for _, val := range page.metricValues {
			metric := &model.PerformanceMetric{
				ProjectID:  1,
				AppID:      "test-app",
				PageURL:    page.url,
				MetricName: "lcp",
				Value:      val,
				Rating:     model.GetRating("lcp", val),
				CreatedAt:  now.UnixMilli(),
			}
			if err := db.InsertPerformanceMetric(metric); err != nil {
				t.Fatalf("Failed to insert metric: %v", err)
			}
		}
	}

	summary, err := db.GetPerformanceSummaryByPage(1, "lcp", "1d")
	if err != nil {
		t.Fatalf("GetPerformanceSummaryByPage returned error: %v", err)
	}

	if len(summary) != 3 {
		t.Errorf("Expected 3 pages, got %d", len(summary))
	}

	// Results should be sorted by P75 ascending (fastest first)
	if summary[0].PageURL != "/fast" {
		t.Errorf("Expected first page to be /fast, got %s", summary[0].PageURL)
	}
}

func TestGetPerformanceSummaryByPage_ClosedDB(t *testing.T) {
	db := setupTestDB(t)
	if err := db.EnsurePerformanceMetricsTable(); err != nil {
		t.Fatalf("Failed to ensure performance_metrics table: %v", err)
	}
	db.Close()

	_, err := db.GetPerformanceSummaryByPage(1, "fcp", "1d")
	if err == nil {
		t.Error("expected error for closed database")
	}
}

// ---- Test GetPerformanceTrendByPage ----

func TestGetPerformanceTrendByPage_Basic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if err := db.EnsurePerformanceMetricsTable(); err != nil {
		t.Fatalf("Failed to ensure performance_metrics table: %v", err)
	}

	now := time.Now()
	// Insert metrics over 3 days
	for day := 0; day < 3; day++ {
		for i := 0; i < 5; i++ {
			metric := &model.PerformanceMetric{
				ProjectID:  1,
				AppID:      "test-app",
				PageURL:    "/page1",
				MetricName: "lcp",
				Value:      2000 + float64(day*100) + float64(i*50),
				Rating:     model.GetRating("lcp", 2000),
				CreatedAt:  now.Add(-time.Duration(day) * 24 * time.Hour).UnixMilli(),
			}
			if err := db.InsertPerformanceMetric(metric); err != nil {
				t.Fatalf("Failed to insert metric: %v", err)
			}
		}
	}

	trend, err := db.GetPerformanceTrendByPage(1, "/page1", "lcp", 30)
	if err != nil {
		t.Fatalf("GetPerformanceTrend returned error: %v", err)
	}

	if len(trend) == 0 {
		t.Fatal("expected at least one trend data point")
	}

	// Verify trend structure
	for _, point := range trend {
		if point.Date == "" {
			t.Error("Date should not be empty")
		}
		if point.P75 == 0 {
			t.Error("P75 should not be zero")
		}
		if point.Count == 0 {
			t.Error("Count should not be zero")
		}
		if point.AvgRating == "" {
			t.Error("AvgRating should not be empty")
		}
	}
}

func TestGetPerformanceTrendByPage_ClosedDB(t *testing.T) {
	db := setupTestDB(t)
	if err := db.EnsurePerformanceMetricsTable(); err != nil {
		t.Fatalf("Failed to ensure performance_metrics table: %v", err)
	}
	db.Close()

	_, err := db.GetPerformanceTrendByPage(1, "/page1", "lcp", 30)
	if err == nil {
		t.Error("expected error for closed database")
	}
}

// ---- Test GetPerformanceComparison ----

func TestGetPerformanceComparison(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if err := db.EnsurePerformanceMetricsTable(); err != nil {
		t.Fatalf("Failed to ensure performance_metrics table: %v", err)
	}

	now := time.Now()

	// Insert release A metrics (better performance)
	for i := 0; i < 5; i++ {
		metric := &model.PerformanceMetric{
			ProjectID:  1,
			AppID:      "test-app",
			PageURL:    "/page1",
			MetricName: "lcp",
			Value:      2000 + float64(i*100),
			Rating:     model.GetRating("lcp", 2000),
			Release:    "v1.0.0",
			CreatedAt:  now.UnixMilli(),
		}
		if err := db.InsertPerformanceMetric(metric); err != nil {
			t.Fatalf("Failed to insert metric: %v", err)
		}
	}

	// Insert release B metrics (worse performance)
	for i := 0; i < 5; i++ {
		metric := &model.PerformanceMetric{
			ProjectID:  1,
			AppID:      "test-app",
			PageURL:    "/page1",
			MetricName: "lcp",
			Value:      3000 + float64(i*100),
			Rating:     model.GetRating("lcp", 3000),
			Release:    "v2.0.0",
			CreatedAt:  now.UnixMilli(),
		}
		if err := db.InsertPerformanceMetric(metric); err != nil {
			t.Fatalf("Failed to insert metric: %v", err)
		}
	}

	comparison, err := db.GetPerformanceComparison(1, "lcp", "v1.0.0", "v2.0.0")
	if err != nil {
		t.Fatalf("GetPerformanceComparison returned error: %v", err)
	}

	if len(comparison) == 0 {
		t.Fatal("expected at least one comparison entry")
	}

	comp := comparison[0]
	if comp.MetricName != "lcp" {
		t.Errorf("MetricName = %s, want lcp", comp.MetricName)
	}
	if comp.ReleaseA != "v1.0.0" {
		t.Errorf("ReleaseA = %s, want v1.0.0", comp.ReleaseA)
	}
	if comp.ReleaseB != "v2.0.0" {
		t.Errorf("ReleaseB = %s, want v2.0.0", comp.ReleaseB)
	}
	if comp.ValueA == 0 || comp.ValueB == 0 {
		t.Error("Values should not be zero")
	}
	if comp.CountA == 0 || comp.CountB == 0 {
		t.Error("Counts should not be zero")
	}
	// Release B should be worse (higher value), so Improved should be false
	if comp.Improved {
		t.Errorf("Expected Improved to be false (release B is worse). Got ValueA=%f, ValueB=%f, Improved=%v", comp.ValueA, comp.ValueB, comp.Improved)
	}
}

func TestGetPerformanceComparison_ClosedDB(t *testing.T) {
	db := setupTestDB(t)
	if err := db.EnsurePerformanceMetricsTable(); err != nil {
		t.Fatalf("Failed to ensure performance_metrics table: %v", err)
	}
	db.Close()

	_, err := db.GetPerformanceComparison(1, "lcp", "v1.0.0", "v2.0.0")
	if err == nil {
		t.Error("expected error for closed database")
	}
}

// ---- Test EnsurePerformanceMetricsTable ----

func TestEnsurePerformanceMetricsTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := db.EnsurePerformanceMetricsTable()
	if err != nil {
		t.Fatalf("EnsurePerformanceMetricsTable returned error: %v", err)
	}

	// Verify we can insert into the table
	metric := &model.PerformanceMetric{
		ProjectID:  1,
		AppID:      "test-app",
		PageURL:    "/test",
		MetricName: "fcp",
		Value:      1000,
		Rating:     "good",
		CreatedAt:  time.Now().UnixMilli(),
	}

	err = db.InsertPerformanceMetric(metric)
	if err != nil {
		t.Errorf("Failed to insert after ensuring table: %v", err)
	}
}

// ---- Test Rating Calculation (model.GetRating) ----

func TestGetRating(t *testing.T) {
	tests := []struct {
		metric    string
		value     float64
		wantRating string
	}{
		// FCP: good<=1800, needs-improvement<=3000
		{"fcp", 1000, "good"},
		{"fcp", 1800, "good"},
		{"fcp", 2000, "needs-improvement"},
		{"fcp", 3000, "needs-improvement"},
		{"fcp", 3500, "poor"},
		// LCP: good<=2500, needs-improvement<=4000
		{"lcp", 1000, "good"},
		{"lcp", 2500, "good"},
		{"lcp", 3000, "needs-improvement"},
		{"lcp", 4000, "needs-improvement"},
		{"lcp", 4500, "poor"},
		// CLS: good<=0.1, needs-improvement<=0.25
		{"cls", 0.05, "good"},
		{"cls", 0.1, "good"},
		{"cls", 0.15, "needs-improvement"},
		{"cls", 0.25, "needs-improvement"},
		{"cls", 0.3, "poor"},
		// INP: good<=200, needs-improvement<=500
		{"inp", 100, "good"},
		{"inp", 200, "good"},
		{"inp", 300, "needs-improvement"},
		{"inp", 500, "needs-improvement"},
		{"inp", 600, "poor"},
		// TTFB: good<=800, needs-improvement<=1800
		{"ttfb", 500, "good"},
		{"ttfb", 800, "good"},
		{"ttfb", 1000, "needs-improvement"},
		{"ttfb", 1800, "needs-improvement"},
		{"ttfb", 2000, "poor"},
		// Unknown metric
		{"unknown", 100, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.metric+"_"+tt.wantRating, func(t *testing.T) {
			got := model.GetRating(tt.metric, tt.value)
			if got != tt.wantRating {
				t.Errorf("GetRating(%q, %v) = %q, want %q", tt.metric, tt.value, got, tt.wantRating)
			}
		})
	}
}

// ---- Test IsLowerBetter ----

func TestIsLowerBetter(t *testing.T) {
	tests := []struct {
		metric string
		want   bool
	}{
		{"fcp", true},
		{"lcp", true},
		{"cls", true},
		{"inp", true},
		{"ttfb", true},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.metric, func(t *testing.T) {
			got := model.IsLowerBetter(tt.metric)
			if got != tt.want {
				t.Errorf("IsLowerBetter(%q) = %v, want %v", tt.metric, got, tt.want)
			}
		})
	}
}
