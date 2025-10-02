package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"agent/logger"
	"agent/services/billing"
	"agent/services/browser_pool"
	"agent/services/geo"
	"agent/services/health"
	"agent/services/recorder"
	"agent/services/tenant"
	"agent/services/tunnel"
)

// nkk: Simple test runner to demonstrate the system
// Runs without external dependencies for demo purposes

func main() {
	fmt.Println("🚀 Starting Agent Test System")
	fmt.Println("================================")

	// Initialize logger
	logger.InitLogger("debug")

	// Create services
	fmt.Println("\n📦 Initializing Services...")

	// nkk: Test Docker pool with ARM64 compatible images
	var browserPool *browser_pool.BrowserPoolManager
	var err error

	browserPool, err = browser_pool.NewBrowserPoolManager(5)
	if err != nil {
		fmt.Printf("⚠️  Browser Pool: Not available (Docker issue: %v)\n", err)
		browserPool = nil
	} else {
		fmt.Printf("✅ Docker Browser Pool: Initialized with 5 slots\n")
		defer browserPool.Shutdown()
	}

	// Other services (work without external dependencies)
	tunnelService := tunnel.NewTunnelService()
	fmt.Printf("✅ Tunnel Service: Initialized\n")

	tenantManager := tenant.NewManager()
	fmt.Printf("✅ Tenant Manager: Initialized\n")

	billingService := billing.NewService()
	fmt.Printf("✅ Billing Service: Initialized\n")

	geoRouter := geo.NewRouter()
	fmt.Printf("✅ Geo Router: Initialized (us-east region)\n")

	sessionRecorder := recorder.NewSessionRecorder()
	fmt.Printf("✅ Session Recorder: Initialized\n")

	// nkk: Create health handler - pass Docker pool for compatibility
	// Health handler expects BrowserPoolManager type, not Playwright
	healthHandler := health.NewHealthHandler(
		browserPool,
		tunnelService,
		tenantManager,
		billingService,
		geoRouter,
		sessionRecorder,
	)

	// Start health monitoring
	healthHandler.StartBackgroundChecks(30 * time.Second)
	fmt.Printf("✅ Health Monitoring: Started\n")

	// Setup HTTP routes
	http.HandleFunc("/health", healthHandler.ServeHTTP)
	http.HandleFunc("/health/detailed", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Query().Set("detailed", "true")
		healthHandler.ServeHTTP(w, r)
	})

	// Demo endpoints
	http.HandleFunc("/demo/tenant", demoTenant(tenantManager))
	http.HandleFunc("/demo/billing", demoBilling(billingService))
	http.HandleFunc("/demo/tunnel", demoTunnel(tunnelService))
	http.HandleFunc("/demo/geo", demoGeo(geoRouter))

	// nkk: Real API endpoints for test client compatibility
	// Browser Pool endpoints
	if browserPool != nil {
		http.HandleFunc("/browser/acquire", handleBrowserAcquire(browserPool))
		http.HandleFunc("/browser/release", handleBrowserRelease(browserPool))
	}

	// Recording endpoints
	http.HandleFunc("/recording/start", handleRecordingStart(sessionRecorder))
	http.HandleFunc("/recording/stop", handleRecordingStop(sessionRecorder))

	// Tunnel endpoints
	http.HandleFunc("/tunnel/create", handleTunnelCreate(tunnelService))
	http.HandleFunc("/tunnel/ws", handleTunnelWebSocket(tunnelService))

	// Billing endpoints
	http.HandleFunc("/billing/subscription", handleBillingSubscription(billingService))
	http.HandleFunc("/billing/usage", handleBillingUsage(billingService))
	http.HandleFunc("/billing/invoice", handleBillingInvoice(billingService))

	// Tenant endpoints
	http.HandleFunc("/tenant/create", handleTenantCreate(tenantManager))
	http.HandleFunc("/tenant/session/allocate", handleSessionAllocate(tenantManager))
	http.HandleFunc("/tenant/session/release", handleSessionRelease(tenantManager))

	fmt.Println("\n🌐 Starting HTTP Server on :8081")
	fmt.Println("================================")
	fmt.Println("\n📍 Available Endpoints:")
	fmt.Println("  GET  /health           - Basic health check")
	fmt.Println("  GET  /health?detailed=true - Detailed health")
	fmt.Println("  GET  /demo/tenant      - Test multi-tenancy")
	fmt.Println("  GET  /demo/billing     - Test billing")
	fmt.Println("  GET  /demo/tunnel      - Test tunnel service")
	fmt.Println("  GET  /demo/geo         - Test geo routing")
	fmt.Println("\n================================")

	// Run demo sequence
	go runDemoSequence(tenantManager, billingService, geoRouter, tunnelService)

	// Start server
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func runDemoSequence(tm *tenant.Manager, bs *billing.Service, gr *geo.Router, ts *tunnel.TunnelService) {
	time.Sleep(2 * time.Second)

	fmt.Println("\n🎯 Running Demo Sequence")
	fmt.Println("------------------------")

	ctx := context.Background()

	// 1. Create tenant
	fmt.Print("\n1. Creating tenant (org: demo-org, tier: pro)... ")
	tenant, err := tm.CreateTenant("demo-org", "pro")
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Printf("✅ Created (Max sessions: %d)\n", tenant.MaxSessions)
	}

	// 2. Allocate session
	fmt.Print("2. Allocating session for tenant... ")
	err = tm.AllocateSession(ctx, "demo-org")
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Printf("✅ Allocated (1/%d sessions in use)\n", tenant.MaxSessions)
	}

	// 3. Route request
	fmt.Print("3. Routing request through geo router... ")
	region, err := gr.RouteRequest(ctx, "192.168.1.1")
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Printf("✅ Routed to %s region\n", region.Name)
	}

	// 4. Create tunnel
	fmt.Print("4. Creating secure tunnel... ")
	tunnel, err := ts.CreateTunnel("demo-user", "localhost:3000")
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Printf("✅ Created (Subdomain: %s)\n", tunnel.Subdomain)
	}

	// 5. Track usage
	fmt.Print("5. Tracking billing usage (30 minutes)... ")
	bs.TrackUsage(ctx, "demo-org", 30.0)
	fmt.Println("✅ Tracked")

	// 6. Calculate bill
	fmt.Print("6. Calculating monthly bill... ")
	bill, err := bs.CalculateBill("demo-org", time.Now().Format("2006-01"))
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Printf("✅ Bill: $%.2f\n", bill)
	}

	// 7. Release resources
	fmt.Print("7. Releasing resources... ")
	tm.ReleaseSession("demo-org")
	gr.ReleaseCapacity(region.Name)
	ts.CloseTunnel(tunnel.ID)
	fmt.Println("✅ Released")

	fmt.Println("\n✨ Demo sequence complete!")
	fmt.Println("\nℹ️  Try these commands:")
	fmt.Println("  curl http://localhost:8081/health")
	fmt.Println("  curl http://localhost:8081/health?detailed=true")
	fmt.Println("  curl http://localhost:8081/demo/tenant")
}

