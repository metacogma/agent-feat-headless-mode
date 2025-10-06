package tunnel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

/*
nkk: Comprehensive unit tests for TunnelService
Tests cover:
- Basic tunnel lifecycle (create, get, close)
- WebSocket connection handling
- HTTP proxy functionality
- Cleanup and resource management
- Error handling and edge cases
- Concurrent access safety
*/

func TestTunnelService_CreateTunnel(t *testing.T) {
	service := NewTunnelService()

	tunnel, err := service.CreateTunnel("user123", "localhost:3000")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}

	if tunnel.ID == "" {
		t.Error("Tunnel ID should not be empty")
	}

	if tunnel.Subdomain == "" {
		t.Error("Tunnel subdomain should not be empty")
	}

	if tunnel.UserID != "user123" {
		t.Errorf("Expected UserID to be 'user123', got '%s'", tunnel.UserID)
	}

	if tunnel.LocalAddr != "localhost:3000" {
		t.Errorf("Expected LocalAddr to be 'localhost:3000', got '%s'", tunnel.LocalAddr)
	}

	if !tunnel.Active {
		t.Error("New tunnel should be active")
	}

	if tunnel.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestTunnelService_GetTunnel(t *testing.T) {
	service := NewTunnelService()

	// Create a tunnel first
	original, err := service.CreateTunnel("user123", "localhost:3000")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}

	// Retrieve the tunnel
	retrieved, err := service.GetTunnel(original.ID)
	if err != nil {
		t.Fatalf("Failed to get tunnel: %v", err)
	}

	if retrieved.ID != original.ID {
		t.Errorf("Retrieved tunnel ID mismatch: expected %s, got %s", original.ID, retrieved.ID)
	}

	// Test non-existent tunnel
	_, err = service.GetTunnel("non-existent")
	if err == nil {
		t.Error("Expected error when getting non-existent tunnel")
	}
}

func TestTunnelService_CloseTunnel(t *testing.T) {
	service := NewTunnelService()

	// Create a tunnel
	tunnel, err := service.CreateTunnel("user123", "localhost:3000")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}

	// Close the tunnel
	err = service.CloseTunnel(tunnel.ID)
	if err != nil {
		t.Fatalf("Failed to close tunnel: %v", err)
	}

	// Verify tunnel is removed
	_, err = service.GetTunnel(tunnel.ID)
	if err == nil {
		t.Error("Expected error when getting closed tunnel")
	}

	// Test closing non-existent tunnel
	err = service.CloseTunnel("non-existent")
	if err == nil {
		t.Error("Expected error when closing non-existent tunnel")
	}
}

func TestTunnelService_ConcurrentAccess(t *testing.T) {
	service := NewTunnelService()
	const numGoroutines = 100
	const numOperations = 10

	var wg sync.WaitGroup
	tunnelIDs := make([]string, 0, numGoroutines)
	var idsMutex sync.Mutex

	// Create tunnels concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				tunnel, err := service.CreateTunnel("user", "localhost:3000")
				if err != nil {
					t.Errorf("Failed to create tunnel: %v", err)
					return
				}

				idsMutex.Lock()
				tunnelIDs = append(tunnelIDs, tunnel.ID)
				idsMutex.Unlock()

				// Randomly get or close tunnels
				if j%2 == 0 {
					_, err := service.GetTunnel(tunnel.ID)
					if err != nil {
						t.Errorf("Failed to get tunnel: %v", err)
					}
				} else {
					err := service.CloseTunnel(tunnel.ID)
					if err != nil {
						t.Errorf("Failed to close tunnel: %v", err)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify no data races occurred
	if len(tunnelIDs) != numGoroutines*numOperations {
		t.Errorf("Expected %d tunnel IDs, got %d", numGoroutines*numOperations, len(tunnelIDs))
	}
}

func TestTunnelService_CleanupStaleTunnels(t *testing.T) {
	service := NewTunnelService()

	// Create a tunnel
	tunnel, err := service.CreateTunnel("user123", "localhost:3000")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}

	// Mark it as inactive and old
	tunnel.Active = false
	tunnel.LastUsed = time.Now().Add(-45 * time.Minute) // Older than 30 minutes

	// Store it back
	service.tunnels.Store(tunnel.ID, tunnel)

	// Run cleanup manually
	service.cleanupStaleTunnels()

	// Verify tunnel was cleaned up
	_, err = service.GetTunnel(tunnel.ID)
	if err == nil {
		t.Error("Expected stale tunnel to be cleaned up")
	}
}

func TestTunnelService_ProxyHTTPRequest(t *testing.T) {
	service := NewTunnelService()

	// Create a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer mockServer.Close()

	// Extract host from mock server URL
	serverAddr := strings.TrimPrefix(mockServer.URL, "http://")

	// Create HTTP client
	client := &http.Client{Timeout: 10 * time.Second}

	// Test valid request
	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "test-agent",
	}
	headersJSON := `{"Content-Type":"application/json","User-Agent":"test-agent"}`

	payload := []byte("GET|/test|" + headersJSON + "|")
	response := service.proxyHTTPRequest(client, serverAddr, payload)

	responseStr := string(response)
	if !strings.Contains(responseStr, "200") {
		t.Errorf("Expected response to contain status 200, got: %s", responseStr)
	}

	if !strings.Contains(responseStr, "success") {
		t.Errorf("Expected response to contain 'success', got: %s", responseStr)
	}
}

