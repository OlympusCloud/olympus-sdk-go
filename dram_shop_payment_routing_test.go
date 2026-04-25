package olympus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Tests for the dram-shop + payment-routing wrappers landed in
// olympus-cloud-gcp PRs #3525, #3528, #3530:
//
//	ComplianceService.RecordDramShopEvent → POST /platform/compliance/dram-shop-events
//	ComplianceService.ListDramShopEvents  → GET  /platform/compliance/dram-shop-events
//	ComplianceService.ListDramShopRules   → GET  /platform/compliance/dram-shop-rules
//	PayService.ConfigureRouting           → POST /platform/pay/routing
//	PayService.GetRouting                 → GET  /platform/pay/routing/{location_id}

// ----------------------------------------------------------------------------
// RecordDramShopEvent
// ----------------------------------------------------------------------------

func TestRecordDramShopEvent_CanonicalBody(t *testing.T) {
	var capturedPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"event_id": "evt-1",
			"tenant_id": "tnt-1",
			"location_id": "loc-1",
			"event_type": "id_check_passed",
			"customer_ref": "hashed-cust-key",
			"staff_user_id": "usr-staff",
			"estimated_bac": 0.04,
			"bac_inputs": {"gender":"F","weight_kg":65},
			"vertical_extensions": {"food_weight_g":240},
			"notes": "first scan of the night",
			"occurred_at": "2026-04-25T13:00:00Z",
			"created_at": "2026-04-25T13:00:01Z"
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	occurred := time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
	bac := 0.04
	evt, err := oc.Compliance().RecordDramShopEvent(context.Background(), RecordDramShopEventParams{
		LocationID:         "loc-1",
		EventType:          DramShopEventIDCheckPassed,
		CustomerRef:        "hashed-cust-key",
		StaffUserID:        "usr-staff",
		EstimatedBAC:       &bac,
		BACInputs:          map[string]interface{}{"gender": "F", "weight_kg": 65},
		VerticalExtensions: map[string]interface{}{"food_weight_g": 240},
		Notes:              "first scan of the night",
		OccurredAt:         &occurred,
	})
	if err != nil {
		t.Fatalf("RecordDramShopEvent: %v", err)
	}

	if capturedPath != "/platform/compliance/dram-shop-events" {
		t.Errorf("wrong path: %q", capturedPath)
	}

	// Canonical body shape — every optional field included.
	wantKeys := []string{
		"location_id", "event_type", "customer_ref", "staff_user_id",
		"estimated_bac", "bac_inputs", "vertical_extensions", "notes", "occurred_at",
	}
	for _, k := range wantKeys {
		if _, ok := gotBody[k]; !ok {
			t.Errorf("expected body key %q, missing in %v", k, gotBody)
		}
	}
	if gotBody["location_id"] != "loc-1" {
		t.Errorf("wrong location_id: %v", gotBody["location_id"])
	}
	if gotBody["event_type"] != "id_check_passed" {
		t.Errorf("wrong event_type: %v", gotBody["event_type"])
	}
	if gotBody["estimated_bac"] != 0.04 {
		t.Errorf("wrong estimated_bac: %v", gotBody["estimated_bac"])
	}
	occ, _ := gotBody["occurred_at"].(string)
	if !strings.HasPrefix(occ, "2026-04-25T13:00:00") {
		t.Errorf("occurred_at should be RFC3339 UTC: %q", occ)
	}

	// Response parsed correctly.
	if evt.EventID != "evt-1" || evt.TenantID != "tnt-1" {
		t.Errorf("unexpected event payload: %+v", evt)
	}
	if evt.OccurredAt.IsZero() {
		t.Errorf("occurred_at should parse to non-zero time")
	}
	if evt.EstimatedBAC == nil || *evt.EstimatedBAC != 0.04 {
		t.Errorf("estimated_bac should round-trip 0.04: %v", evt.EstimatedBAC)
	}
	if evt.BACInputs["gender"] != "F" {
		t.Errorf("bac_inputs gender lost: %v", evt.BACInputs)
	}
}

