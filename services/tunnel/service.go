package tunnel

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: Simplified TunnelService for localhost testing
Based on ngrok/localtunnel patterns
No over-engineering - add complexity only when needed
*/

type Tunnel struct {
	ID        string
	Subdomain string
	LocalAddr string
	UserID    string
	CreatedAt time.Time
	LastUsed  time.Time
	Active    bool
	conn      *websocket.Conn
}

type TunnelService struct {
	tunnels      sync.Map // map[string]*Tunnel
	upgrader     websocket.Upgrader
	cleanupTimer *time.Timer
	mu           sync.RWMutex
}

// NewTunnelService creates a new tunnel service
func NewTunnelService() *TunnelService {
	s := &TunnelService{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // nkk: Configure properly in production
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}

	// nkk: Start cleanup routine for stale tunnels
	go s.cleanupStaleTunnels()

	return s
}

// cleanupStaleTunnels removes inactive tunnels
func (s *TunnelService) cleanupStaleTunnels() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		var toDelete []string

		s.tunnels.Range(func(key, value interface{}) bool {
			tunnel := value.(*Tunnel)
			// nkk: Remove tunnels inactive for > 30 minutes
			if !tunnel.Active && time.Since(tunnel.LastUsed) > 30*time.Minute {
				toDelete = append(toDelete, tunnel.ID)
			}
			return true
		})

		for _, id := range toDelete {
			s.tunnels.Delete(id)
			logger.Debug("Cleaned up stale tunnel", zap.String("id", id))
		}
	}
}

// CreateTunnel creates a new tunnel
func (s *TunnelService) CreateTunnel(userID, localAddr string) (*Tunnel, error) {
	tunnel := &Tunnel{
		ID:        generateID(),
		Subdomain: generateSubdomain(),
		LocalAddr: localAddr,
		UserID:    userID,
		CreatedAt: time.Now(),
		Active:    true,
	}

	s.tunnels.Store(tunnel.ID, tunnel)

	logger.Info("Created tunnel",
		zap.String("id", tunnel.ID),
		zap.String("subdomain", tunnel.Subdomain))

	return tunnel, nil
}

// GetTunnel retrieves a tunnel
func (s *TunnelService) GetTunnel(tunnelID string) (*Tunnel, error) {
	if val, ok := s.tunnels.Load(tunnelID); ok {
		return val.(*Tunnel), nil
	}
	return nil, fmt.Errorf("tunnel not found")
}

// CloseTunnel closes a tunnel
func (s *TunnelService) CloseTunnel(tunnelID string) error {
	if val, ok := s.tunnels.Load(tunnelID); ok {
		tunnel := val.(*Tunnel)
		tunnel.Active = false
		if tunnel.conn != nil {
			tunnel.conn.Close()
		}
		s.tunnels.Delete(tunnelID)
		return nil
	}
	return fmt.Errorf("tunnel not found")
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateSubdomain() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// HandleWebSocket handles tunnel WebSocket connections
func (s *TunnelService) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// nkk: Simplified WebSocket handling
	// In production, implement full proxy logic

	tunnelID := r.Header.Get("X-Tunnel-ID")
	tunnel, err := s.GetTunnel(tunnelID)
	if err != nil {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade WebSocket", zap.Error(err))
		return
	}

	tunnel.conn = conn
	tunnel.LastUsed = time.Now()

	// nkk: TODO: Implement proxy logic
	// For now, just echo messages
	go s.handleConnection(tunnel)
}

func (s *TunnelService) handleConnection(tunnel *Tunnel) {
	defer tunnel.conn.Close()

	// nkk: Create HTTP client for proxying to local service
	// Based on ngrok's approach: WebSocket -> HTTP proxy
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	for {
		messageType, payload, err := tunnel.conn.ReadMessage()
		if err != nil {
			logger.Debug("Connection closed", zap.String("tunnel_id", tunnel.ID))
			return
		}

		// nkk: Parse incoming request from WebSocket
		// Format: METHOD|PATH|HEADERS|BODY
		if messageType == websocket.TextMessage {
			// Simple protocol for HTTP over WebSocket
			response := s.proxyHTTPRequest(client, tunnel.LocalAddr, payload)

			if err := tunnel.conn.WriteMessage(websocket.TextMessage, response); err != nil {
				logger.Error("Failed to write response", zap.Error(err))
				return
			}
		}

		tunnel.LastUsed = time.Now()
	}
}

