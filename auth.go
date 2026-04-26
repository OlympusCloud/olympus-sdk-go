package olympus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"sync"
)

// AuthService handles authentication, user management, and API key operations.
//
// Wraps the Olympus Auth service (Rust) via the Go API Gateway.
// Routes: /auth/*, /platform/users/*.
//
// Also owns the silent token refresh goroutine + session event stream
// (olympus-cloud-gcp#3412). See silent_refresh.go.
type AuthService struct {
	http *httpClient

	// stateOnce + state lazily initialise the silent-refresh bookkeeping.
	// Keeping this off the critical path (non-refresh users pay nothing).
	stateOnce sync.Once
	state     *sessionState
}

// Login authenticates with email and password. On success the returned
// session's access token is automatically set on the HTTP client for
// subsequent requests. Emits SessionLoggedIn to subscribers and nudges the
// silent-refresh goroutine (if running) to reschedule from the new exp.
func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthSession, error) {
	resp, err := s.http.post(ctx, "/auth/login", map[string]interface{}{
		"email":    email,
		"password": password,
	})
	if err != nil {
		return nil, err
	}

	session := parseAuthSession(resp)
	s.http.SetAccessToken(session.AccessToken)
	s.onSessionAcquired(session, false)
	return session, nil
}

// LoginSSO initiates SSO login via an external provider (e.g., "google", "apple").
func (s *AuthService) LoginSSO(ctx context.Context, provider string) (*AuthSession, error) {
	resp, err := s.http.post(ctx, "/auth/sso/initiate", map[string]interface{}{
		"provider": provider,
	})
	if err != nil {
		return nil, err
	}

	session := parseAuthSession(resp)
	s.http.SetAccessToken(session.AccessToken)
	s.onSessionAcquired(session, false)
	return session, nil
}

// LoginPin authenticates staff using a PIN code.
func (s *AuthService) LoginPin(ctx context.Context, pin string, locationID string) (*AuthSession, error) {
	body := map[string]interface{}{
		"pin": pin,
	}
	if locationID != "" {
		body["location_id"] = locationID
	}

	resp, err := s.http.post(ctx, "/auth/login/pin", body)
	if err != nil {
		return nil, err
	}

	session := parseAuthSession(resp)
	s.http.SetAccessToken(session.AccessToken)
	s.onSessionAcquired(session, false)
	return session, nil
}

// Me returns the currently authenticated user profile.
func (s *AuthService) Me(ctx context.Context) (*User, error) {
	resp, err := s.http.get(ctx, "/auth/me", nil)
	if err != nil {
		return nil, err
	}
	return parseUser(resp), nil
}

// Refresh exchanges a refresh token for a new token pair. Emits
// SessionRefreshed to subscribers and nudges the silent-refresh goroutine
// (if running) to reschedule from the new exp.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*AuthSession, error) {
	resp, err := s.http.post(ctx, "/auth/refresh", map[string]interface{}{
		"refresh_token": refreshToken,
	})
	if err != nil {
		return nil, err
	}

	session := parseAuthSession(resp)
	s.http.SetAccessToken(session.AccessToken)
	s.onSessionAcquired(session, true)
	return session, nil
}

// Logout invalidates the current session, cancels the silent-refresh
// goroutine (if running), clears the access token, and emits
// SessionLoggedOut.
func (s *AuthService) Logout(ctx context.Context) error {
	_, err := s.http.post(ctx, "/auth/logout", nil)
	s.http.ClearAccessToken()
	s.onSessionLoggedOut()
	return err
}

// CreateUserRequest holds the parameters for creating a new user.
type CreateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Password string `json:"password,omitempty"`
}

// CreateUser creates a new user on the platform.
func (s *AuthService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	body := map[string]interface{}{
		"name":  req.Name,
		"email": req.Email,
		"role":  req.Role,
	}
	if req.Password != "" {
		body["password"] = req.Password
	}

	resp, err := s.http.post(ctx, "/platform/users", body)
	if err != nil {
		return nil, err
	}
	return parseUser(resp), nil
}

// AssignRole assigns a role to a user.
func (s *AuthService) AssignRole(ctx context.Context, userID, role string) error {
	_, err := s.http.post(ctx, fmt.Sprintf("/platform/users/%s/roles", userID), map[string]interface{}{
		"role": role,
	})
	return err
}

