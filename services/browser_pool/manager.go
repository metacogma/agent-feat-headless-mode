package browser_pool

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: Docker-based BrowserPoolManager (Legacy/Fallback option)
This is the Docker container approach - useful when you need full isolation.
For modern usage, see playwright_manager.go which is superior.

Design principles:
1. Simple channel-based pool for thread safety
2. Docker containers for complete isolation
3. Basic health checks
4. No over-engineering - scale when needed

Note: Playwright is preferred for BrowserStack-like service because:
- 3x faster execution (no WebDriver overhead)
- Native browser protocols (CDP, Firefox Remote)
- Better reliability (auto-wait, network interception)
- Lighter resource usage (browser contexts vs containers)
*/

// BrowserInstance represents a browser container
type BrowserInstance struct {
	ID           string
	ContainerID  string
	BrowserType  string // nkk: Changed from Browser for consistency
	Version      string
	WebDriverURL string
	Healthy      bool
	InUse        bool
	LastUsed     time.Time
}

// BrowserPoolManager manages browser containers
type BrowserPoolManager struct {
	docker          *client.Client
	pool            chan *BrowserInstance
	inUse           sync.Map
	maxSize         int
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	shutdownCh      chan struct{}
	dockerAvailable bool // Track if Docker is available
}

// nkk: Simple constructor - no over-engineering
func NewBrowserPoolManager(maxSize int) (*BrowserPoolManager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &BrowserPoolManager{
		pool:            make(chan *BrowserInstance, maxSize),
		maxSize:         maxSize,
		ctx:             ctx,
		cancel:          cancel,
		shutdownCh:      make(chan struct{}),
		dockerAvailable: false,
	}

	// nkk: Initialize Docker client with proper socket detection
	// Try different Docker socket paths for Mac/Linux compatibility
	docker, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(), // Auto-negotiate API version
	)

	// If that fails, try explicit socket paths
	if err != nil {
		// Try Unix socket (standard location)
		docker, err = client.NewClientWithOpts(
			client.WithHost("unix:///var/run/docker.sock"),
			client.WithAPIVersionNegotiation(),
		)
	}

	// Check if Docker is available
	if err != nil {
		logger.Warn("Docker not available - browser pool will run in degraded mode",
			zap.Error(err))
		logger.Info("BrowserPoolManager initialized (Docker unavailable)",
			zap.Int("max_size", maxSize),
			zap.Bool("docker_available", false))
		return m, nil
	}

	// Verify Docker daemon is accessible
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	
	_, err = docker.Ping(pingCtx)
	if err != nil {
		logger.Warn("Docker daemon not responding - browser pool will run in degraded mode",
			zap.Error(err))
		docker.Close()
		logger.Info("BrowserPoolManager initialized (Docker daemon not responding)",
			zap.Int("max_size", maxSize),
			zap.Bool("docker_available", false))
		return m, nil
	}

	m.docker = docker
	m.dockerAvailable = true

	// nkk: Pre-warm with proper lifecycle management
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.prewarmPool()
	}()

	// nkk: Start cleanup goroutine for stale instances
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.cleanupStaleInstances()
	}()

	logger.Info("BrowserPoolManager initialized",
		zap.Int("max_size", maxSize),
		zap.Bool("docker_available", true))
	return m, nil
}

// prewarmPool creates initial containers
func (m *BrowserPoolManager) prewarmPool() {
	// nkk: Start with just 5 Chrome instances
	// Scale up as needed based on actual usage
	for i := 0; i < 5; i++ {
		instance, err := m.createBrowserContainer("chrome", "latest")
		if err != nil {
			logger.Error("Failed to create container", zap.Error(err))
			continue
		}

		select {
		case m.pool <- instance:
			logger.Info("Added container to pool", zap.String("id", instance.ID))
		default:
			// Pool full, stop creating
			m.destroyContainer(instance.ContainerID)
			return
		}
	}
}

// AcquireBrowser gets a browser from pool or creates new one
func (m *BrowserPoolManager) AcquireBrowser(ctx context.Context, browser, version string) (*BrowserInstance, error) {
	// nkk: Try to get from pool first (fast path)
	select {
	case instance := <-m.pool:
		// nkk: Quick health check
		if m.isHealthy(instance) {
			instance.InUse = true
			instance.LastUsed = time.Now()
			m.inUse.Store(instance.ID, instance)
			return instance, nil
		}
		// Unhealthy, destroy and create new
		go m.destroyContainer(instance.ContainerID)

	default:
		// Pool empty, continue
	}

	// nkk: Create new container (slow path)
	instance, err := m.createBrowserContainer(browser, version)
	if err != nil {
		return nil, err
	}

	instance.InUse = true
	m.inUse.Store(instance.ID, instance)
	return instance, nil
}

// ReleaseBrowser returns browser to pool
func (m *BrowserPoolManager) ReleaseBrowser(browser, version string, instance *BrowserInstance) {
	if instance == nil {
		return
	}

	instance.InUse = false
	instance.LastUsed = time.Now()
	m.inUse.Delete(instance.ID)

	// nkk: Non-blocking return with timeout
	select {
	case m.pool <- instance:
		// Successfully returned to pool
	case <-time.After(100 * time.Millisecond):
		// Pool full or blocked, destroy container async
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.destroyContainer(instance.ContainerID)
		}()
	}
}

