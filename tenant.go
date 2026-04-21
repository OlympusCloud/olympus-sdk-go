package olympus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// TenantService wraps the canonical /tenant/* surface (#3403 §2 + §4.4).
//
// Routes (backed by PR #3410 — rust-platform + rust-auth):
//   - POST   /tenant/create      — self-service signup (idempotent on idempotency_key, 24h window)
//   - GET    /tenant/current     — read the caller's active tenant
//   - PATCH  /tenant/current     — update brand_name / plan / billing / locale / timezone (tenant_admin)
//   - POST   /tenant/retire      — MFA + typed confirmation_slug; 30d grace window
//   - POST   /tenant/unretire    — reverse retire within the grace window
//   - GET    /tenant/mine        — list all tenants the caller has access to
//   - POST   /auth/switch-tenant — mint a new session scoped to a sibling tenant
//
// This service replaces the raw `INSERT INTO tenants` hack in pizza-os
// `admin_app.dart:215` and gives every app a typed entry point for tenant
// lifecycle.
//
// Authorization boundaries are enforced server-side:
//   - Create: platform_admin JWT OR a Firebase-verified email-only token
//   - Current/MyTenants/SwitchTenant: any tenant-scoped JWT
//   - Update/Retire/Unretire: tenant_admin (Retire additionally requires
//     a recent MFA attestation on the session — the server returns 403
//     with code `mfa_required` otherwise)
type TenantService struct {
	http *httpClient
}

// --------------------------------------------------------------------------
// Types — mirror #3403 §2 + §4.4 exactly. All JSON tags match the backend
// snake_case shape on the wire.
// --------------------------------------------------------------------------

// Tenant is the canonical tenant record returned from /tenant/current and
// /tenant/create. Mirrors the rust-platform `Tenant` model.
type Tenant struct {
	ID                     string                 `json:"id"`
	Name                   string                 `json:"name"`
	Slug                   string                 `json:"slug"`
	LegalName              string                 `json:"legal_name,omitempty"`
	Industry               string                 `json:"industry,omitempty"`
	SubscriptionTier       string                 `json:"subscription_tier,omitempty"`
	ParentID               string                 `json:"parent_id,omitempty"`
	Path                   string                 `json:"path,omitempty"`
	Settings               map[string]interface{} `json:"settings,omitempty"`
	Features               map[string]interface{} `json:"features,omitempty"`
	Branding               map[string]interface{} `json:"branding,omitempty"`
	Locale                 string                 `json:"locale,omitempty"`
	Timezone               string                 `json:"timezone,omitempty"`
	BillingEmail           string                 `json:"billing_email,omitempty"`
	StripeCustomerID       string                 `json:"stripe_customer_id,omitempty"`
	StripeConnectAccountID string                 `json:"stripe_connect_account_id,omitempty"`
	TrialEndsAt            *time.Time             `json:"trial_ends_at,omitempty"`
	IsActive               bool                   `json:"is_active"`
	IsSuspended            bool                   `json:"is_suspended,omitempty"`
	RetiredAt              *time.Time             `json:"retired_at,omitempty"`
	Metadata               map[string]interface{} `json:"metadata,omitempty"`
	Tags                   []string               `json:"tags,omitempty"`
	CreatedAt              *time.Time             `json:"created_at,omitempty"`
	UpdatedAt              *time.Time             `json:"updated_at,omitempty"`
}

