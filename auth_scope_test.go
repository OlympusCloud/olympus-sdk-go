package olympus

import (
	"errors"
	"testing"
)

// ----------------------------------------------------------------------------
// AuthService.HasScope / RequireScope / GrantedScopes (olympus-cloud-gcp#3403 §1.2).
// ----------------------------------------------------------------------------

// scopeTestToken builds a JWT whose payload carries the provided canonical
// scope strings in the `app_scopes` claim.
func scopeTestToken(scopes []string) string {
	claims := map[string]interface{}{
		"sub":        "u",
		"tenant_id":  "t",
		"session_id": "s",
		"iat":        0,
		"exp":        9999999999,
		"iss":        "i",
		"aud":        "a",
	}
	if scopes != nil {
		arr := make([]interface{}, 0, len(scopes))
		for _, s := range scopes {
			arr = append(arr, s)
		}
		claims["app_scopes"] = arr
	}
	return makeJWT(claims)
}

func TestAuthService_GrantedScopes_NoToken(t *testing.T) {
	oc := testClient(t, "http://ignored")
	if got := oc.Auth().GrantedScopes(); got != nil {
		t.Errorf("expected nil without token, got %v", got)
	}
}

func TestAuthService_GrantedScopes_PlatformShellToken(t *testing.T) {
	oc := testClient(t, "http://ignored")
	oc.SetAccessToken(scopeTestToken(nil)) // no app_scopes claim
	if got := oc.Auth().GrantedScopes(); got != nil {
		t.Errorf("platform-shell token should yield nil GrantedScopes, got %v", got)
	}
}

func TestAuthService_GrantedScopes_NonJWT(t *testing.T) {
	oc := testClient(t, "http://ignored")
	oc.SetAccessToken("not-a-jwt")
	if got := oc.Auth().GrantedScopes(); got != nil {
		t.Errorf("non-JWT token should yield nil GrantedScopes, got %v", got)
	}
}

func TestAuthService_GrantedScopes_AppScopedToken(t *testing.T) {
	oc := testClient(t, "http://ignored")
	scopes := []string{
		"auth.session.read@user",
		"commerce.order.write@tenant",
		"platform.tenant.read@tenant",
	}
	oc.SetAccessToken(scopeTestToken(scopes))

	got := oc.Auth().GrantedScopes()
	if len(got) != len(scopes) {
		t.Fatalf("expected %d scopes, got %d (%v)", len(scopes), len(got), got)
	}
	for i, want := range scopes {
		if got[i] != want {
			t.Errorf("scope[%d]: expected %q, got %q", i, want, got[i])
		}
	}
}

func TestAuthService_GrantedScopes_ReturnsDefensiveCopy(t *testing.T) {
	oc := testClient(t, "http://ignored")
	oc.SetAccessToken(scopeTestToken([]string{"auth.session.read@user"}))

	first := oc.Auth().GrantedScopes()
	if len(first) == 0 {
		t.Fatal("expected scope slice, got empty")
	}
	// Mutate the returned slice — the helper should not observe this.
	first[0] = "mutated.scope@user"

	second := oc.Auth().GrantedScopes()
	if len(second) != 1 || second[0] != "auth.session.read@user" {
		t.Errorf("GrantedScopes should return a fresh slice; got %v after mutation", second)
	}
}

func TestAuthService_HasScope_Granted(t *testing.T) {
	oc := testClient(t, "http://ignored")
	oc.SetAccessToken(scopeTestToken([]string{
		"auth.session.read@user",
		"commerce.order.write@tenant",
	}))
	if !oc.Auth().HasScope("commerce.order.write@tenant") {
		t.Error("expected HasScope = true for granted scope")
	}
}

func TestAuthService_HasScope_NotGranted(t *testing.T) {
	oc := testClient(t, "http://ignored")
	oc.SetAccessToken(scopeTestToken([]string{"auth.session.read@user"}))
	if oc.Auth().HasScope("commerce.order.write@tenant") {
		t.Error("expected HasScope = false for ungranted scope")
	}
}

