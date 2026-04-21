package olympus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --------------------------------------------------------------------------
// TenantService.Create
// --------------------------------------------------------------------------

func TestTenant_Create_PostsFullPayload(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/tenant/create": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{
				"tenant": map[string]interface{}{
					"id":        "tenant-1",
					"slug":      "pizza-palace",
					"name":      "Pizza Palace",
					"is_active": true,
				},
				"admin_user_id": "user-1",
				"session": map[string]interface{}{
					"access_token": "",
				},
				"installed_apps": []interface{}{
					map[string]interface{}{
						"app_id":       "order-echo",
						"status":       "installed",
						"installed_at": "2026-04-21T10:00:00Z",
					},
				},
				"idempotent": false,
			})
		},
	})

	out, err := client.Tenant().Create(context.Background(), TenantCreateRequest{
		BrandName: "Pizza Palace",
		Slug:      "pizza-palace",
		Plan:      "starter",
		FirstAdmin: TenantFirstAdmin{
			Email:        "admin@pizza.test",
			FirstName:    "Pat",
			LastName:     "Pizzaiolo",
			FirebaseLink: "fb-uid-abc",
		},
		InstallApps:    []string{"order-echo"},
		IdempotencyKey: "fb-uid-abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %s, want POST", gotMethod)
	}
	if gotPath != "/tenant/create" {
		t.Errorf("path: got %s, want /tenant/create", gotPath)
	}
	// Canonical field names — verify snake_case tags match server.
	if gotBody["brand_name"] != "Pizza Palace" {
		t.Errorf("brand_name not sent: %v", gotBody)
	}
	if gotBody["idempotency_key"] != "fb-uid-abc" {
		t.Errorf("idempotency_key not sent: %v", gotBody)
	}
	admin, ok := gotBody["first_admin"].(map[string]interface{})
	if !ok {
		t.Fatalf("first_admin missing: %v", gotBody)
	}
	if admin["firebase_link"] != "fb-uid-abc" {
		t.Errorf("firebase_link not sent: %v", admin)
	}
	if admin["first_name"] != "Pat" {
		t.Errorf("first_name wrong: %v", admin)
	}
	installs, ok := gotBody["install_apps"].([]interface{})
	if !ok || len(installs) != 1 || installs[0] != "order-echo" {
		t.Errorf("install_apps wrong: %v", gotBody)
	}
	if out.Tenant.ID != "tenant-1" || out.Tenant.Slug != "pizza-palace" {
		t.Errorf("tenant not parsed: %+v", out.Tenant)
	}
	if len(out.InstalledApps) != 1 || out.InstalledApps[0].AppID != "order-echo" {
		t.Errorf("installed_apps not parsed: %+v", out.InstalledApps)
	}
	if out.Idempotent != false {
		t.Errorf("idempotent should be false on fresh create")
	}
}