func TestTunnelService_ProxyHTTPRequest_ErrorCases(t *testing.T) {
	service := NewTunnelService()
	client := &http.Client{Timeout: 1 * time.Second}

	testCases := []struct {
		name        string
		payload     []byte
		localAddr   string
		expectError bool
	}{
		{
			name:        "Invalid request format",
			payload:     []byte("invalid"),
			localAddr:   "localhost:3000",
			expectError: true,
		},
		{
			name:        "Invalid headers JSON",
			payload:     []byte("GET|/test|invalid-json|"),
			localAddr:   "localhost:3000",
			expectError: true,
		},
		{
			name:        "Request body too large",
			payload:     append([]byte("POST|/test|{}|"), make([]byte, 11*1024*1024)...),
			localAddr:   "localhost:3000",
			expectError: true,
		},
		{
			name:        "Unreachable server",
			payload:     []byte("GET|/test|{}|"),
			localAddr:   "localhost:99999",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := service.proxyHTTPRequest(client, tc.localAddr, tc.payload)
			responseStr := string(response)

			if tc.expectError {
				// Check for error status codes (4xx, 5xx)
				if strings.Contains(responseStr, "200|") {
					t.Errorf("Expected error response, got success: %s", responseStr)
				}
			}
		})
	}
}

func TestTunnelService_WebSocketHandling(t *testing.T) {
	service := NewTunnelService()

	// Create a tunnel
	tunnel, err := service.CreateTunnel("user123", "localhost:3000")
	if err != nil {
		t.Fatalf("Failed to create tunnel: %v", err)
	}

	// Create a test HTTP server with WebSocket upgrade
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Tunnel-ID", tunnel.ID)
		service.HandleWebSocket(w, r)
	}))
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Create WebSocket connection
	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Set("X-Tunnel-ID", tunnel.ID)

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Test sending a message
	testMessage := "GET|/test|{}|"
	err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
	if err != nil {
		t.Fatalf("Failed to send WebSocket message: %v", err)
	}

	// Set read deadline to prevent test hanging
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read response
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read WebSocket message: %v", err)
	}

	if messageType != websocket.TextMessage {
		t.Errorf("Expected text message, got type: %v", messageType)
	}

	if len(message) == 0 {
		t.Error("Expected non-empty response message")
	}
}

func TestTunnelService_WebSocketHandling_InvalidTunnel(t *testing.T) {
	service := NewTunnelService()

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Tunnel-ID", "non-existent-tunnel")
		service.HandleWebSocket(w, r)
	}))
	defer server.Close()

	// Try to connect with invalid tunnel ID
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Set("X-Tunnel-ID", "non-existent-tunnel")

	_, resp, err := dialer.Dial(wsURL, header)
	if err == nil {
		t.Error("Expected WebSocket connection to fail with invalid tunnel ID")
	}

	if resp != nil && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 status code, got: %d", resp.StatusCode)
	}
}

func TestTunnelService_CloseAll(t *testing.T) {
	service := NewTunnelService()

	// Create multiple tunnels
	tunnelIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		tunnel, err := service.CreateTunnel("user", "localhost:3000")
		if err != nil {
			t.Fatalf("Failed to create tunnel %d: %v", i, err)
		}
		tunnelIDs[i] = tunnel.ID
	}

	// Verify tunnels exist
	for _, id := range tunnelIDs {
		_, err := service.GetTunnel(id)
		if err != nil {
			t.Errorf("Tunnel %s should exist: %v", id, err)
		}
	}

	// Close all tunnels
	service.CloseAll()

	// Verify all tunnels are closed
	for _, id := range tunnelIDs {
		_, err := service.GetTunnel(id)
		if err == nil {
			t.Errorf("Tunnel %s should be closed", id)
		}
	}
}

func TestGenerateID(t *testing.T) {
	// Test ID generation
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Error("Generated ID should not be empty")
	}

	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}

	if len(id1) != 32 { // 16 bytes * 2 for hex encoding
		t.Errorf("Expected ID length of 32, got %d", len(id1))
	}
}

func TestGenerateSubdomain(t *testing.T) {
	// Test subdomain generation
	subdomain1 := generateSubdomain()
	subdomain2 := generateSubdomain()

	if subdomain1 == "" {
		t.Error("Generated subdomain should not be empty")
	}

	if subdomain1 == subdomain2 {
		t.Error("Generated subdomains should be unique")
	}

	if len(subdomain1) != 16 { // 8 bytes * 2 for hex encoding
		t.Errorf("Expected subdomain length of 16, got %d", len(subdomain1))
	}
}

// Benchmark tests
func BenchmarkTunnelService_CreateTunnel(b *testing.B) {
	service := NewTunnelService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.CreateTunnel("user", "localhost:3000")
		if err != nil {
			b.Fatalf("Failed to create tunnel: %v", err)
		}
	}
}

func BenchmarkTunnelService_GetTunnel(b *testing.B) {
	service := NewTunnelService()

	// Create a tunnel to get
	tunnel, err := service.CreateTunnel("user", "localhost:3000")
	if err != nil {
		b.Fatalf("Failed to create tunnel: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GetTunnel(tunnel.ID)
		if err != nil {
			b.Fatalf("Failed to get tunnel: %v", err)
		}
	}
}

func BenchmarkTunnelService_ProxyHTTPRequest(b *testing.B) {
	service := NewTunnelService()

	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	serverAddr := strings.TrimPrefix(mockServer.URL, "http://")
	client := &http.Client{Timeout: 10 * time.Second}
	payload := []byte("GET|/test|{}|")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.proxyHTTPRequest(client, serverAddr, payload)
	}
}