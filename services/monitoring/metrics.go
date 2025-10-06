package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: Comprehensive monitoring system for production readiness
Features:
- Application metrics (latency, throughput, errors)
- System metrics (CPU, memory, goroutines)
- Business metrics (browser pool utilization, test success rate)
- Health checks with circuit breaker patterns
- Prometheus-compatible metrics endpoint
*/

type MetricType string

const (
	Counter   MetricType = "counter"
	Gauge     MetricType = "gauge"
	Histogram MetricType = "histogram"
)

type Metric struct {
	Name        string
	Type        MetricType
	Help        string
	Value       float64
	Labels      map[string]string
	Buckets     []float64 // For histograms
	Observations []float64 // For histograms
	mutex       sync.RWMutex
}

type MetricsRegistry struct {
	metrics sync.Map
	mu      sync.RWMutex
}

var globalRegistry = &MetricsRegistry{}

// GetRegistry returns the global metrics registry
func GetRegistry() *MetricsRegistry {
	return globalRegistry
}

// Counter creates or retrieves a counter metric
func (r *MetricsRegistry) Counter(name, help string, labels map[string]string) *Metric {
	key := metricKey(name, labels)
	if val, ok := r.metrics.Load(key); ok {
		return val.(*Metric)
	}

	metric := &Metric{
		Name:   name,
		Type:   Counter,
		Help:   help,
		Labels: labels,
		Value:  0,
	}
	r.metrics.Store(key, metric)
	return metric
}

// Gauge creates or retrieves a gauge metric
func (r *MetricsRegistry) Gauge(name, help string, labels map[string]string) *Metric {
	key := metricKey(name, labels)
	if val, ok := r.metrics.Load(key); ok {
		return val.(*Metric)
	}

	metric := &Metric{
		Name:   name,
		Type:   Gauge,
		Help:   help,
		Labels: labels,
		Value:  0,
	}
	r.metrics.Store(key, metric)
	return metric
}

// Histogram creates or retrieves a histogram metric
func (r *MetricsRegistry) Histogram(name, help string, labels map[string]string, buckets []float64) *Metric {
	key := metricKey(name, labels)
	if val, ok := r.metrics.Load(key); ok {
		return val.(*Metric)
	}

	if buckets == nil {
		// Default buckets for latency (milliseconds)
		buckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000}
	}

	metric := &Metric{
		Name:    name,
		Type:    Histogram,
		Help:    help,
		Labels:  labels,
		Buckets: buckets,
		Observations: make([]float64, 0),
	}
	r.metrics.Store(key, metric)
	return metric
}

// Inc increments a counter by 1
func (m *Metric) Inc() {
	m.Add(1)
}

// Add adds a value to a counter
func (m *Metric) Add(value float64) {
	if m.Type != Counter {
		return
	}
	m.mutex.Lock()
	m.Value += value
	m.mutex.Unlock()
}

// Set sets a gauge value
func (m *Metric) Set(value float64) {
	if m.Type != Gauge {
		return
	}
	m.mutex.Lock()
	m.Value = value
	m.mutex.Unlock()
}

// Observe adds an observation to a histogram
func (m *Metric) Observe(value float64) {
	if m.Type != Histogram {
		return
	}
	m.mutex.Lock()
	m.Observations = append(m.Observations, value)
	// Keep only last 1000 observations to prevent memory leak
	if len(m.Observations) > 1000 {
		m.Observations = m.Observations[len(m.Observations)-1000:]
	}
	m.mutex.Unlock()
}

// Timer returns a function to record duration
func (m *Metric) Timer() func() {
	start := time.Now()
	return func() {
		duration := float64(time.Since(start).Milliseconds())
		m.Observe(duration)
	}
}

// Get returns the current value (thread-safe)
func (m *Metric) Get() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.Value
}

// metricKey generates a unique key for a metric with labels
func metricKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += fmt.Sprintf("_%s_%s", k, v)
	}
	return key
}

