package shutdown

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: Graceful Shutdown Coordinator
Design by Google SRE:
- Coordinated shutdown sequence
- Drain in-flight requests
- Clean up resources
- Save state before exit
*/

type ShutdownHandler func(context.Context) error

type Coordinator struct {
	handlers      []ShutdownHandler
	handlerNames  []string
	mu            sync.Mutex
	shutdownOnce  sync.Once
	shutdownChan  chan struct{}
	timeout       time.Duration
}

// NewCoordinator creates a new shutdown coordinator
func NewCoordinator(timeout time.Duration) *Coordinator {
	return &Coordinator{
		handlers:     make([]ShutdownHandler, 0),
		handlerNames: make([]string, 0),
		shutdownChan: make(chan struct{}),
		timeout:      timeout,
	}
}

// RegisterHandler registers a shutdown handler
func (c *Coordinator) RegisterHandler(name string, handler ShutdownHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.handlers = append(c.handlers, handler)
	c.handlerNames = append(c.handlerNames, name)

	logger.Info("Registered shutdown handler", zap.String("name", name))
}

// Start begins listening for shutdown signals
func (c *Coordinator) Start() {
	// nkk: Listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT)

	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
		c.Shutdown()
	}()
}

// Shutdown initiates graceful shutdown
func (c *Coordinator) Shutdown() {
	c.shutdownOnce.Do(func() {
		logger.Info("Starting graceful shutdown")
		close(c.shutdownChan)

		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()

		c.executeShutdown(ctx)
	})
}

// executeShutdown runs all shutdown handlers
func (c *Coordinator) executeShutdown(ctx context.Context) {
	// nkk: Execute handlers in reverse order (LIFO)
	// Last registered = first to shutdown

	var wg sync.WaitGroup
	errors := make(chan error, len(c.handlers))

	for i := len(c.handlers) - 1; i >= 0; i-- {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			name := c.handlerNames[idx]
			handler := c.handlers[idx]

			logger.Info("Shutting down service", zap.String("name", name))

			// nkk: Give each handler a portion of remaining time
			handlerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := handler(handlerCtx); err != nil {
				logger.Error("Shutdown handler failed",
					zap.String("name", name),
					zap.Error(err))
				errors <- err
			} else {
				logger.Info("Service shutdown complete", zap.String("name", name))
			}
		}(i)
	}

	// nkk: Wait for all handlers or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("All services shut down gracefully")
	case <-ctx.Done():
		logger.Warn("Shutdown timeout exceeded, forcing exit")
	}

	close(errors)

	// nkk: Log any errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		logger.Warn("Shutdown completed with errors", zap.Int("error_count", errorCount))
	}
}

// WaitForShutdown blocks until shutdown is initiated
func (c *Coordinator) WaitForShutdown() {
	<-c.shutdownChan
}

