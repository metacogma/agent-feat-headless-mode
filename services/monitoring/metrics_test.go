package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

/*
nkk: Comprehensive unit tests for monitoring system
Tests cover:
- Metrics registry operations
- Counter, gauge, and histogram functionality
- Prometheus metrics formatting
- Health checks
- System metrics collection
- Thread safety and concurrent access
*/

func TestMetricsRegistry_Counter(t *testing.T) {
	registry := &MetricsRegistry{}

	// Create counter
	counter := registry.Counter("test_counter", "Test counter metric", map[string]string{"label": "value"})

	if counter.Name != "test_counter" {
		t.Errorf("Expected counter name 'test_counter', got '%s'", counter.Name)
	}

	if counter.Type != Counter {
		t.Errorf("Expected counter type, got %v", counter.Type)
	}

	if counter.Get() != 0 {
		t.Errorf("Expected initial counter value 0, got %f", counter.Get())
	}

	// Test increment
	counter.Inc()
	if counter.Get() != 1 {
		t.Errorf("Expected counter value 1 after Inc(), got %f", counter.Get())
	}

	// Test add
	counter.Add(5)
	if counter.Get() != 6 {
		t.Errorf("Expected counter value 6 after Add(5), got %f", counter.Get())
	}

	// Test that same key returns same metric
	counter2 := registry.Counter("test_counter", "Test counter metric", map[string]string{"label": "value"})
	if counter != counter2 {
		t.Error("Expected same metric instance for same key")
	}
}

func TestMetricsRegistry_Gauge(t *testing.T) {
	registry := &MetricsRegistry{}

	// Create gauge
	gauge := registry.Gauge("test_gauge", "Test gauge metric", map[string]string{"type": "test"})

	if gauge.Type != Gauge {
		t.Errorf("Expected gauge type, got %v", gauge.Type)
	}

	// Test set
	gauge.Set(42.5)
	if gauge.Get() != 42.5 {
		t.Errorf("Expected gauge value 42.5, got %f", gauge.Get())
	}

	// Test that add doesn't work on gauge
	originalValue := gauge.Get()
	gauge.Add(10) // Should be ignored
	if gauge.Get() != originalValue {
		t.Error("Add() should not work on gauge")
	}
}

func TestMetricsRegistry_Histogram(t *testing.T) {
	registry := &MetricsRegistry{}

	buckets := []float64{1, 5, 10, 25, 50}
	histogram := registry.Histogram("test_histogram", "Test histogram metric", nil, buckets)

	if histogram.Type != Histogram {
		t.Errorf("Expected histogram type, got %v", histogram.Type)
	}

	if len(histogram.Buckets) != len(buckets) {
		t.Errorf("Expected %d buckets, got %d", len(buckets), len(histogram.Buckets))
	}

	// Test observations
	histogram.Observe(3)
	histogram.Observe(7)
	histogram.Observe(15)
	histogram.Observe(30)

	histogram.mutex.RLock()
	observations := len(histogram.Observations)
	histogram.mutex.RUnlock()

	if observations != 4 {
		t.Errorf("Expected 4 observations, got %d", observations)
	}
}

func TestHistogram_Timer(t *testing.T) {
	registry := &MetricsRegistry{}
	histogram := registry.Histogram("test_timer", "Test timer", nil, nil)

	// Test timer
	timer := histogram.Timer()
	time.Sleep(10 * time.Millisecond) // Small delay
	timer()

	histogram.mutex.RLock()
	observations := len(histogram.Observations)
	histogram.mutex.RUnlock()

	if observations != 1 {
		t.Errorf("Expected 1 observation from timer, got %d", observations)
	}

	// Verify observation is reasonable (should be at least 10ms)
	histogram.mutex.RLock()
	value := histogram.Observations[0]
	histogram.mutex.RUnlock()

	if value < 10 {
		t.Errorf("Expected timer value >= 10ms, got %f", value)
	}
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	registry := &MetricsRegistry{}
	counter := registry.Counter("concurrent_counter", "Test counter", nil)
	gauge := registry.Gauge("concurrent_gauge", "Test gauge", nil)
	histogram := registry.Histogram("concurrent_histogram", "Test histogram", nil, nil)

	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup

	// Test concurrent counter operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				counter.Inc()
			}
		}()
	}

	// Test concurrent gauge operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				gauge.Set(float64(i*numOperations + j))
			}
		}(i)
	}

	// Test concurrent histogram operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				histogram.Observe(float64(i*numOperations + j))
			}
		}(i)
	}

	wg.Wait()

	// Verify final counter value
	expectedCount := float64(numGoroutines * numOperations)
	if counter.Get() != expectedCount {
		t.Errorf("Expected counter value %f, got %f", expectedCount, counter.Get())
	}

	// Verify histogram has observations (exact count may vary due to memory limit)
	histogram.mutex.RLock()
	obsCount := len(histogram.Observations)
	histogram.mutex.RUnlock()

	if obsCount == 0 {
		t.Error("Expected histogram to have observations")
	}
}