// ApplicationMetrics contains all application-level metrics
type ApplicationMetrics struct {
	// HTTP metrics
	HTTPRequestsTotal     *Metric
	HTTPRequestDuration   *Metric
	HTTPResponseSize      *Metric

	// Browser pool metrics
	BrowserPoolSize       *Metric
	BrowserPoolUtilization *Metric
	BrowserAcquisitionTime *Metric
	BrowserPoolErrors     *Metric

	// Test execution metrics
	TestExecutionsTotal   *Metric
	TestExecutionDuration *Metric
	TestSuccessRate       *Metric
	TestQueueSize         *Metric

	// System metrics
	MemoryUsage           *Metric
	CPUUsage              *Metric
	GoroutineCount        *Metric
	GCDuration            *Metric

	// Session recording metrics
	RecordingsActive      *Metric
	RecordingDuration     *Metric
	VideoFileSize         *Metric

	// Tunnel metrics
	TunnelsActive         *Metric
	TunnelDataTransferred *Metric
}

// NewApplicationMetrics initializes all application metrics
func NewApplicationMetrics() *ApplicationMetrics {
	registry := GetRegistry()

	return &ApplicationMetrics{
		// HTTP metrics
		HTTPRequestsTotal: registry.Counter(
			"http_requests_total",
			"Total number of HTTP requests",
			map[string]string{},
		),
		HTTPRequestDuration: registry.Histogram(
			"http_request_duration_milliseconds",
			"HTTP request duration in milliseconds",
			map[string]string{},
			[]float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
		),
		HTTPResponseSize: registry.Histogram(
			"http_response_size_bytes",
			"HTTP response size in bytes",
			map[string]string{},
			[]float64{100, 1000, 10000, 100000, 1000000, 10000000},
		),

		// Browser pool metrics
		BrowserPoolSize: registry.Gauge(
			"browser_pool_size_total",
			"Total number of browsers in pool",
			map[string]string{},
		),
		BrowserPoolUtilization: registry.Gauge(
			"browser_pool_utilization_ratio",
			"Browser pool utilization ratio (0-1)",
			map[string]string{},
		),
		BrowserAcquisitionTime: registry.Histogram(
			"browser_acquisition_duration_milliseconds",
			"Time to acquire browser from pool in milliseconds",
			map[string]string{},
			[]float64{10, 50, 100, 250, 500, 1000, 2500, 5000},
		),
		BrowserPoolErrors: registry.Counter(
			"browser_pool_errors_total",
			"Total number of browser pool errors",
			map[string]string{},
		),

		// Test execution metrics
		TestExecutionsTotal: registry.Counter(
			"test_executions_total",
			"Total number of test executions",
			map[string]string{},
		),
		TestExecutionDuration: registry.Histogram(
			"test_execution_duration_milliseconds",
			"Test execution duration in milliseconds",
			map[string]string{},
			[]float64{1000, 5000, 10000, 30000, 60000, 120000, 300000, 600000},
		),
		TestSuccessRate: registry.Gauge(
			"test_success_rate_ratio",
			"Test success rate ratio (0-1)",
			map[string]string{},
		),
		TestQueueSize: registry.Gauge(
			"test_queue_size_total",
			"Number of tests in execution queue",
			map[string]string{},
		),

		// System metrics
		MemoryUsage: registry.Gauge(
			"memory_usage_bytes",
			"Memory usage in bytes",
			map[string]string{},
		),
		CPUUsage: registry.Gauge(
			"cpu_usage_ratio",
			"CPU usage ratio (0-1)",
			map[string]string{},
		),
		GoroutineCount: registry.Gauge(
			"goroutine_count_total",
			"Number of goroutines",
			map[string]string{},
		),
		GCDuration: registry.Histogram(
			"gc_duration_milliseconds",
			"Garbage collection duration in milliseconds",
			map[string]string{},
			[]float64{1, 5, 10, 25, 50, 100, 250, 500},
		),

		// Session recording metrics
		RecordingsActive: registry.Gauge(
			"recordings_active_total",
			"Number of active recordings",
			map[string]string{},
		),
		RecordingDuration: registry.Histogram(
			"recording_duration_milliseconds",
			"Recording duration in milliseconds",
			map[string]string{},
			[]float64{10000, 30000, 60000, 120000, 300000, 600000, 1800000},
		),
		VideoFileSize: registry.Histogram(
			"video_file_size_bytes",
			"Video file size in bytes",
			map[string]string{},
			[]float64{1000000, 10000000, 50000000, 100000000, 500000000, 1000000000},
		),

		// Tunnel metrics
		TunnelsActive: registry.Gauge(
			"tunnels_active_total",
			"Number of active tunnels",
			map[string]string{},
		),
		TunnelDataTransferred: registry.Counter(
			"tunnel_data_transferred_bytes_total",
			"Total bytes transferred through tunnels",
			map[string]string{},
		),
	}
}