func TestRecordDramShopEvent_OmitsOptionalFields(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"event_id":"e","tenant_id":"t","location_id":"loc-1",
			"event_type":"service_refused",
			"occurred_at":"2026-04-25T13:00:00Z","created_at":"2026-04-25T13:00:00Z"
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Compliance().RecordDramShopEvent(context.Background(), RecordDramShopEventParams{
		LocationID: "loc-1",
		EventType:  DramShopEventServiceRefused,
	})
	if err != nil {
		t.Fatalf("RecordDramShopEvent: %v", err)
	}

	// Optional fields must NOT be present when not set.
	skipKeys := []string{
		"customer_ref", "staff_user_id", "estimated_bac",
		"bac_inputs", "vertical_extensions", "notes", "occurred_at",
	}
	for _, k := range skipKeys {
		if _, present := gotBody[k]; present {
			t.Errorf("expected %q omitted when not provided, got %v", k, gotBody[k])
		}
	}
	// Required fields stay.
	if gotBody["location_id"] != "loc-1" || gotBody["event_type"] != "service_refused" {
		t.Errorf("wrong required fields: %v", gotBody)
	}
}

// ----------------------------------------------------------------------------
// ListDramShopEvents
// ----------------------------------------------------------------------------

func TestListDramShopEvents_AllFilters(t *testing.T) {
	var capturedPath, capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"events": [
				{
					"event_id":"e1","tenant_id":"t","location_id":"loc-1",
					"event_type":"id_check_passed",
					"occurred_at":"2026-04-25T13:00:00Z","created_at":"2026-04-25T13:00:00Z"
				}
			],
			"total_returned": 1
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	from := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	out, err := oc.Compliance().ListDramShopEvents(context.Background(), ListDramShopEventsParams{
		LocationID: "loc-1",
		From:       &from,
		To:         &to,
		EventType:  DramShopEventIDCheckPassed,
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("ListDramShopEvents: %v", err)
	}

	if capturedPath != "/platform/compliance/dram-shop-events" {
		t.Errorf("wrong path: %q", capturedPath)
	}
	wantSubstrings := []string{
		"location_id=loc-1",
		"from=2026-04-25T00%3A00%3A00Z",
		"to=2026-04-26T00%3A00%3A00Z",
		"event_type=id_check_passed",
		"limit=50",
	}
	for _, w := range wantSubstrings {
		if !strings.Contains(capturedQuery, w) {
			t.Errorf("query missing %q in %q", w, capturedQuery)
		}
	}

	if out.TotalReturned != 1 || len(out.Events) != 1 {
		t.Errorf("expected 1 event, got %+v", out)
	}
	if out.Events[0].EventID != "e1" {
		t.Errorf("wrong event_id: %q", out.Events[0].EventID)
	}
}

func TestListDramShopEvents_NoFilters(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"events":[],"total_returned":0}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	out, err := oc.Compliance().ListDramShopEvents(context.Background(), ListDramShopEventsParams{})
	if err != nil {
		t.Fatalf("ListDramShopEvents: %v", err)
	}
	if capturedQuery != "" {
		t.Errorf("expected empty query, got %q", capturedQuery)
	}
	if out.TotalReturned != 0 || len(out.Events) != 0 {
		t.Errorf("expected empty list, got %+v", out)
	}
}

// ----------------------------------------------------------------------------
// ListDramShopRules
// ----------------------------------------------------------------------------

