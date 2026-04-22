package olympus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// --------------------------------------------------------------------------
// AppsService.Install — POST /apps/install
// --------------------------------------------------------------------------

func TestApps_Install_PostsFullPayload(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/install": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{
				"pending_install_id": "pi-uuid-1",
				"consent_url":        "https://platform.olympuscloud.ai/apps/consent/pi-uuid-1",
				"expires_at":         "2026-04-21T10:10:00Z",
			})
		},
	})

	out, err := client.Apps().Install(context.Background(), AppInstallRequest{
		AppID:          "com.pizzaos",
		Scopes:         []string{"pizza.menu.read@tenant", "pizza.orders.write@tenant"},
		ReturnTo:       "https://pizza.test/settings/permissions",
		IdempotencyKey: "device-fp-abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %s, want POST", gotMethod)
	}
	if gotPath != "/apps/install" {
		t.Errorf("path: got %s, want /apps/install", gotPath)
	}
	if gotBody["app_id"] != "com.pizzaos" {
		t.Errorf("app_id not sent: %v", gotBody)
	}
	if gotBody["return_to"] != "https://pizza.test/settings/permissions" {
		t.Errorf("return_to not sent: %v", gotBody)
	}
	if gotBody["idempotency_key"] != "device-fp-abc" {
		t.Errorf("idempotency_key not sent: %v", gotBody)
	}
	scopes, ok := gotBody["scopes"].([]interface{})
	if !ok || len(scopes) != 2 || scopes[0] != "pizza.menu.read@tenant" {
		t.Errorf("scopes not sent: %v", gotBody)
	}
	if out.PendingInstallID != "pi-uuid-1" {
		t.Errorf("pending_install_id not parsed: %+v", out)
	}
	if out.ConsentURL != "https://platform.olympuscloud.ai/apps/consent/pi-uuid-1" {
		t.Errorf("consent_url not parsed: %+v", out)
	}
	if out.ExpiresAt == nil {
		t.Error("expires_at not parsed")
	}
}

func TestApps_Install_OmitsEmptyIdempotencyKey(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/install": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{
				"pending_install_id": "pi-uuid-2",
				"consent_url":        "https://platform.olympuscloud.ai/apps/consent/pi-uuid-2",
				"expires_at":         "2026-04-21T10:10:00Z",
			})
		},
	})

	_, err := client.Apps().Install(context.Background(), AppInstallRequest{
		AppID:    "com.pizzaos",
		Scopes:   []string{"pizza.menu.read@tenant"},
		ReturnTo: "https://pizza.test/return",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := gotBody["idempotency_key"]; present {
		t.Errorf("idempotency_key should be omitted when empty: %v", gotBody)
	}
}

func TestApps_Install_NormalizesNilScopesToEmptyArray(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/install": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{
				"pending_install_id": "pi-uuid-3",
				"consent_url":        "https://platform.olympuscloud.ai/apps/consent/pi-uuid-3",
				"expires_at":         "2026-04-21T10:10:00Z",
			})
		},
	})
	_, err := client.Apps().Install(context.Background(), AppInstallRequest{
		AppID:    "com.pizzaos",
		Scopes:   nil,
		ReturnTo: "https://pizza.test/return",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Server always sees `scopes` as an array (possibly empty) — never null or missing.
	scopes, ok := gotBody["scopes"].([]interface{})
	if !ok {
		t.Fatalf("scopes should be an array (nil normalized to []): %v", gotBody)
	}
	if len(scopes) != 0 {
		t.Errorf("expected empty scopes array, got %v", scopes)
	}
}

func TestApps_Install_RequiresCoreFields(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	cases := []struct {
		name string
		req  AppInstallRequest
	}{
		{"no app_id", AppInstallRequest{Scopes: []string{"s"}, ReturnTo: "https://r"}},
		{"no return_to", AppInstallRequest{AppID: "com.app", Scopes: []string{"s"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.Apps().Install(context.Background(), tc.req)
			if err == nil {
				t.Fatalf("expected client-side validation error, got nil")
			}
			if !strings.Contains(err.Error(), "olympus-sdk:") {
				t.Errorf("error should be SDK-authored: %v", err)
			}
		})
	}
}

func TestApps_Install_MFARequiredPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/install": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 403, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "FORBIDDEN",
					"message": "mfa_required",
				},
			})
		},
	})
	_, err := client.Apps().Install(context.Background(), AppInstallRequest{
		AppID:    "com.pizzaos",
		Scopes:   []string{"pizza.menu.read@tenant"},
		ReturnTo: "https://pizza.test/return",
	})
	if err == nil {
		t.Fatal("expected 403, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected *OlympusAPIError, got %T", err)
	}
	if !apiErr.IsForbidden() || apiErr.Message != "mfa_required" {
		t.Errorf("mfa_required signal not surfaced: %+v", apiErr)
	}
}

