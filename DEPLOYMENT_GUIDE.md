# Agent Service - Production Deployment Guide

## Overview

This guide provides comprehensive instructions for deploying the agent service in production environments. The service has been optimized for reliability, testability, and production readiness.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Configuration](#configuration)
4. [Running Modes](#running-modes)
5. [Health Checks](#health-checks)
6. [Monitoring](#monitoring)
7. [Troubleshooting](#troubleshooting)
8. [Production Checklist](#production-checklist)

---

## Prerequisites

### System Requirements

- **Operating System**: macOS, Linux, or Windows
- **Go**: Version 1.21 or higher
- **Node.js**: Version 16 or higher (for Playwright)
- **Docker**: Optional (for browser pool containers)
- **Memory**: Minimum 4GB RAM (8GB+ recommended)
- **CPU**: 2+ cores recommended

### External Services (Production Mode)

- **Aurora Dev Service**: Port 5476 (for device registration)
- **Executor Service**: Port 9123 (for execution management)
- **Dashboard**: Port 3000 (for UI)

### Optional Services

- **Docker Daemon**: For browser container management
- **Prometheus**: For metrics collection (port 9090)

---

## Installation

### 1. Clone Repository

```bash
git clone https://github.com/your-org/agent-feat-headless-mode.git
cd agent-feat-headless-mode
```

### 2. Install Dependencies

```bash
# Install Go dependencies
go mod download

# Install Node.js dependencies (Playwright)
cd executions
npm install
npx playwright install
cd ..
```

### 3. Build Binary (Optional)

```bash
# Build for production
go build -o agent cmd/agent/main.go

# Or use the Makefile
make build
```

---

## Configuration

### Machine Configuration File

Create `configuration/machine_config.json`:

```json
{
  "application": "agent",
  "logger": {
    "Level": "info",
    "HostName": "production-agent-01"
  },
  "listen": ":5000",
  "server_domain": "http://aurora-service:5476/aurora-dev/v1",
  "execution_service_domain": "http://executor-service:9123/executor/v1",
  "dashboard_domain": "https://autotest.apxor.com",
  "prefix": "/agent",
  "hostname": "production-agent-01",
  "machine_id": "auto-generated-on-first-run",
  "cors": {
    "AllowedOrigins": [
      "http://*.apxor.com",
      "https://*.apxor.com",
      "https://internal.cors.com",
      "https://localhost",
      "https://localhost:3000",
      "http://localhost:3000"
    ]
  },
  "project_id": "",
  "org_id": ""
}
```

### Environment Variables

```bash
# Optional: Override configuration
export AGENT_PORT=5000
export AGENT_LOG_LEVEL=info
export AGENT_SERVER_DOMAIN=http://aurora-service:5476/aurora-dev/v1
```

---

## Running Modes

### Production Mode (Default)

Requires all external services to be available.

```bash
# Using binary
./agent start

# Using go run
go run cmd/agent/main.go start

# Background mode (opens browser for registration)
./agent start

# Background mode (skips browser opening)
./agent start --background
```

### Test Mode (Standalone)

Runs without external service dependencies - ideal for development and testing.

```bash
# Test mode - bypasses external services
go run cmd/agent/main.go start --test-mode

# Test mode + background
go run cmd/agent/main.go start --test-mode --background
```

**Test Mode Features:**
- ✅ Skips Aurora service registration
- ✅ Skips Executor service calls
- ✅ Handles Docker unavailability gracefully
- ✅ All API endpoints functional
- ✅ Generates local machine ID

### Configuration Mode

Update configuration without starting the server.

```bash
# Set custom port
go run cmd/agent/main.go set -p 8080

# Set dashboard domain
go run cmd/agent/main.go set --dashboard-domain https://dashboard.example.com

# Set server domain
go run cmd/agent/main.go set --server-domain http://aurora:5476/aurora-dev/v1

# Set execution domain
go run cmd/agent/main.go set --execution-domain http://executor:9123/executor/v1
```

---

## Health Checks

### Simple Health Check

```bash
# Quick health check (returns OK/UNHEALTHY)
curl http://localhost:5000/health

# Expected response: HTTP 200 OK
```

### Detailed Health Check

```bash
# Detailed health information
curl http://localhost:5000/health?detailed=true

# Expected response (JSON):
{
  "status": "healthy",
  "timestamp": 1696723200,
  "services": [
    {
      "name": "browser_pool",
      "status": "degraded",
      "latency_ms": 5,
      "details": {
        "total_available": 0,
        "total_in_use": 0,
        "docker_available": false
      },
      "last_check": "2025-10-07T17:00:00Z"
    },
    {
      "name": "tunnel_service",
      "status": "healthy",
      "latency_ms": 2
    }
  ]
}
```

### Monitoring Endpoint

```bash
# Prometheus metrics
curl http://localhost:9090/metrics

# Health check endpoint
curl http://localhost:9090/health
```

---

## Monitoring

### Metrics Available

The service exposes Prometheus-compatible metrics on port 9090:

```
# Service health status
service_health{service="browser_pool"} 0.5
service_health{service="tunnel_service"} 1.0

# Service latency
service_health_latency_ms{service="browser_pool"} 5
service_health_latency_ms{service="tunnel_service"} 2
```

### Log Levels

Configure logging in `configuration/machine_config.json`:

```json
{
  "logger": {
    "Level": "info"  // Options: debug, info, warn, error
  }
}
```

### Viewing Logs

```bash
# Real-time logs
tail -f /var/log/agent.log

# Or if running in foreground
go run cmd/agent/main.go start
```

---

## Troubleshooting

### Common Issues

#### 1. Port Already in Use

**Error:** `listen tcp :5000: bind: address already in use`

**Solution:**
```bash
# Find and kill the process
lsof -ti:5000 | xargs kill -9

# Or use a different port
go run cmd/agent/main.go set -p 8080
```

#### 2. Docker Daemon Not Available

**Error:** `Cannot connect to the Docker daemon`

**Impact:** Browser pool runs in degraded mode (still functional)

**Solutions:**
- Start Docker Desktop (macOS/Windows)
- Start Docker daemon (Linux): `sudo systemctl start docker`
- Or continue in degraded mode - the service will work without Docker

#### 3. External Service Connection Refused

**Error:** `dial tcp :5476: connect: connection refused`

**Solutions:**
- Use `--test-mode` flag to bypass external services
- Ensure Aurora/Executor services are running
- Check network connectivity and firewall rules

#### 4. Playwright Installation Failed

**Error:** `Failed to install Playwright browsers`

**Solution:**
```bash
cd executions
npx playwright install
```

### Debug Mode

Enable debug logging:

```bash
# Set log level to debug
go run cmd/agent/main.go set --log-level debug

# Then start server
go run cmd/agent/main.go start --test-mode
```

---

## Production Checklist

### Pre-Deployment

- [ ] All dependencies installed (Go, Node.js, Playwright)
- [ ] Configuration file created and reviewed
- [ ] External services accessible (or test mode enabled)
- [ ] Firewall rules configured for ports 5000, 9090
- [ ] SSL/TLS certificates configured (if using HTTPS)
- [ ] Log rotation configured
- [ ] Monitoring/alerting configured

### Deployment

- [ ] Binary built and tested
- [ ] Service configuration validated
- [ ] Health checks passing
- [ ] Test suite executed successfully
- [ ] Load testing completed
- [ ] Backup and rollback plan ready

### Post-Deployment

- [ ] Service started and running
- [ ] Health checks reporting healthy
- [ ] Metrics being collected
- [ ] Logs being written
- [ ] API endpoints responding
- [ ] External service connectivity verified
- [ ] Documentation updated

---

## API Endpoints

### Core Endpoints

```
POST   /agent/v1/start
GET    /agent/v1/organisations/{org_id}/projects/{project_id}/apps/{app_id}/local-agent/status
POST   /agent/v1/organisations/{org_id}/projects/{project_id}/apps/{app_id}/{testlab}/sessions/
PUT    /agent/v1/organisations/{org_id}/projects/{project_id}/apps/{app_id}/{testlab}/sessions/
POST   /agent/v1/organisations/{org_id}/projects/{project_id}/apps/{app_id}/{testlab}/{execution_id}/update-status
PUT    /agent/v1/organisations/{org_id}/projects/{project_id}/apps/{app_id}/{testlab}/{execution_id}/update-stepcount
POST   /agent/v1/organisations/{org_id}/projects/{project_id}/apps/{app_id}/{testlab}/{execution_id}/upload-screenshots
POST   /agent/v1/organisations/{org_id}/projects/{project_id}/apps/{app_id}/{testlab}/{execution_id}/take-screenshot
POST   /agent/v1/organisations/{org_id}/projects/{project_id}/apps/{app_id}/local-agent/network-logs
```

### Testing Endpoints

Use the provided test client:

```bash
cd agent-client-test
go run main.go
```

---

## Security Considerations

### Network Security

- Use firewall rules to restrict access to management ports (9090)
- Enable TLS/SSL for production deployments
- Use API keys/tokens for authentication
- Whitelist allowed CORS origins

### Data Security

- Encrypt sensitive configuration data
- Use secrets management (e.g., HashiCorp Vault)
- Rotate API keys regularly
- Enable audit logging

### Access Control

- Run service with minimal privileges
- Use dedicated service account
- Implement rate limiting
- Monitor for suspicious activity

---

## Performance Tuning

### Browser Pool Configuration

```go
// In services/browser_pool/manager.go
browserPoolManager, err := browser_pool.NewBrowserPoolManager(100) // Increase pool size
```

### Resource Limits

```bash
# Set Go runtime limits
export GOMAXPROCS=4
export GOGC=100
```

### Docker Resources (if using)

```yaml
# docker-compose.yml
services:
  agent:
    deploy:
      resources:
        limits:
          memory: 8G
          cpus: '4'
        reservations:
          memory: 4G
          cpus: '2'
```

---

## Support & Contact

For issues, questions, or feature requests:

- **GitHub Issues**: https://github.com/your-org/agent-feat-headless-mode/issues
- **Documentation**: https://docs.your-org.com/agent
- **Email**: support@your-org.com
- **Slack**: #agent-support

---

## Changelog

### Version 1.1.0 (2025-10-07)

**New Features:**
- ✅ Test mode for standalone operation
- ✅ Graceful Docker unavailability handling
- ✅ Improved error handling and logging
- ✅ Enhanced health check endpoint

**Bug Fixes:**
- ✅ Fixed Agent Status endpoint (EOF error)
- ✅ Fixed step count data type validation
- ✅ Improved error messages

**Improvements:**
- ✅ Better Docker detection and fallback
- ✅ Comprehensive health checks
- ✅ Production-ready error handling

---

## License

[Your License Here]
