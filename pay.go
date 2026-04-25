package olympus

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// PayService handles payment processing, refunds, balance, payouts, and terminal management.
//
// Wraps the Olympus Payment Orchestration service via the Go API Gateway.
// Routes: /payments/*, /finance/*, /stripe/terminal/*.
type PayService struct {
	http *httpClient
}

// Charge creates a payment intent for an order using the given payment method.
// Amount is in cents. Method is a payment method token or ID (e.g., a Stripe
// payment method ID or "cash").
func (s *PayService) Charge(ctx context.Context, orderID string, amount int, method string) (*Payment, error) {
	resp, err := s.http.post(ctx, "/payments/intents", map[string]interface{}{
		"order_id":       orderID,
		"amount":         amount,
		"payment_method": method,
	})
	if err != nil {
		return nil, err
	}
	return parsePayment(resp), nil
}

// Capture captures a previously authorized payment.
func (s *PayService) Capture(ctx context.Context, paymentID string) (*Payment, error) {
	resp, err := s.http.post(ctx, fmt.Sprintf("/payments/%s/capture", paymentID), nil)
	if err != nil {
		return nil, err
	}
	return parsePayment(resp), nil
}

// RefundOptions holds optional parameters for a refund.
type RefundOptions struct {
	Amount int    // Partial amount in cents; 0 = full refund.
	Reason string // Reason for the refund.
}

// Refund issues a refund against a payment.
func (s *PayService) Refund(ctx context.Context, paymentID string, opts *RefundOptions) (*Refund, error) {
	body := map[string]interface{}{}
	if opts != nil {
		if opts.Amount > 0 {
			body["amount"] = opts.Amount
		}
		if opts.Reason != "" {
			body["reason"] = opts.Reason
		}
	}

	resp, err := s.http.post(ctx, fmt.Sprintf("/payments/%s/refund", paymentID), body)
	if err != nil {
		return nil, err
	}
	return parseRefund(resp), nil
}

// GetBalance returns the current account balance.
func (s *PayService) GetBalance(ctx context.Context) (*Balance, error) {
	resp, err := s.http.get(ctx, "/finance/balance", nil)
	if err != nil {
		return nil, err
	}
	return parseBalance(resp), nil
}

// CreatePayoutRequest holds the parameters for creating a payout.
type CreatePayoutRequest struct {
	Amount      int    `json:"amount"` // Amount in cents.
	Destination string `json:"destination"`
	Currency    string `json:"currency,omitempty"`
	Method      string `json:"method,omitempty"` // "standard" or "instant"
	Description string `json:"description,omitempty"`
}

// CreatePayout initiates a payout to an external destination.
func (s *PayService) CreatePayout(ctx context.Context, req CreatePayoutRequest) (*Payout, error) {
	body := map[string]interface{}{
		"amount":      req.Amount,
		"destination": req.Destination,
	}
	if req.Currency != "" {
		body["currency"] = req.Currency
	}
	if req.Method != "" {
		body["method"] = req.Method
	}
	if req.Description != "" {
		body["description"] = req.Description
	}

	resp, err := s.http.post(ctx, "/finance/payouts", body)
	if err != nil {
		return nil, err
	}
	return parsePayout(resp), nil
}

// ListPaymentsOptions holds optional filters for listing payments.
type ListPaymentsOptions struct {
	Page   int
	Limit  int
	Status string
}

// ListPayments lists recent payments for the tenant.
func (s *PayService) ListPayments(ctx context.Context, opts *ListPaymentsOptions) ([]Payment, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			q.Set("page", fmt.Sprintf("%d", opts.Page))
		}
		if opts.Limit > 0 {
			q.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		if opts.Status != "" {
			q.Set("status", opts.Status)
		}
	}

	resp, err := s.http.get(ctx, "/payments", q)
	if err != nil {
		return nil, err
	}

	payments := parseSlice(resp, "payments", parsePayment)
	if len(payments) == 0 {
		payments = parseSlice(resp, "data", parsePayment)
	}
	return payments, nil
}

// CreateTerminalReaderRequest holds the parameters for registering a card reader.
type CreateTerminalReaderRequest struct {
	LocationID       string `json:"location_id"`
	RegistrationCode string `json:"registration_code"`
	Label            string `json:"label,omitempty"`
}

// CreateTerminalReader registers a physical card reader.
func (s *PayService) CreateTerminalReader(ctx context.Context, req CreateTerminalReaderRequest) (*TerminalReader, error) {
	body := map[string]interface{}{
		"location_id":       req.LocationID,
		"registration_code": req.RegistrationCode,
	}
	if req.Label != "" {
		body["label"] = req.Label
	}

	resp, err := s.http.post(ctx, "/stripe/terminal/readers", body)
	if err != nil {
		return nil, err
	}
	return parseTerminalReader(resp), nil
}

// CaptureTerminalPayment creates a PaymentIntent and presents it to a terminal reader.
func (s *PayService) CaptureTerminalPayment(ctx context.Context, readerID string, amount int, currency, description string) (*TerminalPayment, error) {
	body := map[string]interface{}{
		"amount": amount,
	}
	if currency != "" {
		body["currency"] = currency
	}
	if description != "" {
		body["description"] = description
	}

	resp, err := s.http.post(ctx, fmt.Sprintf("/stripe/terminal/readers/%s/process", readerID), body)
	if err != nil {
		return nil, err
	}
	return parseTerminalPayment(resp), nil
}

// ----------------------------------------------------------------------------
// Per-location payment routing (#3312)
// ----------------------------------------------------------------------------

