package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"agent/logger"
	"agent/services/billing"
	"agent/services/browser_pool"
	"agent/services/geo"
	"agent/services/recorder"
	"agent/services/tenant"
	"agent/services/tunnel"
)

/*
nkk: Centralized Health Check Handler
Design by Meta SRE team:
- Parallel health checks for all services
- Configurable timeouts
- Detailed status reporting
- Prometheus-compatible metrics
*/

type ServiceHealth struct {
	Name      string                 `json:"name"`
	Status    string                 `json:"status"` // healthy, degraded, unhealthy
	Latency   time.Duration          `json:"latency_ms"`
	Details   map[string]interface{} `json:"details,omitempty"`
	LastCheck time.Time              `json:"last_check"`
}

type HealthHandler struct {
	browserPool     *browser_pool.BrowserPoolManager
	tunnelService   *tunnel.TunnelService
	tenantManager   *tenant.Manager
	billingService  *billing.Service
	geoRouter       *geo.Router
	sessionRecorder *recorder.SessionRecorder

	mu              sync.RWMutex
	serviceStatuses map[string]*ServiceHealth
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(
	browserPool *browser_pool.BrowserPoolManager,
	tunnelService *tunnel.TunnelService,
	tenantManager *tenant.Manager,
	billingService *billing.Service,
	geoRouter *geo.Router,
	sessionRecorder *recorder.SessionRecorder,
) *HealthHandler {
	return &HealthHandler{
		browserPool:     browserPool,
		tunnelService:   tunnelService,
		tenantManager:   tenantManager,
		billingService:  billingService,
		geoRouter:       geoRouter,
		sessionRecorder: sessionRecorder,
		serviceStatuses: make(map[string]*ServiceHealth),
	}
}

// ServeHTTP handles health check requests
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// nkk: Support both simple and detailed health checks
	detailed := r.URL.Query().Get("detailed") == "true"

	if detailed {
		h.handleDetailedHealth(w, r)
	} else {
		h.handleSimpleHealth(w, r)
	}
}

// handleSimpleHealth returns simple health status
func (h *HealthHandler) handleSimpleHealth(w http.ResponseWriter, r *http.Request) {
	// nkk: Quick health check for load balancers
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	healthy := h.checkAllServices(ctx)

	if healthy {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("UNHEALTHY"))
	}
}

