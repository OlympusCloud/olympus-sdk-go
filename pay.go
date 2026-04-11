package olympus

import (
	"context"
	"fmt"
	"net/url"
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
