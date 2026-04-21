package olympus

import (
	"context"
	"fmt"
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