// --------------------------------------------------------------------------
// AppsService.ListInstalled — GET /apps/installed
// --------------------------------------------------------------------------

func TestApps_ListInstalled_BareArrayResponse(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/installed": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = io.WriteString(w, `[
				{"tenant_id":"t-1","app_id":"com.pizzaos","installed_at":"2026-04-21T09:00:00Z","installed_by":"user-1","scopes_granted":["pizza.menu.read@tenant"],"status":"active"},
				{"tenant_id":"t-1","app_id":"com.orderecho","installed_at":"2026-04-20T09:00:00Z","installed_by":"user-1","scopes_granted":["orders.read@tenant"],"status":"active"}
			]`)
		},
	})
	out, err := client.Apps().ListInstalled(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 installs, got %d", len(out))
	}
	if out[0].AppID != "com.pizzaos" || out[0].Status != "active" {
		t.Errorf("first install not parsed: %+v", out[0])
	}
	if len(out[0].ScopesGranted) != 1 || out[0].ScopesGranted[0] != "pizza.menu.read@tenant" {
		t.Errorf("scopes_granted not parsed: %+v", out[0])
	}
	if out[0].InstalledAt == nil {
		t.Error("installed_at not parsed")
	}
	if out[1].AppID != "com.orderecho" {
		t.Errorf("second install not parsed: %+v", out[1])
	}
}

func TestApps_ListInstalled_EnvelopeItemsResponse(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/installed": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"tenant_id":      "t-1",
						"app_id":         "com.pizzaos",
						"installed_at":   "2026-04-21T09:00:00Z",
						"installed_by":   "user-1",
						"scopes_granted": []string{"pizza.menu.read@tenant"},
						"status":         "active",
					},
				},
			})
		},
	})
	out, err := client.Apps().ListInstalled(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].AppID != "com.pizzaos" {
		t.Errorf("envelope items not parsed: %+v", out)
	}
}

func TestApps_ListInstalled_UnauthorizedPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/installed": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 401, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "UNAUTHORIZED",
					"message": "Invalid token",
				},
			})
		},
	})
	_, err := client.Apps().ListInstalled(context.Background())
	if err == nil {
		t.Fatal("expected 401, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok || !apiErr.IsUnauthorized() {
		t.Errorf("expected unauthorized OlympusAPIError, got %T: %v", err, err)
	}
}

// --------------------------------------------------------------------------
// AppsService.Uninstall — POST /apps/uninstall/:app_id
// --------------------------------------------------------------------------

func TestApps_Uninstall_PostsToAppPath(t *testing.T) {
	var gotMethod, gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/uninstall/com.pizzaos": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			jsonResponse(w, 200, map[string]interface{}{
				"tenant_id":       "t-1",
				"app_id":          "com.pizzaos",
				"uninstalled_at":  "2026-04-21T12:00:00Z",
				"uninstalled_by":  "user-1",
			})
		},
	})
	err := client.Apps().Uninstall(context.Background(), "com.pizzaos")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %s, want POST", gotMethod)
	}
	if gotPath != "/apps/uninstall/com.pizzaos" {
		t.Errorf("path: got %s, want /apps/uninstall/com.pizzaos", gotPath)
	}
}

func TestApps_Uninstall_RequiresAppID(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	err := client.Apps().Uninstall(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty appID")
	}
}

func TestApps_Uninstall_NotFoundPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/uninstall/com.ghostapp": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 404, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "NOT_FOUND",
					"message": "app not installed on this tenant",
				},
			})
		},
	})
	err := client.Apps().Uninstall(context.Background(), "com.ghostapp")
	if err == nil {
		t.Fatal("expected 404, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok || !apiErr.IsNotFound() {
		t.Errorf("expected not-found OlympusAPIError, got %T: %v", err, err)
	}
}

// --------------------------------------------------------------------------
// AppsService.GetManifest — GET /apps/manifest/:app_id
// --------------------------------------------------------------------------

func TestApps_GetManifest_ParsesFullShape(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/manifest/com.pizzaos": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			jsonResponse(w, 200, map[string]interface{}{
				"app_id":          "com.pizzaos",
				"version":         "1.4.0",
				"name":            "PizzaOS",
				"publisher":       "NebusAI",
				"logo_url":        "https://cdn.olympuscloud.ai/apps/com.pizzaos/logo.png",
				"scopes_required": []string{"pizza.menu.read@tenant", "pizza.orders.write@tenant"},
				"scopes_optional": []string{"pizza.delivery.track@tenant"},
				"privacy_url":     "https://pizzaos.test/privacy",
				"tos_url":         "https://pizzaos.test/tos",
			})
		},
	})
	out, err := client.Apps().GetManifest(context.Background(), "com.pizzaos")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.AppID != "com.pizzaos" || out.Version != "1.4.0" {
		t.Errorf("manifest not parsed: %+v", out)
	}
	if len(out.ScopesRequired) != 2 || out.ScopesRequired[0] != "pizza.menu.read@tenant" {
		t.Errorf("scopes_required not parsed: %+v", out)
	}
	if len(out.ScopesOptional) != 1 || out.ScopesOptional[0] != "pizza.delivery.track@tenant" {
		t.Errorf("scopes_optional not parsed: %+v", out)
	}
	if out.LogoURL == "" || out.PrivacyURL == "" || out.TOSURL == "" {
		t.Errorf("optional urls not parsed: %+v", out)
	}
}

func TestApps_GetManifest_RequiresAppID(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	_, err := client.Apps().GetManifest(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty appID")
	}
}

func TestApps_GetManifest_NotFoundPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/manifest/com.ghostapp": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 404, map[string]interface{}{
				"error": map[string]interface{}{"code": "NOT_FOUND", "message": "manifest not found"},
			})
		},
	})
	_, err := client.Apps().GetManifest(context.Background(), "com.ghostapp")
	if err == nil {
		t.Fatal("expected 404, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok || !apiErr.IsNotFound() {
		t.Errorf("expected not-found OlympusAPIError, got %T: %v", err, err)
	}
}

// --------------------------------------------------------------------------
// AppsService.GetPendingInstall — GET /apps/pending_install/:id  (anonymous)
// --------------------------------------------------------------------------

func TestApps_GetPendingInstall_ParsesDetailWithManifest(t *testing.T) {
	var gotMethod, gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/pending_install/pi-uuid-1": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			jsonResponse(w, 200, map[string]interface{}{
				"id":               "pi-uuid-1",
				"app_id":           "com.pizzaos",
				"tenant_id":        "t-1",
				"requested_scopes": []string{"pizza.menu.read@tenant"},
				"return_to":        "https://pizza.test/settings",
				"status":           "pending",
				"expires_at":       "2026-04-21T10:10:00Z",
				"manifest": map[string]interface{}{
					"app_id":          "com.pizzaos",
					"version":         "1.4.0",
					"name":            "PizzaOS",
					"publisher":       "NebusAI",
					"scopes_required": []string{"pizza.menu.read@tenant"},
					"scopes_optional": []string{},
				},
			})
		},
	})
	out, err := client.Apps().GetPendingInstall(context.Background(), "pi-uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method: got %s, want GET", gotMethod)
	}
	if gotPath != "/apps/pending_install/pi-uuid-1" {
		t.Errorf("path: got %s", gotPath)
	}
	if out.ID != "pi-uuid-1" || out.Status != "pending" {
		t.Errorf("detail not parsed: %+v", out)
	}
	if out.ExpiresAt == nil {
		t.Error("expires_at not parsed")
	}
	if out.Manifest == nil {
		t.Fatalf("manifest should be eager-loaded: %+v", out)
	}
	if out.Manifest.AppID != "com.pizzaos" || out.Manifest.Name != "PizzaOS" {
		t.Errorf("manifest not parsed: %+v", out.Manifest)
	}
}

func TestApps_GetPendingInstall_ParsesDetailWithoutManifest(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/pending_install/pi-uuid-2": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id":               "pi-uuid-2",
				"app_id":           "com.delisted",
				"tenant_id":        "t-1",
				"requested_scopes": []string{},
				"return_to":        "",
				"status":           "pending",
				"expires_at":       "2026-04-21T10:10:00Z",
			})
		},
	})
	out, err := client.Apps().GetPendingInstall(context.Background(), "pi-uuid-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Manifest != nil {
		t.Errorf("manifest should be nil when missing from response: %+v", out.Manifest)
	}
}

func TestApps_GetPendingInstall_RequiresID(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	_, err := client.Apps().GetPendingInstall(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestApps_GetPendingInstall_GonePropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/pending_install/pi-expired": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 410, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "GONE",
					"message": "resource is no longer available",
				},
			})
		},
	})
	_, err := client.Apps().GetPendingInstall(context.Background(), "pi-expired")
	if err == nil {
		t.Fatal("expected 410, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected *OlympusAPIError, got %T", err)
	}
	if apiErr.StatusCode != 410 {
		t.Errorf("expected status 410, got %d", apiErr.StatusCode)
	}
	if apiErr.Code != "GONE" {
		t.Errorf("expected code GONE, got %s", apiErr.Code)
	}
}

// --------------------------------------------------------------------------
// AppsService.ApprovePendingInstall — POST /apps/pending_install/:id/approve
// --------------------------------------------------------------------------

func TestApps_ApprovePendingInstall_PostsAndParsesInstall(t *testing.T) {
	var gotMethod, gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/pending_install/pi-uuid-1/approve": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			jsonResponse(w, 201, map[string]interface{}{
				"tenant_id":      "t-1",
				"app_id":         "com.pizzaos",
				"installed_at":   "2026-04-21T10:05:00Z",
				"installed_by":   "user-1",
				"scopes_granted": []string{"pizza.menu.read@tenant", "pizza.orders.write@tenant"},
				"status":         "active",
			})
		},
	})
	out, err := client.Apps().ApprovePendingInstall(context.Background(), "pi-uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %s, want POST", gotMethod)
	}
	if gotPath != "/apps/pending_install/pi-uuid-1/approve" {
		t.Errorf("path: got %s", gotPath)
	}
	if out.TenantID != "t-1" || out.AppID != "com.pizzaos" {
		t.Errorf("install not parsed: %+v", out)
	}
	if out.Status != "active" || out.InstalledBy != "user-1" {
		t.Errorf("install fields not parsed: %+v", out)
	}
	if len(out.ScopesGranted) != 2 {
		t.Errorf("scopes_granted not parsed: %+v", out)
	}
	if out.InstalledAt == nil {
		t.Error("installed_at not parsed")
	}
}

func TestApps_ApprovePendingInstall_RequiresID(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	_, err := client.Apps().ApprovePendingInstall(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestApps_ApprovePendingInstall_AlreadyResolvedPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/pending_install/pi-resolved/approve": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 400, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "BAD_REQUEST",
					"message": "pending install already resolved (status=approved)",
				},
			})
		},
	})
	_, err := client.Apps().ApprovePendingInstall(context.Background(), "pi-resolved")
	if err == nil {
		t.Fatal("expected 400, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected *OlympusAPIError, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("expected 400, got %d", apiErr.StatusCode)
	}
}

// --------------------------------------------------------------------------
// AppsService.DenyPendingInstall — POST /apps/pending_install/:id/deny
// --------------------------------------------------------------------------

func TestApps_DenyPendingInstall_Posts204(t *testing.T) {
	var gotMethod, gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/pending_install/pi-uuid-1/deny": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(204)
		},
	})
	err := client.Apps().DenyPendingInstall(context.Background(), "pi-uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %s, want POST", gotMethod)
	}
	if gotPath != "/apps/pending_install/pi-uuid-1/deny" {
		t.Errorf("path: got %s", gotPath)
	}
}

func TestApps_DenyPendingInstall_RequiresID(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	err := client.Apps().DenyPendingInstall(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestApps_DenyPendingInstall_ForbiddenPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/apps/pending_install/pi-x/deny": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 403, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "FORBIDDEN",
					"message": "tenant_admin on target tenant required",
				},
			})
		},
	})
	err := client.Apps().DenyPendingInstall(context.Background(), "pi-x")
	if err == nil {
		t.Fatal("expected 403, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok || !apiErr.IsForbidden() {
		t.Errorf("expected forbidden OlympusAPIError, got %T: %v", err, err)
	}
}

// --------------------------------------------------------------------------
// Client accessor
// --------------------------------------------------------------------------

func TestClient_Apps_IsCached(t *testing.T) {
	client := NewClient(Config{AppID: "test", APIKey: "key"})
	if client.Apps() == nil {
		t.Fatal("Apps() returned nil")
	}
	if client.Apps() != client.Apps() {
		t.Error("Apps() not cached")
	}
}