// TenantFirstAdmin is the first admin user seeded into a fresh tenant by
// /tenant/create. If `FirebaseLink` is set, the server stashes the Firebase
// UID on the tenant so a follow-up `/auth/firebase/exchange` call links the
// identity and mints a session.
type TenantFirstAdmin struct {
	Email        string `json:"email"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	FirebaseLink string `json:"firebase_link,omitempty"`
}

// TenantCreateRequest is the input to Create.
//
// IdempotencyKey is required — signup funnels should pass the Firebase UID
// so retries within 24h return the original tenant instead of provisioning
// a second one.
//
// Plan must be one of `starter` / `pro` / `enterprise` / `demo`. Slug must
// match `[a-z0-9-]{3,63}` (validated server-side against the same regex).
type TenantCreateRequest struct {
	BrandName      string           `json:"brand_name"`
	Slug           string           `json:"slug"`
	Plan           string           `json:"plan"`
	FirstAdmin     TenantFirstAdmin `json:"first_admin"`
	InstallApps    []string         `json:"install_apps,omitempty"`
	BillingAddress string           `json:"billing_address,omitempty"`
	TaxID          string           `json:"tax_id,omitempty"`
	IdempotencyKey string           `json:"idempotency_key"`
}

// ExchangedSession is the session subset returned by /tenant/create when the
// caller passed a `first_admin.firebase_link` — in that case the server can
// (optionally, today: returns nil) mint a session inline. Callers that need
// a session should follow up with /auth/firebase/exchange.
//
// On /auth/switch-tenant this wraps the full token pair.
type ExchangedSession struct {
	AccessToken     string     `json:"access_token,omitempty"`
	RefreshToken    string     `json:"refresh_token,omitempty"`
	AccessExpiresAt *time.Time `json:"access_expires_at,omitempty"`
}

// AppInstall is the record emitted when /tenant/create auto-installs an app
// from TenantCreateRequest.InstallApps.
type AppInstall struct {
	AppID       string     `json:"app_id"`
	Status      string     `json:"status"`
	InstalledAt *time.Time `json:"installed_at,omitempty"`
}

// TenantProvisionResult is the response from /tenant/create.
//
// Idempotent is true when the server detected a live idempotency_key row
// within the 24h window and returned the original tenant without re-provisioning.
// On an idempotent hit the session fields are nil — callers already have
// their session from the first call.
type TenantProvisionResult struct {
	Tenant        Tenant           `json:"tenant"`
	AdminUserID   string           `json:"admin_user_id,omitempty"`
	Session       ExchangedSession `json:"session"`
	InstalledApps []AppInstall     `json:"installed_apps,omitempty"`
	Idempotent    bool             `json:"idempotent"`
}

// TenantUpdate is the patch payload for PATCH /tenant/current. All fields
// are optional — only non-empty fields are applied, everything else is left
// untouched.
type TenantUpdate struct {
	BrandName      string `json:"brand_name,omitempty"`
	Plan           string `json:"plan,omitempty"`
	BillingAddress string `json:"billing_address,omitempty"`
	TaxID          string `json:"tax_id,omitempty"`
	Locale         string `json:"locale,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
}

// TenantRetireResult is the response from /tenant/retire. The caller gets
// PurgeEligibleAt so the UI can show a "restore by <date>" hint before the
// 30-day grace window closes and the background purge job runs.
type TenantRetireResult struct {
	TenantID        string     `json:"tenant_id"`
	RetiredAt       *time.Time `json:"retired_at"`
	PurgeEligibleAt *time.Time `json:"purge_eligible_at"`
}

// TenantUnretireResult is the response from /tenant/unretire.
type TenantUnretireResult struct {
	TenantID    string     `json:"tenant_id"`
	UnretiredAt *time.Time `json:"unretired_at"`
}