func TestTenant_Create_RequiresCoreFields(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	cases := []struct {
		name string
		req  TenantCreateRequest
	}{
		{"no brand", TenantCreateRequest{Slug: "s", Plan: "starter", FirstAdmin: TenantFirstAdmin{Email: "a@b"}, IdempotencyKey: "k"}},
		{"no slug", TenantCreateRequest{BrandName: "b", Plan: "starter", FirstAdmin: TenantFirstAdmin{Email: "a@b"}, IdempotencyKey: "k"}},
		{"no plan", TenantCreateRequest{BrandName: "b", Slug: "s", FirstAdmin: TenantFirstAdmin{Email: "a@b"}, IdempotencyKey: "k"}},
		{"no admin email", TenantCreateRequest{BrandName: "b", Slug: "s", Plan: "starter", IdempotencyKey: "k"}},
		{"no idempotency key", TenantCreateRequest{BrandName: "b", Slug: "s", Plan: "starter", FirstAdmin: TenantFirstAdmin{Email: "a@b"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.Tenant().Create(context.Background(), tc.req)
			if err == nil {
				t.Fatalf("expected client-side validation error, got nil")
			}
			if !strings.Contains(err.Error(), "olympus-sdk:") {
				t.Errorf("error should be SDK-authored: %v", err)
			}
		})
	}
}

func TestTenant_Create_IdempotentRetryParses(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/tenant/create": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"tenant": map[string]interface{}{
					"id":   "tenant-1",
					"slug": "pizza-palace",
					"name": "Pizza Palace",
				},
				"admin_user_id":  "",
				"session":        map[string]interface{}{},
				"installed_apps": []interface{}{},
				"idempotent":     true,
			})
		},
	})
	out, err := client.Tenant().Create(context.Background(), TenantCreateRequest{
		BrandName:      "Pizza Palace",
		Slug:           "pizza-palace",
		Plan:           "starter",
		FirstAdmin:     TenantFirstAdmin{Email: "a@b.test", FirstName: "A", LastName: "B"},
		IdempotencyKey: "k",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Idempotent {
		t.Errorf("expected idempotent=true, got false")
	}
}

// --------------------------------------------------------------------------
// TenantService.Current / Update
// --------------------------------------------------------------------------

func TestTenant_Current_Get(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/tenant/current": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			jsonResponse(w, 200, map[string]interface{}{
				"id":        "tenant-1",
				"slug":      "pizza-palace",
				"name":      "Pizza Palace",
				"is_active": true,
				"settings":  map[string]interface{}{"plan": "starter"},
			})
		},
	})
	out, err := client.Tenant().Current(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID != "tenant-1" || out.Slug != "pizza-palace" {
		t.Errorf("tenant not parsed: %+v", out)
	}
	if out.Settings["plan"] != "starter" {
		t.Errorf("settings not parsed: %+v", out.Settings)
	}
}

func TestTenant_Update_PatchesOnlyProvidedFields(t *testing.T) {
	var gotMethod string
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/tenant/current": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{
				"id":   "tenant-1",
				"name": "Pizza Palace Deluxe",
				"slug": "pizza-palace",
			})
		},
	})
	out, err := client.Tenant().Update(context.Background(), TenantUpdate{
		BrandName: "Pizza Palace Deluxe",
		Plan:      "pro",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("expected PATCH, got %s", gotMethod)
	}
	if gotBody["brand_name"] != "Pizza Palace Deluxe" {
		t.Errorf("brand_name not sent: %v", gotBody)
	}
	if gotBody["plan"] != "pro" {
		t.Errorf("plan not sent: %v", gotBody)
	}
	if _, has := gotBody["locale"]; has {
		t.Errorf("unset locale should not be sent (omitempty): %v", gotBody)
	}
	if out.Name != "Pizza Palace Deluxe" {
		t.Errorf("updated tenant not parsed: %+v", out)
	}
}

// --------------------------------------------------------------------------
// TenantService.Retire / Unretire
// --------------------------------------------------------------------------

func TestTenant_Retire_RequiresConfirmationSlug(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	_, err := client.Tenant().Retire(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty confirmation_slug")
	}
}

func TestTenant_Retire_SendsConfirmationSlug(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/tenant/retire": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{
				"tenant_id":         "tenant-1",
				"retired_at":        "2026-04-21T10:00:00Z",
				"purge_eligible_at": "2026-05-21T10:00:00Z",
			})
		},
	})
	out, err := client.Tenant().Retire(context.Background(), "pizza-palace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["confirmation_slug"] != "pizza-palace" {
		t.Errorf("confirmation_slug not sent: %v", gotBody)
	}
	if out.TenantID != "tenant-1" {
		t.Errorf("tenant_id not parsed: %+v", out)
	}
	if out.RetiredAt == nil || out.PurgeEligibleAt == nil {
		t.Errorf("timestamps not parsed: %+v", out)
	}
}

func TestTenant_Retire_MFARequiredPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/tenant/retire": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 403, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "FORBIDDEN",
					"message": "mfa_required",
				},
			})
		},
	})
	_, err := client.Tenant().Retire(context.Background(), "pizza-palace")
	if err == nil {
		t.Fatal("expected 403 error, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected *OlympusAPIError, got %T", err)
	}
	if !apiErr.IsForbidden() || apiErr.Message != "mfa_required" {
		t.Errorf("mfa_required signal not surfaced: %+v", apiErr)
	}
}

