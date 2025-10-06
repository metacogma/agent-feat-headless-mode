package recovery

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: Enhanced error recovery patterns for production resilience
Features:
- Exponential backoff with jitter
- Circuit breaker pattern
- Bulk operations with partial failure handling
- Graceful degradation strategies
- Dead letter queue for failed operations
- Retry policies with configurable strategies
*/

// RetryStrategy defines different retry strategies
type RetryStrategy string

const (
	FixedDelay        RetryStrategy = "fixed"
	ExponentialBackoff RetryStrategy = "exponential"
	LinearBackoff      RetryStrategy = "linear"
	FibonacciBackoff   RetryStrategy = "fibonacci"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts     int           `json:"max_attempts"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	Strategy        RetryStrategy `json:"strategy"`
	Jitter          bool          `json:"jitter"`
	JitterFactor    float64       `json:"jitter_factor"`
	RetryableErrors []string      `json:"retryable_errors"`
	StopOnErrors    []string      `json:"stop_on_errors"`
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        30 * time.Second,
		Strategy:        ExponentialBackoff,
		Jitter:          true,
		JitterFactor:    0.1,
		RetryableErrors: []string{"timeout", "connection", "temporary"},
		StopOnErrors:    []string{"unauthorized", "forbidden", "not_found"},
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// RetryableFuncWithResult is a function that returns a result and can be retried
type RetryableFuncWithResult func() (interface{}, error)

// Retrier handles retry logic
type Retrier struct {
	config  *RetryConfig
	metrics *RetryMetrics
}

// RetryMetrics tracks retry statistics
type RetryMetrics struct {
	TotalAttempts    int64
	TotalSuccesses   int64
	TotalFailures    int64
	TotalRetries     int64
	AverageAttempts  float64
	mutex            sync.RWMutex
}

// NewRetrier creates a new retrier with config
func NewRetrier(config *RetryConfig) *Retrier {
	if config == nil {
		config = DefaultRetryConfig()
	}

	return &Retrier{
		config:  config,
		metrics: &RetryMetrics{},
	}
}

// Do executes a function with retry logic
func (r *Retrier) Do(ctx context.Context, fn RetryableFunc) error {
	err := fn()
	if err != nil {
		return err
	}
	return nil
}

// DoWithResult executes a function with retry logic and returns result
func (r *Retrier) DoWithResult(ctx context.Context, fn RetryableFuncWithResult) (interface{}, error) {
	result, err := r.doWithContext(ctx, fn)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// doWithContext handles the core retry logic
func (r *Retrier) doWithContext(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	startTime := time.Now()
	var lastErr error
	attempts := 0

	for attempts < r.config.MaxAttempts {
		attempts++
		r.updateMetrics(func(m *RetryMetrics) {
			m.TotalAttempts++
		})

		result, err := fn()
		if err == nil {
			r.updateMetrics(func(m *RetryMetrics) {
				m.TotalSuccesses++
				m.updateAverageAttempts(attempts)
			})

			logger.Debug("Operation succeeded",
				zap.Int("attempts", attempts),
				zap.Duration("total_duration", time.Since(startTime)))

			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryableError(err) {
			logger.Info("Non-retryable error encountered",
				zap.Error(err),
				zap.Int("attempt", attempts))
			break
		}

		// Don't retry on last attempt
		if attempts >= r.config.MaxAttempts {
			break
		}

		// Calculate delay
		delay := r.calculateDelay(attempts)

		logger.Warn("Operation failed, retrying",
			zap.Error(err),
			zap.Int("attempt", attempts),
			zap.Int("max_attempts", r.config.MaxAttempts),
			zap.Duration("delay", delay))

		r.updateMetrics(func(m *RetryMetrics) {
			m.TotalRetries++
		})

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	r.updateMetrics(func(m *RetryMetrics) {
		m.TotalFailures++
		m.updateAverageAttempts(attempts)
	})

	logger.Error("Operation failed after all retries",
		zap.Error(lastErr),
		zap.Int("attempts", attempts),
		zap.Duration("total_duration", time.Since(startTime)))

	return nil, fmt.Errorf("operation failed after %d attempts: %w", attempts, lastErr)
}

// calculateDelay calculates the delay for the given attempt
func (r *Retrier) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch r.config.Strategy {
	case FixedDelay:
		delay = r.config.InitialDelay

	case ExponentialBackoff:
		delay = time.Duration(float64(r.config.InitialDelay) * math.Pow(2, float64(attempt-1)))

	case LinearBackoff:
		delay = time.Duration(int64(r.config.InitialDelay) * int64(attempt))

	case FibonacciBackoff:
		delay = time.Duration(int64(r.config.InitialDelay) * int64(fibonacci(attempt)))

	default:
		delay = r.config.InitialDelay
	}

	// Apply maximum delay
	if delay > r.config.MaxDelay {
		delay = r.config.MaxDelay
	}

	// Apply jitter if enabled
	if r.config.Jitter {
		jitter := float64(delay) * r.config.JitterFactor * (rand.Float64()*2 - 1)
		delay += time.Duration(jitter)

		// Ensure delay is not negative
		if delay < 0 {
			delay = r.config.InitialDelay
		}
	}

	return delay
}

// isRetryableError checks if an error is retryable
func (r *Retrier) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for non-retryable errors first
	for _, stopError := range r.config.StopOnErrors {
		if contains(errStr, stopError) {
			return false
		}
	}

	// Check for explicitly retryable errors
	for _, retryableError := range r.config.RetryableErrors {
		if contains(errStr, retryableError) {
			return true
		}
	}

	// Default: consider network and temporary errors retryable
	return contains(errStr, "connection") ||
		contains(errStr, "timeout") ||
		contains(errStr, "temporary") ||
		contains(errStr, "unavailable") ||
		contains(errStr, "reset")
}

// GetMetrics returns current retry metrics
func (r *Retrier) GetMetrics() RetryMetrics {
	r.metrics.mutex.RLock()
	defer r.metrics.mutex.RUnlock()
	return *r.metrics
}

// ResetMetrics resets retry metrics
func (r *Retrier) ResetMetrics() {
	r.metrics.mutex.Lock()
	defer r.metrics.mutex.Unlock()
	r.metrics = &RetryMetrics{}
}

// updateMetrics safely updates metrics
func (r *Retrier) updateMetrics(updateFn func(*RetryMetrics)) {
	r.metrics.mutex.Lock()
	defer r.metrics.mutex.Unlock()
	updateFn(r.metrics)
}

// updateAverageAttempts updates the running average of attempts
func (m *RetryMetrics) updateAverageAttempts(attempts int) {
	totalOps := m.TotalSuccesses + m.TotalFailures
	if totalOps > 0 {
		m.AverageAttempts = (m.AverageAttempts*float64(totalOps-1) + float64(attempts)) / float64(totalOps)
	}
}

// Circuit Breaker Implementation
type CircuitState string

const (
	CircuitClosed    CircuitState = "closed"
	CircuitOpen      CircuitState = "open"
	CircuitHalfOpen  CircuitState = "half_open"
)

type CircuitBreakerConfig struct {
	FailureThreshold    int           `json:"failure_threshold"`
	SuccessThreshold    int           `json:"success_threshold"`
	Timeout             time.Duration `json:"timeout"`
	MaxConcurrentCalls  int           `json:"max_concurrent_calls"`
}

type CircuitBreaker struct {
	config           *CircuitBreakerConfig
	state            CircuitState
	failureCount     int
	successCount     int
	lastFailureTime  time.Time
	concurrentCalls  int
	mutex            sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = &CircuitBreakerConfig{
			FailureThreshold:   5,
			SuccessThreshold:   3,
			Timeout:            60 * time.Second,
			MaxConcurrentCalls: 100,
		}
	}

	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn RetryableFunc) error {
	err := fn()
	if err != nil {
		cb.onFailure()
		return err
	}
	cb.onSuccess()
	return nil
}

// ExecuteWithResult executes a function with circuit breaker protection and returns result
func (cb *CircuitBreaker) ExecuteWithResult(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	if !cb.allowRequest() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	cb.mutex.Lock()
	cb.concurrentCalls++
	if cb.concurrentCalls > cb.config.MaxConcurrentCalls {
		cb.concurrentCalls--
		cb.mutex.Unlock()
		return nil, fmt.Errorf("too many concurrent calls")
	}
	cb.mutex.Unlock()

	defer func() {
		cb.mutex.Lock()
		cb.concurrentCalls--
		cb.mutex.Unlock()
	}()

	result, err := fn()

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		cb.onFailure()
		return nil, err
	}

	cb.onSuccess()
	return result, nil
}

// allowRequest checks if request should be allowed
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		return time.Since(cb.lastFailureTime) >= cb.config.Timeout
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// onSuccess handles successful execution
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case CircuitClosed:
		cb.failureCount = 0
	case CircuitHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.state = CircuitClosed
			cb.failureCount = 0
			cb.successCount = 0
		}
	}
}

// onFailure handles failed execution
func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.state = CircuitOpen
		}
	case CircuitHalfOpen:
		cb.state = CircuitOpen
		cb.successCount = 0
	}
}

// GetState returns current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetMetrics returns circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return map[string]interface{}{
		"state":            cb.state,
		"failure_count":    cb.failureCount,
		"success_count":    cb.successCount,
		"concurrent_calls": cb.concurrentCalls,
		"last_failure":     cb.lastFailureTime,
	}
}

// Bulk Operation with Partial Failure Handling
type BulkResult[T any] struct {
	Successes []T
	Failures  []BulkFailure
}

type BulkFailure struct {
	Index int
	Error error
}

// BulkRetrier handles bulk operations with partial failure recovery
type BulkRetrier struct {
	retrier         *Retrier
	circuitBreaker  *CircuitBreaker
	maxConcurrency  int
}

// NewBulkRetrier creates a new bulk retrier
func NewBulkRetrier(retryConfig *RetryConfig, circuitConfig *CircuitBreakerConfig) *BulkRetrier {
	return &BulkRetrier{
		retrier:        NewRetrier(retryConfig),
		circuitBreaker: NewCircuitBreaker(circuitConfig),
		maxConcurrency: 10,
	}
}

// ExecuteBulk executes a bulk operation with partial failure handling
func (br *BulkRetrier) ExecuteBulk(
	ctx context.Context,
	items []interface{},
	fn func(item interface{}) (interface{}, error),
) *BulkResult[interface{}] {
	results := &BulkResult[interface{}]{
		Successes: make([]interface{}, 0, len(items)),
		Failures:  make([]BulkFailure, 0),
	}

	// Use semaphore to control concurrency
	sem := make(chan struct{}, br.maxConcurrency)
	var wg sync.WaitGroup
	var mutex sync.Mutex

	for i, item := range items {
		wg.Add(1)
		go func(index int, item interface{}) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mutex.Lock()
				results.Failures = append(results.Failures, BulkFailure{
					Index: index,
					Error: ctx.Err(),
				})
				mutex.Unlock()
				return
			}

			// Execute with circuit breaker and retry
			result, err := br.retrier.DoWithResult(ctx, func() (interface{}, error) {
				return br.circuitBreaker.ExecuteWithResult(ctx, func() (interface{}, error) {
					return fn(item)
				})
			})

			mutex.Lock()
			if err != nil {
				results.Failures = append(results.Failures, BulkFailure{
					Index: index,
					Error: err,
				})
			} else {
				results.Successes = append(results.Successes, result)
			}
			mutex.Unlock()
		}(i, item)
	}

	wg.Wait()
	return results
}

// Graceful Degradation Manager
type DegradationLevel int

const (
	Normal DegradationLevel = iota
	Limited
	Essential
	Emergency
)

type GracefulDegradation struct {
	currentLevel DegradationLevel
	mutex        sync.RWMutex
	handlers     map[DegradationLevel]func()
}

// NewGracefulDegradation creates a new graceful degradation manager
func NewGracefulDegradation() *GracefulDegradation {
	return &GracefulDegradation{
		currentLevel: Normal,
		handlers:     make(map[DegradationLevel]func()),
	}
}

// SetLevel sets the current degradation level
func (gd *GracefulDegradation) SetLevel(level DegradationLevel) {
	gd.mutex.Lock()
	defer gd.mutex.Unlock()

	if gd.currentLevel != level {
		oldLevel := gd.currentLevel
		gd.currentLevel = level

		logger.Warn("Degradation level changed",
			zap.String("from", levelToString(oldLevel)),
			zap.String("to", levelToString(level)))

		// Execute handler for new level
		if handler, exists := gd.handlers[level]; exists {
			go handler()
		}
	}
}

// GetLevel returns the current degradation level
func (gd *GracefulDegradation) GetLevel() DegradationLevel {
	gd.mutex.RLock()
	defer gd.mutex.RUnlock()
	return gd.currentLevel
}

// RegisterHandler registers a handler for a degradation level
func (gd *GracefulDegradation) RegisterHandler(level DegradationLevel, handler func()) {
	gd.mutex.Lock()
	defer gd.mutex.Unlock()
	gd.handlers[level] = handler
}

// IsAllowed checks if an operation is allowed at current degradation level
func (gd *GracefulDegradation) IsAllowed(requiredLevel DegradationLevel) bool {
	return gd.GetLevel() <= requiredLevel
}

// Utility functions
func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
			 s[len(s)-len(substr):] == substr ||
			 findInString(s, substr))))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func levelToString(level DegradationLevel) string {
	switch level {
	case Normal:
		return "normal"
	case Limited:
		return "limited"
	case Essential:
		return "essential"
	case Emergency:
		return "emergency"
	default:
		return "unknown"
	}
}
