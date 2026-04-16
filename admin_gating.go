package olympus

import (
	"context"
	"fmt"
)

// AdminGatingService manages feature flags, plan-level feature assignment,
// resource limits, and evaluation.
//
// Distinct from GatingService which is the tenant-facing policy evaluation API.
// Routes: /admin/gating/*.
//
// Requires: admin role (super_admin, platform_admin).
type AdminGatingService struct {
	http *httpClient
}

// ---------------------------------------------------------------------------
// Feature Definitions
// ---------------------------------------------------------------------------

// DefineFeature defines a new feature flag.
func (s *AdminGatingService) DefineFeature(ctx context.Context, key string, description string, enabled bool) (map[string]interface{}, error) {
	return s.http.post(ctx, "/admin/gating/features", map[string]interface{}{
		"key":         key,
		"description": description,
		"enabled":     enabled,
	})
}

// UpdateFeature updates an existing feature flag.
func (s *AdminGatingService) UpdateFeature(ctx context.Context, key string, updates map[string]interface{}) error {
	_, err := s.http.put(ctx, fmt.Sprintf("/admin/gating/features/%s", key), updates)
	return err
}

// ListFeatures lists all defined feature flags.
func (s *AdminGatingService) ListFeatures(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := s.http.get(ctx, "/admin/gating/features", nil)
	if err != nil {
		return nil, err
	}
	return extractList(resp, "features"), nil
}

// ---------------------------------------------------------------------------
// Plan-Level Feature Assignment
// ---------------------------------------------------------------------------

// SetPlanFeatures sets the list of feature keys enabled for a billing plan.
func (s *AdminGatingService) SetPlanFeatures(ctx context.Context, planID string, featureKeys []string) error {
	_, err := s.http.put(ctx, fmt.Sprintf("/admin/gating/plans/%s/features", planID), map[string]interface{}{
		"feature_keys": featureKeys,
	})
	return err
}

// GetPlanFeatures returns the features assigned to a billing plan.
func (s *AdminGatingService) GetPlanFeatures(ctx context.Context, planID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/admin/gating/plans/%s/features", planID), nil)
}

// ---------------------------------------------------------------------------
// Resource Limits
// ---------------------------------------------------------------------------

// SetResourceLimit sets a resource limit for a billing plan (e.g. max_agents, max_voice_min).
func (s *AdminGatingService) SetResourceLimit(ctx context.Context, planID, resource string, limit int) error {
	_, err := s.http.put(ctx, fmt.Sprintf("/admin/gating/plans/%s/limits/%s", planID, resource), map[string]interface{}{
		"limit": limit,
	})
	return err
}

// ---------------------------------------------------------------------------
// Evaluation
// ---------------------------------------------------------------------------

// EvaluateFeature evaluates a feature flag with optional tenant/user context.
func (s *AdminGatingService) EvaluateFeature(ctx context.Context, featureKey string, tenantID, userID string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"feature_key": featureKey,
	}
	if tenantID != "" {
		body["tenant_id"] = tenantID
	}
	if userID != "" {
		body["user_id"] = userID
	}
	return s.http.post(ctx, "/admin/gating/evaluate", body)
}
