package olympus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// AppsService wraps the canonical `/apps/*` surface — the apps.install consent
// ceremony shipped in olympus-cloud-gcp#3413 §3 (handlers + routes merged via
// olympus-cloud-gcp#3422).
//
// Routes:
//
//	| Method | Route                                   | Auth                      |
//	|--------|-----------------------------------------|---------------------------|
//	| POST   | /apps/install                           | tenant_admin + recent MFA |
//	| GET    | /apps/installed                         | any tenant-scoped JWT     |
//	| POST   | /apps/uninstall/:app_id                 | tenant_admin + recent MFA |
//	| GET    | /apps/manifest/:app_id                  | any authenticated         |
//	| GET    | /apps/pending_install/:id               | **anonymous**             |
//	| POST   | /apps/pending_install/:id/approve       | tenant_admin              |
//	| POST   | /apps/pending_install/:id/deny          | tenant_admin              |
//
// Drives the four-state consent ceremony:
//
//  1. Install creates a pending-install row + returns PendingInstall with a
//     consent URL + 10-minute TTL.
//  2. The consent UI fetches GetPendingInstall anonymously (the unguessable id
//     IS the bearer) to render the Approve/Deny screen. The manifest is eager-
//     loaded on the detail so no second round-trip is needed.
//  3. Tenant_admin approves via ApprovePendingInstall (returns the fresh
//     AppInstall) or denies via DenyPendingInstall.
//  4. ListInstalled / Uninstall / GetManifest cover the steady-state
//     app-management surface.
//
// MFA gate (install / uninstall / approve): the tenant_admin's session must
// carry a `mfa_verified_at:<epoch>` permission stamp within the last 10 minutes.
// If missing, the server returns 403 with `mfa_required` which the SDK surfaces
// as *OlympusAPIError — callers should trigger a step-up flow and retry.
type AppsService struct {
	http *httpClient
}

// --------------------------------------------------------------------------
// Types — mirror the Rust platform handlers in apps_install.rs exactly.
// JSON tags match the server's snake_case wire shape.
// --------------------------------------------------------------------------

// AppInstallRequest is the body of POST /apps/install.
//
// IdempotencyKey is optional. When supplied, retrying the same
// (tenant_id, app_id, idempotency_key) within the 10-minute pending window
// returns the ORIGINAL PendingInstall rather than creating a second row.
// Use the calling user's device fingerprint or a UUID generated per "Install"
// button press to de-dupe retry noise without cross-user collisions.
type AppInstallRequest struct {
	AppID          string   `json:"app_id"`
	Scopes         []string `json:"scopes"`
	ReturnTo       string   `json:"return_to"`
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
}

