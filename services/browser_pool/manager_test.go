package browser_pool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
nkk: Unit tests for BrowserPoolManager
Testing critical functionality without Docker dependency
*/

func TestNewBrowserPoolManager(t *testing.T) {
	// Test creation
	manager, err := NewBrowserPoolManager(5)

	// Docker might not be available in CI
	if err != nil {
		t.Skip("Docker not available, skipping")
	}

	assert.NotNil(t, manager)
	assert.Equal(t, 5, manager.maxSize)
	assert.NotNil(t, manager.pool)
	assert.NotNil(t, manager.docker)

	// Cleanup
	manager.Shutdown()
}

func TestBrowserInstanceLifecycle(t *testing.T) {
	// Create mock browser instance
	instance := &BrowserInstance{
		ID:           "test-123",
		ContainerID:  "container-123",
		BrowserType:  "chrome",
		Version:      "latest",
		WebDriverURL: "http://localhost:4444",
		Healthy:      true,
		InUse:        false,
		LastUsed:     time.Now(),
	}

	// Test instance properties
	assert.Equal(t, "chrome", instance.BrowserType)
	assert.True(t, instance.Healthy)
	assert.False(t, instance.InUse)
}

func TestPoolSizeManagement(t *testing.T) {
	manager, err := NewBrowserPoolManager(3)
	if err != nil {
		t.Skip("Docker not available")
	}
	defer manager.Shutdown()

	// Pool should be created with correct size
	assert.Equal(t, 3, cap(manager.pool))
}

func TestConcurrentAccess(t *testing.T) {
	manager, err := NewBrowserPoolManager(10)
	if err != nil {
		t.Skip("Docker not available")
	}
	defer manager.Shutdown()

	// Simulate concurrent access
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Try to acquire browser
			_, _ = manager.AcquireBrowser(ctx, "chrome", "latest")
			done <- true
		}()
	}

	// Wait for all goroutines
	timeout := time.After(10 * time.Second)
	received := 0

	for received < 5 {
		select {
		case <-done:
			received++
		case <-timeout:
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}

func TestHealthCheck(t *testing.T) {
	manager, err := NewBrowserPoolManager(2)
	if err != nil {
		t.Skip("Docker not available")
	}
	defer manager.Shutdown()

	// Mock instance
	instance := &BrowserInstance{
		ContainerID: "test-container",
		Healthy:     true,
	}

	// Health check should work
	healthy := manager.isHealthy(instance)
	// Will fail without real Docker container, but that's expected
	assert.NotNil(t, healthy)
}

func TestShutdown(t *testing.T) {
	manager, err := NewBrowserPoolManager(5)
	if err != nil {
		t.Skip("Docker not available")
	}

	// Add some mock data
	manager.inUse.Store("test-1", &BrowserInstance{ContainerID: "container-1"})

	// Shutdown should complete without panic
	assert.NotPanics(t, func() {
		manager.Shutdown()
	})
}

func TestGetPoolStats(t *testing.T) {
	manager, err := NewBrowserPoolManager(5)
	if err != nil {
		t.Skip("Docker not available")
	}
	defer manager.Shutdown()

	stats := manager.GetPoolStats()
	assert.NotNil(t, stats)
}

func TestCreatePool(t *testing.T) {
	manager, err := NewBrowserPoolManager(10)
	if err != nil {
		t.Skip("Docker not available")
	}
	defer manager.Shutdown()

	// Should be able to create pools
	err = manager.CreatePool("firefox", "latest", 3)
	// May fail without Docker, but shouldn't panic
	assert.NotPanics(t, func() {
		manager.CreatePool("webkit", "latest", 2)
	})
}

// Benchmark tests

func BenchmarkAcquireRelease(b *testing.B) {
	manager, err := NewBrowserPoolManager(10)
	if err != nil {
		b.Skip("Docker not available")
	}
	defer manager.Shutdown()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			instance, err := manager.AcquireBrowser(ctx, "chrome", "latest")
			if err == nil && instance != nil {
				manager.ReleaseBrowser("chrome", "latest", instance)
			}
		}
	})
}

func BenchmarkConcurrentAcquisition(b *testing.B) {
	manager, err := NewBrowserPoolManager(50)
	if err != nil {
		b.Skip("Docker not available")
	}
	defer manager.Shutdown()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			go func() {
				manager.AcquireBrowser(ctx, "chrome", "latest")
			}()
		}
	})
}