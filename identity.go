package olympus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// IdentityService is the Olympus ID surface — global, cross-tenant identity
// and federation. Wraps the Olympus Platform (Rust) Identity handler via
// the Go API Gateway.
//
// Routes:
//   - POST /api/v1/platform/identities        — get-or-create identity
//   - POST /api/v1/platform/identities/links  — link identity to a tenant
//
// Age-verification routes (/identity/*) back the Document AI flow (#3009).
//
// An OlympusIdentity is keyed by Firebase UID and represents one human
// across every app on the platform. Call GetOrCreateFromFirebase right
// after a successful Firebase sign-in and LinkToTenant when the user first
// transacts with a tenant so the global identity can be cross-referenced
// with the tenant's commerce customer.
type IdentityService struct {
	http *httpClient
}

// OlympusIdentity is the global identity representing a consumer or business
// operator across all Olympus Cloud apps. Backed by
// platform_olympus_identities in Spanner.
type OlympusIdentity struct {
	// ID is the server-assigned global identity UUID. Stable across tenants.
	ID string `json:"id"`
	// FirebaseUID is the Firebase Auth UID. Unique per signed-in user.
	FirebaseUID string `json:"firebase_uid"`

	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`

	// GlobalPreferences is free-form JSON for cross-app preferences
	// (theme, locale, accessibility).
	GlobalPreferences map[string]interface{} `json:"global_preferences,omitempty"`

	// StripeCustomerID is the cross-tenant Stripe customer ID, used by
	// Olympus Pay for federated checkout flows.
	StripeCustomerID string `json:"stripe_customer_id,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// IdentityLink is a binding between an OlympusIdentity and a tenant-scoped
// commerce customer. One Olympus identity can have many links — one per
// tenant the user has done business with.
type IdentityLink struct {
	OlympusID          string `json:"olympus_id"`
	TenantID           string `json:"tenant_id"`
	CommerceCustomerID string `json:"commerce_customer_id"`
	LinkedAt           string `json:"linked_at"`
}

// GetOrCreateFromFirebaseRequest holds the optional fields supplied on the
// first get-or-create call for a Firebase user. Only FirebaseUID is required.
type GetOrCreateFromFirebaseRequest struct {
	FirebaseUID       string                 `json:"firebase_uid"`
	Email             string                 `json:"email,omitempty"`
	Phone             string                 `json:"phone,omitempty"`
	FirstName         string                 `json:"first_name,omitempty"`
	LastName          string                 `json:"last_name,omitempty"`
	GlobalPreferences map[string]interface{} `json:"global_preferences,omitempty"`
}

// GetOrCreateFromFirebase returns the global Olympus identity for a Firebase
// user, creating a new row if one does not already exist. Safe to call on
// every sign-in — the server is idempotent on firebase_uid.
func (s *IdentityService) GetOrCreateFromFirebase(ctx context.Context, req GetOrCreateFromFirebaseRequest) (*OlympusIdentity, error) {
	if req.FirebaseUID == "" {
		return nil, fmt.Errorf("olympus-sdk: GetOrCreateFromFirebase requires firebase_uid")
	}
	body := map[string]interface{}{
		"firebase_uid":       req.FirebaseUID,
		"email":              req.Email,
		"phone":              req.Phone,
		"first_name":         req.FirstName,
		"last_name":          req.LastName,
		"global_preferences": req.GlobalPreferences,
	}
	raw, err := s.http.post(ctx, "/platform/identities", body)
	if err != nil {
		return nil, err
	}
	var out OlympusIdentity
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// LinkToTenant binds a global identity to a tenant-scoped commerce customer.
// Should be called the first time a federated user transacts with a new
// tenant. Safe to call again; the platform de-duplicates by
// (olympus_id, tenant_id).
func (s *IdentityService) LinkToTenant(ctx context.Context, olympusID, tenantID, commerceCustomerID string) error {
	_, err := s.http.post(ctx, "/platform/identities/links", map[string]interface{}{
		"olympus_id":           olympusID,
		"tenant_id":            tenantID,
		"commerce_customer_id": commerceCustomerID,
	})
	return err
}

// ---------------------------------------------------------------------------
// Age Verification (Document AI) — #3009
// ---------------------------------------------------------------------------

// ScanID scans an ID document for age verification via Google Document AI.
// The image is processed and immediately deleted — only DOB hash + age are
// stored.
//
// NOTE: Mirrors the dart SDK transport — JSON body with image bytes inlined
// as an integer array (matching dart's `List<int>` marshaling). The
// production server (`/identity/scan-id` in
// `backend/python/app/api/identity_verification_routes.py`) expects a
// multipart upload, so this method currently parity-mirrors a known-broken
// dart call. Aligning the SDKs to multipart is tracked separately; do NOT
// silently fix here. Callers that need real multipart uploads should drop
// down to net/http directly.
func (s *IdentityService) ScanID(ctx context.Context, phone string, imageBytes []byte) (map[string]interface{}, error) {
	// Convert []byte to []int so JSON encodes it as a numeric array, not a
	// base64 string — matching dart's List<int> wire format byte-for-byte.
	imageInts := make([]int, len(imageBytes))
	for i, b := range imageBytes {
		imageInts[i] = int(b)
	}
	return s.http.post(ctx, "/identity/scan-id", map[string]interface{}{
		"phone": phone,
		"image": imageInts,
	})
}

// CheckVerificationStatus checks a caller's age-verification status.
func (s *IdentityService) CheckVerificationStatus(ctx context.Context, phone string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/identity/status/%s", phone), nil)
}

// VerifyPassphrase compares a caller's passphrase against the bcrypt hash.
func (s *IdentityService) VerifyPassphrase(ctx context.Context, phone, passphrase string) (map[string]interface{}, error) {
	return s.http.post(ctx, "/identity/verify-passphrase", map[string]interface{}{
		"phone":      phone,
		"passphrase": passphrase,
	})
}

// SetPassphrase sets or updates a caller's passphrase (bcrypt hashed on the
// server).
func (s *IdentityService) SetPassphrase(ctx context.Context, phone, passphrase string) (map[string]interface{}, error) {
	return s.http.post(ctx, "/identity/set-passphrase", map[string]interface{}{
		"phone":      phone,
		"passphrase": passphrase,
	})
}

// CreateUploadSession returns a signed upload URL for the caller to upload
// their ID photo.
func (s *IdentityService) CreateUploadSession(ctx context.Context) (map[string]interface{}, error) {
	return s.http.post(ctx, "/identity/create-upload-session", nil)
}

// ---------------------------------------------------------------------------
// Identity invites (#3403 §4.2 + §4.4) — canonical /identity/invite* surface.
//
// Replaces the raw createUser loop in pizza-os onboarding. Gives every app
// one place to invite staff/managers, list pending invites, accept/revoke
// invites, and remove users from a tenant (Firebase identity preserved).
//
// Routes (backed by PR #3410):
//   - POST /identity/invite                          — manager or tenant_admin
//   - GET  /identity/invites                         — manager or tenant_admin
//   - POST /identity/invites/:token/accept           — Firebase ID token body
//   - POST /identity/invites/:id/revoke              — manager or tenant_admin
//   - POST /identity/remove_from_tenant              — tenant_admin
//
// Invite tokens are short-lived JWTs (default 7d, capped at 30d) signed with
// the platform JWT key. The token is returned only once — on InviteCreate —
// and is SHA-256 hashed server-side for idempotent lookup on accept.
// ---------------------------------------------------------------------------

// InviteStatus mirrors the server enum. Snake-case on the wire.
type InviteStatus string

const (
	// InviteStatusPending — created, not yet accepted or revoked.
	InviteStatusPending InviteStatus = "pending"
	// InviteStatusAccepted — recipient completed acceptance.
	InviteStatusAccepted InviteStatus = "accepted"
	// InviteStatusRevoked — invite owner cancelled the invite.
	InviteStatusRevoked InviteStatus = "revoked"
	// InviteStatusExpired — ttl_seconds elapsed before acceptance.
	InviteStatusExpired InviteStatus = "expired"
)

// InviteCreateRequest is the body for Invite.
//
// Role must match docs/platform/roles.yaml — typically one of:
// `tenant_admin`, `manager`, `staff`, `employee`, `viewer`, `accountant`,
// `developer`. An unknown role returns 422 without any DB write.
//
// TTLSeconds defaults to 7d when zero, and is capped at 30d server-side.
type InviteCreateRequest struct {
	Email      string `json:"email"`
	Role       string `json:"role"`
	LocationID string `json:"location_id,omitempty"`
	Message    string `json:"message,omitempty"`
	TTLSeconds int64  `json:"ttl_seconds,omitempty"`
}

// InviteHandle is the invite envelope returned by Invite/ListInvites/RevokeInvite.
//
// Token is populated only on the Invite response — subsequent reads return
// it as an empty string (the server stores only a SHA-256 hash).
type InviteHandle struct {
	ID         string       `json:"id"`
	Token      string       `json:"token,omitempty"`
	Email      string       `json:"email"`
	Role       string       `json:"role"`
	LocationID string       `json:"location_id,omitempty"`
	TenantID   string       `json:"tenant_id"`
	ExpiresAt  *time.Time   `json:"expires_at,omitempty"`
	Status     InviteStatus `json:"status"`
	CreatedAt  *time.Time   `json:"created_at,omitempty"`
	AcceptedAt *time.Time   `json:"accepted_at,omitempty"`
}

// Invite creates a new pending invitation for `email` with `role`. Returns
// the InviteHandle with the signed invite-token JWT set in `Token`.
//
// The caller must hold the `manager` or `tenant_admin` role on their current
// tenant (server-enforced).
//
// Deliver the token to the invitee out-of-band (email, SMS, QR code). The
// invitee accepts by POSTing their Firebase ID token to
// `POST /identity/invites/:token/accept`, which this SDK exposes via
// `AcceptInvite`.
func (s *IdentityService) Invite(ctx context.Context, req InviteCreateRequest) (*InviteHandle, error) {
	if req.Email == "" {
		return nil, fmt.Errorf("olympus-sdk: Invite requires Email")
	}
	if req.Role == "" {
		return nil, fmt.Errorf("olympus-sdk: Invite requires Role")
	}
	raw, err := s.http.post(ctx, "/identity/invite", req)
	if err != nil {
		return nil, err
	}
	var out InviteHandle
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AcceptInvite exchanges an invite token + the recipient's Firebase ID
// token for a full AuthSession scoped to the invite's tenant + role.
//
// This is a login — on success the returned session's access token is
// automatically installed on the HTTP client (same behaviour as
// AuthService.Login).
//
// NOTE: AcceptInvite does NOT currently emit SessionLoggedIn events to
// `AuthService.SessionEvents` subscribers, and it does NOT nudge the
// silent-refresh goroutine. Callers using `StartSilentRefresh` should start
// the goroutine AFTER the AcceptInvite call completes. See
// olympus-cloud-gcp#3403 §4.2 follow-up for the event-plumbing issue.
//
// The Firebase token's email must match the invite's email (case-insensitive);
// a mismatch bubbles as 403. Replay is blocked — an accepted, revoked, or
// expired invite returns 400.
func (s *IdentityService) AcceptInvite(ctx context.Context, inviteToken, firebaseIDToken string) (*AuthSession, error) {
	if inviteToken == "" {
		return nil, fmt.Errorf("olympus-sdk: AcceptInvite requires an invite token")
	}
	if firebaseIDToken == "" {
		return nil, fmt.Errorf("olympus-sdk: AcceptInvite requires a Firebase ID token")
	}
	path := fmt.Sprintf("/identity/invites/%s/accept", inviteToken)
	raw, err := s.http.post(ctx, path, map[string]interface{}{
		"firebase_id_token": firebaseIDToken,
	})
	if err != nil {
		return nil, err
	}
	session := parseAuthSession(raw)
	if session.AccessToken != "" {
		s.http.SetAccessToken(session.AccessToken)
	}
	return session, nil
}

// ListInvites returns every invite for the caller's current tenant, newest
// first, capped at 500 by the server. The `Token` field is never populated
// on listed invites — it's stored server-side as a SHA-256 hash only.
//
// Requires `manager` or `tenant_admin` role.
func (s *IdentityService) ListInvites(ctx context.Context) ([]InviteHandle, error) {
	body, err := s.http.getRaw(ctx, "/identity/invites", nil)
	if err != nil {
		return nil, err
	}
	// Try bare-array decode first (canonical backend shape), then envelope.
	var out []InviteHandle
	if err := json.Unmarshal(body, &out); err == nil {
		return out, nil
	}
	var envelope struct {
		Items []InviteHandle `json:"items"`
		Data  []InviteHandle `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Items) > 0 {
		return envelope.Items, nil
	}
	return envelope.Data, nil
}

// RevokeInvite cancels a pending invite. Idempotent — revoking an already-
// revoked or already-accepted invite returns the current state without
// erroring. Requires `manager` or `tenant_admin` role.
func (s *IdentityService) RevokeInvite(ctx context.Context, inviteID string) (*InviteHandle, error) {
	if inviteID == "" {
		return nil, fmt.Errorf("olympus-sdk: RevokeInvite requires an invite ID")
	}
	path := fmt.Sprintf("/identity/invites/%s/revoke", inviteID)
	raw, err := s.http.post(ctx, path, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var out InviteHandle
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveFromTenantResult is the response from RemoveFromTenant.
type RemoveFromTenantResult struct {
	TenantID  string     `json:"tenant_id"`
	UserID    string     `json:"user_id"`
	RemovedAt *time.Time `json:"removed_at,omitempty"`
}

// RemoveFromTenant offboards a user from the caller's current tenant. The
// user's Firebase identity and any links to other tenants are preserved
// (per #3403 §4.4); only the auth_users row + role assignments + active
// sessions in this tenant are revoked.
//
// Requires `tenant_admin`. Removing yourself returns 400 — transfer admin
// to another user first via an Invite + role assignment.
//
// `reason` is optional and, when set, is recorded on the audit event emitted
// to Pub/Sub topic `platform.identity.removed_from_tenant`.
func (s *IdentityService) RemoveFromTenant(ctx context.Context, userID, reason string) (*RemoveFromTenantResult, error) {
	if userID == "" {
		return nil, fmt.Errorf("olympus-sdk: RemoveFromTenant requires a user_id")
	}
	body := map[string]interface{}{
		"user_id": userID,
	}
	if reason != "" {
		body["reason"] = reason
	}
	raw, err := s.http.post(ctx, "/identity/remove_from_tenant", body)
	if err != nil {
		return nil, err
	}
	var out RemoveFromTenantResult
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
