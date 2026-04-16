package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// AdminEtherService manages the Ether AI model catalog at runtime.
//
// Provides CRUD for models and tiers, plus hot-reload of the catalog cache.
// Routes: /admin/ether/*.
//
// Requires: admin role (super_admin, platform_admin).
type AdminEtherService struct {
	http *httpClient
}

// ---------------------------------------------------------------------------
// Model CRUD
// ---------------------------------------------------------------------------

// CreateModel registers a new AI model in the Ether catalog.
func (s *AdminEtherService) CreateModel(ctx context.Context, model map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/admin/ether/models", model)
}

// UpdateModel updates an existing model's configuration.
func (s *AdminEtherService) UpdateModel(ctx context.Context, modelID string, updates map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, fmt.Sprintf("/admin/ether/models/%s", modelID), updates)
}

// DeleteModel removes a model from the catalog.
func (s *AdminEtherService) DeleteModel(ctx context.Context, modelID string) error {
	return s.http.del(ctx, fmt.Sprintf("/admin/ether/models/%s", modelID))
}

// ListModels lists models in the catalog, optionally filtered by tier or provider.
func (s *AdminEtherService) ListModels(ctx context.Context, tier, provider string) ([]map[string]interface{}, error) {
	q := url.Values{}
	if tier != "" {
		q.Set("tier", tier)
	}
	if provider != "" {
		q.Set("provider", provider)
	}
	resp, err := s.http.get(ctx, "/admin/ether/models", q)
	if err != nil {
		return nil, err
	}
	return extractList(resp, "models"), nil
}

// ---------------------------------------------------------------------------
// Tier Management
// ---------------------------------------------------------------------------

// ListTiers lists all Ether tiers (T1-T6) with current configuration.
func (s *AdminEtherService) ListTiers(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := s.http.get(ctx, "/admin/ether/tiers", nil)
	if err != nil {
		return nil, err
	}
	return extractList(resp, "tiers"), nil
}

// UpdateTier updates a tier's configuration (e.g. default model, rate limits).
func (s *AdminEtherService) UpdateTier(ctx context.Context, tierNumber int, updates map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, fmt.Sprintf("/admin/ether/tiers/%d", tierNumber), updates)
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

// ReloadCatalog forces a hot-reload of the model catalog from the backing store.
func (s *AdminEtherService) ReloadCatalog(ctx context.Context) error {
	_, err := s.http.post(ctx, "/admin/ether/catalog/reload", nil)
	return err
}

// extractList pulls a list of maps from a response under the given key,
// falling back to "data" if the primary key is absent.
func extractList(resp map[string]interface{}, key string) []map[string]interface{} {
	var raw []interface{}
	if items, ok := resp[key].([]interface{}); ok {
		raw = items
	} else if items, ok := resp["data"].([]interface{}); ok {
		raw = items
	}
	result := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}
