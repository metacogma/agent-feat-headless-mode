# Browser Platform Integration Guide

## Overview

This guide demonstrates how to integrate the enhanced EDC test framework with your BrowserStack-competitive browser automation platform. The integration provides seamless browser provisioning, session management, recording, and billing capabilities.

---

## Architecture Overview

```
Test Client (Enhanced EDC/Fixture)
            ↓
    Platform API Gateway
            ↓
┌─────────────────────────────────┐
│    Browser Automation Platform  │
├─────────────────────────────────┤
│ • Browser Pool (Playwright/Docker) │
│ • Session Management             │
│ • Tunnel Service                │
│ • Recording Service             │
│ • Billing Service              │
│ • Tenant Management            │
└─────────────────────────────────┘
            ↓
    Browsers (Chromium, Firefox, Safari)
```

---

## API Integration Points

### 1. Browser Acquisition

**Client Code:**
```typescript
const platformClient = new BrowserPlatformClient(
  'http://localhost:8081',
  'demo-tenant'
);

// Acquire browser from platform pool
const { browserId, webdriverUrl } = await platformClient.acquireBrowser({
  browser: 'chromium',
  version: 'latest',
  headless: true
});
```

**Platform API Call:**
```http
POST /browser/acquire
Content-Type: application/json

{
  "browser": "chromium",
  "version": "latest",
  "headless": true,
  "tenant_id": "demo-tenant"
}
```

**Platform Response:**
```json
{
  "browser_id": "chromium-1759413810487742000",
  "status": "acquired",
  "type": "chromium",
  "webdriver_url": "ws://localhost:9222/devtools/browser/...",
  "session_timeout": 3600
}
```

### 2. Session Management

**Tenant Allocation:**
```http
POST /tenant/session/allocate
Content-Type: application/json

{
  "org_id": "demo-org"
}
```

**Response:**
```json
{
  "org_id": "demo-org",
  "status": "allocated",
  "session_id": "sess_1759413840",
  "max_sessions": 25,
  "current_usage": 1
}
```

### 3. Recording Services

**Start Recording:**
```typescript
const recordingId = await platformClient.startRecording();
```

**API Call:**
```http
POST /recording/start
Content-Type: application/json

{
  "session_id": "sess_1759413840",
  "browser_id": "chromium-1759413810487742000",
  "format": "mp4",
  "quality": "720p"
}
```

**Stop Recording:**
```typescript
const recordingUrl = await platformClient.stopRecording(recordingId);
```

### 4. Billing Integration

**Usage Tracking:**
```typescript
await platformClient.trackUsage(30); // 30 minutes
```

**API Call:**
```http
POST /billing/usage
Content-Type: application/json

{
  "customer_id": "demo-tenant",
  "minutes": 30,
  "resource_type": "browser_session",
  "metadata": {
    "browser_type": "chromium",
    "test_name": "EDC Form Validation"
  }
}
```

---

## Complete Integration Example

### Test Setup with Platform

```typescript
import { test } from './fixture-enhanced';

test.describe('EDC Integration with Platform', () => {
  test('should run test using platform browser', async ({
    page,
    utils,
    browserPlatform
  }) => {
    // 1. Configure platform integration
    utils.config = {
      source: 'EDC',
      PLATFORM_API_URL: 'http://localhost:8081',
      TENANT_ID: 'demo-tenant',
      VAULT_DNS: 'your-vault.veevavault.com',
      VAULT_VERSION: 'v23.1',
      VAULT_STUDY_NAME: 'DEMO_STUDY',
      VAULT_STUDY_COUNTRY: 'United States',
      VAULT_SITE_NAME: 'Site 001',
      VAULT_SUBJECT_NAME: 'SUBJ-001',
      VAULT_USER_NAME: 'test@example.com',
      VAULT_PASSWORD: 'secure_password',
    };

    // 2. Initialize enhanced EDC with platform integration
    const edc = new EnhancedEDC({
      vaultDNS: utils.config.VAULT_DNS,
      version: utils.config.VAULT_VERSION,
      studyName: utils.config.VAULT_STUDY_NAME,
      studyCountry: utils.config.VAULT_STUDY_COUNTRY,
      siteName: utils.config.VAULT_SITE_NAME,
      subjectName: utils.config.VAULT_SUBJECT_NAME,
      utils,
      // Platform integration
      browserPlatform: {
        apiUrl: utils.config.PLATFORM_API_URL,
        tenantId: utils.config.TENANT_ID,
        sessionId: utils.config.SESSION_ID,
      }
    });

    // 3. Authenticate with Veeva Vault
    const authenticated = await edc.authenticate(
      utils.config.VAULT_USER_NAME,
      utils.config.VAULT_PASSWORD
    );
    expect(authenticated).toBe(true);

    // 4. Navigate to form using enhanced navigation
    const formDetails = {
      eventGroupId: 'EG_SCREENING',
      eventId: 'EV_VISIT_1',
      formId: 'FM_DEMOGRAPHICS',
      formName: 'Demographics',
      eventName: 'Visit 1',
      resetForm: true
    };

    await utils.goto(page, JSON.stringify(formDetails));

    // 5. Fill form using secure methods
    await utils.fillDate(
      page,
      "//input[@data-item-name='birth_date']",
      "new Date(1990, 0, 1)",
      "DD-MM-YYYY"
    );

    await utils.safeClick(
      page,
      "//input[@data-item-name='gender'][@value='Male']"
    );

    // 6. Submit form
    await utils.submitForm(page);

    // 7. Verify submission
    await utils.assertElement(
      page,
      "//span[contains(@class, 'success-message')]",
      "Form submitted successfully"
    );

    // Platform automatically tracks usage and stops recording
  });
});
```