func TestApplicationMetrics_NewApplicationMetrics(t *testing.T) {
	metrics := NewApplicationMetrics()

	// Verify all metrics are created
	if metrics.HTTPRequestsTotal == nil {
		t.Error("HTTPRequestsTotal metric should be created")
	}

	if metrics.BrowserPoolSize == nil {
		t.Error("BrowserPoolSize metric should be created")
	}

	if metrics.TestExecutionsTotal == nil {
		t.Error("TestExecutionsTotal metric should be created")
	}

	if metrics.MemoryUsage == nil {
		t.Error("MemoryUsage metric should be created")
	}

	// Test metric types
	if metrics.HTTPRequestsTotal.Type != Counter {
		t.Error("HTTPRequestsTotal should be a counter")
	}

	if metrics.BrowserPoolSize.Type != Gauge {
		t.Error("BrowserPoolSize should be a gauge")
	}

	if metrics.HTTPRequestDuration.Type != Histogram {
		t.Error("HTTPRequestDuration should be a histogram")
	}
}

func TestSystemMetricsCollector(t *testing.T) {
	metrics := NewApplicationMetrics()
	collector := NewSystemMetricsCollector(metrics)

	// Test collecting metrics
	collector.collectMetrics()

	// Verify memory usage is set (should be > 0)
	if metrics.MemoryUsage.Get() <= 0 {
		t.Error("Expected memory usage to be > 0")
	}

	// Verify goroutine count is set (should be > 0)
	if metrics.GoroutineCount.Get() <= 0 {
		t.Error("Expected goroutine count to be > 0")
	}
}

func TestSystemMetricsCollector_Start(t *testing.T) {
	metrics := NewApplicationMetrics()
	collector := NewSystemMetricsCollector(metrics)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start collector in background
	done := make(chan struct{})
	go func() {
		collector.Start(ctx)
		close(done)
	}()

	// Wait for context to be cancelled
	<-ctx.Done()

	// Wait for collector to stop
	select {
	case <-done:
		// Success - collector stopped
	case <-time.After(1 * time.Second):
		t.Error("Collector did not stop within timeout")
	}
}

func TestPrometheusHandler(t *testing.T) {
	// Create test metrics
	registry := GetRegistry()
	counter := registry.Counter("test_requests_total", "Total test requests", map[string]string{"method": "GET"})
	gauge := registry.Gauge("test_connections", "Active connections", nil)
	histogram := registry.Histogram("test_duration_seconds", "Request duration", nil, []float64{0.1, 0.5, 1, 2.5, 5})

	// Add some data
	counter.Add(42)
	gauge.Set(15)
	histogram.Observe(0.3)
	histogram.Observe(1.2)
	histogram.Observe(2.8)

	// Create test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Call handler
	handler := PrometheusHandler()
	handler(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	expectedContentType := "text/plain; version=0.0.4; charset=utf-8"
	if contentType != expectedContentType {
		t.Errorf("Expected content type '%s', got '%s'", expectedContentType, contentType)
	}

	body := w.Body.String()

	// Check that metrics are present
	if !strings.Contains(body, "test_requests_total") {
		t.Error("Expected counter metric in output")
	}

	if !strings.Contains(body, "test_connections") {
		t.Error("Expected gauge metric in output")
	}

	if !strings.Contains(body, "test_duration_seconds_bucket") {
		t.Error("Expected histogram buckets in output")
	}

	if !strings.Contains(body, "test_duration_seconds_sum") {
		t.Error("Expected histogram sum in output")
	}

	if !strings.Contains(body, "test_duration_seconds_count") {
		t.Error("Expected histogram count in output")
	}

	// Check values
	if !strings.Contains(body, "42") {
		t.Error("Expected counter value 42 in output")
	}

	if !strings.Contains(body, "15") {
		t.Error("Expected gauge value 15 in output")
	}
}

func TestHealthChecker(t *testing.T) {
	checker := NewHealthChecker()

	// Add some health checks
	checker.AddCheck("database", func() error {
		return nil // Healthy
	})

	checker.AddCheck("cache", func() error {
		return nil // Healthy
	})

	checker.AddCheck("external_service", func() error {
		return fmt.Errorf("service unavailable") // Unhealthy
	})

	// Run checks
	results := checker.Check()

	if len(results) != 3 {
		t.Errorf("Expected 3 health check results, got %d", len(results))
	}

	if results["database"] != nil {
		t.Error("Expected database check to be healthy")
	}

	if results["cache"] != nil {
		t.Error("Expected cache check to be healthy")
	}

	if results["external_service"] == nil {
		t.Error("Expected external_service check to be unhealthy")
	}

	// Test removing a check
	checker.RemoveCheck("cache")
	results = checker.Check()

	if len(results) != 2 {
		t.Errorf("Expected 2 health check results after removal, got %d", len(results))
	}

	if _, exists := results["cache"]; exists {
		t.Error("Expected cache check to be removed")
	}
}

func TestHealthHandler(t *testing.T) {
	checker := NewHealthChecker()

	// Add healthy check
	checker.AddCheck("service1", func() error {
		return nil
	})

	// Add unhealthy check
	checker.AddCheck("service2", func() error {
		return fmt.Errorf("connection failed")
	})

	// Test healthy endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler := checker.HealthHandler()
	handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 (due to unhealthy service), got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected JSON content type, got '%s'", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "unhealthy") {
		t.Error("Expected 'unhealthy' in response body")
	}

	if !strings.Contains(body, "service1") {
		t.Error("Expected 'service1' in response body")
	}

	if !strings.Contains(body, "service2") {
		t.Error("Expected 'service2' in response body")
	}

	if !strings.Contains(body, "connection failed") {
		t.Error("Expected error message in response body")
	}
}