// AssignRolesRequest is the input to AuthService.AssignRoles.
//
// Mirrors the canonical Dart contract shipped in olympus-sdk-dart#45 (W12-1 /
// olympus-cloud-gcp#3599). GrantScopes / RevokeScopes are sets — duplicates
// are removed and the wire payload is sorted for deterministic JSON.
type AssignRolesRequest struct {
	UserID       string
	TenantID     string
	GrantScopes  []string
	RevokeScopes []string
	Note         string // optional, omitted when empty
}

// AssignRoles assigns or revokes scopes for a user within a tenant.
//
// W12-1 / olympus-cloud-gcp#3599. Server-side: writes the scope mask, fires
// the FCM topic the platform-side IntentBus broker subscribes to so every
// open app on the target user's device sees an `identity.scopes.granted` /
// `identity.scopes.revoked` CrossAppIntent.
//
// Errors (all surfaced as *OlympusAPIError):
//   - 400 ROLES_VALIDATION_ERROR — empty grant + revoke sets, or unknown scope
//   - 403 INSUFFICIENT_PERMISSIONS — caller lacks
//     `platform.founder.roles.assign@tenant`
//   - 404 USER_NOT_FOUND — UserID is not a member of TenantID
func (s *AuthService) AssignRoles(ctx context.Context, req AssignRolesRequest) error {
	body := map[string]interface{}{
		"tenant_id":     req.TenantID,
		"grant_scopes":  dedupSorted(req.GrantScopes),
		"revoke_scopes": dedupSorted(req.RevokeScopes),
	}
	if req.Note != "" {
		body["note"] = req.Note
	}
	_, err := s.http.post(
		ctx,
		fmt.Sprintf("/platform/users/%s/roles/assign", req.UserID),
		body,
	)
	return err
}

// ListTeammatesOptions filters the result of AuthService.ListTeammates.
type ListTeammatesOptions struct {
	// TenantID, when non-empty, restricts results to teammates of the given
	// tenant. When empty the server returns teammates the caller can manage
	// across all tenants they hold the assignment scope in.
	TenantID string
}

// ListTeammates lists teammates the caller can manage.
//
// Server-side filters by caller's `platform.founder.roles.assign` scope.
// Mirrors the canonical Dart contract shipped in olympus-sdk-dart#45
// (W12-1 / olympus-cloud-gcp#3599).
func (s *AuthService) ListTeammates(ctx context.Context, opts ListTeammatesOptions) ([]OlympusTeammate, error) {
	var query url.Values
	if opts.TenantID != "" {
		query = url.Values{}
		query.Set("tenant_id", opts.TenantID)
	}
	// The server may return either { "data": [...] } or a bare array. Try
	// the envelope first (cheap path) and fall back to raw decode otherwise.
	resp, err := s.http.get(ctx, "/platform/teammates", query)
	if err == nil {
		if rows, ok := resp["data"].([]interface{}); ok {
			return parseTeammates(rows), nil
		}
	}
	// Bare-array path.
	raw, rawErr := s.http.getRaw(ctx, "/platform/teammates", query)
	if rawErr != nil {
		// Prefer the structured envelope error if both paths fail.
		if err != nil {
			return nil, err
		}
		return nil, rawErr
	}
	var rows []map[string]interface{}
	if jsonErr := json.Unmarshal(raw, &rows); jsonErr != nil {
		// Maybe it was an envelope after all.
		if err != nil {
			return nil, err
		}
		return nil, jsonErr
	}
	out := make([]OlympusTeammate, 0, len(rows))
	for _, r := range rows {
		out = append(out, parseTeammate(r))
	}
	return out, nil
}