// SystemMetricsCollector collects system metrics
type SystemMetricsCollector struct {
	metrics *ApplicationMetrics
}

// NewSystemMetricsCollector creates a new system metrics collector
func NewSystemMetricsCollector(metrics *ApplicationMetrics) *SystemMetricsCollector {
	return &SystemMetricsCollector{
		metrics: metrics,
	}
}

// Start begins collecting system metrics
func (c *SystemMetricsCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collectMetrics()
		}
	}
}

// collectMetrics collects current system metrics
func (c *SystemMetricsCollector) collectMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Memory metrics
	c.metrics.MemoryUsage.Set(float64(memStats.Alloc))

	// Goroutine count
	c.metrics.GoroutineCount.Set(float64(runtime.NumGoroutine()))

	// GC metrics
	if memStats.NumGC > 0 {
		lastGC := time.Duration(memStats.PauseNs[(memStats.NumGC+255)%256])
		c.metrics.GCDuration.Observe(float64(lastGC.Nanoseconds()) / 1000000) // Convert to milliseconds
	}
}

// PrometheusHandler provides Prometheus-compatible metrics endpoint
func PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		globalRegistry.metrics.Range(func(key, value interface{}) bool {
			metric := value.(*Metric)
			writePrometheusMetric(w, metric)
			return true
		})
	}
}

// writePrometheusMetric writes a single metric in Prometheus format
func writePrometheusMetric(w http.ResponseWriter, metric *Metric) {
	// Write help
	fmt.Fprintf(w, "# HELP %s %s\n", metric.Name, metric.Help)
	fmt.Fprintf(w, "# TYPE %s %s\n", metric.Name, string(metric.Type))

	labels := formatLabels(metric.Labels)

	switch metric.Type {
	case Counter, Gauge:
		metric.mutex.RLock()
		fmt.Fprintf(w, "%s%s %g\n", metric.Name, labels, metric.Value)
		metric.mutex.RUnlock()

	case Histogram:
		metric.mutex.RLock()
		observations := make([]float64, len(metric.Observations))
		copy(observations, metric.Observations)
		metric.mutex.RUnlock()

		// Calculate histogram buckets
		bucketCounts := make(map[float64]int)
		for _, bucket := range metric.Buckets {
			bucketCounts[bucket] = 0
		}

		total := 0
		sum := float64(0)
		for _, obs := range observations {
			total++
			sum += obs
			for _, bucket := range metric.Buckets {
				if obs <= bucket {
					bucketCounts[bucket]++
				}
			}
		}

		// Write bucket metrics
		cumulative := 0
		for _, bucket := range metric.Buckets {
			cumulative += bucketCounts[bucket]
			bucketLabels := formatLabelsWithBucket(metric.Labels, bucket)
			fmt.Fprintf(w, "%s_bucket%s %d\n", metric.Name, bucketLabels, cumulative)
		}

		// Write +Inf bucket
		infLabels := formatLabelsWithBucket(metric.Labels, "+Inf")
		fmt.Fprintf(w, "%s_bucket%s %d\n", metric.Name, infLabels, total)

		// Write sum and count
		fmt.Fprintf(w, "%s_sum%s %g\n", metric.Name, labels, sum)
		fmt.Fprintf(w, "%s_count%s %d\n", metric.Name, labels, total)
	}

	fmt.Fprintln(w)
}