func TestTenant_Unretire_Roundtrips(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/tenant/unretire": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			jsonResponse(w, 200, map[string]interface{}{
				"tenant_id":    "tenant-1",
				"unretired_at": "2026-04-21T11:00:00Z",
			})
		},
	})
	out, err := client.Tenant().Unretire(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TenantID != "tenant-1" {
		t.Errorf("tenant_id not parsed: %+v", out)
	}
	if out.UnretiredAt == nil {
		t.Errorf("unretired_at not parsed")
	}
}

// --------------------------------------------------------------------------
// TenantService.MyTenants — both bare array + envelope responses
// --------------------------------------------------------------------------

func TestTenant_MyTenants_BareArrayResponse(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/tenant/mine": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = io.WriteString(w, `[
				{"tenant_id":"t-1","slug":"pizza-palace","name":"Pizza Palace","role":"tenant_admin"},
				{"tenant_id":"t-2","slug":"bar-olympus","name":"Bar Olympus"}
			]`)
		},
	})
	out, err := client.Tenant().MyTenants(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(out))
	}
	if out[0].TenantID != "t-1" || out[0].Role != "tenant_admin" {
		t.Errorf("first tenant not parsed: %+v", out[0])
	}
	if out[1].Slug != "bar-olympus" {
		t.Errorf("second tenant not parsed: %+v", out[1])
	}
}

func TestTenant_MyTenants_EnvelopeItemsResponse(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/tenant/mine": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"tenant_id": "t-1", "slug": "s-1", "name": "N-1"},
				},
			})
		},
	})
	out, err := client.Tenant().MyTenants(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].TenantID != "t-1" {
		t.Errorf("envelope items not parsed: %+v", out)
	}
}

// --------------------------------------------------------------------------
// TenantService.SwitchTenant
// --------------------------------------------------------------------------

func TestTenant_SwitchTenant_InstallsTokenOnClient(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/auth/switch-tenant": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			// Mirrors rust-auth `TokenResponse`: top-level {access_token,
			// refresh_token, token_type, expires_in}; user details nest
			// under `user`. See backend/rust/auth/src/models.rs:366.
			jsonResponse(w, 200, map[string]interface{}{
				"access_token":  "new-access-xyz",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "new-refresh-xyz",
				"user": map[string]interface{}{
					"id":        "user-1",
					"tenant_id": "tenant-2",
					"email":     "admin@pizza.test",
					"roles":     []string{"tenant_admin"},
				},
			})
		},
	})
	out, err := client.Tenant().SwitchTenant(context.Background(), "tenant-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["tenant_id"] != "tenant-2" {
		t.Errorf("tenant_id not sent: %v", gotBody)
	}
	if out.AccessToken != "new-access-xyz" || out.RefreshToken != "new-refresh-xyz" {
		t.Errorf("session not parsed: %+v", out)
	}
	if out.AccessExpiresAt == nil {
		t.Errorf("expires_at not computed")
	} else if time.Until(*out.AccessExpiresAt) > time.Hour+time.Minute || time.Until(*out.AccessExpiresAt) < 30*time.Minute {
		t.Errorf("expires_at drift out of tolerance: %v", *out.AccessExpiresAt)
	}
	// Access token must be installed on the httpClient for subsequent calls.
	if got := client.HTTPClient().GetAccessToken(); got != "new-access-xyz" {
		t.Errorf("access token not installed on http client: got %q", got)
	}
}

func TestTenant_SwitchTenant_RequiresTenantID(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	_, err := client.Tenant().SwitchTenant(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty tenant_id")
	}
}

func TestTenant_SwitchTenant_ForbiddenPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/auth/switch-tenant": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 403, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "FORBIDDEN",
					"message": "You don't have access to this tenant",
				},
			})
		},
	})
	_, err := client.Tenant().SwitchTenant(context.Background(), "tenant-2")
	if err == nil {
		t.Fatal("expected 403 error, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok || !apiErr.IsForbidden() {
		t.Errorf("expected forbidden OlympusAPIError, got %T: %v", err, err)
	}
}

// --------------------------------------------------------------------------
// Client accessor
// --------------------------------------------------------------------------

func TestClient_Tenant_IsCached(t *testing.T) {
	client := NewClient(Config{AppID: "test", APIKey: "key"})
	if client.Tenant() == nil {
		t.Fatal("Tenant() returned nil")
	}
	if client.Tenant() != client.Tenant() {
		t.Error("Tenant() not cached")
	}
}