func TestAuthService_HasScope_EmptyString(t *testing.T) {
	oc := testClient(t, "http://ignored")
	oc.SetAccessToken(scopeTestToken([]string{"auth.session.read@user"}))
	if oc.Auth().HasScope("") {
		t.Error("empty-string scope should return false")
	}
}

func TestAuthService_HasScope_NoToken(t *testing.T) {
	oc := testClient(t, "http://ignored")
	if oc.Auth().HasScope("auth.session.read@user") {
		t.Error("expected HasScope = false without token")
	}
}

func TestAuthService_HasScope_UsesGeneratedConstants(t *testing.T) {
	oc := testClient(t, "http://ignored")
	oc.SetAccessToken(scopeTestToken([]string{ScopeAuthSessionReadAtUser}))
	if !oc.Auth().HasScope(ScopeAuthSessionReadAtUser) {
		t.Errorf("expected HasScope(ScopeAuthSessionReadAtUser) = true")
	}
}

func TestAuthService_RequireScope_Granted(t *testing.T) {
	oc := testClient(t, "http://ignored")
	oc.SetAccessToken(scopeTestToken([]string{"commerce.order.write@tenant"}))
	if err := oc.Auth().RequireScope("commerce.order.write@tenant"); err != nil {
		t.Errorf("expected nil error for granted scope, got %v", err)
	}
}

func TestAuthService_RequireScope_NotGranted(t *testing.T) {
	oc := testClient(t, "http://ignored")
	oc.SetAccessToken(scopeTestToken([]string{"auth.session.read@user"}))
	err := oc.Auth().RequireScope("commerce.order.write@tenant")
	if err == nil {
		t.Fatal("expected error for ungranted scope")
	}
	var sre *ScopeRequiredError
	if !errors.As(err, &sre) {
		t.Fatalf("expected *ScopeRequiredError, got %T", err)
	}
	if sre.Scope != "commerce.order.write@tenant" {
		t.Errorf("expected scope %q, got %q", "commerce.order.write@tenant", sre.Scope)
	}
}

func TestAuthService_RequireScope_NoToken(t *testing.T) {
	oc := testClient(t, "http://ignored")
	err := oc.Auth().RequireScope("commerce.order.write@tenant")
	if err == nil {
		t.Fatal("expected error without token")
	}
	var sre *ScopeRequiredError
	if !errors.As(err, &sre) {
		t.Fatalf("expected *ScopeRequiredError, got %T", err)
	}
}

func TestScopeRequiredError_Message(t *testing.T) {
	err := &ScopeRequiredError{Scope: "commerce.order.write@tenant"}
	if got := err.Error(); got != "ScopeRequired: commerce.order.write@tenant" {
		t.Errorf("unexpected Error() output: %q", got)
	}
}

// ----------------------------------------------------------------------------
// parseAuthSession — AppScopes round-trips from server payload + JWT fallback.
// ----------------------------------------------------------------------------

func TestParseAuthSession_PopulatesAppScopesFromEnvelope(t *testing.T) {
	session := parseAuthSession(map[string]interface{}{
		"access_token": "opaque",
		"token_type":   "Bearer",
		"expires_in":   float64(3600),
		"app_scopes": []interface{}{
			"auth.session.read@user",
			"commerce.order.write@tenant",
		},
	})
	if len(session.AppScopes) != 2 {
		t.Fatalf("expected 2 scopes from envelope, got %d", len(session.AppScopes))
	}
	if session.AppScopes[0] != "auth.session.read@user" {
		t.Errorf("unexpected scope[0]: %q", session.AppScopes[0])
	}
}

func TestParseAuthSession_FallsBackToJWTClaim(t *testing.T) {
	token := scopeTestToken([]string{"platform.tenant.read@tenant"})
	session := parseAuthSession(map[string]interface{}{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   float64(3600),
		// no app_scopes in envelope → parser falls back to JWT claim
	})
	if len(session.AppScopes) != 1 || session.AppScopes[0] != "platform.tenant.read@tenant" {
		t.Errorf("expected JWT-fallback scope, got %v", session.AppScopes)
	}
}
