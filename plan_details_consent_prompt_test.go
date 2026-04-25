package olympus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Tests for the two platform endpoints landed via olympus-cloud-gcp PRs
// #3519 + #3520:
//
//	GatingService.GetPlanDetails  → GET /platform/gating/plan-details
//	ConsentService.Describe       → GET /platform/consent-prompt
//
// Spin a real httptest server, intercept the request, assert the path +
// query string, return the canonical Rust handler envelope, verify
// parsed Go DTOs.

func TestGetPlanDetails_NoTenantID(t *testing.T) {
	var capturedPath, capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"current_plan": "growth",
			"plans": [
				{"tier_id":"free","display_name":"Free","monthly_price_usd":0.0,"features":["basic"],"usage_limits":{},"ranks_higher_than_current":false,"is_current":false,"diff_vs_current":[],"contact_sales":false},
				{"tier_id":"growth","display_name":"Growth","monthly_price_usd":99.0,"features":["basic","analytics"],"usage_limits":{"voice_minutes":60},"ranks_higher_than_current":false,"is_current":true,"diff_vs_current":[],"contact_sales":false},
				{"tier_id":"enterprise","display_name":"Enterprise","monthly_price_usd":null,"features":["basic","analytics","sla"],"usage_limits":{"voice_minutes":300},"ranks_higher_than_current":true,"is_current":false,"diff_vs_current":["unlocks: sla","+240 voice_minutes"],"contact_sales":true}
			],
			"as_of": "2026-04-25T13:00:00Z"
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	details, err := oc.Gating().GetPlanDetails(context.Background(), GetPlanDetailsParams{})
	if err != nil {
		t.Fatalf("GetPlanDetails: %v", err)
	}

	if capturedPath != "/platform/gating/plan-details" {
		t.Errorf("wrong path: %q", capturedPath)
	}
	if capturedQuery != "" {
		t.Errorf("expected empty query when tenant_id omitted, got %q", capturedQuery)
	}

	if details.CurrentPlan == nil || *details.CurrentPlan != "growth" {
		t.Errorf("wrong current_plan: %v", details.CurrentPlan)
	}
	if len(details.Plans) != 3 {
		t.Fatalf("expected 3 plans, got %d", len(details.Plans))
	}

	free := details.Plans[0]
	if free.MonthlyPriceUSD == nil || *free.MonthlyPriceUSD != 0.0 {
		t.Errorf("free should price at 0.0, got %v", free.MonthlyPriceUSD)
	}
	if free.IsCurrent {
		t.Errorf("free should not be current")
	}

	growth := details.Plans[1]
	if !growth.IsCurrent {
		t.Errorf("growth should be current")
	}
	if growth.MonthlyPriceUSD == nil || *growth.MonthlyPriceUSD != 99.0 {
		t.Errorf("growth price 99.0 expected, got %v", growth.MonthlyPriceUSD)
	}

	ent := details.Plans[2]
	if !ent.ContactSales {
		t.Errorf("enterprise should be contact_sales")
	}
	if ent.MonthlyPriceUSD != nil {
		t.Errorf("enterprise price should be nil, got %v", *ent.MonthlyPriceUSD)
	}
	if !ent.RanksHigherThanCurrent {
		t.Errorf("enterprise should rank higher than growth")
	}
	if !sliceContains(ent.DiffVsCurrent, "unlocks: sla") {
		t.Errorf("missing 'unlocks: sla' in diff: %v", ent.DiffVsCurrent)
	}
}

func TestGetPlanDetails_WithTenantID(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"current_plan":null,"plans":[],"as_of":"2026-04-25T13:00:00Z"}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	if _, err := oc.Gating().GetPlanDetails(context.Background(), GetPlanDetailsParams{TenantID: "ten-abc"}); err != nil {
		t.Fatalf("GetPlanDetails: %v", err)
	}
	if !strings.Contains(capturedQuery, "tenant_id=ten-abc") {
		t.Errorf("expected tenant_id=ten-abc in query, got %q", capturedQuery)
	}
}

func TestGetPlanDetails_NullCurrentPlan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"current_plan":null,"plans":[],"as_of":"2026-04-25T13:00:00Z"}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	details, err := oc.Gating().GetPlanDetails(context.Background(), GetPlanDetailsParams{})
	if err != nil {
		t.Fatalf("GetPlanDetails: %v", err)
	}
	if details.CurrentPlan != nil {
		t.Errorf("expected nil current_plan, got %v", *details.CurrentPlan)
	}
	if len(details.Plans) != 0 {
		t.Errorf("expected empty plans, got %d", len(details.Plans))
	}
}

func TestDescribeConsentPrompt_FullEnvelope(t *testing.T) {
	var capturedPath, capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"app_id": "com.olympuscloud.maximus",
			"scope": "auth.session.read@user",
			"prompt_text": "Maximus will be able to see your active sessions.",
			"prompt_hash": "0000000000000000000000000000000000000000000000000000000000000000",
			"is_destructive": false,
			"requires_mfa": false,
			"app_may_request": true
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	prompt, err := oc.Consent().Describe(context.Background(), DescribeParams{
		AppID: "com.olympuscloud.maximus",
		Scope: "auth.session.read@user",
	})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}

	// Path realigned: no `/api/v1/` prefix double-up.
	if capturedPath != "/platform/consent-prompt" {
		t.Errorf("wrong path: %q", capturedPath)
	}
	if !strings.Contains(capturedQuery, "app_id=com.olympuscloud.maximus") {
		t.Errorf("missing app_id in query: %q", capturedQuery)
	}
	// '@' encodes to %40
	if !strings.Contains(capturedQuery, "scope=auth.session.read%40user") {
		t.Errorf("missing url-encoded scope in query: %q", capturedQuery)
	}

	if prompt.AppID != "com.olympuscloud.maximus" {
		t.Errorf("wrong app_id: %q", prompt.AppID)
	}
	if !strings.HasPrefix(prompt.PromptText, "Maximus") {
		t.Errorf("wrong prompt_text: %q", prompt.PromptText)
	}
	if len(prompt.PromptHash) != 64 {
		t.Errorf("wrong prompt_hash length: %d", len(prompt.PromptHash))
	}
	if !prompt.AppMayRequest {
		t.Errorf("expected app_may_request=true")
	}
}

func TestDescribeConsentPrompt_DestructiveAndMFA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"app_id":"com.x","scope":"auth.session.delete@user",
			"prompt_text":"X will sign you out of other devices.",
			"prompt_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"is_destructive":true,"requires_mfa":true,"app_may_request":true
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	prompt, err := oc.Consent().Describe(context.Background(), DescribeParams{AppID: "com.x", Scope: "auth.session.delete@user"})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if !prompt.IsDestructive {
		t.Errorf("expected is_destructive=true")
	}
	if !prompt.RequiresMFA {
		t.Errorf("expected requires_mfa=true")
	}
}

func TestDescribeConsentPrompt_AppMayRequestFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"app_id":"com.untrusted","scope":"pizza.menu.read@tenant",
			"prompt_text":"untrusted will read pizza menu data.",
			"prompt_hash":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"is_destructive":false,"requires_mfa":false,"app_may_request":false
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	prompt, err := oc.Consent().Describe(context.Background(), DescribeParams{AppID: "com.untrusted", Scope: "pizza.menu.read@tenant"})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if prompt.AppMayRequest {
		t.Errorf("expected app_may_request=false for cross-app scope")
	}
}

func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
