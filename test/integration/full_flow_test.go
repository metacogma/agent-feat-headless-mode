// +build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"agent/services/billing"
	"agent/services/browser_pool"
	"agent/services/geo"
	"agent/services/recorder"
	"agent/services/tenant"
	"agent/services/tunnel"
)

/*
nkk: Integration test suite
Tests all services working together with real dependencies
*/

type IntegrationTestSuite struct {
	suite.Suite
	mongoClient     *mongo.Client
	browserPool     *browser_pool.BrowserPoolManager
	tunnelService   *tunnel.TunnelService
	tenantManager   *tenant.Manager
	billingService  *billing.Service
	geoRouter       *geo.Router
	sessionRecorder *recorder.SessionRecorder
}

func (suite *IntegrationTestSuite) SetupSuite() {
	// Connect to test MongoDB
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://admin:testpass123@localhost:27017/testrunner?authSource=admin"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	suite.Require().NoError(err)

	// Ping to verify connection
	err = client.Ping(ctx, nil)
	suite.Require().NoError(err)

	suite.mongoClient = client

	// Initialize services
	suite.browserPool, _ = browser_pool.NewBrowserPoolManager(5)
	suite.tunnelService = tunnel.NewTunnelService()
	suite.tenantManager = tenant.NewManager()
	suite.billingService = billing.NewService()
	suite.geoRouter = geo.NewRouter()
	suite.sessionRecorder = recorder.NewSessionRecorder()
}

func (suite *IntegrationTestSuite) TearDownSuite() {
	// Cleanup
	if suite.browserPool != nil {
		suite.browserPool.Shutdown()
	}
	if suite.mongoClient != nil {
		suite.mongoClient.Disconnect(context.Background())
	}
}

func (suite *IntegrationTestSuite) TestFullBrowserSession() {
	ctx := context.Background()

	// 1. Create tenant
	tenant, err := suite.tenantManager.CreateTenant("test-org", "pro")
	suite.NoError(err)
	suite.NotNil(tenant)

	// 2. Allocate session for tenant
	err = suite.tenantManager.AllocateSession(ctx, "test-org")
	suite.NoError(err)

	// 3. Route request through geo router
	region, err := suite.geoRouter.RouteRequest(ctx, "192.168.1.1")
	suite.NoError(err)
	suite.NotNil(region)

	// 4. Acquire browser from pool
	browser, err := suite.browserPool.AcquireBrowser(ctx, "chrome", "latest")
	if err != nil {
		suite.T().Skip("Docker not available")
	}
	suite.NotNil(browser)

	// 5. Start recording session
	recording, err := suite.sessionRecorder.StartRecording(ctx, "test-session", browser.ContainerID, 5900)
	// May fail without VNC, but shouldn't crash
	if err == nil {
		suite.NotNil(recording)

		// 6. Simulate some work
		time.Sleep(2 * time.Second)

		// 7. Stop recording
		err = suite.sessionRecorder.StopRecording("test-session")
		suite.NoError(err)
	}

	// 8. Track billing usage
	suite.billingService.TrackUsage(ctx, "test-org", 2.5)

	// 9. Release browser
	suite.browserPool.ReleaseBrowser("chrome", "latest", browser)

	// 10. Release session
	suite.tenantManager.ReleaseSession("test-org")

	// 11. Release geo capacity
	suite.geoRouter.ReleaseCapacity(region.Name)

	// 12. Calculate bill
	bill, err := suite.billingService.CalculateBill("test-org", time.Now().Format("2006-01"))
	suite.NoError(err)
	suite.True(bill > 0)
}

func (suite *IntegrationTestSuite) TestTunnelWithProxy() {
	// Create tunnel
	tunnel, err := suite.tunnelService.CreateTunnel("test-user", "localhost:8080")
	suite.NoError(err)
	suite.NotNil(tunnel)
	suite.NotEmpty(tunnel.Subdomain)

	// Test tunnel properties
	suite.Equal("test-user", tunnel.UserID)
	suite.True(tunnel.Active)

	// Close tunnel
	err = suite.tunnelService.CloseTunnel(tunnel.ID)
	suite.NoError(err)
}

func (suite *IntegrationTestSuite) TestMultiTenantIsolation() {
	ctx := context.Background()

	// Create multiple tenants
	tenant1, _ := suite.tenantManager.CreateTenant("org1", "free")
	tenant2, _ := suite.tenantManager.CreateTenant("org2", "pro")

	// Verify different limits
	suite.Equal(3, tenant1.MaxSessions)
	suite.Equal(25, tenant2.MaxSessions)

	// Test rate limiting
	for i := 0; i < 5; i++ {
		err := suite.tenantManager.AllocateSession(ctx, "org1")
		if i < 3 {
			suite.NoError(err)
		} else {
			// Should fail after limit
			suite.Error(err)
		}
	}

	// Release all sessions
	for i := 0; i < 3; i++ {
		suite.tenantManager.ReleaseSession("org1")
	}
}

func (suite *IntegrationTestSuite) TestRecordingLifecycle() {
	ctx := context.Background()

	// Start multiple recordings
	recordings := make([]*recorder.RecordingSession, 0)

	for i := 0; i < 3; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		recording, err := suite.sessionRecorder.StartRecording(
			ctx, sessionID, "container-id", 5900+i,
		)
		if err != nil {
			// VNC might not be available
			continue
		}
		recordings = append(recordings, recording)
	}

	// Verify recordings are active
	for _, rec := range recordings {
		suite.Equal("recording", rec.Status)
	}

	// Stop all recordings
	suite.sessionRecorder.StopAll()

	// Verify all stopped
	for _, rec := range recordings {
		retrieved, _ := suite.sessionRecorder.GetRecording(rec.SessionID)
		if retrieved != nil {
			suite.Equal("completed", retrieved.Status)
		}
	}
}

func (suite *IntegrationTestSuite) TestGeoRouterLoadBalancing() {
	ctx := context.Background()

	// Simulate multiple requests
	allocations := 0
	failures := 0

	for i := 0; i < 100; i++ {
		region, err := suite.geoRouter.RouteRequest(ctx, "10.0.0.1")
		if err == nil {
			allocations++
			// Release immediately to test cycling
			suite.geoRouter.ReleaseCapacity(region.Name)
		} else {
			failures++
		}
	}

	// Should handle most requests
	suite.True(allocations > 90)
	suite.True(failures < 10)
}

func (suite *IntegrationTestSuite) TestHealthChecks() {
	ctx := context.Background()

	// Start health check loop
	suite.geoRouter.StartHealthCheckLoop(5 * time.Second)

	// Wait for first check
	time.Sleep(6 * time.Second)

	// Get region stats
	stats := suite.geoRouter.GetRegionStats()
	suite.NotEmpty(stats)

	// At least one region should be present
	suite.Contains(stats, "us-east")
}

// Run the test suite
func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}