func TestListDramShopRules_EnvelopeParsing(t *testing.T) {
	var capturedPath, capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"rules": [
				{
					"tenant_id":"t1","rule_id":"r1",
					"jurisdiction_code":"US-FL","rule_type":"max_drinks_per_hour",
					"rule_payload":{"max":3},
					"effective_from":"2026-01-01T00:00:00Z",
					"effective_until":null,
					"override_app_id":null,
					"notes":null,
					"created_at":"2025-12-01T00:00:00Z"
				},
				{
					"tenant_id":"t1","rule_id":"r2",
					"jurisdiction_code":"US-FL","rule_type":"food_required",
					"rule_payload":null,
					"effective_from":"2026-01-01T00:00:00Z",
					"effective_until":"2027-01-01T00:00:00Z",
					"override_app_id":"pizza-os",
					"notes":"PizzaOS override",
					"created_at":"2025-12-15T00:00:00Z"
				}
			]
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	rules, err := oc.Compliance().ListDramShopRules(context.Background(), ListDramShopRulesParams{
		JurisdictionCode: "US-FL",
		AppID:            "pizza-os",
		RuleType:         "food_required",
	})
	if err != nil {
		t.Fatalf("ListDramShopRules: %v", err)
	}

	if capturedPath != "/platform/compliance/dram-shop-rules" {
		t.Errorf("wrong path: %q", capturedPath)
	}
	for _, w := range []string{"jurisdiction_code=US-FL", "app_id=pizza-os", "rule_type=food_required"} {
		if !strings.Contains(capturedQuery, w) {
			t.Errorf("query missing %q in %q", w, capturedQuery)
		}
	}

	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	r1 := rules[0]
	if r1.RuleID != "r1" || r1.JurisdictionCode != "US-FL" {
		t.Errorf("wrong r1: %+v", r1)
	}
	if r1.EffectiveUntil != nil {
		t.Errorf("r1.effective_until should be nil, got %v", *r1.EffectiveUntil)
	}
	if r1.OverrideAppID != nil {
		t.Errorf("r1.override_app_id should be nil, got %v", *r1.OverrideAppID)
	}
	if r1.RulePayload["max"] != float64(3) {
		t.Errorf("r1.rule_payload max should round-trip 3, got %v", r1.RulePayload["max"])
	}

	r2 := rules[1]
	if r2.OverrideAppID == nil || *r2.OverrideAppID != "pizza-os" {
		t.Errorf("r2.override_app_id should be 'pizza-os', got %v", r2.OverrideAppID)
	}
	if r2.EffectiveUntil == nil {
		t.Errorf("r2.effective_until should not be nil")
	}
	if r2.RulePayload != nil {
		t.Errorf("r2.rule_payload should be nil, got %v", r2.RulePayload)
	}
}

func TestListDramShopRules_NoFilters(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rules":[]}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	rules, err := oc.Compliance().ListDramShopRules(context.Background(), ListDramShopRulesParams{})
	if err != nil {
		t.Fatalf("ListDramShopRules: %v", err)
	}
	if capturedQuery != "" {
		t.Errorf("expected empty query, got %q", capturedQuery)
	}
	if len(rules) != 0 {
		t.Errorf("expected empty list, got %d", len(rules))
	}
}

// ----------------------------------------------------------------------------
// ConfigureRouting
// ----------------------------------------------------------------------------

func TestConfigureRouting_CanonicalBody(t *testing.T) {
	var capturedPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tenant_id":"tnt-1",
			"location_id":"loc-1",
			"preferred_processor":"square",
			"fallback_processors":["olympus_pay"],
			"credentials_secret_ref":"olympus-merchant-credentials-loc-1-square-dev",
			"merchant_id":"MERCH-123",
			"is_active":true,
			"notes":null,
			"created_at":null,
			"updated_at":null
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	cfg, err := oc.Pay().ConfigureRouting(context.Background(), ConfigureRoutingParams{
		LocationID:           "loc-1",
		PreferredProcessor:   PaymentProcessorSquare,
		FallbackProcessors:   []string{PaymentProcessorOlympusPay},
		CredentialsSecretRef: "olympus-merchant-credentials-loc-1-square-dev",
		MerchantID:           "MERCH-123",
		IsActive:             true,
		IsActiveSet:          true,
	})
	if err != nil {
		t.Fatalf("ConfigureRouting: %v", err)
	}

	if capturedPath != "/platform/pay/routing" {
		t.Errorf("wrong path: %q", capturedPath)
	}
	if gotBody["location_id"] != "loc-1" {
		t.Errorf("wrong location_id: %v", gotBody["location_id"])
	}
	if gotBody["preferred_processor"] != "square" {
		t.Errorf("wrong preferred_processor: %v", gotBody["preferred_processor"])
	}
	fbs, ok := gotBody["fallback_processors"].([]interface{})
	if !ok || len(fbs) != 1 || fbs[0] != "olympus_pay" {
		t.Errorf("wrong fallback_processors: %v", gotBody["fallback_processors"])
	}
	if gotBody["credentials_secret_ref"] != "olympus-merchant-credentials-loc-1-square-dev" {
		t.Errorf("wrong credentials_secret_ref: %v", gotBody["credentials_secret_ref"])
	}
	if gotBody["merchant_id"] != "MERCH-123" {
		t.Errorf("wrong merchant_id: %v", gotBody["merchant_id"])
	}
	if gotBody["is_active"] != true {
		t.Errorf("wrong is_active: %v", gotBody["is_active"])
	}

	if cfg.TenantID != "tnt-1" || cfg.LocationID != "loc-1" {
		t.Errorf("unexpected response: %+v", cfg)
	}
	if cfg.CredentialsSecretRef == nil || *cfg.CredentialsSecretRef != "olympus-merchant-credentials-loc-1-square-dev" {
		t.Errorf("credentials_secret_ref should round-trip: %v", cfg.CredentialsSecretRef)
	}
	if !containsStringDR(cfg.FallbackProcessors, "olympus_pay") {
		t.Errorf("fallback chain lost: %v", cfg.FallbackProcessors)
	}
}

