package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// AdminBillingService manages the global billing plan catalog, add-ons,
// minute packs, and usage metering.
//
// Distinct from BillingService which is the tenant-facing billing API.
// Routes: /admin/billing/*.
//
// Requires: admin role (super_admin, platform_admin).
type AdminBillingService struct {
	http *httpClient
}

// ---------------------------------------------------------------------------
// Plan CRUD
// ---------------------------------------------------------------------------

// CreatePlan creates a new billing plan in the catalog.
func (s *AdminBillingService) CreatePlan(ctx context.Context, plan map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/admin/billing/plans", plan)
}

// UpdatePlan updates an existing billing plan.
func (s *AdminBillingService) UpdatePlan(ctx context.Context, planID string, updates map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, fmt.Sprintf("/admin/billing/plans/%s", planID), updates)
}

// DeletePlan deletes a billing plan. Fails if tenants are actively subscribed.
func (s *AdminBillingService) DeletePlan(ctx context.Context, planID string) error {
	return s.http.del(ctx, fmt.Sprintf("/admin/billing/plans/%s", planID))
}

// ListPlans lists all billing plans in the catalog.
func (s *AdminBillingService) ListPlans(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := s.http.get(ctx, "/admin/billing/plans", nil)
	if err != nil {
		return nil, err
	}
	return extractList(resp, "plans"), nil
}

// ---------------------------------------------------------------------------
// Add-ons & Minute Packs
// ---------------------------------------------------------------------------

// CreateAddon creates a purchasable add-on (e.g. extra SMS bundle, premium support).
func (s *AdminBillingService) CreateAddon(ctx context.Context, addon map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/admin/billing/addons", addon)
}

// CreateMinutePack creates a minute pack (pre-paid voice minutes bundle).
func (s *AdminBillingService) CreateMinutePack(ctx context.Context, pack map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/admin/billing/minute-packs", pack)
}

// ---------------------------------------------------------------------------
// Usage Metering
// ---------------------------------------------------------------------------

// GetUsage returns usage data for a tenant, optionally filtered by meter type.
func (s *AdminBillingService) GetUsage(ctx context.Context, tenantID string, meterType string) (map[string]interface{}, error) {
	q := url.Values{}
	if meterType != "" {
		q.Set("meter_type", meterType)
	}
	return s.http.get(ctx, fmt.Sprintf("/admin/billing/usage/%s", tenantID), q)
}

// RecordUsage records a usage event for a tenant's meter.
func (s *AdminBillingService) RecordUsage(ctx context.Context, tenantID, meterType string, quantity float64) error {
	_, err := s.http.post(ctx, fmt.Sprintf("/admin/billing/usage/%s", tenantID), map[string]interface{}{
		"meter_type": meterType,
		"quantity":   quantity,
	})
	return err
}
