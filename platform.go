package olympus

import "context"

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
