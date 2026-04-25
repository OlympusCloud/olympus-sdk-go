package olympus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// FirebaseLinkResult is returned by AuthService.LinkFirebase on success.
//
// Linked is the wall-clock at which the link was first established. For
// idempotent re-link calls this is the ORIGINAL link time, not "now".
type FirebaseLinkResult struct {
	OlympusID   string    `json:"olympus_id"`
	FirebaseUID string    `json:"firebase_uid"`
	LinkedAt    time.Time `json:"linked_at"`
}

// FirebaseTenantOption is one candidate in a 409 multiple_tenants_match
// response from /auth/firebase/exchange. Apps render a picker from the
// candidate list and retry with an explicit tenant slug.
type FirebaseTenantOption struct {
	TenantID   string `json:"tenant_id"`
	TenantSlug string `json:"tenant_slug"`
	TenantName string `json:"tenant_name"`
}

// LoginWithFirebaseOptions are the optional inputs to
// AuthService.LoginWithFirebase. Both fields can be empty.
type LoginWithFirebaseOptions struct {
	// TenantSlug, when set, skips tenant auto-resolution. When empty the
	// backend resolves the tenant from the Firebase UID's identity link or
	// the caller's email; if more than one matches, a TenantAmbiguousError
	// is returned.
	TenantSlug string
	// InviteToken is used for first-time signup flows where the caller has
	// no existing identity link but has been issued an invite.
	InviteToken string
}

// FirebaseLoginError is the marker interface for typed Firebase federation
// errors. Use errors.As to unwrap the specific subtype.
type FirebaseLoginError interface {
	error
	firebaseLoginErrorMarker()
}

// TenantAmbiguousError — 409 multiple_tenants_match.
type TenantAmbiguousError struct {
	Candidates []FirebaseTenantOption
}

func (e *TenantAmbiguousError) Error() string {
	return fmt.Sprintf("TenantAmbiguous: %d candidates", len(e.Candidates))
}
func (e *TenantAmbiguousError) firebaseLoginErrorMarker() {}

// FirebaseUidAlreadyLinkedError — 409 firebase_uid_already_linked.
type FirebaseUidAlreadyLinkedError struct {
	ExistingOlympusID string
}

func (e *FirebaseUidAlreadyLinkedError) Error() string {
	if e.ExistingOlympusID != "" {
		return fmt.Sprintf("FirebaseUidAlreadyLinked: existing=%s", e.ExistingOlympusID)
	}
	return "FirebaseUidAlreadyLinked"
}
func (e *FirebaseUidAlreadyLinkedError) firebaseLoginErrorMarker() {}

// IdentityUnlinkedError — 403 identity_unlinked. SignupURL, when present,
// is the URL the app should redirect the user to in order to complete
// signup before retrying.
type IdentityUnlinkedError struct {
	SignupURL string
	Hint      string
}

func (e *IdentityUnlinkedError) Error() string {
	return fmt.Sprintf("IdentityUnlinked: signup_url=%s", e.SignupURL)
}
func (e *IdentityUnlinkedError) firebaseLoginErrorMarker() {}

// NoTenantMatchError — 404 no_tenant_match. Auto-resolution found nothing
// and no invite token was supplied.
type NoTenantMatchError struct{}

func (e *NoTenantMatchError) Error() string             { return "NoTenantMatch" }
func (e *NoTenantMatchError) firebaseLoginErrorMarker() {}

// InvalidFirebaseTokenError — 400 invalid_firebase_token. The supplied
// Firebase ID token failed verification (bad signature, expired, wrong
// audience, etc.).
type InvalidFirebaseTokenError struct{}

func (e *InvalidFirebaseTokenError) Error() string             { return "InvalidFirebaseToken" }
func (e *InvalidFirebaseTokenError) firebaseLoginErrorMarker() {}

