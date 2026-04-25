// Tests for PlatformService.ListScopeRegistry + GetScopeRegistryDigest
// (gcp#3517).
//
//	GET /platform/scope-registry?namespace=&owner_app_id=&include_drafts=
//	    -> { scopes: ScopeRow[], total: int }
//	GET /platform/scope-registry/digest
//	    -> { platform_catalog_digest: hex, row_count: int }

package olympus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// ---------------------------------------------------------------------------
// PlatformService.ListScopeRegistry
// ---------------------------------------------------------------------------

func TestListScopeRegistry_NoFiltersReturnsCatalog(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"scopes": [
				{"scope":"auth.session.read@user","resource":"session","action":"read","holder":"user","namespace":"auth","owner_app_id":null,"description":"Read your own session metadata","is_destructive":false,"requires_mfa":false,"grace_behavior":"extend","consent_prompt_copy":"View your session","workshop_status":"approved","bit_id":0},
				{"scope":"voice.call.write@tenant","resource":"call","action":"write","holder":"tenant","namespace":"voice","owner_app_id":"orderecho-ai","description":"Place outbound voice calls on the tenant","is_destructive":true,"requires_mfa":true,"grace_behavior":"deny","consent_prompt_copy":"Place outbound calls","workshop_status":"service_ok","bit_id":12}
			],
			"total": 2
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	listing, err := oc.Platform().ListScopeRegistry(context.Background(), ListScopeRegistryParams{})
	if err != nil {
		t.Fatalf("ListScopeRegistry: %v", err)
	}
	if capturedQuery != "" {
		t.Errorf("expected empty query, got %q", capturedQuery)
	}
	if listing.Total != 2 {
		t.Errorf("Total = %d, want 2", listing.Total)
	}
	if len(listing.Scopes) != 2 {
		t.Fatalf("len(Scopes) = %d, want 2", len(listing.Scopes))
	}
	if listing.Scopes[0].Scope != "auth.session.read@user" {
		t.Errorf("Scopes[0].Scope = %q", listing.Scopes[0].Scope)
	}
	if listing.Scopes[0].BitID == nil || *listing.Scopes[0].BitID != 0 {
		t.Errorf("Scopes[0].BitID = %v, want 0", listing.Scopes[0].BitID)
	}
	if listing.Scopes[0].OwnerAppID != nil {
		t.Errorf("Scopes[0].OwnerAppID = %v, want nil", listing.Scopes[0].OwnerAppID)
	}
	if listing.Scopes[1].OwnerAppID == nil || *listing.Scopes[1].OwnerAppID != "orderecho-ai" {
		t.Errorf("Scopes[1].OwnerAppID = %v, want orderecho-ai", listing.Scopes[1].OwnerAppID)
	}
	if !listing.Scopes[1].IsDestructive || !listing.Scopes[1].RequiresMFA {
		t.Errorf("Scopes[1] flags wrong: destructive=%v mfa=%v", listing.Scopes[1].IsDestructive, listing.Scopes[1].RequiresMFA)
	}
}

func TestListScopeRegistry_FiltersForwarded(t *testing.T) {
	var capturedQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"scopes":[],"total":0}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Platform().ListScopeRegistry(context.Background(), ListScopeRegistryParams{
		Namespace:     "voice",
		OwnerAppID:    "orderecho-ai",
		OwnerAppIDSet: true,
		IncludeDrafts: true,
	})
	if err != nil {
		t.Fatalf("ListScopeRegistry: %v", err)
	}
	if got := capturedQuery.Get("namespace"); got != "voice" {
		t.Errorf("namespace = %q, want voice", got)
	}
	if got := capturedQuery.Get("owner_app_id"); got != "orderecho-ai" {
		t.Errorf("owner_app_id = %q, want orderecho-ai", got)
	}
	if got := capturedQuery.Get("include_drafts"); got != "true" {
		t.Errorf("include_drafts = %q, want true", got)
	}
}