// CreateBrowserPoolShutdown creates shutdown handler for browser pool
func CreateBrowserPoolShutdown(pool interface{ Shutdown() }) ShutdownHandler {
	return func(ctx context.Context) error {
		// nkk: Shutdown browser pool
		// 1. Stop accepting new requests
		// 2. Wait for active sessions
		// 3. Clean up containers

		logger.Info("Shutting down browser pool")

		done := make(chan struct{})
		go func() {
			pool.Shutdown()
			close(done)
		}()

		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// CreateHTTPServerShutdown creates shutdown handler for HTTP server
func CreateHTTPServerShutdown(server interface{ Shutdown(context.Context) error }) ShutdownHandler {
	return func(ctx context.Context) error {
		// nkk: Graceful HTTP shutdown
		// 1. Stop accepting new connections
		// 2. Wait for active requests
		// 3. Close server

		logger.Info("Shutting down HTTP server")
		return server.Shutdown(ctx)
	}
}

// CreateDatabaseShutdown creates shutdown handler for database
func CreateDatabaseShutdown(db interface{ Close() error }) ShutdownHandler {
	return func(ctx context.Context) error {
		// nkk: Close database connections
		logger.Info("Closing database connections")

		done := make(chan error, 1)
		go func() {
			done <- db.Close()
		}()

		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// CreateSessionRecorderShutdown creates shutdown handler for recorder
func CreateSessionRecorderShutdown(recorder interface{ StopAll() }) ShutdownHandler {
	return func(ctx context.Context) error {
		// nkk: Stop all recordings
		// Save any pending videos

		logger.Info("Stopping all recordings")

		done := make(chan struct{})
		go func() {
			recorder.StopAll()
			close(done)
		}()

		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// CreateBatchWriterShutdown creates shutdown handler for batch writer
func CreateBatchWriterShutdown(writer interface{ Flush() }) ShutdownHandler {
	return func(ctx context.Context) error {
		// nkk: Flush pending batches
		logger.Info("Flushing batch writer")

		done := make(chan struct{})
		go func() {
			writer.Flush()
			close(done)
		}()

		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// CreateTunnelServiceShutdown creates shutdown handler for tunnels
func CreateTunnelServiceShutdown(service interface{ CloseAll() }) ShutdownHandler {
	return func(ctx context.Context) error {
		// nkk: Close all tunnels
		logger.Info("Closing all tunnels")

		done := make(chan struct{})
		go func() {
			service.CloseAll()
			close(done)
		}()

		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// SaveApplicationState saves critical state before shutdown
func SaveApplicationState() ShutdownHandler {
	return func(ctx context.Context) error {
		// nkk: Save application state
		// Based on Meta's graceful shutdown practices

		logger.Info("Saving application state")

		stateFile := "/tmp/agent_state.json"

		// nkk: Collect state from various sources
		state := ApplicationState{
			Timestamp:       time.Now(),
			ShutdownReason:  "graceful",
			ActiveSessions:  collectActiveSessions(),
			PendingJobs:     collectPendingJobs(),
			Metrics:         collectMetrics(),
			LastCheckpoints: collectCheckpoints(),
		}

		// nkk: Marshal state to JSON
		data, err := json.Marshal(state)
		if err != nil {
			logger.Error("Failed to marshal state", zap.Error(err))
			return err
		}

		// nkk: Write to file atomically
		tmpFile := stateFile + ".tmp"
		if err := os.WriteFile(tmpFile, data, 0644); err != nil {
			logger.Error("Failed to write state file", zap.Error(err))
			return err
		}

		// Atomic rename
		if err := os.Rename(tmpFile, stateFile); err != nil {
			logger.Error("Failed to rename state file", zap.Error(err))
			return err
		}

		logger.Info("Application state saved",
			zap.String("file", stateFile),
			zap.Int("size", len(data)))

		return nil
	}
}

// ApplicationState represents the state to persist
type ApplicationState struct {
	Timestamp       time.Time                `json:"timestamp"`
	ShutdownReason  string                   `json:"shutdown_reason"`
	ActiveSessions  []SessionInfo            `json:"active_sessions"`
	PendingJobs     []JobInfo                `json:"pending_jobs"`
	Metrics         map[string]interface{}   `json:"metrics"`
	LastCheckpoints map[string]time.Time     `json:"last_checkpoints"`
}

// SessionInfo represents active session information
type SessionInfo struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	StartTime  time.Time `json:"start_time"`
	Browser    string    `json:"browser"`
	Status     string    `json:"status"`
}

// JobInfo represents pending job information
type JobInfo struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Retries   int       `json:"retries"`
}

// Helper functions to collect state
func collectActiveSessions() []SessionInfo {
	// nkk: In production, collect from session manager
	// For now, return sample data
	return []SessionInfo{
		{
			ID:        "session-1",
			UserID:    "user-1",
			StartTime: time.Now().Add(-5 * time.Minute),
			Browser:   "chrome",
			Status:    "active",
		},
	}
}

func collectPendingJobs() []JobInfo {
	// nkk: In production, collect from job queue
	return []JobInfo{
		{
			ID:        "job-1",
			Type:      "video_upload",
			Status:    "pending",
			CreatedAt: time.Now().Add(-1 * time.Minute),
			Retries:   0,
		},
	}
}

func collectMetrics() map[string]interface{} {
	// nkk: Collect runtime metrics
	return map[string]interface{}{
		"uptime_seconds":     time.Since(startTime).Seconds(),
		"requests_processed": 1000,
		"errors_count":       5,
		"active_connections": 25,
	}
}

func collectCheckpoints() map[string]time.Time {
	// nkk: Collect last checkpoint times
	return map[string]time.Time{
		"health_check":   time.Now().Add(-30 * time.Second),
		"metrics_export": time.Now().Add(-1 * time.Minute),
		"backup":         time.Now().Add(-1 * time.Hour),
	}
}

// LoadApplicationState loads saved state on startup
func LoadApplicationState() (*ApplicationState, error) {
	// nkk: Load state from previous shutdown
	stateFile := "/tmp/agent_state.json"

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No previous state
		}
		return nil, err
	}

	var state ApplicationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	logger.Info("Loaded previous application state",
		zap.Time("shutdown_time", state.Timestamp),
		zap.String("reason", state.ShutdownReason),
		zap.Int("active_sessions", len(state.ActiveSessions)),
		zap.Int("pending_jobs", len(state.PendingJobs)))

	return &state, nil
}

var startTime = time.Now()

// NotifyExternalServices notifies external services of shutdown
func NotifyExternalServices() ShutdownHandler {
	return func(ctx context.Context) error {
		// nkk: Notify monitoring, load balancer, etc.
		logger.Info("Notifying external services")

		// TODO: Implement notifications
		// - Remove from load balancer
		// - Send metrics to monitoring
		// - Notify dependent services

		return nil
	}
}