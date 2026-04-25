package olympus

import (
	"context"
	"net/url"
)

// PlatformService wraps the Rust Platform backend (port 8002) for tenant
// lifecycle: signup, cleanup, onboarding progress.
//
// v0.3.0 — Issues #2845, #2846
type PlatformService struct {
	http *httpClient
}

// SignupRequest represents an automated tenant signup.
type SignupRequest struct {
	CompanyName string `json:"company_name"`
	AdminEmail  string `json:"admin_email"`
	AdminName   string `json:"admin_name"`
	Industry    string `json:"industry"`
	TrialDays   int    `json:"trial_days,omitempty"`
}

// Signup executes the automated tenant signup workflow.
func (s *PlatformService) Signup(ctx context.Context, req SignupRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"company_name": req.CompanyName,
		"admin_email":  req.AdminEmail,
		"admin_name":   req.AdminName,
		"industry":     req.Industry,
	}
	if req.TrialDays > 0 {
		body["trial_days"] = req.TrialDays
	} else {
		body["trial_days"] = 14
	}
	return s.http.post(ctx, "/platform/signup", body)
}

// CleanupRequest represents an automated tenant offboarding.
type CleanupRequest struct {
	TenantID        string `json:"tenant_id"`
	Reason          string `json:"reason"`
	ExportData      bool   `json:"export_data"`
	GracePeriodDays int    `json:"grace_period_days,omitempty"`
}

// Cleanup executes the automated tenant cleanup workflow.
func (s *PlatformService) Cleanup(ctx context.Context, req CleanupRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"tenant_id":   req.TenantID,
		"reason":      req.Reason,
		"export_data": req.ExportData,
	}
	if req.GracePeriodDays > 0 {
		body["grace_period_days"] = req.GracePeriodDays
	} else {
		body["grace_period_days"] = 30
	}
	return s.http.post(ctx, "/platform/cleanup", body)
}

// GetTenantStatus returns the lifecycle status of a tenant.
func (s *PlatformService) GetTenantStatus(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/platform/tenants/"+tenantID+"/lifecycle/status", nil)
}

// GetTenantHealth returns the health score of a tenant.
func (s *PlatformService) GetTenantHealth(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/platform/tenants/"+tenantID+"/lifecycle/health", nil)
}

// GetOnboardingProgress returns the onboarding progress for a tenant.
func (s *PlatformService) GetOnboardingProgress(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/platform/tenants/"+tenantID+"/lifecycle/onboarding", nil)
}


// ---------------------------------------------------------------------------
// Scope registry (gcp#3236 / PR #3517)
// ---------------------------------------------------------------------------

// ListScopeRegistryParams controls ListScopeRegistry.
//
// All filters optional. OwnerAppIDSet allows callers to send an empty
// OwnerAppID — the server treats that as the explicit "platform-owned
// only" filter (semantically distinct from omitted/no filter). When
// OwnerAppIDSet is false, OwnerAppID is omitted from the query string.
type ListScopeRegistryParams struct {
	Namespace      string
	OwnerAppID     string
	OwnerAppIDSet  bool
	IncludeDrafts  bool
}

// ScopeRow is one row of the platform scope registry (#3517).
//
// BitID is *int64 because it's nil when the scope hasn't been allocated
// a bit yet (workshop_status pre-`service_ok`). Pre-allocation rows can
// still appear in authoring views.
type ScopeRow struct {
	Scope             string  `json:"scope"`
	Resource          string  `json:"resource"`
	Action            string  `json:"action"`
	Holder            string  `json:"holder"`
	Namespace         string  `json:"namespace"`
	OwnerAppID        *string `json:"owner_app_id"`
	Description       string  `json:"description"`
	IsDestructive     bool    `json:"is_destructive"`
	RequiresMFA       bool    `json:"requires_mfa"`
	GraceBehavior     string  `json:"grace_behavior"`
	ConsentPromptCopy string  `json:"consent_prompt_copy"`
	WorkshopStatus    string  `json:"workshop_status"`
	BitID             *int64  `json:"bit_id"`
}

// ScopeRegistryListing is the response from ListScopeRegistry (#3517).
type ScopeRegistryListing struct {
	Scopes []ScopeRow `json:"scopes"`
	Total  int        `json:"total"`
}

// ScopeRegistryDigest is the response from GetScopeRegistryDigest (#3517).
//
// PlatformCatalogDigest is the SHA-256 hex matching
// scripts/seed_platform_scopes.py byte-for-byte. JWT mints embed this
// value so the gateway middleware can detect stale tokens after a
// catalog rotation.
type ScopeRegistryDigest struct {
	PlatformCatalogDigest string `json:"platform_catalog_digest"`
	RowCount              int    `json:"row_count"`
}

// ListScopeRegistry fetches the seeded scope catalog (#3517).
//
// Optional filters: Namespace, OwnerAppID (with OwnerAppIDSet=true to
// send an empty value), IncludeDrafts.
func (s *PlatformService) ListScopeRegistry(ctx context.Context, p ListScopeRegistryParams) (*ScopeRegistryListing, error) {
	q := url.Values{}
	if p.Namespace != "" {
		q.Set("namespace", p.Namespace)
	}
	if p.OwnerAppIDSet {
		q.Set("owner_app_id", p.OwnerAppID)
	}
	if p.IncludeDrafts {
		q.Set("include_drafts", "true")
	}
	raw, err := s.http.get(ctx, "/platform/scope-registry", q)
	if err != nil {
		return nil, err
	}
	var out ScopeRegistryListing
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetScopeRegistryDigest fetches the deterministic platform catalog digest (#3517).
func (s *PlatformService) GetScopeRegistryDigest(ctx context.Context) (*ScopeRegistryDigest, error) {
	raw, err := s.http.get(ctx, "/platform/scope-registry/digest", nil)
	if err != nil {
		return nil, err
	}
	var out ScopeRegistryDigest
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
