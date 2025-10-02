package tenant

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: Simplified Tenant Manager for resource isolation
Focus on core multi-tenancy without over-engineering
Scale complexity when actual requirements emerge
*/

type Tenant struct {
	ID             string
	OrganizationID string
	Tier           string // free, pro, enterprise
	RateLimiter    *rate.Limiter
	CreatedAt      time.Time

	// Resource limits
	MaxSessions    int
	CurrentSessions int
	mu             sync.Mutex
}

type Manager struct {
	tenants sync.Map // map[string]*Tenant
}

// NewManager creates a new tenant manager
func NewManager() *Manager {
	return &Manager{}
}

// CreateTenant creates a new tenant
func (m *Manager) CreateTenant(orgID, tier string) (*Tenant, error) {
	// nkk: Simple tier-based limits
	limits := map[string]struct {
		sessions int
		rps      rate.Limit
	}{
		"free":       {3, 1},
		"pro":        {25, 10},
		"enterprise": {100, 100},
	}

	limit, ok := limits[tier]
	if !ok {
		limit = limits["free"]
	}

	tenant := &Tenant{
		ID:             generateID(),
		OrganizationID: orgID,
		Tier:           tier,
		MaxSessions:    limit.sessions,
		RateLimiter:    rate.NewLimiter(limit.rps, int(limit.rps)*2),
		CreatedAt:      time.Now(),
	}

	m.tenants.Store(orgID, tenant)

	logger.Info("Created tenant",
		zap.String("org_id", orgID),
		zap.String("tier", tier))

	return tenant, nil
}

// GetTenant retrieves a tenant
func (m *Manager) GetTenant(orgID string) (*Tenant, error) {
	if val, ok := m.tenants.Load(orgID); ok {
		return val.(*Tenant), nil
	}
	return nil, fmt.Errorf("tenant not found")
}

// AllocateSession allocates a session for a tenant
func (m *Manager) AllocateSession(ctx context.Context, orgID string) error {
	tenant, err := m.GetTenant(orgID)
	if err != nil {
		return err
	}

	// Check rate limit
	if !tenant.RateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	// Check session limit
	tenant.mu.Lock()
	defer tenant.mu.Unlock()

	if tenant.CurrentSessions >= tenant.MaxSessions {
		return fmt.Errorf("session limit exceeded")
	}

	tenant.CurrentSessions++
	return nil
}

// ReleaseSession releases a session
func (m *Manager) ReleaseSession(orgID string) {
	if val, ok := m.tenants.Load(orgID); ok {
		tenant := val.(*Tenant)
		tenant.mu.Lock()
		if tenant.CurrentSessions > 0 {
			tenant.CurrentSessions--
		}
		tenant.mu.Unlock()
	}
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}