// handleDetailedHealth returns detailed health information
func (h *HealthHandler) handleDetailedHealth(w http.ResponseWriter, r *http.Request) {
	// nkk: Detailed health for debugging
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	statuses := h.checkAllServicesDetailed(ctx)

	response := map[string]interface{}{
		"status":    h.getOverallStatus(statuses),
		"timestamp": time.Now().Unix(),
		"services":  statuses,
	}

	// nkk: Set appropriate status code
	if response["status"] == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else if response["status"] == "degraded" {
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// checkAllServices performs quick health check
func (h *HealthHandler) checkAllServices(ctx context.Context) bool {
	// nkk: Parallel health checks using goroutines
	checks := []func(context.Context) bool{
		h.checkBrowserPool,
		h.checkTunnelService,
		h.checkTenantManager,
		h.checkBillingService,
		h.checkGeoRouter,
		h.checkSessionRecorder,
	}

	var wg sync.WaitGroup
	results := make(chan bool, len(checks))

	for _, check := range checks {
		wg.Add(1)
		go func(fn func(context.Context) bool) {
			defer wg.Done()
			results <- fn(ctx)
		}(check)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// nkk: All services must be healthy
	for result := range results {
		if !result {
			return false
		}
	}

	return true
}

// checkAllServicesDetailed performs detailed health check
func (h *HealthHandler) checkAllServicesDetailed(ctx context.Context) []ServiceHealth {
	// nkk: Collect detailed metrics from each service
	var wg sync.WaitGroup
	statuses := make([]ServiceHealth, 0, 6)
	statusChan := make(chan ServiceHealth, 6)

	services := []struct {
		name  string
		check func(context.Context) ServiceHealth
	}{
		{"browser_pool", h.checkBrowserPoolDetailed},
		{"tunnel_service", h.checkTunnelServiceDetailed},
		{"tenant_manager", h.checkTenantManagerDetailed},
		{"billing_service", h.checkBillingServiceDetailed},
		{"geo_router", h.checkGeoRouterDetailed},
		{"session_recorder", h.checkSessionRecorderDetailed},
	}

	for _, svc := range services {
		wg.Add(1)
		go func(name string, checkFn func(context.Context) ServiceHealth) {
			defer wg.Done()
			start := time.Now()
			status := checkFn(ctx)
			status.Name = name
			status.Latency = time.Since(start)
			status.LastCheck = time.Now()
			statusChan <- status
		}(svc.name, svc.check)
	}

	go func() {
		wg.Wait()
		close(statusChan)
	}()

	for status := range statusChan {
		statuses = append(statuses, status)
		// nkk: Cache status for monitoring
		h.mu.Lock()
		h.serviceStatuses[status.Name] = &status
		h.mu.Unlock()
	}

	return statuses
}

// Service-specific health checks

func (h *HealthHandler) checkBrowserPool(ctx context.Context) bool {
	if h.browserPool == nil {
		return false
	}
	stats := h.browserPool.GetPoolStats()
	return len(stats) > 0
}

func (h *HealthHandler) checkBrowserPoolDetailed(ctx context.Context) ServiceHealth {
	status := ServiceHealth{Status: "unhealthy"}

	if h.browserPool == nil {
		return status
	}

	stats := h.browserPool.GetPoolStats()
	totalAvailable := 0
	totalInUse := 0

	for _, browserStats := range stats {
		if available, ok := browserStats["available"].(int); ok {
			totalAvailable += available
		}
		if inUse, ok := browserStats["in_use"].(int); ok {
			totalInUse += inUse
		}
	}

	status.Details = map[string]interface{}{
		"total_available": totalAvailable,
		"total_in_use":    totalInUse,
		"pools":           stats,
	}

	if totalAvailable > 0 {
		status.Status = "healthy"
	} else if totalInUse > 0 {
		status.Status = "degraded"
	}

	return status
}

func (h *HealthHandler) checkTunnelService(ctx context.Context) bool {
	return h.tunnelService != nil
}

func (h *HealthHandler) checkTunnelServiceDetailed(ctx context.Context) ServiceHealth {
	status := ServiceHealth{Status: "unhealthy"}

	if h.tunnelService != nil {
		status.Status = "healthy"
		// nkk: Add tunnel metrics when available
	}

	return status
}

func (h *HealthHandler) checkTenantManager(ctx context.Context) bool {
	return h.tenantManager != nil
}

func (h *HealthHandler) checkTenantManagerDetailed(ctx context.Context) ServiceHealth {
	status := ServiceHealth{Status: "unhealthy"}

	if h.tenantManager != nil {
		status.Status = "healthy"
		// nkk: Add tenant metrics when available
	}

	return status
}

func (h *HealthHandler) checkBillingService(ctx context.Context) bool {
	return h.billingService != nil
}

func (h *HealthHandler) checkBillingServiceDetailed(ctx context.Context) ServiceHealth {
	status := ServiceHealth{Status: "unhealthy"}

	if h.billingService != nil {
		status.Status = "healthy"
		// nkk: Add billing metrics when available
	}

	return status
}

func (h *HealthHandler) checkGeoRouter(ctx context.Context) bool {
	if h.geoRouter == nil {
		return false
	}
	stats := h.geoRouter.GetRegionStats()
	return len(stats) > 0
}

func (h *HealthHandler) checkGeoRouterDetailed(ctx context.Context) ServiceHealth {
	status := ServiceHealth{Status: "unhealthy"}

	if h.geoRouter == nil {
		return status
	}

	regionStats := h.geoRouter.GetRegionStats()

	status.Details = map[string]interface{}{
		"regions": regionStats,
	}

	// nkk: Check if any region is healthy
	for _, region := range regionStats {
		if healthy, ok := region["healthy"].(bool); ok && healthy {
			status.Status = "healthy"
			break
		}
	}

	return status
}

func (h *HealthHandler) checkSessionRecorder(ctx context.Context) bool {
	return h.sessionRecorder != nil
}

func (h *HealthHandler) checkSessionRecorderDetailed(ctx context.Context) ServiceHealth {
	status := ServiceHealth{Status: "unhealthy"}

	if h.sessionRecorder != nil {
		status.Status = "healthy"
		// nkk: Add recording metrics when available
	}

	return status
}

// getOverallStatus determines overall system health
func (h *HealthHandler) getOverallStatus(statuses []ServiceHealth) string {
	// nkk: System is healthy only if all services are healthy
	unhealthy := 0
	degraded := 0

	for _, status := range statuses {
		switch status.Status {
		case "unhealthy":
			unhealthy++
		case "degraded":
			degraded++
		}
	}

	if unhealthy > 0 {
		return "unhealthy"
	} else if degraded > 0 {
		return "degraded"
	}

	return "healthy"
}

// StartBackgroundChecks starts periodic health checks
func (h *HealthHandler) StartBackgroundChecks(interval time.Duration) {
	// nkk: Periodic health checks for monitoring
	ticker := time.NewTicker(interval)

	go func() {
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			h.checkAllServicesDetailed(ctx)
			cancel()

			// nkk: Log unhealthy services
			h.mu.RLock()
			for name, status := range h.serviceStatuses {
				if status.Status != "healthy" {
					logger.Warn("Service unhealthy",
						zap.String("service", name),
						zap.String("status", status.Status))
				}
			}
			h.mu.RUnlock()
		}
	}()
}

// GetMetrics returns Prometheus-compatible metrics
func (h *HealthHandler) GetMetrics() []byte {
	// nkk: Export metrics in Prometheus format
	h.mu.RLock()
	defer h.mu.RUnlock()

	metrics := "# HELP service_health Service health status (1=healthy, 0.5=degraded, 0=unhealthy)\n"
	metrics += "# TYPE service_health gauge\n"

	for name, status := range h.serviceStatuses {
		value := 0.0
		switch status.Status {
		case "healthy":
			value = 1.0
		case "degraded":
			value = 0.5
		}

		metrics += fmt.Sprintf("service_health{service=\"%s\"} %f\n", name, value)
		metrics += fmt.Sprintf("service_health_latency_ms{service=\"%s\"} %d\n", name, status.Latency.Milliseconds())
	}

	return []byte(metrics)
}