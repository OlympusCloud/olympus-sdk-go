package olympus

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ----------------------------------------------------------------------------
// Test helpers
// ----------------------------------------------------------------------------

func testClient(t *testing.T, serverURL string) *OlympusClient {
	t.Helper()
	return NewClient(Config{
		AppID:   "test-app",
		APIKey:  "oc_test",
		BaseURL: serverURL,
	})
}

func b64urlNoPad(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

func makeJWT(claims map[string]interface{}) string {
	header := b64urlNoPad([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, _ := json.Marshal(claims)
	return header + "." + b64urlNoPad(payload) + ".sig-placeholder"
}

func makeBitset(bits []int, sizeBytes int) string {
	bs := make([]byte, sizeBytes)
	for _, b := range bits {
		bs[b/8] |= 1 << (b % 8)
	}
	return b64urlNoPad(bs)
}

// ----------------------------------------------------------------------------
// Client helpers
// ----------------------------------------------------------------------------

func TestConsentAndGovernanceAccessors(t *testing.T) {
	oc := testClient(t, "http://ignored")
	if oc.Consent() == nil {
		t.Error("Consent() returned nil")
	}
	if oc.Governance() == nil {
		t.Error("Governance() returned nil")
	}
}

func TestHasScopeBit_NoToken(t *testing.T) {
	oc := testClient(t, "http://ignored")
	if oc.HasScopeBit(0) {
		t.Error("expected false without token")
	}
	if oc.HasScopeBit(-1) {
		t.Error("expected false for negative bitID")
	}
	if oc.HasScopeBit(2048) {
		t.Error("expected false for out-of-range bitID without token")
	}
}

func TestHasScopeBit_PlatformShellToken(t *testing.T) {
	oc := testClient(t, "http://ignored")
	token := makeJWT(map[string]interface{}{
		"sub": "u", "tenant_id": "t", "session_id": "s",
		"roles": []string{"tenant_admin"},
		"iat":   0, "exp": 9999999999, "iss": "i", "aud": "a",
	})
	oc.SetAccessToken(token)
	if oc.HasScopeBit(0) {
		t.Error("platform-shell token should have no bitset")
	}
	if oc.IsAppScoped() {
		t.Error("platform-shell token should not be app-scoped")
	}
}

func TestHasScopeBit_AppScopedToken(t *testing.T) {
	oc := testClient(t, "http://ignored")
	bitset := makeBitset([]int{0, 7, 8, 127, 1023}, 128)
	token := makeJWT(map[string]interface{}{
		"sub": "u", "tenant_id": "t", "session_id": "s",
		"roles":                    []string{"staff"},
		"app_id":                   "pizza-os",
		"app_scopes_bitset":        bitset,
		"platform_catalog_digest":  "d1",
		"app_catalog_digest":       "d2",
		"iat": 0, "exp": 9999999999, "iss": "i", "aud": "a",
	})
	oc.SetAccessToken(token)

	if !oc.IsAppScoped() {
		t.Fatal("expected IsAppScoped = true")
	}
	setBits := []int{0, 7, 8, 127, 1023}
	for _, b := range setBits {
		if !oc.HasScopeBit(b) {
			t.Errorf("expected bit %d set", b)
		}
	}
	unsetBits := []int{1, 6, 9, 500}
	for _, b := range unsetBits {
		if oc.HasScopeBit(b) {
			t.Errorf("expected bit %d unset", b)
		}
	}
	if oc.HasScopeBit(2048) {
		t.Error("out-of-range bit should be false")
	}
	if oc.HasScopeBit(-1) {
		t.Error("negative bit should be false")
	}
}

func TestBitsetCacheInvalidatedOnTokenChange(t *testing.T) {
	oc := testClient(t, "http://ignored")
	tokenA := makeJWT(map[string]interface{}{
		"sub": "u", "tenant_id": "t", "session_id": "s", "roles": []string{},
		"app_id": "a", "app_scopes_bitset": makeBitset([]int{0}, 128),
		"iat": 0, "exp": 9999999999, "iss": "i", "aud": "a",
	})
	tokenB := makeJWT(map[string]interface{}{
		"sub": "u", "tenant_id": "t", "session_id": "s", "roles": []string{},
		"app_id": "b", "app_scopes_bitset": makeBitset([]int{5}, 128),
		"iat": 0, "exp": 9999999999, "iss": "i", "aud": "a",
	})
	oc.SetAccessToken(tokenA)
	if !oc.HasScopeBit(0) || oc.HasScopeBit(5) {
		t.Fatal("tokenA should have bit 0, not 5")
	}
	oc.SetAccessToken(tokenB)
	if oc.HasScopeBit(0) || !oc.HasScopeBit(5) {
		t.Fatal("tokenB should have bit 5, not 0")
	}
}

// ----------------------------------------------------------------------------
// Typed error dispatch
// ----------------------------------------------------------------------------

func TestErrorRouter_ConsentRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"scope_not_granted","message":"commerce.order.write required","scope":"commerce.order.write@tenant","consent_url":"https://platform/authorize"}}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Consent().ListGranted(context.Background(), ListGrantedParams{AppID: "pizza-os"})
	if err == nil {
		t.Fatal("expected error")
	}
	var cre *ConsentRequiredError
	if !errors.As(err, &cre) {
		t.Fatalf("expected *ConsentRequiredError, got %T", err)
	}
	if cre.Scope != "commerce.order.write@tenant" {
		t.Errorf("wrong scope: %q", cre.Scope)
	}
	if cre.ConsentURL != "https://platform/authorize" {
		t.Errorf("wrong consent_url: %q", cre.ConsentURL)
	}
}