func TestConfigureRouting_OmitsCredentialsSecretRefWhenEmpty(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tenant_id":"t","location_id":"loc-1",
			"preferred_processor":"adyen",
			"fallback_processors":[],
			"is_active":true
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Pay().ConfigureRouting(context.Background(), ConfigureRoutingParams{
		LocationID:         "loc-1",
		PreferredProcessor: PaymentProcessorAdyen,
		// CredentialsSecretRef intentionally empty.
	})
	if err != nil {
		t.Fatalf("ConfigureRouting: %v", err)
	}
	if _, present := gotBody["credentials_secret_ref"]; present {
		t.Errorf("credentials_secret_ref should be omitted when empty, got %v", gotBody["credentials_secret_ref"])
	}
	if _, present := gotBody["merchant_id"]; present {
		t.Errorf("merchant_id should be omitted when empty")
	}
	if _, present := gotBody["notes"]; present {
		t.Errorf("notes should be omitted when empty")
	}
	if _, present := gotBody["is_active"]; present {
		t.Errorf("is_active should be omitted when IsActiveSet=false (let server default)")
	}
	// fallback_processors is always included so an empty slice can clear the chain.
	fbs, ok := gotBody["fallback_processors"].([]interface{})
	if !ok || len(fbs) != 0 {
		t.Errorf("fallback_processors should be empty array, got %v", gotBody["fallback_processors"])
	}
}

// ----------------------------------------------------------------------------
// GetRouting
// ----------------------------------------------------------------------------

func TestGetRouting_URLEncodesLocationID(t *testing.T) {
	var capturedEscapedPath, capturedDecodedPath, capturedRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// EscapedPath returns the path with %xx escapes preserved; this is
		// what we sent on the wire. Path is the decoded form.
		capturedEscapedPath = r.URL.EscapedPath()
		capturedDecodedPath = r.URL.Path
		capturedRequestURI = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tenant_id":"tnt-1",
			"location_id":"loc/with/slashes",
			"preferred_processor":"square",
			"fallback_processors":[],
			"is_active":true
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	cfg, err := oc.Pay().GetRouting(context.Background(), GetRoutingParams{
		LocationID: "loc/with/slashes",
	})
	if err != nil {
		t.Fatalf("GetRouting: %v", err)
	}

	// The SDK must url-escape '/' as %2F so the location_id stays a single
	// path segment instead of getting split. Both EscapedPath and the raw
	// RequestURI should preserve the encoding.
	wantEscaped := "/platform/pay/routing/loc%2Fwith%2Fslashes"
	if capturedEscapedPath != wantEscaped {
		t.Errorf("expected escaped path %q, got %q", wantEscaped, capturedEscapedPath)
	}
	if !strings.Contains(capturedRequestURI, "loc%2Fwith%2Fslashes") {
		t.Errorf("expected raw request URI to contain loc%%2Fwith%%2Fslashes, got %q", capturedRequestURI)
	}
	// Sanity: the decoded path puts the slashes back.
	if capturedDecodedPath != "/platform/pay/routing/loc/with/slashes" {
		t.Errorf("wrong decoded path: %q", capturedDecodedPath)
	}
	if cfg.LocationID != "loc/with/slashes" {
		t.Errorf("wrong location_id round-trip: %q", cfg.LocationID)
	}
}

