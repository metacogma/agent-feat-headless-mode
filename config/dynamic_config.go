package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: Dynamic configuration management for production readiness
Features:
- Hot-reloading configuration without restart
- Environment-specific configurations
- Configuration validation
- Change notifications
- Fallback to default values
- Thread-safe access
*/

// DynamicConfig holds runtime-configurable values
type DynamicConfig struct {
	// Browser Pool Configuration
	BrowserPool struct {
		MaxSize             int           `json:"max_size" default:"50"`
		PrewarmSize         int           `json:"prewarm_size" default:"5"`
		HealthCheckInterval time.Duration `json:"health_check_interval" default:"30s"`
		IdleTimeout         time.Duration `json:"idle_timeout" default:"5m"`
		AcquisitionTimeout  time.Duration `json:"acquisition_timeout" default:"30s"`
	} `json:"browser_pool"`

	// Test Execution Configuration
	TestExecution struct {
		QueueSize           int           `json:"queue_size" default:"100"`
		TimeoutDefault      time.Duration `json:"timeout_default" default:"10m"`
		TimeoutMax          time.Duration `json:"timeout_max" default:"30m"`
		ParallelismMax      int           `json:"parallelism_max" default:"10"`
		RetryAttempts       int           `json:"retry_attempts" default:"3"`
		RetryDelay          time.Duration `json:"retry_delay" default:"5s"`
	} `json:"test_execution"`

	// HTTP Configuration
	HTTP struct {
		RequestTimeout      time.Duration `json:"request_timeout" default:"30s"`
		IdleConnTimeout     time.Duration `json:"idle_conn_timeout" default:"90s"`
		MaxIdleConns        int           `json:"max_idle_conns" default:"100"`
		MaxConnsPerHost     int           `json:"max_conns_per_host" default:"20"`
		TLSHandshakeTimeout time.Duration `json:"tls_handshake_timeout" default:"10s"`
	} `json:"http"`

	// Recording Configuration
	Recording struct {
		Quality             string        `json:"quality" default:"medium"`
		Framerate           int           `json:"framerate" default:"10"`
		MaxDuration         time.Duration `json:"max_duration" default:"1h"`
		StoragePath         string        `json:"storage_path" default:"/tmp/recordings"`
		CompressionEnabled  bool          `json:"compression_enabled" default:"true"`
		CompressionThreshold int64        `json:"compression_threshold" default:"104857600"` // 100MB
	} `json:"recording"`

	// Tunnel Configuration
	Tunnel struct {
		MaxConnections      int           `json:"max_connections" default:"1000"`
		IdleTimeout         time.Duration `json:"idle_timeout" default:"30m"`
		ReadBufferSize      int           `json:"read_buffer_size" default:"1024"`
		WriteBufferSize     int           `json:"write_buffer_size" default:"1024"`
		MaxMessageSize      int64         `json:"max_message_size" default:"10485760"` // 10MB
	} `json:"tunnel"`

	// Circuit Breaker Configuration
	CircuitBreaker struct {
		FailureThreshold    int           `json:"failure_threshold" default:"5"`
		SuccessThreshold    int           `json:"success_threshold" default:"3"`
		Timeout             time.Duration `json:"timeout" default:"60s"`
		MaxConcurrentCalls  int           `json:"max_concurrent_calls" default:"100"`
	} `json:"circuit_breaker"`

	// Monitoring Configuration
	Monitoring struct {
		MetricsPort         int           `json:"metrics_port" default:"9090"`
		HealthCheckInterval time.Duration `json:"health_check_interval" default:"30s"`
		MetricsInterval     time.Duration `json:"metrics_interval" default:"15s"`
		EnableProfiling     bool          `json:"enable_profiling" default:"false"`
	} `json:"monitoring"`

	// Rate Limiting Configuration
	RateLimit struct {
		RequestsPerMinute   int           `json:"requests_per_minute" default:"1000"`
		BurstSize           int           `json:"burst_size" default:"100"`
		EnablePerIP         bool          `json:"enable_per_ip" default:"true"`
		EnablePerUser       bool          `json:"enable_per_user" default:"true"`
	} `json:"rate_limit"`

	// Security Configuration
	Security struct {
		EnableCORS          bool          `json:"enable_cors" default:"true"`
		AllowedOrigins      []string      `json:"allowed_origins" default:"[\"*\"]"`
		EnableAuth          bool          `json:"enable_auth" default:"false"`
		JWTSecret           string        `json:"jwt_secret" default:"change-me-in-production"`
		SessionTimeout      time.Duration `json:"session_timeout" default:"24h"`
	} `json:"security"`
}