func TestListScopeRegistry_OwnerAppIDEmptyForwardedDistinctly(t *testing.T) {
	// OwnerAppIDSet=true with empty OwnerAppID = "platform-owned only" filter.
	// Without OwnerAppIDSet=true the field MUST be omitted (no filter).
	var capturedQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"scopes":[],"total":0}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Platform().ListScopeRegistry(context.Background(), ListScopeRegistryParams{
		OwnerAppIDSet: true, // OwnerAppID is "" (zero)
	})
	if err != nil {
		t.Fatalf("ListScopeRegistry: %v", err)
	}
	if !capturedQuery.Has("owner_app_id") {
		t.Errorf("expected owner_app_id in query (platform-owned filter), got %v", capturedQuery)
	}
	if capturedQuery.Get("owner_app_id") != "" {
		t.Errorf("owner_app_id should be empty string, got %q", capturedQuery.Get("owner_app_id"))
	}
}

func TestListScopeRegistry_OwnerAppIDOmittedWhenNotSet(t *testing.T) {
	var capturedQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"scopes":[],"total":0}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Platform().ListScopeRegistry(context.Background(), ListScopeRegistryParams{
		Namespace: "platform",
		// OwnerAppIDSet=false (zero) → should NOT appear in query.
	})
	if err != nil {
		t.Fatalf("ListScopeRegistry: %v", err)
	}
	if capturedQuery.Has("owner_app_id") {
		t.Errorf("expected owner_app_id absent, got %q", capturedQuery.Get("owner_app_id"))
	}
}

func TestListScopeRegistry_BitIDNullToleratedForPreAllocation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"scopes":[{"scope":"creator.draft.write@tenant","resource":"draft","action":"write","holder":"tenant","namespace":"creator","owner_app_id":null,"description":"","is_destructive":false,"requires_mfa":false,"grace_behavior":"extend","consent_prompt_copy":"","workshop_status":"pending","bit_id":null}],"total":1}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	listing, err := oc.Platform().ListScopeRegistry(context.Background(), ListScopeRegistryParams{
		IncludeDrafts: true,
	})
	if err != nil {
		t.Fatalf("ListScopeRegistry: %v", err)
	}
	if len(listing.Scopes) != 1 {
		t.Fatalf("len(Scopes) = %d, want 1", len(listing.Scopes))
	}
	if listing.Scopes[0].BitID != nil {
		t.Errorf("BitID = %v, want nil for pre-allocation row", listing.Scopes[0].BitID)
	}
	if listing.Scopes[0].WorkshopStatus != "pending" {
		t.Errorf("WorkshopStatus = %q, want pending", listing.Scopes[0].WorkshopStatus)
	}
}

// ---------------------------------------------------------------------------
// PlatformService.GetScopeRegistryDigest
// ---------------------------------------------------------------------------

func TestGetScopeRegistryDigest_ParsesHexAndCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform/scope-registry/digest" {
			t.Errorf("wrong path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"platform_catalog_digest":"12398a9b0517a3576d0e4d88807a34573d940aaada6bb61def2d540009c7bc19","row_count":3}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	digest, err := oc.Platform().GetScopeRegistryDigest(context.Background())
	if err != nil {
		t.Fatalf("GetScopeRegistryDigest: %v", err)
	}
	wantHex := "12398a9b0517a3576d0e4d88807a34573d940aaada6bb61def2d540009c7bc19"
	if digest.PlatformCatalogDigest != wantHex {
		t.Errorf("PlatformCatalogDigest = %q, want %q", digest.PlatformCatalogDigest, wantHex)
	}
	if digest.RowCount != 3 {
		t.Errorf("RowCount = %d, want 3", digest.RowCount)
	}
}

func TestGetScopeRegistryDigest_EmptyCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"platform_catalog_digest":"4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945","row_count":0}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	digest, err := oc.Platform().GetScopeRegistryDigest(context.Background())
	if err != nil {
		t.Fatalf("GetScopeRegistryDigest: %v", err)
	}
	if digest.RowCount != 0 {
		t.Errorf("RowCount = %d, want 0", digest.RowCount)
	}
	if len(digest.PlatformCatalogDigest) != 64 {
		t.Errorf("PlatformCatalogDigest length = %d, want 64 (sha256 hex)", len(digest.PlatformCatalogDigest))
	}
}
