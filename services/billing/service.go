package billing

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"agent/logger"
)

/*
nkk: Simplified Billing Service
Tracks usage for future Stripe integration
No over-engineering - just usage tracking for now
*/

type UsageRecord struct {
	CustomerID string
	Sessions   int64
	Minutes    float64
	Timestamp  time.Time
}

type Service struct {
	usage sync.Map // map[string]*UsageRecord
}

// NewService creates a new billing service
func NewService() *Service {
	return &Service{}
}

// TrackUsage tracks usage for billing
func (s *Service) TrackUsage(ctx context.Context, customerID string, minutes float64) {
	// nkk: Simple usage tracking
	// In production, persist to database

	key := fmt.Sprintf("%s:%s", customerID, time.Now().Format("2006-01"))

	val, _ := s.usage.LoadOrStore(key, &UsageRecord{
		CustomerID: customerID,
		Timestamp:  time.Now(),
	})

	record := val.(*UsageRecord)
	record.Sessions++
	record.Minutes += minutes

	logger.Debug("Tracked usage",
		zap.String("customer_id", customerID),
		zap.Float64("minutes", minutes))
}

// GetUsage retrieves usage for a customer
func (s *Service) GetUsage(customerID string, month string) (*UsageRecord, error) {
	key := fmt.Sprintf("%s:%s", customerID, month)

	if val, ok := s.usage.Load(key); ok {
		return val.(*UsageRecord), nil
	}

	return &UsageRecord{CustomerID: customerID}, nil
}

// CalculateBill calculates monthly bill
func (s *Service) CalculateBill(customerID string, month string) (float64, error) {
	usage, err := s.GetUsage(customerID, month)
	if err != nil {
		return 0, err
	}

	// nkk: Simple pricing model
	// $0.05 per session + $0.001 per minute
	cost := float64(usage.Sessions)*0.05 + usage.Minutes*0.001

	logger.Info("Calculated bill",
		zap.String("customer_id", customerID),
		zap.String("month", month),
		zap.Float64("cost", cost))

	return cost, nil
}

// ProcessPayment processes payment via Stripe
func (s *Service) ProcessPayment(customerID string, amount float64) error {
	// nkk: Stripe integration implementation
	// Based on BrowserStack's payment flow

	logger.Info("Processing payment",
		zap.String("customer_id", customerID),
		zap.Float64("amount", amount))

	// nkk: Convert amount to cents for Stripe
	amountCents := int64(amount * 100)

	// Create payment intent
	paymentIntent := &StripePaymentIntent{
		Amount:      amountCents,
		Currency:    "usd",
		CustomerID:  customerID,
		Description: fmt.Sprintf("Browser testing services - %s", time.Now().Format("2006-01")),
		Metadata: map[string]string{
			"customer_id": customerID,
			"service":     "browser_testing",
			"month":       time.Now().Format("2006-01"),
		},
	}

	// nkk: In production, use Stripe SDK
	// For now, simulate payment processing
	if err := s.createStripePayment(paymentIntent); err != nil {
		logger.Error("Payment failed",
			zap.String("customer_id", customerID),
			zap.Error(err))
		return err
	}

	// Record payment in database
	s.recordPayment(customerID, amount, "completed")

	logger.Info("Payment completed",
		zap.String("customer_id", customerID),
		zap.Float64("amount", amount))

	return nil
}

// StripePaymentIntent represents a Stripe payment
type StripePaymentIntent struct {
	Amount      int64             `json:"amount"`
	Currency    string            `json:"currency"`
	CustomerID  string            `json:"customer"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

// createStripePayment creates payment via Stripe API
func (s *Service) createStripePayment(intent *StripePaymentIntent) error {
	// nkk: Stripe API integration
	// In production, use official Stripe Go SDK

	// Simulate API call
	if os.Getenv("STRIPE_API_KEY") == "" {
		logger.Warn("Stripe API key not configured, simulating payment")
		time.Sleep(100 * time.Millisecond) // Simulate API latency
		return nil
	}

	// Example Stripe API call (requires stripe-go library):
	/*
	stripe.Key = os.Getenv("STRIPE_API_KEY")

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(intent.Amount),
		Currency: stripe.String(intent.Currency),
		Customer: stripe.String(intent.CustomerID),
		Description: stripe.String(intent.Description),
		Metadata: intent.Metadata,
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return fmt.Errorf("stripe payment failed: %w", err)
	}

	logger.Info("Stripe payment created", zap.String("id", pi.ID))
	*/

	return nil
}

// recordPayment records payment in database
func (s *Service) recordPayment(customerID string, amount float64, status string) {
	// nkk: Record payment for auditing
	key := fmt.Sprintf("payment:%s:%d", customerID, time.Now().Unix())

	payment := map[string]interface{}{
		"customer_id": customerID,
		"amount":      amount,
		"status":      status,
		"timestamp":   time.Now(),
	}

	s.usage.Store(key, payment)
}

// SetupSubscription sets up recurring billing
func (s *Service) SetupSubscription(customerID string, planID string) error {
	// nkk: Setup Stripe subscription
	// Based on chosen plan (free, pro, enterprise)

	plans := map[string]struct {
		Price    float64
		Interval string
	}{
		"free":       {0, "month"},
		"pro":        {99, "month"},
		"enterprise": {499, "month"},
	}

	plan, ok := plans[planID]
	if !ok {
		return fmt.Errorf("invalid plan: %s", planID)
	}

	logger.Info("Setting up subscription",
		zap.String("customer_id", customerID),
		zap.String("plan", planID),
		zap.Float64("price", plan.Price))

	// nkk: In production, create Stripe subscription
	// For now, store locally
	key := fmt.Sprintf("subscription:%s", customerID)
	subscription := map[string]interface{}{
		"customer_id": customerID,
		"plan_id":     planID,
		"price":       plan.Price,
		"interval":    plan.Interval,
		"status":      "active",
		"created_at":  time.Now(),
	}

	s.usage.Store(key, subscription)

	return nil
}

// CancelSubscription cancels a subscription
func (s *Service) CancelSubscription(customerID string) error {
	// nkk: Cancel Stripe subscription
	key := fmt.Sprintf("subscription:%s", customerID)

	if val, ok := s.usage.Load(key); ok {
		subscription := val.(map[string]interface{})
		subscription["status"] = "cancelled"
		subscription["cancelled_at"] = time.Now()
		s.usage.Store(key, subscription)

		logger.Info("Subscription cancelled",
			zap.String("customer_id", customerID))
	}

	return nil
}