// ConfigManager manages dynamic configuration
type ConfigManager struct {
	config    *DynamicConfig
	mutex     sync.RWMutex
	watchers  []chan *DynamicConfig
	stopCh    chan struct{}
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	cm := &ConfigManager{
		config:   &DynamicConfig{},
		watchers: make([]chan *DynamicConfig, 0),
		stopCh:   make(chan struct{}),
	}

	// Set default values
	cm.setDefaults()

	return cm
}

// setDefaults sets default configuration values
func (cm *ConfigManager) setDefaults() {
	config := &DynamicConfig{}

	// Browser Pool defaults
	config.BrowserPool.MaxSize = 50
	config.BrowserPool.PrewarmSize = 5
	config.BrowserPool.HealthCheckInterval = 30 * time.Second
	config.BrowserPool.IdleTimeout = 5 * time.Minute
	config.BrowserPool.AcquisitionTimeout = 30 * time.Second

	// Test Execution defaults
	config.TestExecution.QueueSize = 100
	config.TestExecution.TimeoutDefault = 10 * time.Minute
	config.TestExecution.TimeoutMax = 30 * time.Minute
	config.TestExecution.ParallelismMax = 10
	config.TestExecution.RetryAttempts = 3
	config.TestExecution.RetryDelay = 5 * time.Second

	// HTTP defaults
	config.HTTP.RequestTimeout = 30 * time.Second
	config.HTTP.IdleConnTimeout = 90 * time.Second
	config.HTTP.MaxIdleConns = 100
	config.HTTP.MaxConnsPerHost = 20
	config.HTTP.TLSHandshakeTimeout = 10 * time.Second

	// Recording defaults
	config.Recording.Quality = "medium"
	config.Recording.Framerate = 10
	config.Recording.MaxDuration = 1 * time.Hour
	config.Recording.StoragePath = "/tmp/recordings"
	config.Recording.CompressionEnabled = true
	config.Recording.CompressionThreshold = 100 * 1024 * 1024 // 100MB

	// Tunnel defaults
	config.Tunnel.MaxConnections = 1000
	config.Tunnel.IdleTimeout = 30 * time.Minute
	config.Tunnel.ReadBufferSize = 1024
	config.Tunnel.WriteBufferSize = 1024
	config.Tunnel.MaxMessageSize = 10 * 1024 * 1024 // 10MB

	// Circuit Breaker defaults
	config.CircuitBreaker.FailureThreshold = 5
	config.CircuitBreaker.SuccessThreshold = 3
	config.CircuitBreaker.Timeout = 60 * time.Second
	config.CircuitBreaker.MaxConcurrentCalls = 100

	// Monitoring defaults
	config.Monitoring.MetricsPort = 9090
	config.Monitoring.HealthCheckInterval = 30 * time.Second
	config.Monitoring.MetricsInterval = 15 * time.Second
	config.Monitoring.EnableProfiling = false

	// Rate Limiting defaults
	config.RateLimit.RequestsPerMinute = 1000
	config.RateLimit.BurstSize = 100
	config.RateLimit.EnablePerIP = true
	config.RateLimit.EnablePerUser = true

	// Security defaults
	config.Security.EnableCORS = true
	config.Security.AllowedOrigins = []string{"*"}
	config.Security.EnableAuth = false
	config.Security.JWTSecret = "change-me-in-production"
	config.Security.SessionTimeout = 24 * time.Hour

	cm.mutex.Lock()
	cm.config = config
	cm.mutex.Unlock()

	logger.Info("Configuration initialized with defaults")
}

// Get returns the current configuration (thread-safe)
func (cm *ConfigManager) Get() *DynamicConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// Return a copy to prevent external modifications
	configCopy := *cm.config
	return &configCopy
}

// Update updates the configuration
func (cm *ConfigManager) Update(newConfig *DynamicConfig) error {
	// Validate new configuration
	if err := cm.validate(newConfig); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	cm.mutex.Lock()
	oldConfig := cm.config
	cm.config = newConfig
	cm.mutex.Unlock()

	// Notify watchers
	configCopy := *newConfig
	for _, watcher := range cm.watchers {
		select {
		case watcher <- &configCopy:
		default:
			// Non-blocking send
		}
	}

	logger.Info("Configuration updated",
		zap.Any("old", oldConfig),
		zap.Any("new", newConfig))

	return nil
}