### Environment Configuration

**playwright.config.ts:**
```typescript
import { defineConfig } from '@playwright/test';

export default defineConfig({
  projects: [
    {
      name: 'platform-chrome',
      use: {
        // Connect to platform-provided browser
        connectOptions: {
          wsEndpoint: process.env.PLATFORM_WS_ENDPOINT,
        },
      },
      metadata: {
        PLATFORM_API_URL: 'http://localhost:8081',
        TENANT_ID: 'demo-tenant',
      },
    },
    {
      name: 'platform-firefox',
      use: {
        connectOptions: {
          wsEndpoint: process.env.PLATFORM_WS_ENDPOINT_FIREFOX,
        },
      },
    },
  ],

  // Platform integration hooks
  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',
});
```

**global-setup.ts:**
```typescript
import { BrowserPlatformClient } from './fixture-enhanced';

async function globalSetup() {
  const platform = new BrowserPlatformClient(
    process.env.PLATFORM_API_URL!,
    process.env.TENANT_ID!
  );

  // Allocate session for test suite
  const session = await platform.allocateSession();
  process.env.PLATFORM_SESSION_ID = session.sessionId;

  // Pre-warm browser pool
  await platform.prewarmPool({
    browsers: ['chromium', 'firefox'],
    count: 5,
  });
}

export default globalSetup;
```

---

## Platform Health Monitoring

### Health Check Integration

```typescript
// Monitor platform health during tests
test.beforeEach(async ({ browserPlatform }) => {
  if (browserPlatform) {
    const health = await fetch(`${process.env.PLATFORM_API_URL}/health?detailed=true`);
    const status = await health.json();

    if (status.status !== 'healthy') {
      test.skip('Platform unhealthy, skipping test');
    }
  }
});
```

### Real-time Monitoring

```typescript
class PlatformMonitor {
  async trackTestMetrics(testInfo: any, metrics: any) {
    await fetch(`${process.env.PLATFORM_API_URL}/metrics/test`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        test_name: testInfo.title,
        duration: metrics.duration,
        success: testInfo.status === 'passed',
        browser_type: metrics.browserType,
        errors: metrics.errors,
        timestamp: new Date().toISOString(),
      }),
    });
  }
}
```

---

## Scaling and Load Testing

### Parallel Test Execution

```typescript
// Configure for parallel execution
export default defineConfig({
  workers: process.env.CI ? 10 : 5,
  fullyParallel: true,

  projects: [
    {
      name: 'parallel-chrome',
      testDir: './tests/parallel',
      use: {
        browserName: 'chromium',
      },
    },
  ],
});
```

### Load Testing Script

```typescript
// Load test the platform with multiple concurrent sessions
import { BrowserPlatformClient } from './fixture-enhanced';

async function loadTest() {
  const clients = Array.from({ length: 50 }, (_, i) =>
    new BrowserPlatformClient(
      'http://localhost:8081',
      `tenant-${i}`
    )
  );

  // Simulate concurrent browser acquisition
  const acquisitions = clients.map(async (client, index) => {
    const start = Date.now();

    try {
      const browser = await client.acquireBrowser({
        browser: 'chromium',
      });

      // Simulate test execution
      await new Promise(resolve => setTimeout(resolve, 30000));

      await client.releaseBrowser();

      return {
        index,
        success: true,
        duration: Date.now() - start,
      };
    } catch (error) {
      return {
        index,
        success: false,
        error: error.message,
        duration: Date.now() - start,
      };
    }
  });

  const results = await Promise.all(acquisitions);

  // Analyze results
  const successful = results.filter(r => r.success).length;
  const avgDuration = results.reduce((acc, r) => acc + r.duration, 0) / results.length;

  console.log(`Load Test Results:
    Total: ${results.length}
    Successful: ${successful}
    Success Rate: ${(successful / results.length * 100).toFixed(2)}%
    Avg Duration: ${avgDuration.toFixed(2)}ms
  `);
}
```

