package olympus

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// Canonical dev-gateway response captured 2026-04-18T01:31 UTC against
// dev.api.olympuscloud.ai, agent 41f239da-c492-5fe6-9334-7bbc47804a36.
var effectiveConfigFixture = map[string]interface{}{
	"agentId":  "41f239da-c492-5fe6-9334-7bbc47804a36",
	"tenantId": "550e8400-e29b-41d4-a716-446655449100",
	"pipeline": "olympus_native",
	"pipelineConfig": map[string]interface{}{
		"defaultLogLevel":  "INFO",
		"tenantSeededAt":   "2026-04-17-V2-005-fix-verification",
	},
	"tierOverride":            "T3",
	"logLevel":                "INFO",
	"debugTranscriptsEnabled": false,
	"v2ShadowEnabled":         false,
	"v2PrimaryEnabled":        false,
	"telephonyProvider":       "telnyx",
	"providerAccountRef":      "telnyx-dev-acct-v2-005-test",
	"preferredCodec":          "opus",
	"preferredSampleRate":     48000,
	"hdAudioEnabled":          true,
	"webhookPathOverride":     "/v2/voice/inbound",
	"v2Routed":                true,
	"voiceDefaults": map[string]interface{}{
		"platform": nil,
		"app":      nil,
		"tenant": map[string]interface{}{
			"pipelineConfig": map[string]interface{}{
				"defaultLogLevel":  "INFO",
				"tenantSeededAt":   "2026-04-17-V2-005-fix-verification",
			},
			"tierOverride": "T3",
		},
		"agent": map[string]interface{}{
			"pipeline":                "olympus_native",
			"pipelineConfig":          map[string]interface{}{},
			"tierOverride":            nil,
			"logLevel":                "INFO",
			"debugTranscriptsEnabled": false,
			"v2ShadowEnabled":         false,
			"v2PrimaryEnabled":        false,
		},
	},
	"resolvedAt":     "2026-04-18T01:31:52.064682+00:00",
	"cascadeVersion": "v2.1-rename",
}

var pipelineFixture = map[string]interface{}{
	"agentId":  "41f239da-c492-5fe6-9334-7bbc47804a36",
	"pipeline": "olympus_native",
	"pipelineConfig": map[string]interface{}{
		"defaultLogLevel":  "INFO",
		"tenantSeededAt":   "2026-04-17-V2-005-fix-verification",
	},
	"resolvedAt":     "2026-04-18T01:32:52.722382+00:00",
	"cascadeVersion": "v2.1-rename",
}

func TestVoiceGetEffectiveConfig(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/configs/abc123/effective-config": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			jsonResponse(w, 200, effectiveConfigFixture)
		},
	})

	cfg, err := client.Voice().GetEffectiveConfig(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AgentID != "41f239da-c492-5fe6-9334-7bbc47804a36" {
		t.Errorf("agentId: got %s", cfg.AgentID)
	}
	if cfg.TenantID != "550e8400-e29b-41d4-a716-446655449100" {
		t.Errorf("tenantId: got %s", cfg.TenantID)
	}
	if cfg.Pipeline != "olympus_native" {
		t.Errorf("pipeline: got %s", cfg.Pipeline)
	}
	if cfg.TierOverride == nil || *cfg.TierOverride != "T3" {
		t.Errorf("tierOverride: got %v", cfg.TierOverride)
	}
	if cfg.TelephonyProvider != "telnyx" {
		t.Errorf("telephonyProvider: got %s", cfg.TelephonyProvider)
	}
	if cfg.PreferredSampleRate != 48000 {
		t.Errorf("preferredSampleRate: got %d", cfg.PreferredSampleRate)
	}
	if cfg.VoiceDefaults.Tenant == nil {
		t.Fatalf("voiceDefaults.tenant was nil, expected populated")
	}
	if cfg.VoiceDefaults.Tenant.TierOverride == nil || *cfg.VoiceDefaults.Tenant.TierOverride != "T3" {
		t.Errorf("voiceDefaults.tenant.tierOverride: got %v", cfg.VoiceDefaults.Tenant.TierOverride)
	}
	if cfg.VoiceDefaults.Agent == nil || cfg.VoiceDefaults.Agent.Pipeline != "olympus_native" {
		t.Errorf("voiceDefaults.agent.pipeline: got %v", cfg.VoiceDefaults.Agent)
	}
	if cfg.CascadeVersion != "v2.1-rename" {
		t.Errorf("cascadeVersion: got %s", cfg.CascadeVersion)
	}
}

func TestVoiceGetPipeline(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/configs/abc123/pipeline": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, pipelineFixture)
		},
	})
	p, err := client.Voice().GetPipeline(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Pipeline != "olympus_native" {
		t.Errorf("pipeline: got %s", p.Pipeline)
	}
	if p.CascadeVersion != "v2.1-rename" {
		t.Errorf("cascadeVersion: got %s", p.CascadeVersion)
	}
}

func TestConnectCreateLead(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/leads": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["email"] != "scott@example.com" {
				t.Errorf("email: got %v", body["email"])
			}
			utm, ok := body["utm"].(map[string]interface{})
			if !ok {
				t.Fatalf("utm not an object: %v", body["utm"])
			}
			if utm["source"] != "twitter" {
				t.Errorf("utm.source: got %v", utm["source"])
			}
			jsonResponse(w, 201, map[string]interface{}{
				"lead_id":    "lead-xyz",
				"status":     "created",
				"created_at": "2026-04-18T03:00:00Z",
			})
		},
	})

	res, err := client.Connect().CreateLead(context.Background(), CreateLeadRequest{
		Email:   "scott@example.com",
		Name:    "Scott",
		Company: "Olympus",
		Source:  "marketing-landing",
		UTM: &UTM{
			Source:   "twitter",
			Campaign: "spring-launch",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.LeadID != "lead-xyz" {
		t.Errorf("LeadID: got %s", res.LeadID)
	}
	if res.Status != "created" {
		t.Errorf("Status: got %s", res.Status)
	}
}

func TestConnectCreateLeadRequiresEmail(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	_, err := client.Connect().CreateLead(context.Background(), CreateLeadRequest{})
	if err == nil {
		t.Fatal("expected error for missing email, got nil")
	}
}