func TestMonitoringServer(t *testing.T) {
	healthChecker := NewHealthChecker()
	metrics := NewApplicationMetrics()

	// Use a random port for testing
	server := NewMonitoringServer(0, healthChecker, metrics)

	// The server would normally be started with server.Start()
	// For testing, we just verify the server is configured correctly
	if server.healthChecker != healthChecker {
		t.Error("Health checker not set correctly")
	}

	if server.metrics != metrics {
		t.Error("Metrics not set correctly")
	}

	if server.server == nil {
		t.Error("HTTP server not configured")
	}
}

func TestFormatLabels(t *testing.T) {
	testCases := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name:     "Empty labels",
			labels:   map[string]string{},
			expected: "",
		},
		{
			name:     "Single label",
			labels:   map[string]string{"method": "GET"},
			expected: `{method="GET"}`,
		},
		{
			name: "Multiple labels",
			labels: map[string]string{
				"method": "GET",
				"status": "200",
			},
			expected: `{method="GET",status="200"}`, // Note: order may vary
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatLabels(tc.labels)

			if tc.expected == "" {
				if result != "" {
					t.Errorf("Expected empty string, got '%s'", result)
				}
				return
			}

			// For multiple labels, just check that all labels are present
			for key, value := range tc.labels {
				expectedPart := fmt.Sprintf(`%s="%s"`, key, value)
				if !strings.Contains(result, expectedPart) {
					t.Errorf("Expected result to contain '%s', got '%s'", expectedPart, result)
				}
			}

			if !strings.HasPrefix(result, "{") || !strings.HasSuffix(result, "}") {
				t.Errorf("Expected result to be wrapped in braces, got '%s'", result)
			}
		})
	}
}

func TestFormatLabelsWithBucket(t *testing.T) {
	labels := map[string]string{"method": "GET"}
	result := formatLabelsWithBucket(labels, 10.5)

	if !strings.Contains(result, `method="GET"`) {
		t.Error("Expected original labels to be preserved")
	}

	if !strings.Contains(result, `le="10.5"`) {
		t.Error("Expected 'le' label to be added")
	}

	// Test with +Inf bucket
	result = formatLabelsWithBucket(labels, "+Inf")
	if !strings.Contains(result, `le="+Inf"`) {
		t.Error("Expected 'le' label with +Inf value")
	}
}

// Benchmark tests
func BenchmarkCounter_Inc(b *testing.B) {
	registry := &MetricsRegistry{}
	counter := registry.Counter("bench_counter", "Benchmark counter", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Inc()
	}
}

func BenchmarkGauge_Set(b *testing.B) {
	registry := &MetricsRegistry{}
	gauge := registry.Gauge("bench_gauge", "Benchmark gauge", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gauge.Set(float64(i))
	}
}

func BenchmarkHistogram_Observe(b *testing.B) {
	registry := &MetricsRegistry{}
	histogram := registry.Histogram("bench_histogram", "Benchmark histogram", nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		histogram.Observe(float64(i))
	}
}

func BenchmarkPrometheusHandler(b *testing.B) {
	// Create some test metrics
	registry := GetRegistry()
	counter := registry.Counter("bench_requests_total", "Total requests", nil)
	gauge := registry.Gauge("bench_connections", "Active connections", nil)
	histogram := registry.Histogram("bench_duration", "Request duration", nil, nil)

	// Add some data
	counter.Add(1000)
	gauge.Set(50)
	for i := 0; i < 100; i++ {
		histogram.Observe(float64(i))
	}

	handler := PrometheusHandler()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()
		handler(w, req)
	}
}