// dedupSorted returns a new slice with duplicates removed and entries sorted
// in ascending order. Returns an empty (non-nil) slice when the input is nil
// so the JSON wire payload always carries `[]` instead of `null`.
func dedupSorted(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func parseTeammates(rows []interface{}) []OlympusTeammate {
	out := make([]OlympusTeammate, 0, len(rows))
	for _, raw := range rows {
		if m, ok := raw.(map[string]interface{}); ok {
			out = append(out, parseTeammate(m))
		}
	}
	return out
}

func parseTeammate(data map[string]interface{}) OlympusTeammate {
	scopes := getStringSlice(data, "assigned_scopes")
	set := make(map[string]struct{}, len(scopes))
	for _, s := range scopes {
		set[s] = struct{}{}
	}
	return OlympusTeammate{
		UserID:         getString(data, "user_id"),
		DisplayName:    getString(data, "display_name"),
		Role:           getString(data, "role"),
		AssignedScopes: set,
	}
}

// CheckPermission checks whether a user has a specific permission.
func (s *AuthService) CheckPermission(ctx context.Context, userID, permission string) (bool, error) {
	resp, err := s.http.get(ctx, fmt.Sprintf("/platform/users/%s/permissions/check", userID), mapToValues(map[string]string{
		"permission": permission,
	}))
	if err != nil {
		return false, err
	}
	return getBool(resp, "allowed"), nil
}

// CreateAPIKey creates a new API key for programmatic access.
func (s *AuthService) CreateAPIKey(ctx context.Context, name string, scopes []string) (*APIKey, error) {
	resp, err := s.http.post(ctx, "/platform/tenants/me/api-keys", map[string]interface{}{
		"name":   name,
		"scopes": scopes,
	})
	if err != nil {
		return nil, err
	}
	return parseAPIKey(resp), nil
}

// RevokeAPIKey revokes an existing API key.
func (s *AuthService) RevokeAPIKey(ctx context.Context, keyID string) error {
	return s.http.del(ctx, fmt.Sprintf("/platform/tenants/me/api-keys/%s", keyID))
}

// SetPin sets or updates a user's PIN.
func (s *AuthService) SetPin(ctx context.Context, userID, pin string) error {
	_, err := s.http.post(ctx, fmt.Sprintf("/auth/users/%s/pin", userID), map[string]interface{}{
		"pin": pin,
	})
	return err
}

// LoginMFA completes an MFA challenge during login.
func (s *AuthService) LoginMFA(ctx context.Context, mfaToken, code string) (*AuthSession, error) {
	resp, err := s.http.post(ctx, "/auth/login/mfa", map[string]interface{}{
		"mfa_token": mfaToken,
		"code":      code,
	})
	if err != nil {
		return nil, err
	}

	session := parseAuthSession(resp)
	s.http.SetAccessToken(session.AccessToken)
	s.onSessionAcquired(session, false)
	return session, nil
}

// Register creates a new user account.
func (s *AuthService) Register(ctx context.Context, email, password, name string) (*User, error) {
	resp, err := s.http.post(ctx, "/auth/register", map[string]interface{}{
		"email":    email,
		"password": password,
		"name":     name,
	})
	if err != nil {
		return nil, err
	}
	return parseUser(resp), nil
}

// ----------------------------------------------------------------------------
// App-scoped permissions — HasScope / RequireScope / GrantedScopes helpers
// (olympus-cloud-gcp#3403 §1.2). Reads scopes from the active access token's
// `app_scopes` JWT claim. 5-language parity with dart / typescript / python /
// rust SDKs.
// ----------------------------------------------------------------------------

// GrantedScopes returns the canonical scope strings granted to the current
// session, decoded from the access token's `app_scopes` JWT claim. Returns
// nil when no token is set, the token isn't a JWT, or the token has no
// app_scopes claim (i.e. a platform-shell token).
//
// The returned slice is a defensive copy — mutating it does not affect
// subsequent HasScope / RequireScope checks.
func (s *AuthService) GrantedScopes() []string {
	token := s.http.GetAccessToken()
	if token == "" {
		return nil
	}
	claims := parseJWTPayload(token)
	if claims == nil {
		return nil
	}
	raw, ok := claims["app_scopes"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if sv, ok := v.(string); ok {
			out = append(out, sv)
		}
	}
	return out
}

// HasScope reports whether the current session carries the given canonical
// scope (format: "auth.session.read@user"). Returns false for the empty
// string, missing token, or non-JWT tokens.
func (s *AuthService) HasScope(scope string) bool {
	if scope == "" {
		return false
	}
	for _, granted := range s.GrantedScopes() {
		if granted == scope {
			return true
		}
	}
	return false
}

// RequireScope returns a *ScopeRequiredError if the scope is not present in
// the current session. Typically used as a client-side pre-flight:
//
//	if err := client.Auth().RequireScope(olympus.ScopeCommerceOrderWriteAtTenant); err != nil {
//	    // route to consent flow
//	}
func (s *AuthService) RequireScope(scope string) error {
	if !s.HasScope(scope) {
		return &ScopeRequiredError{Scope: scope}
	}
	return nil
}