// PendingInstall is the handle returned by POST /apps/install.
//
// The caller must redirect the tenant_admin to ConsentURL before ExpiresAt
// (10 minutes after creation). Retrying the same (tenant_id, app_id,
// idempotency_key) within the window returns the original row — so losing a
// network round-trip and retrying does NOT create two pending rows.
//
// Apps MUST open ConsentURL in a real browser tab (NOT an in-app webview) so
// the tenant_admin's authenticated cookie session is visible to the platform
// domain — a webview would present a fresh login prompt.
type PendingInstall struct {
	PendingInstallID string     `json:"pending_install_id"`
	ConsentURL       string     `json:"consent_url"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

// AppManifest is the versioned manifest row for an app in the platform catalog.
//
// Returned by GET /apps/manifest/:app_id (latest version) and eager-loaded onto
// PendingInstallDetail.Manifest so the consent screen can render the required
// and optional scope checklists plus publisher / privacy / TOS links without a
// second round-trip.
type AppManifest struct {
	AppID          string   `json:"app_id"`
	Version        string   `json:"version"`
	Name           string   `json:"name"`
	Publisher      string   `json:"publisher"`
	LogoURL        string   `json:"logo_url,omitempty"`
	ScopesRequired []string `json:"scopes_required"`
	ScopesOptional []string `json:"scopes_optional"`
	PrivacyURL     string   `json:"privacy_url,omitempty"`
	TOSURL         string   `json:"tos_url,omitempty"`
}

// PendingInstallDetail is the full pending-install row, returned by
// GET /apps/pending_install/:id.
//
// **This endpoint is anonymous** — no JWT required. The unguessable id IS the
// bearer, and the row expires 10 minutes after creation. Rendered by the
// platform's consent surface (or a tenant-owned consent shell) to drive the
// Approve / Deny buttons.
//
// Manifest is eager-loaded server-side so the consent screen does NOT need to
// make a second GetManifest round-trip. Nil is only possible in the unlikely
// event the manifest was delisted between create and read.
//
// Status values: "pending" (active ceremony row) | "approved" | "denied".
// Rows in a terminal state still return 200 (not 410) so the consent UI can
// show a clear "already approved / already denied" state instead of a generic
// expiry message.
type PendingInstallDetail struct {
	ID               string       `json:"id"`
	AppID            string       `json:"app_id"`
	TenantID         string       `json:"tenant_id"`
	RequestedScopes  []string     `json:"requested_scopes"`
	ReturnTo         string       `json:"return_to"`
	Status           string       `json:"status"`
	ExpiresAt        *time.Time   `json:"expires_at,omitempty"`
	Manifest         *AppManifest `json:"manifest,omitempty"`
}

// AppInstall is a row from `tenant_app_installs`. Returned by
// GET /apps/installed and as the result of POST /apps/pending_install/:id/approve.
//
// This is the canonical AppInstall shape for the /apps/* ceremony. The shorter
// 3-field shape returned inline by POST /tenant/create is the distinct
// TenantAppInstall in tenant.go — same family of data, different cardinality.
//
// Status: "active" during normal operation; "uninstalled" after
// POST /apps/uninstall/:app_id. Uninstalled rows remain in the ledger briefly
// but are filtered out of ListInstalled by default on the server side.
type AppInstall struct {
	TenantID      string     `json:"tenant_id"`
	AppID         string     `json:"app_id"`
	InstalledAt   *time.Time `json:"installed_at,omitempty"`
	InstalledBy   string     `json:"installed_by"`
	ScopesGranted []string   `json:"scopes_granted"`
	Status        string     `json:"status"`
}

// --------------------------------------------------------------------------
// Methods
// --------------------------------------------------------------------------

// Install initiates the install ceremony for the given app.
//
// The server creates a pending-install row, validates req.Scopes against the
// latest AppManifest, and returns a PendingInstall with a consent URL the
// caller should open in a real browser tab (NOT an in-app webview — the
// consent screen needs the tenant_admin's authenticated cookie session on the
// platform domain).
//
// req.ReturnTo is the post-approval deep-link the consent surface will redirect
// to on Approve (or Deny). Typically the app's "settings → permissions" screen.
//
// Requires tenant_admin role + recent MFA on the session. Returns an
// *OlympusAPIError on scope/manifest validation failures, missing MFA
// (403 `mfa_required`), or an unknown app_id (404).
func (s *AppsService) Install(ctx context.Context, req AppInstallRequest) (*PendingInstall, error) {
	if req.AppID == "" {
		return nil, fmt.Errorf("olympus-sdk: AppInstallRequest.AppID is required")
	}
	if req.ReturnTo == "" {
		return nil, fmt.Errorf("olympus-sdk: AppInstallRequest.ReturnTo is required")
	}
	if req.Scopes == nil {
		req.Scopes = []string{}
	}
	raw, err := s.http.post(ctx, "/apps/install", req)
	if err != nil {
		return nil, err
	}
	var out PendingInstall
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListInstalled lists every app currently installed on the caller's tenant.
//
// Returns active installs only — AppInstall.Status is "active" for each row.
// Uninstalled rows are filtered out server-side. Safe to call on any
// tenant-scoped JWT; no role requirement.
//
// Accepts both a bare JSON array and a gateway-wrapped envelope
// ({"items": [...]} or {"data": [...]}) for forward-compatibility.
func (s *AppsService) ListInstalled(ctx context.Context) ([]AppInstall, error) {
	body, err := s.http.getRaw(ctx, "/apps/installed", nil)
	if err != nil {
		return nil, err
	}
	var out []AppInstall
	if err := json.Unmarshal(body, &out); err == nil {
		return out, nil
	}
	var envelope struct {
		Items []AppInstall `json:"items"`
		Data  []AppInstall `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Items) > 0 {
		return envelope.Items, nil
	}
	return envelope.Data, nil
}

// Uninstall removes appID from the caller's tenant.
//
// Marks the install as "uninstalled" and emits `platform.app.uninstalled` on
// Pub/Sub. The auth service consumer for that event kicks session revocation
// for every JWT carrying this (tenant_id, app_id) pair — per AC-7 on #3413 the
// contract is 60-second session invalidation.
//
// Requires tenant_admin role + recent MFA. Returns *OlympusAPIError on a tenant
// that doesn't have appID installed (404) or a missing-MFA session
// (403 `mfa_required`).
func (s *AppsService) Uninstall(ctx context.Context, appID string) error {
	if appID == "" {
		return fmt.Errorf("olympus-sdk: Uninstall requires appID")
	}
	_, err := s.http.post(ctx, "/apps/uninstall/"+appID, nil)
	return err
}

