package olympus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ============================================================================
// App-scoped permissions errors (olympus-cloud-gcp#3234 epic / #3254 issue).
// See docs/platform/APP-SCOPED-PERMISSIONS.md §6 + §17.7.
//
// All typed error subclasses embed *OlympusAPIError so consumers can either
// assert the base type or the specific subclass via errors.As.
// ============================================================================

// ConsentRequiredError is raised when a request targets a scope the user has
// not granted. Consumers should route the user to ConsentURL (when present)
// for the platform-served consent flow, then retry the original call.
type ConsentRequiredError struct {
	*OlympusAPIError
	// Scope is the canonical scope string required, e.g. "pizza.menu.read@tenant".
	Scope string
	// ConsentURL is the platform-served authorization URL (/platform/authorize?...).
	ConsentURL string
}

func (e *ConsentRequiredError) Error() string {
	return fmt.Sprintf("ConsentRequired(%s): %s [status=%d, consent=%s]",
		e.Scope, e.Message, e.StatusCode, e.ConsentURL)
}

// Unwrap exposes the underlying OlympusAPIError for errors.As traversal.
func (e *ConsentRequiredError) Unwrap() error { return e.OlympusAPIError }

// ScopeDeniedError is raised when the scope IS granted but the bitset check
// still fails — typically indicates a stale JWT from before a scope revoke.
// Caller should refresh the access token and retry once.
type ScopeDeniedError struct {
	*OlympusAPIError
	Scope string
}

func (e *ScopeDeniedError) Error() string {
	return fmt.Sprintf("ScopeDenied(%s): %s [status=%d]",
		e.Scope, e.Message, e.StatusCode)
}

func (e *ScopeDeniedError) Unwrap() error { return e.OlympusAPIError }

// BillingGraceExceededError is raised when the tenant's entitlement for this
// app is in a grace state that blocks the requested action.
type BillingGraceExceededError struct {
	*OlympusAPIError
	// GraceUntil is the timestamp after which lapsed state transitions to cancelled.
	GraceUntil string
	// UpgradeURL is the billing surface for resolving payment.
	UpgradeURL string
}

func (e *BillingGraceExceededError) Error() string {
	return fmt.Sprintf("BillingGraceExceeded: %s [status=%d, graceUntil=%s, upgrade=%s]",
		e.Message, e.StatusCode, e.GraceUntil, e.UpgradeURL)
}

func (e *BillingGraceExceededError) Unwrap() error { return e.OlympusAPIError }

// DeviceChangedError is raised when the platform detects a device-fingerprint
// change and requires a fresh WebAuthn assertion before issuing an @user-scope
// token. Launch the platform WebAuthn challenge flow and retry.
type DeviceChangedError struct {
	*OlympusAPIError
	// Challenge is the WebAuthn challenge to present to the authenticator.
	Challenge string
	// RequiresReconsent is true when the triggering scope is destructive —
	// an additional platform-served consent screen is also required.
	RequiresReconsent bool
}

func (e *DeviceChangedError) Error() string {
	return fmt.Sprintf("DeviceChanged: %s [status=%d, reconsent=%v]",
		e.Message, e.StatusCode, e.RequiresReconsent)
}

func (e *DeviceChangedError) Unwrap() error { return e.OlympusAPIError }

// ExceptionRequestInvalidError is raised when a policy exception request is
// rejected by the platform (schema / rate-limit / state conflict).
type ExceptionRequestInvalidError struct {
	*OlympusAPIError
	Reason string
}

func (e *ExceptionRequestInvalidError) Error() string {
	return fmt.Sprintf("ExceptionRequestInvalid(%s): %s [status=%d]",
		e.Reason, e.Message, e.StatusCode)
}

func (e *ExceptionRequestInvalidError) Unwrap() error { return e.OlympusAPIError }

// ExceptionExpiredError is raised when an approved exception has transitioned
// to the `expired` terminal state. Consumers needing continued deviation MUST
// file a new exception (§17.5 — renewal is a new request, not a mutation).
type ExceptionExpiredError struct {
	*OlympusAPIError
	ExceptionID string
}

func (e *ExceptionExpiredError) Error() string {
	return fmt.Sprintf("ExceptionExpired(%s): %s [status=%d]",
		e.ExceptionID, e.Message, e.StatusCode)
}

func (e *ExceptionExpiredError) Unwrap() error { return e.OlympusAPIError }

// ----------------------------------------------------------------------------
// Server error code routing
// ----------------------------------------------------------------------------

// routeAppScopedError inspects an APIError + response body and headers and
// returns a typed subclass when the server's error code matches one of the
// app-scoped categories. Returns nil otherwise — the caller uses the base
// OlympusAPIError.
func routeAppScopedError(base *OlympusAPIError, body []byte, headers http.Header) error {
	if base == nil {
		return nil
	}
	code := strings.ToLower(base.Code)

	switch code {
	case "scope_not_granted", "consent_required":
		return &ConsentRequiredError{
			OlympusAPIError: base,
			Scope:           extractErrField(body, "scope"),
			ConsentURL: coalesce(
				extractErrField(body, "consent_url"),
				headers.Get("X-Olympus-Consent-URL"),
			),
		}
	case "scope_denied":
		return &ScopeDeniedError{
			OlympusAPIError: base,
			Scope:           extractErrField(body, "scope"),
		}
	case "billing_grace_exceeded":
		return &BillingGraceExceededError{
			OlympusAPIError: base,
			GraceUntil: coalesce(
				extractErrField(body, "grace_until"),
				headers.Get("X-Olympus-Grace-Until"),
			),
			UpgradeURL: coalesce(
				extractErrField(body, "upgrade_url"),
				headers.Get("X-Olympus-Upgrade-URL"),
			),
		}
	case "webauthn_required", "device_changed":
		return &DeviceChangedError{
			OlympusAPIError:   base,
			Challenge:         extractErrField(body, "challenge"),
			RequiresReconsent: extractErrBoolField(body, "requires_reconsent"),
		}
	case "exception_request_invalid":
		return &ExceptionRequestInvalidError{
			OlympusAPIError: base,
			Reason:          extractErrField(body, "reason"),
		}
	case "exception_expired":
		return &ExceptionExpiredError{
			OlympusAPIError: base,
			ExceptionID:     extractErrField(body, "exception_id"),
		}
	}

	return nil
}

// extractErrField pulls a string field from the response body — checking both
// the top level and the nested {"error": {...}} envelope.
func extractErrField(body []byte, key string) string {
	if len(body) == 0 {
		return ""
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	if v, ok := parsed[key].(string); ok {
		return v
	}
	if errMap, ok := parsed["error"].(map[string]interface{}); ok {
		if v, ok := errMap[key].(string); ok {
			return v
		}
	}
	return ""
}

// extractErrBoolField — symmetric version of extractErrField for bool fields.
func extractErrBoolField(body []byte, key string) bool {
	if len(body) == 0 {
		return false
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	if v, ok := parsed[key].(bool); ok {
		return v
	}
	if errMap, ok := parsed["error"].(map[string]interface{}); ok {
		if v, ok := errMap[key].(bool); ok {
			return v
		}
	}
	return false
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