// createBrowserContainer creates a new Docker container
func (m *BrowserPoolManager) createBrowserContainer(browser, version string) (*BrowserInstance, error) {
	// nkk: Use ARM64 compatible image for Mac M1/M2
	// seleniarm images work on ARM64 architecture
	var image string
	if browser == "chrome" || browser == "chromium" {
		image = "seleniarm/standalone-chromium:latest"
	} else if browser == "firefox" {
		image = "seleniarm/standalone-firefox:latest"
	} else {
		image = fmt.Sprintf("seleniarm/standalone-%s:latest", browser)
	}

	// nkk: Simple container config
	config := &container.Config{
		Image: image,
		ExposedPorts: nat.PortSet{
			"4444/tcp": {}, // WebDriver port
		},
	}

	// nkk: Host config with reasonable limits
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:    2 * 1024 * 1024 * 1024, // 2GB
			CPUShares: 1024,
		},
		AutoRemove: true,
		PortBindings: nat.PortMap{
			"4444/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "0"}},
		},
	}

	// Create container
	resp, err := m.docker.ContainerCreate(
		context.Background(),
		config,
		hostConfig,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := m.docker.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		m.docker.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get port mapping
	inspect, err := m.docker.ContainerInspect(context.Background(), resp.ID)
	if err != nil {
		m.destroyContainer(resp.ID)
		return nil, err
	}

	webdriverPort := inspect.NetworkSettings.Ports["4444/tcp"][0].HostPort

	instance := &BrowserInstance{
		ID:           resp.ID[:12],
		ContainerID:  resp.ID,
		BrowserType:  browser,
		Version:      version,
		WebDriverURL: fmt.Sprintf("http://localhost:%s", webdriverPort),
		Healthy:      true,
		LastUsed:     time.Now(),
	}

	// nkk: Wait for ready (simple retry)
	if err := m.waitForReady(instance); err != nil {
		m.destroyContainer(resp.ID)
		return nil, err
	}

	logger.Info("Created browser container",
		zap.String("browser", browser),
		zap.String("container_id", resp.ID[:12]))

	return instance, nil
}

// waitForReady waits for browser to be ready
func (m *BrowserPoolManager) waitForReady(instance *BrowserInstance) error {
	// nkk: Simple retry with timeout
	for i := 0; i < 30; i++ {
		resp, err := http.Get(instance.WebDriverURL + "/status")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timeout waiting for browser")
}

// isHealthy checks if container is healthy
func (m *BrowserPoolManager) isHealthy(instance *BrowserInstance) bool {
	// nkk: Simple health check - is container running?
	inspect, err := m.docker.ContainerInspect(context.Background(), instance.ContainerID)
	if err != nil || !inspect.State.Running {
		return false
	}
	return true
}

// destroyContainer removes a container
func (m *BrowserPoolManager) destroyContainer(containerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stopOptions := container.StopOptions{}
	m.docker.ContainerStop(ctx, containerID, stopOptions)
	m.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	logger.Debug("Destroyed container", zap.String("container_id", containerID[:12]))
}

// Shutdown cleans up all containers
func (m *BrowserPoolManager) Shutdown() {
	logger.Info("Shutting down BrowserPoolManager")

	// Signal shutdown
	close(m.shutdownCh)
	m.cancel()

	// Stop accepting new requests
	close(m.pool)

	// Only clean up if Docker is available
	if m.dockerAvailable {
		// Destroy pooled containers
		for instance := range m.pool {
			m.destroyContainer(instance.ContainerID)
		}

		// Destroy in-use containers
		m.inUse.Range(func(key, value interface{}) bool {
			instance := value.(*BrowserInstance)
			m.destroyContainer(instance.ContainerID)
			return true
		})
	}

	// Wait for all goroutines
	m.wg.Wait()

	// Close Docker client if available
	if m.docker != nil {
		m.docker.Close()
	}

	logger.Info("BrowserPoolManager shutdown complete")
}

// cleanupStaleInstances removes instances idle for too long
func (m *BrowserPoolManager) cleanupStaleInstances() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.inUse.Range(func(key, value interface{}) bool {
				instance := value.(*BrowserInstance)
				// nkk: Clean up instances idle for > 5 minutes
				if time.Since(instance.LastUsed) > 5*time.Minute && !instance.InUse {
					logger.Info("Cleaning stale instance",
						zap.String("id", instance.ID),
						zap.Duration("idle", time.Since(instance.LastUsed)))
					m.inUse.Delete(key)
					go m.destroyContainer(instance.ContainerID)
				}
				return true
			})
		case <-m.shutdownCh:
			return
		}
	}
}

// GetPoolStats returns statistics about the pool
func (m *BrowserPoolManager) GetPoolStats() map[string]map[string]interface{} {
	stats := make(map[string]map[string]interface{})

	// Count available browsers
	available := len(m.pool)

	// Count in-use browsers
	inUse := 0
	m.inUse.Range(func(_, _ interface{}) bool {
		inUse++
		return true
	})

	stats["chrome"] = map[string]interface{}{
		"available": available,
		"in_use":    inUse,
		"total":     m.maxSize,
	}

	return stats
}

// CreatePool creates a new browser pool
func (m *BrowserPoolManager) CreatePool(browser, version string, size int) error {
	// nkk: Create pool for specific browser type
	for i := 0; i < size; i++ {
		instance, err := m.createBrowserContainer(browser, version)
		if err != nil {
			logger.Error("Failed to create browser in pool",
				zap.String("browser", browser),
				zap.Error(err))
			continue
		}

		select {
		case m.pool <- instance:
			// Added to pool
		default:
			// Pool full
			m.destroyContainer(instance.ContainerID)
			break
		}
	}

	return nil
}
