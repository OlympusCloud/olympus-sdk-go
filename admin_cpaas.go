package olympus

import "context"

// AdminCpaasService manages CPaaS provider configuration and health.
//
// Controls the Telnyx-primary / Twilio-fallback routing layer, provider
// preferences per scope (tenant, brand, location), and circuit-breaker health.
// Routes: /admin/cpaas/*.
//
// Requires: admin role (super_admin, platform_admin).
type AdminCpaasService struct {
	http *httpClient
}

// SetProviderPreference sets the preferred CPaaS provider for a given scope.
//
// scope is one of "tenant", "brand", or "location".
// scopeID is the ID of the scoped entity.
// provider is "telnyx" or "twilio".
func (s *AdminCpaasService) SetProviderPreference(ctx context.Context, scope, scopeID, provider string) (map[string]interface{}, error) {
	return s.http.put(ctx, "/admin/cpaas/provider-preference", map[string]interface{}{
		"scope":     scope,
		"scope_id":  scopeID,
		"provider":  provider,
	})
}

// GetProviderHealth returns the current health status of all CPaaS providers,
// including circuit-breaker state, latency, and failure counts.
func (s *AdminCpaasService) GetProviderHealth(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/admin/cpaas/health", nil)
}