---

## Deployment and Configuration

### Docker Compose Integration

```yaml
# docker-compose.integration.yml
version: '3.8'

services:
  # Your existing platform services
  platform:
    build: .
    ports:
      - "8081:8081"
    environment:
      - BROWSER_POOL_SIZE=50
      - ENABLE_RECORDING=true
    depends_on:
      - mongodb
      - redis

  # Test runner with platform integration
  test-runner:
    build:
      context: .
      dockerfile: Dockerfile.tests
    environment:
      - PLATFORM_API_URL=http://platform:8081
      - TENANT_ID=test-tenant
      - PARALLEL_WORKERS=10
    volumes:
      - ./test-results:/app/test-results
      - ./recordings:/app/recordings
    depends_on:
      - platform

  # Enhanced test reporting
  test-reporter:
    image: allure-framework/allure-docker-service
    ports:
      - "5050:5050"
    volumes:
      - ./test-results:/app/allure-results
```

### Kubernetes Deployment

```yaml
# k8s-test-platform.yml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-platform
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test-platform
  template:
    metadata:
      labels:
        app: test-platform
    spec:
      containers:
      - name: test-platform
        image: your-platform:latest
        ports:
        - containerPort: 8081
        env:
        - name: BROWSER_POOL_SIZE
          value: "100"
        - name: REDIS_URL
          value: "redis://redis-service:6379"
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "1000m"

---
apiVersion: v1
kind: Service
metadata:
  name: test-platform-service
spec:
  selector:
    app: test-platform
  ports:
  - port: 8081
    targetPort: 8081
  type: LoadBalancer
```

---

## Troubleshooting Guide

### Common Issues

**1. Browser Acquisition Timeout**
```typescript
// Solution: Increase timeout and add retry logic
const browser = await platformClient.acquireBrowser({
  browser: 'chromium',
  timeout: 60000, // 60 seconds
  retries: 3,
});
```

**2. Session Limit Exceeded**
```typescript
// Solution: Check tenant limits before allocation
const stats = await fetch(`${apiUrl}/tenant/stats/${tenantId}`);
const { available_sessions } = await stats.json();

if (available_sessions < 1) {
  throw new Error('No available sessions for tenant');
}
```

**3. Recording Failures**
```typescript
// Solution: Verify recording service health
const health = await fetch(`${apiUrl}/health/detailed`);
const status = await health.json();

const recorderHealthy = status.services.find(s =>
  s.name === 'session_recorder'
)?.status === 'healthy';

if (!recorderHealthy) {
  console.warn('Recording service unavailable, proceeding without recording');
}
```

### Debug Mode

```typescript
// Enable debug logging
process.env.DEBUG = 'platform:*,edc:*';

// Platform client debug
const platformClient = new BrowserPlatformClient(apiUrl, tenantId, {
  debug: true,
  logRequests: true,
  logResponses: true,
});
```

---

## Performance Optimization

### Connection Optimization

```typescript
// Optimize for high-throughput testing
const platformClient = new BrowserPlatformClient(apiUrl, tenantId, {
  connectionPool: {
    maxConnections: 100,
    keepAlive: true,
    timeout: 30000,
  },
  retry: {
    attempts: 3,
    backoff: 'exponential',
  },
});
```

### Caching Strategy

```typescript
// Cache browser sessions for reuse
class SessionCache {
  private cache = new Map<string, { session: any; expires: number }>();

  async getOrCreate(key: string, factory: () => Promise<any>): Promise<any> {
    const cached = this.cache.get(key);

    if (cached && cached.expires > Date.now()) {
      return cached.session;
    }

    const session = await factory();
    this.cache.set(key, {
      session,
      expires: Date.now() + 300000, // 5 minutes
    });

    return session;
  }
}
```

---

## Summary

This integration guide provides a complete blueprint for connecting the enhanced EDC test framework with your browser automation platform. The integration offers:

✅ **Seamless Browser Provisioning** - Automatic browser acquisition/release
✅ **Session Management** - Multi-tenant resource allocation
✅ **Automatic Recording** - Test session capture for debugging
✅ **Usage Tracking** - Billing integration for usage-based pricing
✅ **Health Monitoring** - Real-time platform health checks
✅ **Scalability** - Support for parallel test execution
✅ **Production Ready** - Docker/Kubernetes deployment examples

The enhanced framework serves as both a robust testing solution and a reference implementation for platform integration, demonstrating enterprise-grade capabilities while maintaining security and performance standards.