// containsStringDR mirrors the existing sliceContains helper but with a
// distinct name — sliceContains is declared in
// plan_details_consent_prompt_test.go and Go forbids redeclaration.
func containsStringDR(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------------
// PayService.ListRouting (#3312 pt2 → gcp PR #3537)
// ----------------------------------------------------------------------------

func TestListRouting_NoFiltersReturnsConfigs(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"configs": [
				{"tenant_id":"ten-1","location_id":"loc-a","preferred_processor":"olympus_pay","fallback_processors":["square"],"is_active":true,"created_at":"2026-04-25T13:00:00Z","updated_at":"2026-04-25T13:00:00Z"},
				{"tenant_id":"ten-1","location_id":"loc-b","preferred_processor":"square","fallback_processors":[],"credentials_secret_ref":"olympus-merchant-credentials-acme-square-dev","merchant_id":"sq-acct-7","is_active":true}
			],
			"total_returned": 2
		}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	result, err := oc.Pay().ListRouting(context.Background(), ListRoutingParams{})
	if err != nil {
		t.Fatalf("ListRouting: %v", err)
	}
	if capturedQuery != "" {
		t.Errorf("expected empty query, got %q", capturedQuery)
	}
	if result.TotalReturned != 2 {
		t.Errorf("TotalReturned = %d, want 2", result.TotalReturned)
	}
	if len(result.Configs) != 2 {
		t.Fatalf("len(Configs) = %d, want 2", len(result.Configs))
	}
	if result.Configs[0].LocationID != "loc-a" {
		t.Errorf("Configs[0].LocationID = %q, want loc-a", result.Configs[0].LocationID)
	}
	if result.Configs[1].MerchantID == nil || *result.Configs[1].MerchantID != "sq-acct-7" {
		t.Errorf("Configs[1].MerchantID = %v, want sq-acct-7", result.Configs[1].MerchantID)
	}
}

func TestListRouting_FiltersForwardedAsQueryParams(t *testing.T) {
	var capturedQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"configs":[],"total_returned":0}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Pay().ListRouting(context.Background(), ListRoutingParams{
		IsActive:    false,
		IsActiveSet: true,
		Processor:   PaymentProcessorWorldpay,
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("ListRouting: %v", err)
	}
	if got := capturedQuery.Get("is_active"); got != "false" {
		t.Errorf("is_active = %q, want false", got)
	}
	if got := capturedQuery.Get("processor"); got != "worldpay" {
		t.Errorf("processor = %q, want worldpay", got)
	}
	if got := capturedQuery.Get("limit"); got != "50" {
		t.Errorf("limit = %q, want 50", got)
	}
}

func TestListRouting_OmitsIsActiveWhenUnset(t *testing.T) {
	// Without IsActiveSet=true, the field MUST be omitted (not "false") so
	// the server returns both active + inactive configs. Inverse of the
	// explicit-false test above.
	var capturedQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"configs":[],"total_returned":0}`))
	}))
	defer srv.Close()

	oc := testClient(t, srv.URL)
	_, err := oc.Pay().ListRouting(context.Background(), ListRoutingParams{
		Processor: PaymentProcessorOlympusPay,
		Limit:     25,
	})
	if err != nil {
		t.Fatalf("ListRouting: %v", err)
	}
	if capturedQuery.Has("is_active") {
		t.Errorf("expected is_active to be omitted, got %q", capturedQuery.Get("is_active"))
	}
}

func TestListRouting_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"configs":[],"total_returned":0}`))
	}))
	defer srv.Close()
	oc := testClient(t, srv.URL)
	result, err := oc.Pay().ListRouting(context.Background(), ListRoutingParams{})
	if err != nil {
		t.Fatalf("ListRouting: %v", err)
	}
	if len(result.Configs) != 0 {
		t.Errorf("expected empty Configs, got %d", len(result.Configs))
	}
	if result.TotalReturned != 0 {
		t.Errorf("TotalReturned = %d, want 0", result.TotalReturned)
	}
}