// Demo handlers
func demoTenant(tm *tenant.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Create test tenant
		tenant, _ := tm.CreateTenant("test-org", "pro")

		// Try allocation
		err := tm.AllocateSession(ctx, "test-org")

		result := map[string]interface{}{
			"tenant_id":       tenant.ID,
			"organization":    tenant.OrganizationID,
			"tier":            tenant.Tier,
			"max_sessions":    tenant.MaxSessions,
			"current_sessions": tenant.CurrentSessions,
			"allocation_success": err == nil,
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","tenant":%v}`, result)
	}
}

func demoBilling(bs *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Track some usage
		bs.TrackUsage(ctx, "demo-customer", 45.5)

		// Calculate bill
		bill, _ := bs.CalculateBill("demo-customer", time.Now().Format("2006-01"))

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","usage_minutes":45.5,"bill":%.2f}`, bill)
	}
}

func demoTunnel(ts *tunnel.TunnelService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create tunnel
		tunnel, _ := ts.CreateTunnel("demo-user", "localhost:3000")

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","tunnel_id":"%s","subdomain":"%s"}`, tunnel.ID, tunnel.Subdomain)

		// Clean up
		go func() {
			time.Sleep(5 * time.Second)
			ts.CloseTunnel(tunnel.ID)
		}()
	}
}

func demoGeo(gr *geo.Router) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Route request
		region, err := gr.RouteRequest(ctx, r.RemoteAddr)

		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		// Get stats
		stats := gr.GetRegionStats()

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","routed_to":"%s","stats":%v}`, region.Name, stats)

		// Release
		gr.ReleaseCapacity(region.Name)
	}
}

// nkk: Real API handlers for test client compatibility
func handleBrowserAcquire(bp *browser_pool.BrowserPoolManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bp == nil {
			http.Error(w, `{"error":"browser pool not available"}`, http.StatusServiceUnavailable)
			return
		}

		var req struct {
			Browser string `json:"browser"`
			Version string `json:"version"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Browser == "" {
			req.Browser = "chrome"
		}
		if req.Version == "" {
			req.Version = "latest"
		}

		instance, err := bp.AcquireBrowser(r.Context(), req.Browser, req.Version)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": instance.ID,
			"webdriver_url": instance.WebDriverURL,
			"browser": instance.BrowserType,
			"version": instance.Version,
		})
	}
}

func handleBrowserRelease(bp *browser_pool.BrowserPoolManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bp == nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		var req struct {
			ID string `json:"id"`
			Browser string `json:"browser"`
			Version string `json:"version"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// For demo, just return success
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"released"}`))
	}
}

func handleRecordingStart(sr *recorder.SessionRecorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SessionID string `json:"session_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.SessionID == "" {
			req.SessionID = fmt.Sprintf("session-%d", time.Now().Unix())
		}

		// nkk: Start recording with VNC port (demo values)
		_, err := sr.StartRecording(r.Context(), req.SessionID, "container-123", 5900)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_id": req.SessionID,
			"status": "recording",
		})
	}
}