// proxyHTTPRequest proxies HTTP request to local service
func (s *TunnelService) proxyHTTPRequest(client *http.Client, localAddr string, payload []byte) []byte {
	// nkk: Full HTTP proxy implementation
	// Based on ngrok's protocol design

	// Parse request format: METHOD|PATH|HEADERS_JSON|BODY
	parts := bytes.SplitN(payload, []byte("|"), 4)
	if len(parts) < 3 {
		return s.errorResponse(400, "Invalid request format")
	}

	method := string(parts[0])
	path := string(parts[1])

	// Parse headers
	var headers map[string]string
	if err := json.Unmarshal(parts[2], &headers); err != nil {
		return s.errorResponse(400, "Invalid headers format")
	}

	// Body is optional with size limit
	var body io.Reader
	if len(parts) > 3 && len(parts[3]) > 0 {
		// nkk: Limit request body size
		const maxBodySize = 10 * 1024 * 1024 // 10MB
		if len(parts[3]) > maxBodySize {
			return s.errorResponse(413, "Request body too large")
		}
		body = bytes.NewReader(parts[3])
	}

	// nkk: Build request to local service
	url := fmt.Sprintf("http://%s%s", localAddr, path)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return s.errorResponse(500, fmt.Sprintf("Failed to create request: %v", err))
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// nkk: Forward to local service
	resp, err := client.Do(req)
	if err != nil {
		return s.errorResponse(502, fmt.Sprintf("Failed to reach local service: %v", err))
	}
	defer resp.Body.Close()

	// nkk: Read response with size limit to prevent memory exhaustion
	const maxResponseSize = 10 * 1024 * 1024 // 10MB limit
	limitedReader := io.LimitReader(resp.Body, maxResponseSize)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return s.errorResponse(500, fmt.Sprintf("Failed to read response: %v", err))
	}

	// Check if response was truncated
	if len(respBody) == maxResponseSize {
		logger.Warn("Response truncated at size limit",
			zap.String("url", url),
			zap.Int("max_size", maxResponseSize))
	}

	// nkk: Build response format: STATUS|HEADERS_JSON|BODY
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	headersJSON, _ := json.Marshal(respHeaders)

	response := bytes.Buffer{}
	response.WriteString(fmt.Sprintf("%d|", resp.StatusCode))
	response.Write(headersJSON)
	response.WriteByte('|')
	response.Write(respBody)

	return response.Bytes()
}

// errorResponse creates an error response
func (s *TunnelService) errorResponse(status int, message string) []byte {
	headers := map[string]string{
		"Content-Type": "text/plain",
	}
	headersJSON, _ := json.Marshal(headers)

	response := bytes.Buffer{}
	response.WriteString(fmt.Sprintf("%d|", status))
	response.Write(headersJSON)
	response.WriteByte('|')
	response.WriteString(message)

	return response.Bytes()
}

// CloseAll closes all active tunnels for graceful shutdown
func (s *TunnelService) CloseAll() {
	// nkk: Close all tunnels during shutdown
	logger.Info("Closing all active tunnels")

	var tunnelIDs []string

	// Collect all tunnel IDs
	s.tunnels.Range(func(key, value interface{}) bool {
		if id, ok := key.(string); ok {
			tunnelIDs = append(tunnelIDs, id)
		}
		return true
	})

	// Close each tunnel
	for _, id := range tunnelIDs {
		if err := s.CloseTunnel(id); err != nil {
			logger.Error("Failed to close tunnel during shutdown",
				zap.String("tunnel_id", id),
				zap.Error(err))
		}
	}

	logger.Info("All tunnels closed", zap.Int("count", len(tunnelIDs)))
}