func TestErrorRouter_BillingGraceWithHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Olympus-Grace-Until", "2026-04-25T00:00:00Z")
		w.Header().Set("X-Olympus-Upgrade-URL", "https://billing/upgrade")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"code":"billing_grace_exceeded","message":"lapsed"}}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Governance().ListExceptions(context.Background(), ListExceptionsParams{})
	if err == nil {
		t.Fatal("expected error")
	}
	var bge *BillingGraceExceededError
	if !errors.As(err, &bge) {
		t.Fatalf("expected *BillingGraceExceededError, got %T", err)
	}
	if bge.GraceUntil != "2026-04-25T00:00:00Z" {
		t.Errorf("wrong grace_until: %q", bge.GraceUntil)
	}
	if bge.UpgradeURL != "https://billing/upgrade" {
		t.Errorf("wrong upgrade_url: %q", bge.UpgradeURL)
	}
}

func TestErrorRouter_DeviceChanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"webauthn_required","message":"new device","challenge":"abc"},"requires_reconsent":true}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Consent().Describe(context.Background(), DescribeParams{AppID: "aura-ai", Scope: "aura.calendar.read@user"})
	if err == nil {
		t.Fatal("expected error")
	}
	var dc *DeviceChangedError
	if !errors.As(err, &dc) {
		t.Fatalf("expected *DeviceChangedError, got %T", err)
	}
	if dc.Challenge != "abc" {
		t.Errorf("wrong challenge: %q", dc.Challenge)
	}
	if !dc.RequiresReconsent {
		t.Error("requires_reconsent not preserved")
	}
}

// ----------------------------------------------------------------------------
// Stale catalog header
// ----------------------------------------------------------------------------

func TestOnCatalogStale_FiresOnHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Olympus-Catalog-Stale", "true")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"grants":[]}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	fired := false
	oc.OnCatalogStale(func() { fired = true })

	_, err := oc.Consent().ListGranted(context.Background(), ListGrantedParams{AppID: "pizza-os"})
	if err != nil {
		t.Fatal(err)
	}
	if !fired {
		t.Error("handler not fired on stale header")
	}
}

func TestOnCatalogStale_NotFiredWithoutHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"grants":[]}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	fired := false
	oc.OnCatalogStale(func() { fired = true })

	_, err := oc.Consent().ListGranted(context.Background(), ListGrantedParams{AppID: "pizza-os"})
	if err != nil {
		t.Fatal(err)
	}
	if fired {
		t.Error("handler should not fire without stale header")
	}
}

// ----------------------------------------------------------------------------
// X-App-Token attachment
// ----------------------------------------------------------------------------

func TestAppTokenAttached(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-App-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"grants":[]}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	oc.SetAppToken("app-jwt-xyz")
	_, _ = oc.Consent().ListGranted(context.Background(), ListGrantedParams{AppID: "a"})
	if gotHeader != "app-jwt-xyz" {
		t.Errorf("expected X-App-Token=app-jwt-xyz, got %q", gotHeader)
	}
}

func TestAppTokenNotAttachedWhenUnset(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-App-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"grants":[]}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, _ = oc.Consent().ListGranted(context.Background(), ListGrantedParams{AppID: "a"})
	if gotHeader != "" {
		t.Errorf("expected no X-App-Token, got %q", gotHeader)
	}
}

// ----------------------------------------------------------------------------
// ConsentService URL construction
// ----------------------------------------------------------------------------

func TestConsent_ListGrantedTenantPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"grants":[]}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, _ = oc.Consent().ListGranted(context.Background(), ListGrantedParams{AppID: "pizza-os"})
	if gotPath != "/api/v1/platform/apps/pizza-os/tenant-grants" {
		t.Errorf("wrong path: %q", gotPath)
	}
}

func TestConsent_ListGrantedUserPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"grants":[]}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, _ = oc.Consent().ListGranted(context.Background(), ListGrantedParams{AppID: "aura-ai", Holder: "user"})
	if gotPath != "/api/v1/platform/apps/aura-ai/user-grants" {
		t.Errorf("wrong path: %q", gotPath)
	}
}

func TestConsent_GrantSendsPromptHash(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"tenant_id":"t","app_id":"aura","scope":"aura.calendar.read@user","granted_at":"2026-04-18T00:00:00Z","source":"admin_ui"}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Consent().Grant(context.Background(), GrantParams{
		AppID: "aura-ai", Scope: "aura.calendar.read@user",
		Holder: "user", PromptHash: "abc123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["consent_prompt_hash"] != "abc123" {
		t.Errorf("consent_prompt_hash not sent: %v", gotBody)
	}
}

// ----------------------------------------------------------------------------
// GovernanceService filter params
// ----------------------------------------------------------------------------

func TestGovernance_ListExceptionsFilters(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"exceptions":[]}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, _ = oc.Governance().ListExceptions(context.Background(), ListExceptionsParams{
		AppID: "aura-ai", Status: "approved",
	})
	if !strings.Contains(gotQuery, "app_id=aura-ai") || !strings.Contains(gotQuery, "status=approved") {
		t.Errorf("query missing filters: %q", gotQuery)
	}
}
