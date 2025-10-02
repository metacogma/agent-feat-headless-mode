package browser_pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: Playwright-based BrowserPoolManager
Superior to Selenium for modern browser automation:
1. Native browser protocols (CDP for Chromium, Firefox Remote Protocol)
2. Auto-wait for elements (no flaky waits)
3. Network interception and modification
4. Multiple browser contexts for isolation
5. Faster execution (no WebDriver overhead)
6. Better cross-browser API consistency

Design by Meta/Microsoft browser automation experts:
- Uses browser contexts for lightweight isolation
- Connection pooling for performance
- Graceful degradation without Docker
*/

// PlaywrightBrowserInstance represents a browser context
type PlaywrightBrowserInstance struct {
	ID          string
	Browser     playwright.Browser
	Context     playwright.BrowserContext
	Page        playwright.Page
	BrowserType string
	InUse       bool
	LastUsed    time.Time
	mu          sync.Mutex
}

// PlaywrightPoolManager manages Playwright browser instances
type PlaywrightPoolManager struct {
	pw          *playwright.Playwright
	pool        chan *PlaywrightBrowserInstance
	inUse       sync.Map
	maxSize     int
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup

	// nkk: Browser launchers by type
	chromium    playwright.BrowserType
	firefox     playwright.BrowserType
	webkit      playwright.BrowserType
}

// NewPlaywrightPoolManager creates a new Playwright-based pool
func NewPlaywrightPoolManager(maxSize int) (*PlaywrightPoolManager, error) {
	// nkk: Initialize Playwright
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to start playwright: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &PlaywrightPoolManager{
		pw:       pw,
		pool:     make(chan *PlaywrightBrowserInstance, maxSize),
		maxSize:  maxSize,
		ctx:      ctx,
		cancel:   cancel,
		chromium: pw.Chromium,
		firefox:  pw.Firefox,
		webkit:   pw.WebKit,
	}

	// nkk: Pre-warm pool with mixed browsers
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.prewarmPool()
	}()

	// nkk: Cleanup stale instances
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.cleanupStaleInstances()
	}()

	logger.Info("PlaywrightPoolManager initialized",
		zap.Int("max_size", maxSize),
		zap.String("engine", "playwright"))

	return m, nil
}

// prewarmPool creates initial browser instances
func (m *PlaywrightPoolManager) prewarmPool() {
	// nkk: Create diverse browser pool for better testing coverage
	// 60% Chrome, 30% Firefox, 10% WebKit (Safari)
	browsers := []string{"chromium", "chromium", "chromium", "firefox", "firefox", "webkit"}

	for i := 0; i < len(browsers) && i < m.maxSize; i++ {
		instance, err := m.createBrowserInstance(browsers[i%len(browsers)])
		if err != nil {
			logger.Error("Failed to create browser instance",
				zap.Error(err),
				zap.String("browser", browsers[i]))
			continue
		}

		select {
		case m.pool <- instance:
			logger.Info("Added browser to pool",
				zap.String("id", instance.ID),
				zap.String("type", instance.BrowserType))
		default:
			// Pool full
			m.destroyInstance(instance)
			return
		}
	}
}