func handleRecordingStop(sr *recorder.SessionRecorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SessionID string `json:"session_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		err := sr.StopRecording(req.SessionID)
		url := fmt.Sprintf("https://recordings.example.com/%s.mp4", req.SessionID)
		if err != nil {
			// Still return URL even if stop had issues
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_id": req.SessionID,
			"recording_url": url,
			"status": "stopped",
		})
	}
}

func handleTunnelCreate(ts *tunnel.TunnelService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID    string `json:"user_id"`
			LocalAddr string `json:"local_addr"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		tunnel, err := ts.CreateTunnel(req.UserID, req.LocalAddr)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": tunnel.ID,
			"subdomain": tunnel.Subdomain,
			"public_url": fmt.Sprintf("https://%s.tunnel.example.com", tunnel.Subdomain),
		})
	}
}

func handleTunnelWebSocket(ts *tunnel.TunnelService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tunnelID := r.Header.Get("X-Tunnel-ID")
		if tunnelID == "" {
			http.Error(w, "missing tunnel ID", http.StatusBadRequest)
			return
		}

		// For demo, upgrade to WebSocket and echo
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Simple echo for demo
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				return
			}
			// Echo back
			conn.WriteMessage(messageType, p)
		}
	}
}

func handleBillingSubscription(bs *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CustomerID string `json:"customer_id"`
			PlanID     string `json:"plan_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// For demo, just acknowledge
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"customer_id": req.CustomerID,
			"plan_id": req.PlanID,
			"status": "active",
			"subscription_id": fmt.Sprintf("sub_%d", time.Now().Unix()),
		})
	}
}

func handleBillingUsage(bs *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CustomerID string  `json:"customer_id"`
			Minutes    float64 `json:"minutes"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		bs.TrackUsage(r.Context(), req.CustomerID, req.Minutes)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"customer_id": req.CustomerID,
			"minutes": req.Minutes,
			"status": "tracked",
		})
	}
}

func handleBillingInvoice(bs *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerID := r.URL.Query().Get("customer_id")
		if customerID == "" {
			customerID = "demo-customer"
		}

		month := time.Now().Format("2006-01")
		bill, err := bs.CalculateBill(customerID, month)
		if err != nil {
			bill = 0
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"customer_id": customerID,
			"month": month,
			"amount": bill,
			"status": "pending",
			"invoice_id": fmt.Sprintf("inv_%d", time.Now().Unix()),
		})
	}
}

func handleTenantCreate(tm *tenant.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OrgID string `json:"org_id"`
			Tier  string `json:"tier"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Tier == "" {
			req.Tier = "standard"
		}

		tenant, err := tm.CreateTenant(req.OrgID, req.Tier)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tenant_id": tenant.ID,
			"org_id": tenant.OrganizationID,
			"tier": tenant.Tier,
			"max_sessions": tenant.MaxSessions,
		})
	}
}

func handleSessionAllocate(tm *tenant.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OrgID string `json:"org_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		err := tm.AllocateSession(r.Context(), req.OrgID)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"org_id": req.OrgID,
			"status": "allocated",
			"session_id": fmt.Sprintf("sess_%d", time.Now().Unix()),
		})
	}
}

func handleSessionRelease(tm *tenant.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OrgID string `json:"org_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		tm.ReleaseSession(req.OrgID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"org_id": req.OrgID,
			"status": "released",
		})
	}
}

// nkk: Playwright-specific handlers for browser operations
func handlePlaywrightAcquire(pool *browser_pool.PlaywrightPoolManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Browser string `json:"browser"`
			Version string `json:"version"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Browser == "" {
			req.Browser = "chromium"
		}

		instance, err := pool.AcquireBrowser(r.Context(), req.Browser, req.Version)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"browser_id": instance.ID,
			"type": instance.BrowserType,
			"status": "acquired",
		})
	}
}

func handlePlaywrightRelease(pool *browser_pool.PlaywrightPoolManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			BrowserID string `json:"browser_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// For demo, we just acknowledge the release
		// In real implementation, you'd track instances by ID
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"browser_id": req.BrowserID,
			"status": "released",
		})
	}
}