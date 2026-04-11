package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// BillingService handles subscription billing, usage metering, invoices, and plan management.
//
// Routes: /billing/*, /platform/billing/*.
type BillingService struct {
	http *httpClient
}

// GetCurrentPlan returns the current subscription plan for the tenant.
func (s *BillingService) GetCurrentPlan(ctx context.Context) (*Plan, error) {
	resp, err := s.http.get(ctx, "/billing/subscription", nil)
	if err != nil {
		return nil, err
	}
	return parsePlan(resp), nil
}

// GetUsage returns resource usage for the current billing period.
func (s *BillingService) GetUsage(ctx context.Context, period string) (*UsageReport, error) {
	q := url.Values{}
	if period != "" {
		q.Set("period", period)
	}

	resp, err := s.http.get(ctx, "/billing/stats", q)
	if err != nil {
		return nil, err
	}
	return parseUsageReport(resp), nil
}

// GetInvoices lists invoices for the tenant.
func (s *BillingService) GetInvoices(ctx context.Context) ([]Invoice, error) {
	resp, err := s.http.get(ctx, "/billing/invoices", nil)
	if err != nil {
		return nil, err
	}

	invoices := parseSlice(resp, "invoices", parseInvoice)
	if len(invoices) == 0 {
		invoices = parseSlice(resp, "data", parseInvoice)
	}
	return invoices, nil
}

// GetInvoice returns a single invoice by ID.
func (s *BillingService) GetInvoice(ctx context.Context, invoiceID string) (*Invoice, error) {
	resp, err := s.http.get(ctx, fmt.Sprintf("/billing/invoices/%s", invoiceID), nil)
	if err != nil {
		return nil, err
	}
	return parseInvoice(resp), nil
}

// GetInvoicePDF returns the download URL for an invoice PDF.
func (s *BillingService) GetInvoicePDF(ctx context.Context, invoiceID string) (string, error) {
	resp, err := s.http.get(ctx, fmt.Sprintf("/billing/invoices/%s/pdf", invoiceID), nil)
	if err != nil {
		return "", err
	}

	if v := getString(resp, "url"); v != "" {
		return v, nil
	}
	return getString(resp, "pdf_url"), nil
}

// UpgradePlan changes the subscription to a different plan.
func (s *BillingService) UpgradePlan(ctx context.Context, planID string) (*Plan, error) {
	resp, err := s.http.put(ctx, "/billing/subscription/plan", map[string]interface{}{
		"plan_id": planID,
	})
	if err != nil {
		return nil, err
	}
	return parsePlan(resp), nil
}

// ListPlans lists all available billing plans.
func (s *BillingService) ListPlans(ctx context.Context) ([]Plan, error) {
	resp, err := s.http.get(ctx, "/platform/billing/plans", nil)
	if err != nil {
		return nil, err
	}

	plans := parseSlice(resp, "plans", parsePlan)
	if len(plans) == 0 {
		plans = parseSlice(resp, "data", parsePlan)
	}
	return plans, nil
}

// AddPaymentMethod adds a payment method.
func (s *BillingService) AddPaymentMethod(ctx context.Context, method map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/billing/payment-methods", method)
}

// RemovePaymentMethod removes a payment method.
func (s *BillingService) RemovePaymentMethod(ctx context.Context, methodID string) error {
	return s.http.del(ctx, fmt.Sprintf("/billing/payment-methods/%s", methodID))
}