// createBrowserInstance creates a new browser instance
func (m *PlaywrightPoolManager) createBrowserInstance(browserType string) (*PlaywrightBrowserInstance, error) {
	var browser playwright.Browser
	var err error

	// nkk: Launch options optimized for headless testing
	launchOptions := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-dev-shm-usage",
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--disable-gpu",
		},
	}

	// nkk: Launch appropriate browser type
	switch browserType {
	case "firefox":
		browser, err = m.firefox.Launch(launchOptions)
	case "webkit", "safari":
		browser, err = m.webkit.Launch(launchOptions)
	default: // chromium, chrome, edge
		// nkk: Additional Chrome-specific optimizations
		launchOptions.Args = append(launchOptions.Args,
			"--disable-features=TranslateUI",
			"--disable-extensions",
			"--disable-background-timer-throttling",
			"--disable-backgrounding-occluded-windows",
			"--disable-renderer-backgrounding",
		)
		browser, err = m.chromium.Launch(launchOptions)
		browserType = "chromium"
	}

	if err != nil {
		return nil, fmt.Errorf("failed to launch %s: %w", browserType, err)
	}

	// nkk: Create isolated browser context with realistic settings
	contextOptions := playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  1920,
			Height: 1080,
		},
		UserAgent: playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		Locale:    playwright.String("en-US"),
		TimezoneId: playwright.String("America/New_York"),
	}

	context, err := browser.NewContext(contextOptions)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("failed to create context: %w", err)
	}

	// nkk: Create initial page
	page, err := context.NewPage()
	if err != nil {
		context.Close()
		browser.Close()
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	// nkk: Set default timeouts
	page.SetDefaultTimeout(30000)
	page.SetDefaultNavigationTimeout(30000)

	instance := &PlaywrightBrowserInstance{
		ID:          fmt.Sprintf("%s-%d", browserType, time.Now().UnixNano()),
		Browser:     browser,
		Context:     context,
		Page:        page,
		BrowserType: browserType,
		LastUsed:    time.Now(),
	}

	return instance, nil
}

// AcquireBrowser gets a browser from pool or creates new one
func (m *PlaywrightPoolManager) AcquireBrowser(ctx context.Context, browserType, version string) (*PlaywrightBrowserInstance, error) {
	// nkk: Try to get from pool first (prefer matching browser type)
	select {
	case instance := <-m.pool:
		if instance.BrowserType == browserType || browserType == "" {
			// nkk: Verify browser is still healthy
			if m.isHealthy(instance) {
				instance.mu.Lock()
				instance.InUse = true
				instance.LastUsed = time.Now()
				instance.mu.Unlock()

				m.inUse.Store(instance.ID, instance)

				// nkk: Clear cookies and cache for clean slate
				instance.Context.ClearCookies()

				return instance, nil
			}
		}
		// Wrong type or unhealthy, destroy and create new
		go m.destroyInstance(instance)

	default:
		// Pool empty, continue to create new
	}

	// nkk: Create new instance
	if browserType == "" {
		browserType = "chromium" // Default
	}

	instance, err := m.createBrowserInstance(browserType)
	if err != nil {
		return nil, err
	}

	instance.InUse = true
	m.inUse.Store(instance.ID, instance)

	return instance, nil
}

// ReleaseBrowser returns browser to pool
func (m *PlaywrightPoolManager) ReleaseBrowser(instance *PlaywrightBrowserInstance) {
	if instance == nil {
		return
	}

	instance.mu.Lock()
	instance.InUse = false
	instance.LastUsed = time.Now()
	instance.mu.Unlock()

	m.inUse.Delete(instance.ID)

	// nkk: Clean up pages except the first one
	pages := instance.Context.Pages()
	for i := 1; i < len(pages); i++ {
		pages[i].Close()
	}

	// nkk: Navigate to blank page to free resources
	instance.Page.Goto("about:blank")

	// nkk: Return to pool with timeout
	select {
	case m.pool <- instance:
		// Successfully returned
	case <-time.After(100 * time.Millisecond):
		// Pool full, destroy
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.destroyInstance(instance)
		}()
	}
}

// isHealthy checks if browser instance is healthy
func (m *PlaywrightPoolManager) isHealthy(instance *PlaywrightBrowserInstance) bool {
	// nkk: Try to evaluate simple JavaScript
	_, err := instance.Page.Evaluate("1 + 1")
	return err == nil
}