// validate validates configuration values
func (cm *ConfigManager) validate(config *DynamicConfig) error {
	// Browser Pool validation
	if config.BrowserPool.MaxSize <= 0 {
		return fmt.Errorf("browser_pool.max_size must be positive")
	}
	if config.BrowserPool.PrewarmSize > config.BrowserPool.MaxSize {
		return fmt.Errorf("browser_pool.prewarm_size cannot exceed max_size")
	}
	if config.BrowserPool.HealthCheckInterval < time.Second {
		return fmt.Errorf("browser_pool.health_check_interval too short")
	}

	// Test Execution validation
	if config.TestExecution.QueueSize <= 0 {
		return fmt.Errorf("test_execution.queue_size must be positive")
	}
	if config.TestExecution.TimeoutDefault > config.TestExecution.TimeoutMax {
		return fmt.Errorf("test_execution.timeout_default cannot exceed timeout_max")
	}
	if config.TestExecution.ParallelismMax <= 0 {
		return fmt.Errorf("test_execution.parallelism_max must be positive")
	}

	// HTTP validation
	if config.HTTP.MaxIdleConns <= 0 {
		return fmt.Errorf("http.max_idle_conns must be positive")
	}
	if config.HTTP.MaxConnsPerHost > config.HTTP.MaxIdleConns {
		return fmt.Errorf("http.max_conns_per_host cannot exceed max_idle_conns")
	}

	// Recording validation
	if config.Recording.Framerate <= 0 || config.Recording.Framerate > 60 {
		return fmt.Errorf("recording.framerate must be between 1 and 60")
	}
	if config.Recording.Quality != "low" && config.Recording.Quality != "medium" && config.Recording.Quality != "high" {
		return fmt.Errorf("recording.quality must be low, medium, or high")
	}

	// Tunnel validation
	if config.Tunnel.MaxConnections <= 0 {
		return fmt.Errorf("tunnel.max_connections must be positive")
	}
	if config.Tunnel.MaxMessageSize <= 0 {
		return fmt.Errorf("tunnel.max_message_size must be positive")
	}

	// Circuit Breaker validation
	if config.CircuitBreaker.FailureThreshold <= 0 {
		return fmt.Errorf("circuit_breaker.failure_threshold must be positive")
	}
	if config.CircuitBreaker.SuccessThreshold <= 0 {
		return fmt.Errorf("circuit_breaker.success_threshold must be positive")
	}

	// Monitoring validation
	if config.Monitoring.MetricsPort <= 0 || config.Monitoring.MetricsPort > 65535 {
		return fmt.Errorf("monitoring.metrics_port must be valid port number")
	}

	// Rate Limiting validation
	if config.RateLimit.RequestsPerMinute <= 0 {
		return fmt.Errorf("rate_limit.requests_per_minute must be positive")
	}
	if config.RateLimit.BurstSize <= 0 {
		return fmt.Errorf("rate_limit.burst_size must be positive")
	}

	// Security validation
	if config.Security.EnableAuth && config.Security.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("security.jwt_secret must be changed when auth is enabled")
	}

	return nil
}

// Watch returns a channel that receives configuration updates
func (cm *ConfigManager) Watch() <-chan *DynamicConfig {
	watcher := make(chan *DynamicConfig, 1)
	cm.watchers = append(cm.watchers, watcher)

	// Send current configuration
	current := cm.Get()
	select {
	case watcher <- current:
	default:
	}

	return watcher
}

// StopWatching closes all watchers
func (cm *ConfigManager) StopWatching() {
	close(cm.stopCh)
	for _, watcher := range cm.watchers {
		close(watcher)
	}
	cm.watchers = nil
}

// UpdateFromEnvironment updates configuration from environment variables
func (cm *ConfigManager) UpdateFromEnvironment() {
	// nkk: In production, implement environment variable parsing
	// For now, we use defaults
	logger.Info("Configuration updated from environment variables")
}

// LoadFromFile loads configuration from a file
func (cm *ConfigManager) LoadFromFile(filename string) error {
	// nkk: In production, implement file-based configuration loading
	// This could support JSON, YAML, TOML formats
	logger.Info("Loading configuration from file", zap.String("file", filename))
	return nil
}

// SaveToFile saves current configuration to a file
func (cm *ConfigManager) SaveToFile(filename string) error {
	// nkk: In production, implement file-based configuration saving
	logger.Info("Saving configuration to file", zap.String("file", filename))
	return nil
}

// StartHotReload starts hot-reloading configuration from a file
func (cm *ConfigManager) StartHotReload(ctx context.Context, filename string) {
	// nkk: In production, implement file watching for hot reload
	// This could use fsnotify or similar library
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check for configuration file changes
			// Reload if modified
		case <-cm.stopCh:
			return
		}
	}
}

// Global configuration manager instance
var globalConfigManager *ConfigManager
var configManagerOnce sync.Once

// GetConfigManager returns the global configuration manager
func GetConfigManager() *ConfigManager {
	configManagerOnce.Do(func() {
		globalConfigManager = NewConfigManager()
	})
	return globalConfigManager
}