// TenantOption is one entry in the /tenant/mine list. `Role` may be empty
// — the backend currently returns the membership set without per-tenant
// role expansion; callers should follow up with /auth/me after SwitchTenant
// to load the full role list.
type TenantOption struct {
	TenantID string `json:"tenant_id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Role     string `json:"role,omitempty"`
}

// --------------------------------------------------------------------------
// Methods
// --------------------------------------------------------------------------

// Create provisions a new tenant with a first admin user.
//
// The call is idempotent on `IdempotencyKey` — retries within 24h return the
// original tenant (with `Idempotent: true`) rather than creating a second one.
// Use the caller's Firebase UID as the idempotency key for self-service signup;
// that gives you full retry safety without exposing a new knob.
//
// Validation errors (bad slug, bad plan, missing first_admin fields) bubble
// as 422 OlympusAPIError. Slug collisions bubble as 400.
func (s *TenantService) Create(ctx context.Context, req TenantCreateRequest) (*TenantProvisionResult, error) {
	if req.BrandName == "" {
		return nil, fmt.Errorf("olympus-sdk: TenantCreateRequest.BrandName is required")
	}
	if req.Slug == "" {
		return nil, fmt.Errorf("olympus-sdk: TenantCreateRequest.Slug is required")
	}
	if req.Plan == "" {
		return nil, fmt.Errorf("olympus-sdk: TenantCreateRequest.Plan is required")
	}
	if req.FirstAdmin.Email == "" {
		return nil, fmt.Errorf("olympus-sdk: TenantCreateRequest.FirstAdmin.Email is required")
	}
	if req.IdempotencyKey == "" {
		return nil, fmt.Errorf("olympus-sdk: TenantCreateRequest.IdempotencyKey is required")
	}

	raw, err := s.http.post(ctx, "/tenant/create", req)
	if err != nil {
		return nil, err
	}
	var out TenantProvisionResult
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Current returns the caller's active tenant (the tenant whose ID is in the
// JWT's `tenant_id` claim).
func (s *TenantService) Current(ctx context.Context) (*Tenant, error) {
	raw, err := s.http.get(ctx, "/tenant/current", nil)
	if err != nil {
		return nil, err
	}
	var out Tenant
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update patches the caller's active tenant. Only non-empty fields are sent
// to the server. Requires the `tenant_admin` role (server-enforced).
func (s *TenantService) Update(ctx context.Context, patch TenantUpdate) (*Tenant, error) {
	raw, err := s.http.patch(ctx, "/tenant/current", patch)
	if err != nil {
		return nil, err
	}
	var out Tenant
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Retire soft-deletes the caller's active tenant. The caller must present
// `confirmationSlug` exactly equal to the tenant's slug — typed-to-confirm
// UX that prevents admins from fat-fingering the "Close Organization" button.
//
// Requires (a) tenant_admin role, and (b) a recent MFA attestation on the
// session (the server returns 403 with code `mfa_required` otherwise, so
// the client can trigger a step-up flow without losing state).
//
// After retire, /tenant/unretire can reverse the decision for 30 days; past
// that window the tenant becomes eligible for permanent deletion.
func (s *TenantService) Retire(ctx context.Context, confirmationSlug string) (*TenantRetireResult, error) {
	if confirmationSlug == "" {
		return nil, fmt.Errorf("olympus-sdk: Retire requires confirmationSlug (must equal tenant slug)")
	}
	raw, err := s.http.post(ctx, "/tenant/retire", map[string]interface{}{
		"confirmation_slug": confirmationSlug,
	})
	if err != nil {
		return nil, err
	}
	var out TenantRetireResult
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Unretire reverses a previous Retire within the 30-day grace window.
// Returns a 400 error if the tenant is not currently retired, or if the
// grace window has already expired.
func (s *TenantService) Unretire(ctx context.Context) (*TenantUnretireResult, error) {
	raw, err := s.http.post(ctx, "/tenant/unretire", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var out TenantUnretireResult
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MyTenants lists every tenant the caller has access to, keyed by the email
// on the caller's JWT. Works for Firebase-exchanged tokens and interactive
// login tokens; platform-shell tokens without an email return 400.
//
// Use this to power multi-tenant pickers after login. Follow up with
// SwitchTenant to move the session into a different tenant.
func (s *TenantService) MyTenants(ctx context.Context) ([]TenantOption, error) {
	// /tenant/mine returns a bare JSON array; use getRaw to bypass the
	// map[string]interface{} decode path.
	body, err := s.http.getRaw(ctx, "/tenant/mine", nil)
	if err != nil {
		return nil, err
	}
	var out []TenantOption
	// Try bare-array decode first (canonical backend shape).
	if err := json.Unmarshal(body, &out); err == nil {
		return out, nil
	}
	// Fall back to gateway-wrapped envelopes: {"items": [...]} or {"data": [...]}.
	var envelope struct {
		Items []TenantOption `json:"items"`
		Data  []TenantOption `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Items) > 0 {
		return envelope.Items, nil
	}
	return envelope.Data, nil
}

// SwitchTenant mints a new session scoped to `tenantID`. The caller must be
// a member of the target tenant (platform_admin bypass aside). On success
// the new access token is automatically installed on the HTTP client so
// subsequent calls use the switched session.
//
// NOTE: SwitchTenant does NOT currently emit SessionLoggedIn / SessionRefreshed
// events to `AuthService.SessionEvents` subscribers, and it does NOT nudge
// the silent-refresh goroutine to reschedule from the new exp. Callers using
// `StartSilentRefresh` should call `StopSilentRefresh` before SwitchTenant
// and restart it afterwards, or prefer an explicit logout + login flow. See
// olympus-cloud-gcp#3403 §4.2 follow-up for the event-plumbing issue.
//
// Returns the full ExchangedSession (access + refresh + expiry).
func (s *TenantService) SwitchTenant(ctx context.Context, tenantID string) (*ExchangedSession, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("olympus-sdk: SwitchTenant requires a tenant_id")
	}
	raw, err := s.http.post(ctx, "/auth/switch-tenant", map[string]interface{}{
		"tenant_id": tenantID,
	})
	if err != nil {
		return nil, err
	}
	// /auth/switch-tenant returns a full TokenResponse — parse it as a
	// session, install the token, and return the narrow ExchangedSession
	// shape documented in the method signature.
	session := parseAuthSession(raw)
	if session.AccessToken != "" {
		s.http.SetAccessToken(session.AccessToken)
	}
	out := &ExchangedSession{
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
	}
	if session.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(session.ExpiresIn) * time.Second)
		out.AccessExpiresAt = &t
	}
	return out, nil
}