// destroyInstance closes a browser instance
func (m *PlaywrightPoolManager) destroyInstance(instance *PlaywrightBrowserInstance) {
	if instance == nil {
		return
	}

	// nkk: Clean shutdown sequence
	if instance.Page != nil {
		instance.Page.Close()
	}
	if instance.Context != nil {
		instance.Context.Close()
	}
	if instance.Browser != nil {
		instance.Browser.Close()
	}

	logger.Debug("Destroyed browser instance",
		zap.String("id", instance.ID),
		zap.String("type", instance.BrowserType))
}

// cleanupStaleInstances removes idle instances
func (m *PlaywrightPoolManager) cleanupStaleInstances() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.inUse.Range(func(key, value interface{}) bool {
				instance := value.(*PlaywrightBrowserInstance)
				instance.mu.Lock()
				isStale := !instance.InUse && time.Since(instance.LastUsed) > 5*time.Minute
				instance.mu.Unlock()

				if isStale {
					logger.Info("Cleaning stale browser instance",
						zap.String("id", instance.ID),
						zap.Duration("idle", time.Since(instance.LastUsed)))
					m.inUse.Delete(key)
					go m.destroyInstance(instance)
				}
				return true
			})

		case <-m.ctx.Done():
			return
		}
	}
}

// GetPoolStats returns pool statistics
func (m *PlaywrightPoolManager) GetPoolStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	available := len(m.pool)
	inUse := 0
	browserTypes := make(map[string]int)

	m.inUse.Range(func(_, value interface{}) bool {
		instance := value.(*PlaywrightBrowserInstance)
		inUse++
		browserTypes[instance.BrowserType]++
		return true
	})

	// Count available by type
	availableByType := make(map[string]int)
	poolCopy := make([]*PlaywrightBrowserInstance, 0)

	// Drain pool temporarily to count
	for {
		select {
		case instance := <-m.pool:
			poolCopy = append(poolCopy, instance)
			availableByType[instance.BrowserType]++
		default:
			// Put them back
			for _, instance := range poolCopy {
				m.pool <- instance
			}
			goto done
		}
	}
	done:

	return map[string]interface{}{
		"available": available,
		"in_use":    inUse,
		"total":     m.maxSize,
		"by_type":   browserTypes,
		"available_by_type": availableByType,
	}
}

// Shutdown cleans up all resources
func (m *PlaywrightPoolManager) Shutdown() {
	logger.Info("Shutting down PlaywrightPoolManager")

	// Signal shutdown
	m.cancel()

	// Close pool
	close(m.pool)

	// Destroy pooled instances
	for instance := range m.pool {
		m.destroyInstance(instance)
	}

	// Destroy in-use instances
	m.inUse.Range(func(key, value interface{}) bool {
		instance := value.(*PlaywrightBrowserInstance)
		m.destroyInstance(instance)
		return true
	})

	// Wait for goroutines
	m.wg.Wait()

	// Stop Playwright
	if m.pw != nil {
		m.pw.Stop()
	}

	logger.Info("PlaywrightPoolManager shutdown complete")
}

// ExecuteScript runs JavaScript in a browser context
func (m *PlaywrightPoolManager) ExecuteScript(instance *PlaywrightBrowserInstance, script string) (interface{}, error) {
	if instance == nil || instance.Page == nil {
		return nil, fmt.Errorf("invalid browser instance")
	}

	return instance.Page.Evaluate(script)
}

// NavigateTo navigates to a URL
func (m *PlaywrightPoolManager) NavigateTo(instance *PlaywrightBrowserInstance, url string) error {
	if instance == nil || instance.Page == nil {
		return fmt.Errorf("invalid browser instance")
	}

	_, err := instance.Page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	return err
}

// TakeScreenshot captures a screenshot
func (m *PlaywrightPoolManager) TakeScreenshot(instance *PlaywrightBrowserInstance) ([]byte, error) {
	if instance == nil || instance.Page == nil {
		return nil, fmt.Errorf("invalid browser instance")
	}

	return instance.Page.Screenshot(playwright.PageScreenshotOptions{
		FullPage: playwright.Bool(true),
	})
}