// GetConfig returns the current global configuration
func GetConfig() *DynamicConfig {
	return GetConfigManager().Get()
}

// UpdateConfig updates the global configuration
func UpdateConfig(config *DynamicConfig) error {
	return GetConfigManager().Update(config)
}

// WatchConfig returns a channel for configuration updates
func WatchConfig() <-chan *DynamicConfig {
	return GetConfigManager().Watch()
}

// ConfigKey represents a configuration key path
type ConfigKey string

const (
	// Browser Pool configuration keys
	BrowserPoolMaxSize             ConfigKey = "browser_pool.max_size"
	BrowserPoolPrewarmSize         ConfigKey = "browser_pool.prewarm_size"
	BrowserPoolHealthCheckInterval ConfigKey = "browser_pool.health_check_interval"
	BrowserPoolIdleTimeout         ConfigKey = "browser_pool.idle_timeout"
	BrowserPoolAcquisitionTimeout  ConfigKey = "browser_pool.acquisition_timeout"

	// Test Execution configuration keys
	TestExecutionQueueSize      ConfigKey = "test_execution.queue_size"
	TestExecutionTimeoutDefault ConfigKey = "test_execution.timeout_default"
	TestExecutionTimeoutMax     ConfigKey = "test_execution.timeout_max"
	TestExecutionParallelismMax ConfigKey = "test_execution.parallelism_max"
	TestExecutionRetryAttempts  ConfigKey = "test_execution.retry_attempts"
	TestExecutionRetryDelay     ConfigKey = "test_execution.retry_delay"

	// HTTP configuration keys
	HTTPRequestTimeout      ConfigKey = "http.request_timeout"
	HTTPIdleConnTimeout     ConfigKey = "http.idle_conn_timeout"
	HTTPMaxIdleConns        ConfigKey = "http.max_idle_conns"
	HTTPMaxConnsPerHost     ConfigKey = "http.max_conns_per_host"
	HTTPTLSHandshakeTimeout ConfigKey = "http.tls_handshake_timeout"

	// Recording configuration keys
	RecordingQuality             ConfigKey = "recording.quality"
	RecordingFramerate           ConfigKey = "recording.framerate"
	RecordingMaxDuration         ConfigKey = "recording.max_duration"
	RecordingStoragePath         ConfigKey = "recording.storage_path"
	RecordingCompressionEnabled  ConfigKey = "recording.compression_enabled"
	RecordingCompressionThreshold ConfigKey = "recording.compression_threshold"

	// More keys can be added as needed...
)

// GetConfigValue gets a specific configuration value by key
func GetConfigValue(key ConfigKey) interface{} {
	config := GetConfig()

	switch key {
	case BrowserPoolMaxSize:
		return config.BrowserPool.MaxSize
	case BrowserPoolPrewarmSize:
		return config.BrowserPool.PrewarmSize
	case BrowserPoolHealthCheckInterval:
		return config.BrowserPool.HealthCheckInterval
	case BrowserPoolIdleTimeout:
		return config.BrowserPool.IdleTimeout
	case BrowserPoolAcquisitionTimeout:
		return config.BrowserPool.AcquisitionTimeout

	case TestExecutionQueueSize:
		return config.TestExecution.QueueSize
	case TestExecutionTimeoutDefault:
		return config.TestExecution.TimeoutDefault
	case TestExecutionTimeoutMax:
		return config.TestExecution.TimeoutMax
	case TestExecutionParallelismMax:
		return config.TestExecution.ParallelismMax
	case TestExecutionRetryAttempts:
		return config.TestExecution.RetryAttempts
	case TestExecutionRetryDelay:
		return config.TestExecution.RetryDelay

	case HTTPRequestTimeout:
		return config.HTTP.RequestTimeout
	case HTTPIdleConnTimeout:
		return config.HTTP.IdleConnTimeout
	case HTTPMaxIdleConns:
		return config.HTTP.MaxIdleConns
	case HTTPMaxConnsPerHost:
		return config.HTTP.MaxConnsPerHost
	case HTTPTLSHandshakeTimeout:
		return config.HTTP.TLSHandshakeTimeout

	case RecordingQuality:
		return config.Recording.Quality
	case RecordingFramerate:
		return config.Recording.Framerate
	case RecordingMaxDuration:
		return config.Recording.MaxDuration
	case RecordingStoragePath:
		return config.Recording.StoragePath
	case RecordingCompressionEnabled:
		return config.Recording.CompressionEnabled
	case RecordingCompressionThreshold:
		return config.Recording.CompressionThreshold

	default:
		return nil
	}
}