package geo

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: Simplified Geographic Router
Start with single region, expand to multi-region when needed
No over-engineering - distributed systems on demand
*/

type Region struct {
	Name     string
	Endpoint string
	Capacity int
	Current  int
	Healthy  bool
	mu       sync.Mutex
}

type Router struct {
	regions sync.Map // map[string]*Region
}

// NewRouter creates a new geo router
func NewRouter() *Router {
	r := &Router{}

	// nkk: Start with single region
	// Add more regions when scaling beyond 1000 users
	r.addRegion("us-east", "http://localhost:8080", 1000)

	return r
}

func (r *Router) addRegion(name, endpoint string, capacity int) {
	region := &Region{
		Name:     name,
		Endpoint: endpoint,
		Capacity: capacity,
		Healthy:  true,
	}

	r.regions.Store(name, region)

	logger.Info("Added region",
		zap.String("name", name),
		zap.String("endpoint", endpoint),
		zap.Int("capacity", capacity))
}

// RouteRequest routes to best region
func (r *Router) RouteRequest(ctx context.Context, clientIP string) (*Region, error) {
	// nkk: For now, return default region
	// Add geo-IP routing when scaling globally

	if val, ok := r.regions.Load("us-east"); ok {
		region := val.(*Region)

		region.mu.Lock()
		defer region.mu.Unlock()

		if !region.Healthy {
			return nil, fmt.Errorf("region unhealthy")
		}

		if region.Current >= region.Capacity {
			return nil, fmt.Errorf("region at capacity")
		}

		region.Current++
		return region, nil
	}

	return nil, fmt.Errorf("no regions available")
}

// ReleaseCapacity releases capacity in a region
func (r *Router) ReleaseCapacity(regionName string) {
	if val, ok := r.regions.Load(regionName); ok {
		region := val.(*Region)
		region.mu.Lock()
		if region.Current > 0 {
			region.Current--
		}
		region.mu.Unlock()
	}
}

// GetRegionStats gets stats for all regions
func (r *Router) GetRegionStats() map[string]map[string]interface{} {
	stats := make(map[string]map[string]interface{})

	r.regions.Range(func(key, value interface{}) bool {
		region := value.(*Region)
		region.mu.Lock()
		stats[region.Name] = map[string]interface{}{
			"endpoint":    region.Endpoint,
			"capacity":    region.Capacity,
			"current":     region.Current,
			"healthy":     region.Healthy,
			"utilization": float64(region.Current) / float64(region.Capacity) * 100,
		}
		region.mu.Unlock()
		return true
	})

	return stats
}

// HealthCheck checks health of all regions
func (r *Router) HealthCheck(ctx context.Context) {
	// nkk: Production health check implementation
	// Based on Google's load balancer health checks

	var wg sync.WaitGroup

	r.regions.Range(func(key, value interface{}) bool {
		region := value.(*Region)

		wg.Add(1)
		go func(reg *Region) {
			defer wg.Done()

			// nkk: HTTP health check with timeout
			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			// nkk: Get endpoint without holding lock
			reg.mu.Lock()
			endpoint := reg.Endpoint
			name := reg.Name
			reg.mu.Unlock()

			healthURL := fmt.Sprintf("%s/health", endpoint)
			resp, err := client.Get(healthURL)

			// nkk: Update health status with minimal lock time
			reg.mu.Lock()
			defer reg.mu.Unlock()

			if err != nil {
				logger.Warn("Region health check failed",
					zap.String("region", name),
					zap.Error(err))
				reg.Healthy = false
				return
			}
			defer resp.Body.Close()

			// nkk: Check response status
			if resp.StatusCode == http.StatusOK {
				reg.Healthy = true
				logger.Debug("Region healthy",
					zap.String("region", name))
			} else {
				reg.Healthy = false
				logger.Warn("Region unhealthy",
					zap.String("region", name),
					zap.Int("status", resp.StatusCode))
			}
		}(region)

		return true
	})

	// nkk: Wait for all health checks with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All checks completed
	case <-ctx.Done():
		logger.Warn("Health check timeout exceeded")
	case <-time.After(10 * time.Second):
		logger.Warn("Health check timeout")
	}
}

// StartHealthCheckLoop starts periodic health checks
func (r *Router) StartHealthCheckLoop(interval time.Duration) {
	// nkk: Periodic health monitoring
	ticker := time.NewTicker(interval)

	go func() {
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			r.HealthCheck(ctx)
			cancel()

			// nkk: Log unhealthy regions
			r.regions.Range(func(key, value interface{}) bool {
				region := value.(*Region)
				if !region.Healthy {
					logger.Error("Region is unhealthy",
						zap.String("region", region.Name),
						zap.String("endpoint", region.Endpoint))
				}
				return true
			})
		}
	}()

	logger.Info("Started health check loop",
		zap.Duration("interval", interval))
}