// Supported payment processors. Both PreferredProcessor and the entries in
// FallbackProcessors must be one of these — anything else is rejected
// server-side with a 400. Mirrors the `SUPPORTED_PROCESSORS` slice in
// backend/rust/platform/src/handlers/payment_routing.rs.
const (
	PaymentProcessorOlympusPay = "olympus_pay"
	PaymentProcessorSquare     = "square"
	PaymentProcessorAdyen      = "adyen"
	PaymentProcessorWorldpay   = "worldpay"
)

// ConfigureRoutingParams controls ConfigureRouting.
//
// PreferredProcessor and FallbackProcessors entries must each be one of
// the PaymentProcessor* constants. The fallback chain cannot include the
// preferred processor.
//
// CredentialsSecretRef must be a Secret Manager secret NAME (not the
// credential itself) starting with `olympus-merchant-credentials-` per
// the canonical secrets schema (#2900). Plaintext API keys are rejected
// at the server.
//
// IsActiveSet/IsActive lets callers explicitly send `is_active=false`.
// When IsActiveSet is false, the field is omitted and the server default
// (true) applies.
type ConfigureRoutingParams struct {
	LocationID            string
	PreferredProcessor    string
	FallbackProcessors    []string
	CredentialsSecretRef  string
	MerchantID            string
	IsActive              bool
	IsActiveSet           bool
	Notes                 string
}

// GetRoutingParams controls GetRouting.
type GetRoutingParams struct {
	LocationID string
}

// RoutingConfig is the per-location processor routing config (#3312).
//
// Returned by both ConfigureRouting (echo + tenant_id) and GetRouting
// (full row including server commit timestamps).
type RoutingConfig struct {
	TenantID             string     `json:"tenant_id"`
	LocationID           string     `json:"location_id"`
	PreferredProcessor   string     `json:"preferred_processor"`
	FallbackProcessors   []string   `json:"fallback_processors"`
	CredentialsSecretRef *string    `json:"credentials_secret_ref"`
	MerchantID           *string    `json:"merchant_id"`
	IsActive             bool       `json:"is_active"`
	Notes                *string    `json:"notes"`
	CreatedAt            *time.Time `json:"created_at"`
	UpdatedAt            *time.Time `json:"updated_at"`
}

// ConfigureRouting upserts the per-location processor routing config (#3312).
func (s *PayService) ConfigureRouting(ctx context.Context, p ConfigureRoutingParams) (*RoutingConfig, error) {
	body := map[string]interface{}{
		"location_id":         p.LocationID,
		"preferred_processor": p.PreferredProcessor,
	}
	// Always send fallback_processors (server defaults to []) — pass the
	// caller's slice as-is so an empty list explicitly clears the chain.
	if p.FallbackProcessors != nil {
		body["fallback_processors"] = p.FallbackProcessors
	} else {
		body["fallback_processors"] = []string{}
	}
	if p.CredentialsSecretRef != "" {
		body["credentials_secret_ref"] = p.CredentialsSecretRef
	}
	if p.MerchantID != "" {
		body["merchant_id"] = p.MerchantID
	}
	if p.IsActiveSet {
		body["is_active"] = p.IsActive
	}
	if p.Notes != "" {
		body["notes"] = p.Notes
	}

	raw, err := s.http.post(ctx, "/platform/pay/routing", body)
	if err != nil {
		return nil, err
	}
	var out RoutingConfig
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRouting reads the current routing config for a location (#3312).
//
// The location_id path segment is URL-escaped, so values containing
// slashes or other reserved characters are safe to pass as-is.
func (s *PayService) GetRouting(ctx context.Context, p GetRoutingParams) (*RoutingConfig, error) {
	path := fmt.Sprintf("/platform/pay/routing/%s", url.PathEscape(p.LocationID))
	raw, err := s.http.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out RoutingConfig
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListRoutingParams controls ListRouting (#3312 pt2 → gcp PR #3537).
//
// All filters optional. IsActiveSet/IsActive lets callers explicitly
// filter to is_active=false (without IsActiveSet, the field is omitted
// and the server returns both active + inactive). Processor must be
// one of the PaymentProcessor* constants when set; the server rejects
// anything else with HTTP 400. Limit is clamped to 1..=200 server-side
// and defaults to 100 when omitted (Limit=0 is treated as omitted).
type ListRoutingParams struct {
	IsActive    bool
	IsActiveSet bool
	Processor   string
	Limit       int
}

// RoutingConfigList is the response from ListRouting (#3312 pt2).
//
// TotalReturned is the count of configs the server actually returned;
// compare against the requested Limit to detect a capped response.
type RoutingConfigList struct {
	Configs       []RoutingConfig `json:"configs"`
	TotalReturned int             `json:"total_returned"`
}

// ListRouting lists every routing config for the caller's tenant (#3312 pt2).
//
// Optional filters: IsActiveSet/IsActive, Processor (must be one of
// olympus_pay/square/adyen/worldpay), Limit (1..=200). Server returns
// configs ordered by location_id; pagination by location_id is on the
// roadmap if any tenant exceeds 200 active locations.
func (s *PayService) ListRouting(ctx context.Context, p ListRoutingParams) (*RoutingConfigList, error) {
	q := url.Values{}
	if p.IsActiveSet {
		if p.IsActive {
			q.Set("is_active", "true")
		} else {
			q.Set("is_active", "false")
		}
	}
	if p.Processor != "" {
		q.Set("processor", p.Processor)
	}
	if p.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", p.Limit))
	}
	raw, err := s.http.get(ctx, "/platform/pay/routing", q)
	if err != nil {
		return nil, err
	}
	var out RoutingConfigList
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