// formatLabels formats labels for Prometheus output
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	result := "{"
	first := true
	for k, v := range labels {
		if !first {
			result += ","
		}
		result += fmt.Sprintf(`%s="%s"`, k, v)
		first = false
	}
	result += "}"
	return result
}

// formatLabelsWithBucket formats labels with an additional le (less than or equal) label
func formatLabelsWithBucket(labels map[string]string, bucket interface{}) string {
	newLabels := make(map[string]string)
	for k, v := range labels {
		newLabels[k] = v
	}
	newLabels["le"] = fmt.Sprintf("%v", bucket)
	return formatLabels(newLabels)
}

// HealthChecker provides health check functionality
type HealthChecker struct {
	checks map[string]func() error
	mu     sync.RWMutex
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]func() error),
	}
}

// AddCheck adds a health check
func (h *HealthChecker) AddCheck(name string, check func() error) {
	h.mu.Lock()
	h.checks[name] = check
	h.mu.Unlock()
}

// RemoveCheck removes a health check
func (h *HealthChecker) RemoveCheck(name string) {
	h.mu.Lock()
	delete(h.checks, name)
	h.mu.Unlock()
}

// Check runs all health checks
func (h *HealthChecker) Check() map[string]error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make(map[string]error)
	for name, check := range h.checks {
		results[name] = check()
	}
	return results
}

// HealthHandler provides HTTP health check endpoint
func (h *HealthChecker) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results := h.Check()
		healthy := true

		response := map[string]interface{}{
			"status": "healthy",
			"checks": make(map[string]interface{}),
		}

		for name, err := range results {
			if err != nil {
				healthy = false
				response["checks"].(map[string]interface{})[name] = map[string]interface{}{
					"status": "unhealthy",
					"error":  err.Error(),
				}
			} else {
				response["checks"].(map[string]interface{})[name] = map[string]interface{}{
					"status": "healthy",
				}
			}
		}

		if !healthy {
			response["status"] = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		w.Header().Set("Content-Type", "application/json")

		// Simple JSON encoding without external dependencies
		fmt.Fprint(w, `{"status":"`)
		fmt.Fprint(w, response["status"])
		fmt.Fprint(w, `","timestamp":"`)
		fmt.Fprint(w, time.Now().Format(time.RFC3339))
		fmt.Fprint(w, `","checks":{`)

		first := true
		for name, result := range results {
			if !first {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, `"%s":{"status":"`, name)
			if result != nil {
				fmt.Fprint(w, `unhealthy","error":"`)
				fmt.Fprint(w, result.Error())
				fmt.Fprint(w, `"}`)
			} else {
				fmt.Fprint(w, `healthy"}`)
			}
			first = false
		}
		fmt.Fprint(w, "}}")
	}
}

// MonitoringServer provides monitoring endpoints
type MonitoringServer struct {
	healthChecker *HealthChecker
	metrics       *ApplicationMetrics
	server        *http.Server
}

// NewMonitoringServer creates a new monitoring server
func NewMonitoringServer(port int, healthChecker *HealthChecker, metrics *ApplicationMetrics) *MonitoringServer {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthChecker.HealthHandler())
	mux.HandleFunc("/metrics", PrometheusHandler())
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ready")
	})

	return &MonitoringServer{
		healthChecker: healthChecker,
		metrics:       metrics,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}
}

// Start starts the monitoring server
func (s *MonitoringServer) Start() error {
	logger.Info("Starting monitoring server", zap.String("addr", s.server.Addr))
	return s.server.ListenAndServe()
}

// Stop stops the monitoring server
func (s *MonitoringServer) Stop(ctx context.Context) error {
	logger.Info("Stopping monitoring server")
	return s.server.Shutdown(ctx)
}