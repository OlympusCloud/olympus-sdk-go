package olympus

import "context"

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
