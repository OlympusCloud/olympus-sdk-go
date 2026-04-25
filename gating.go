package olympus

import (
	"context"
	"net/url"
)

// GatingService handles feature gating and policy evaluation.
//
// Wraps the Olympus Gating Engine (10-level policy hierarchy) via the Go API Gateway.
// Routes: /policies/evaluate, /gating/*, /feature-flags/*.
type GatingService struct {
	http *httpClient
}

// IsEnabled checks whether a feature key is enabled for the current context.
func (s *GatingService) IsEnabled(ctx context.Context, featureKey string) (bool, error) {
	resp, err := s.http.post(ctx, "/policies/evaluate", map[string]interface{}{
		"policy_key": featureKey,
	})
	if err != nil {
		return false, err
	}

	if v, ok := resp["allowed"].(bool); ok {
		return v, nil
	}
	if v, ok := resp["enabled"].(bool); ok {
		return v, nil
	}
	if resp["value"] == true {
		return true, nil
	}
	return false, nil
}

// GetPolicy returns the raw policy value for a key. The returned value may be
// a bool, float64, string, or map depending on the policy definition.
func (s *GatingService) GetPolicy(ctx context.Context, policyKey string) (interface{}, error) {
	resp, err := s.http.post(ctx, "/policies/evaluate", map[string]interface{}{
		"policy_key": policyKey,
	})
	if err != nil {
		return nil, err
	}

	if v, ok := resp["value"]; ok {
		return v, nil
	}
	return resp["result"], nil
}

// Evaluate evaluates a policy key with additional context parameters.
func (s *GatingService) Evaluate(ctx context.Context, policyKey string, evalCtx map[string]interface{}) (*PolicyResult, error) {
	resp, err := s.http.post(ctx, "/policies/evaluate", map[string]interface{}{
		"policy_key": policyKey,
		"context":    evalCtx,
	})
	if err != nil {
		return nil, err
	}
	return parsePolicyResult(resp), nil
}

// EvaluateBatch evaluates multiple policy keys at once.
func (s *GatingService) EvaluateBatch(ctx context.Context, policyKeys []string, evalCtx map[string]interface{}) (map[string]*PolicyResult, error) {
	body := map[string]interface{}{
		"policy_keys": policyKeys,
	}
	if evalCtx != nil {
		body["context"] = evalCtx
	}

	resp, err := s.http.post(ctx, "/policies/evaluate/batch", body)
	if err != nil {
		return nil, err
	}

	resultsRaw, ok := resp["results"].(map[string]interface{})
	if !ok {
		return map[string]*PolicyResult{}, nil
	}

	results := make(map[string]*PolicyResult, len(resultsRaw))
	for key, val := range resultsRaw {
		if m, ok := val.(map[string]interface{}); ok {
			results[key] = parsePolicyResult(m)
		}
	}
	return results, nil
}

// ListFeatureFlags lists feature flags for the tenant.
func (s *GatingService) ListFeatureFlags(ctx context.Context) ([]FeatureFlag, error) {
	resp, err := s.http.get(ctx, "/feature-flags", nil)
	if err != nil {
		return nil, err
	}

	return parseSlice(resp, "feature_flags", parseFeatureFlag), nil
}

// PlanEntry is a single tier in the plan matrix returned by
// GET /platform/gating/plan-details (#3313).
type PlanEntry struct {
	TierID                 string      `json:"tier_id"`
	DisplayName            string      `json:"display_name"`
	MonthlyPriceUSD        *float64    `json:"monthly_price_usd"`
	Features               interface{} `json:"features"`
	UsageLimits            interface{} `json:"usage_limits"`
	RanksHigherThanCurrent bool        `json:"ranks_higher_than_current"`
	IsCurrent              bool        `json:"is_current"`
	DiffVsCurrent          []string    `json:"diff_vs_current"`
	ContactSales           bool        `json:"contact_sales"`
}

// PlanDetails is the response shape for GET /platform/gating/plan-details (#3313).
type PlanDetails struct {
	CurrentPlan *string     `json:"current_plan"`
	Plans       []PlanEntry `json:"plans"`
	AsOf        string      `json:"as_of"`
}

// GetPlanDetailsParams controls GetPlanDetails. TenantID is optional —
// when empty, the platform uses the JWT's tenant. Cross-tenant lookup
// requires tenant_admin or higher.
type GetPlanDetailsParams struct {
	TenantID string
}

// GetPlanDetails fetches the full plan matrix + caller's CurrentPlan +
// per-tier DiffVsCurrent for a contextual upgrade UI (#3313).
func (s *GatingService) GetPlanDetails(ctx context.Context, p GetPlanDetailsParams) (*PlanDetails, error) {
	q := url.Values{}
	if p.TenantID != "" {
		q.Set("tenant_id", p.TenantID)
	}
	raw, err := s.http.get(ctx, "/platform/gating/plan-details", q)
	if err != nil {
		return nil, err
	}
	var details PlanDetails
	if err := remarshal(raw, &details); err != nil {
		return nil, err
	}
	return &details, nil
}
