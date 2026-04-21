package olympus

import (
	"context"
	"fmt"
)

// AuthService handles authentication, user management, and API key operations.
//
// Wraps the Olympus Auth service (Rust) via the Go API Gateway.
// Routes: /auth/*, /platform/users/*.
type AuthService struct {
	http *httpClient
}

// Login authenticates with email and password. On success the returned
// session's access token is automatically set on the HTTP client for
// subsequent requests.
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

// Refresh exchanges a refresh token for a new token pair.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*AuthSession, error) {
	resp, err := s.http.post(ctx, "/auth/refresh", map[string]interface{}{
		"refresh_token": refreshToken,
	})
	if err != nil {
		return nil, err
	}

	session := parseAuthSession(resp)
	s.http.SetAccessToken(session.AccessToken)
	return session, nil
}

// Logout invalidates the current session and clears the access token.
func (s *AuthService) Logout(ctx context.Context) error {
	_, err := s.http.post(ctx, "/auth/logout", nil)
	s.http.ClearAccessToken()
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