// GetManifest fetches the latest published AppManifest for appID.
//
// Useful for rendering "available apps" browsers outside the ceremony flow.
// Returns *OlympusAPIError (404) if appID has no manifest in the catalog.
func (s *AppsService) GetManifest(ctx context.Context, appID string) (*AppManifest, error) {
	if appID == "" {
		return nil, fmt.Errorf("olympus-sdk: GetManifest requires appID")
	}
	raw, err := s.http.get(ctx, "/apps/manifest/"+appID, nil)
	if err != nil {
		return nil, err
	}
	var out AppManifest
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPendingInstall fetches the pending-install ceremony row by its unguessable id.
//
// **Anonymous — no JWT required.** The id is an unguessable UUID with a
// 10-minute TTL, issued by the server on Install. The consent surface uses
// this call to render the Approve/Deny screen with eager-loaded
// PendingInstallDetail.Manifest so no second round-trip is needed.
//
// Returns *OlympusAPIError with StatusCode 410 (Gone) if the pending row has
// expired or doesn't exist — the server masks "not found" as "gone" so an
// attacker can't enumerate ids.
//
// Safe to call with or without a session — the server ignores the Authorization
// header on this route. Note: the SDK will still attach the Authorization
// header if a session token is set on the HTTP client, but that is a no-op for
// the server.
func (s *AppsService) GetPendingInstall(ctx context.Context, pendingInstallID string) (*PendingInstallDetail, error) {
	if pendingInstallID == "" {
		return nil, fmt.Errorf("olympus-sdk: GetPendingInstall requires pendingInstallID")
	}
	raw, err := s.http.get(ctx, "/apps/pending_install/"+pendingInstallID, nil)
	if err != nil {
		return nil, err
	}
	var out PendingInstallDetail
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ApprovePendingInstall approves a pending install.
//
// Server runs one Spanner transaction that resolves the pending row
// (status=approved) and upserts the `tenant_app_installs` row — returns the
// fresh AppInstall. Also emits `platform.app.installed` on Pub/Sub for
// downstream consumers (billing activation, analytics, welcome email, etc.).
//
// Requires tenant_admin role on the TARGET tenant (the pending row's
// tenant_id, which may differ from the session's tenant_id if an admin is
// completing consent on a device scoped to a different tenant) + a recent MFA
// stamp. Server re-validates the requested scopes against the latest manifest
// — if the manifest was updated to remove a scope between install and approve,
// the call fails with 400.
//
// Returns *OlympusAPIError:
//   - 410 (Gone) if the pending row expired between create and approve
//   - 403 if caller is not a tenant_admin on the target tenant or MFA is stale
//   - 400 if the pending row is already resolved (approved/denied)
func (s *AppsService) ApprovePendingInstall(ctx context.Context, pendingInstallID string) (*AppInstall, error) {
	if pendingInstallID == "" {
		return nil, fmt.Errorf("olympus-sdk: ApprovePendingInstall requires pendingInstallID")
	}
	raw, err := s.http.post(ctx, "/apps/pending_install/"+pendingInstallID+"/approve", nil)
	if err != nil {
		return nil, err
	}
	var out AppInstall
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DenyPendingInstall denies a pending install.
//
// Marks the pending row status=denied and emits `platform.app.install_denied`
// for analytics / funnel tracking. Returns 204 on success — there's no install
// record to surface on a deny.
//
// Requires tenant_admin role on the target tenant. Does NOT require fresh MFA
// — the deny path is idempotent and a deny-by-default is always safe.
//
// Returns *OlympusAPIError:
//   - 410 (Gone) if the pending row expired
//   - 403 if caller is not a tenant_admin on the target tenant
//   - 400 if the pending row is already resolved
func (s *AppsService) DenyPendingInstall(ctx context.Context, pendingInstallID string) error {
	if pendingInstallID == "" {
		return fmt.Errorf("olympus-sdk: DenyPendingInstall requires pendingInstallID")
	}
	_, err := s.http.post(ctx, "/apps/pending_install/"+pendingInstallID+"/deny", nil)
	return err
}