// LoginWithFirebase wraps POST /auth/firebase/exchange.
//
// On success the returned session's access token is set on the HTTP client
// for subsequent requests. On well-known failures it returns one of the
// typed errors above; use errors.As to dispatch:
//
//	session, err := client.Auth().LoginWithFirebase(ctx, idToken, opts)
//	var amb *olympus.TenantAmbiguousError
//	if errors.As(err, &amb) {
//	    // render picker from amb.Candidates, retry with TenantSlug set
//	}
func (s *AuthService) LoginWithFirebase(
	ctx context.Context,
	firebaseIDToken string,
	opts *LoginWithFirebaseOptions,
) (*AuthSession, error) {
	body := map[string]interface{}{"firebase_id_token": firebaseIDToken}
	if opts != nil {
		if opts.TenantSlug != "" {
			body["tenant_slug"] = opts.TenantSlug
		}
		if opts.InviteToken != "" {
			body["invite_token"] = opts.InviteToken
		}
	}

	resp, err := s.http.post(ctx, "/auth/firebase/exchange", body)
	if err != nil {
		return nil, mapFirebaseError(err)
	}

	session := parseAuthSession(resp)
	s.http.SetAccessToken(session.AccessToken)
	s.onSessionAcquired(session, false)
	return session, nil
}

// LinkFirebase wraps POST /auth/firebase/link. Idempotent — re-linking the
// same (firebase_uid, caller) returns the ORIGINAL linked_at timestamp.
//
// Returns FirebaseUidAlreadyLinkedError if the Firebase UID is already
// bound to a different Olympus user in the caller's tenant.
func (s *AuthService) LinkFirebase(
	ctx context.Context,
	firebaseIDToken string,
) (*FirebaseLinkResult, error) {
	resp, err := s.http.post(ctx, "/auth/firebase/link", map[string]interface{}{
		"firebase_id_token": firebaseIDToken,
	})
	if err != nil {
		return nil, mapFirebaseError(err)
	}

	result := &FirebaseLinkResult{
		OlympusID:   getString(resp, "olympus_id"),
		FirebaseUID: getString(resp, "firebase_uid"),
	}
	if s := getString(resp, "linked_at"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			result.LinkedAt = t
		} else if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			result.LinkedAt = t
		}
	}
	return result, nil
}

// mapFirebaseError unwraps an OlympusAPIError into a typed Firebase error
// when the response code matches one of the known Firebase failure modes.
// Any other error is returned unchanged.
func mapFirebaseError(err error) error {
	var apiErr *OlympusAPIError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.Code {
	case "multiple_tenants_match":
		return &TenantAmbiguousError{
			Candidates: parseFirebaseCandidates(apiErr.Body),
		}
	case "firebase_uid_already_linked":
		return &FirebaseUidAlreadyLinkedError{
			ExistingOlympusID: extractStringField(apiErr.Body, "existing_olympus_id"),
		}
	case "identity_unlinked":
		return &IdentityUnlinkedError{
			SignupURL: extractStringField(apiErr.Body, "signup_url"),
			Hint:      extractStringField(apiErr.Body, "hint"),
		}
	case "no_tenant_match":
		return &NoTenantMatchError{}
	case "invalid_firebase_token":
		return &InvalidFirebaseTokenError{}
	}
	return err
}

// parseFirebaseCandidates pulls a candidate list from a serialized error
// envelope. Tolerates both `{"candidates": [...]}` and
// `{"error": {"candidates": [...]}}` shapes.
func parseFirebaseCandidates(rawBody []byte) []FirebaseTenantOption {
	if len(rawBody) == 0 {
		return nil
	}
	var envelope struct {
		Candidates []FirebaseTenantOption `json:"candidates"`
		Error      struct {
			Candidates []FirebaseTenantOption `json:"candidates"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		return nil
	}
	if len(envelope.Candidates) > 0 {
		return envelope.Candidates
	}
	return envelope.Error.Candidates
}

// extractStringField pulls a top-level or `error`-nested string field from
// a serialized error envelope.
func extractStringField(rawBody []byte, key string) string {
	if len(rawBody) == 0 {
		return ""
	}
	var top map[string]interface{}
	if err := json.Unmarshal(rawBody, &top); err != nil {
		return ""
	}
	if v, ok := top[key].(string); ok {
		return v
	}
	if e, ok := top["error"].(map[string]interface{}); ok {
		if v, ok := e[key].(string); ok {
			return v
		}
	}
	return ""
}
