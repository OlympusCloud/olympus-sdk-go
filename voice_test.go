package olympus

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// ----------------------------------------------------------------------------
// Agent CRUD
// ----------------------------------------------------------------------------

func TestVoice_ListConfigs_Pagination(t *testing.T) {
	var gotQuery string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/configs": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			jsonResponse(w, 200, map[string]interface{}{
				"configs": []map[string]interface{}{{"id": "agent-1"}},
			})
		},
	})
	out, err := client.Voice().ListConfigs(context.Background(), &ListConfigsOptions{
		Page: 2, Limit: 25, TenantID: "t1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "page=2") ||
		!strings.Contains(gotQuery, "limit=25") ||
		!strings.Contains(gotQuery, "tenant_id=t1") {
		t.Errorf("query: %q", gotQuery)
	}
	if len(out) != 1 || out[0]["id"] != "agent-1" {
		t.Errorf("configs parse: %v", out)
	}
}

func TestVoice_GetConfig(t *testing.T) {
	var gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/configs/abc": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			jsonResponse(w, 200, map[string]interface{}{"id": "abc"})
		},
	})
	out, err := client.Voice().GetConfig(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/voice-agents/configs/abc" {
		t.Errorf("path: %s", gotPath)
	}
	if out["id"] != "abc" {
		t.Errorf("response: %v", out)
	}
}

func TestVoice_CreateUpdateDelete_Agent(t *testing.T) {
	state := map[string]interface{}{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/configs": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&state)
			jsonResponse(w, 201, map[string]interface{}{"id": "agent-X"})
		},
		"/voice-agents/configs/agent-X": func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPut:
				_ = json.NewDecoder(r.Body).Decode(&state)
				jsonResponse(w, 200, map[string]interface{}{"id": "agent-X"})
			case http.MethodDelete:
				w.WriteHeader(204)
			}
		},
	})
	created, err := client.Voice().CreateAgent(context.Background(), CreateAgentRequest{
		Name: "Pizza Bot", VoiceID: "v1", Persona: "friendly",
		EscalationRules: []map[string]interface{}{{"keyword": "manager"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created["id"] != "agent-X" {
		t.Errorf("create response: %v", created)
	}
	if state["name"] != "Pizza Bot" {
		t.Errorf("create body: %v", state)
	}

	active := false
	if _, err := client.Voice().UpdateAgent(context.Background(), "agent-X",
		UpdateAgentRequest{Greeting: "hi", IsActive: &active}); err != nil {
		t.Fatal(err)
	}
	if state["greeting"] != "hi" || state["is_active"] != false {
		t.Errorf("update body: %v", state)
	}

	if err := client.Voice().DeleteAgent(context.Background(), "agent-X"); err != nil {
		t.Fatal(err)
	}
}

func TestVoice_CloneAgent(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/configs/src/clone": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{"id": "clone-1"})
		},
	})
	_, err := client.Voice().CloneAgent(context.Background(), "src", CloneAgentRequest{
		NewName: "Copy", PhoneNumber: "+15550000000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/voice-agents/configs/src/clone" {
		t.Errorf("path: %s", gotPath)
	}
	if gotBody["new_name"] != "Copy" || gotBody["phone_number"] != "+15550000000" {
		t.Errorf("body: %v", gotBody)
	}
}

func TestVoice_PreviewAgentVoice_RequiresSampleText(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/configs/agent-1/preview": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{"audio_url": "https://x"})
		},
	})
	_, err := client.Voice().PreviewAgentVoice(context.Background(), "agent-1",
		PreviewAgentVoiceRequest{SampleText: "hello world"})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["sample_text"] != "hello world" {
		t.Errorf("body: %v", gotBody)
	}
}

// ----------------------------------------------------------------------------
// Personas, templates, ambiance
// ----------------------------------------------------------------------------

func TestVoice_ListPersonas_Filters(t *testing.T) {
	var gotQuery string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice/personas": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			jsonResponse(w, 200, map[string]interface{}{"personas": []interface{}{}})
		},
	})
	premium := true
	if _, err := client.Voice().ListPersonas(context.Background(), &ListPersonasOptions{
		Category: "warm", Industry: "restaurant", PremiumOnly: &premium,
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "category=warm") ||
		!strings.Contains(gotQuery, "industry=restaurant") ||
		!strings.Contains(gotQuery, "premium_only=true") {
		t.Errorf("query: %q", gotQuery)
	}
}

func TestVoice_ApplyPersonaToAgent(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/configs/a1/apply-persona": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{"applied": true})
		},
	})
	_, err := client.Voice().ApplyPersonaToAgent(context.Background(), "a1", "warm-host")
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["persona"] != "warm-host" {
		t.Errorf("body: %v", gotBody)
	}
}

func TestVoice_UploadAmbianceBed_Base64(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice/ambiance/upload": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{"r2_key": "k"})
		},
	})
	bytes := []byte{0xFF, 0x00, 0xAB, 0xCD}
	expected := base64.StdEncoding.EncodeToString(bytes)
	_, err := client.Voice().UploadAmbianceBed(context.Background(), UploadAmbianceBedRequest{
		Name: "rain", AudioBytes: bytes, TimeOfDay: "evening",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["audio_base64"] != expected {
		t.Errorf("base64 mismatch: got %v want %v", gotBody["audio_base64"], expected)
	}
	if gotBody["time_of_day"] != "evening" {
		t.Errorf("time_of_day: %v", gotBody["time_of_day"])
	}
}

func TestVoice_UpdateAgentAmbiance_PointerBooleans(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/configs/a1/ambiance": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch {
				t.Errorf("method: %s", r.Method)
			}
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{})
		},
	})
	enabled := true
	intensity := 0.6
	if _, err := client.Voice().UpdateAgentAmbiance(context.Background(), "a1",
		UpdateAgentAmbianceRequest{Enabled: &enabled, Intensity: &intensity}); err != nil {
		t.Fatal(err)
	}
	if gotBody["enabled"] != true {
		t.Errorf("enabled: %v", gotBody["enabled"])
	}
	if v, _ := gotBody["intensity"].(float64); v != 0.6 {
		t.Errorf("intensity: %v", gotBody["intensity"])
	}
}

// ----------------------------------------------------------------------------
// Phone numbers + marketplace
// ----------------------------------------------------------------------------

func TestVoice_AssignNumber(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice/phone-numbers/n-1/assign": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{})
		},
	})
	_, err := client.Voice().AssignNumber(context.Background(), "n-1", "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["agent_id"] != "agent-1" {
		t.Errorf("body: %v", gotBody)
	}
}

func TestVoice_SearchNumbers(t *testing.T) {
	var gotQuery string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice/phone-numbers/search": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			jsonResponse(w, 200, map[string]interface{}{
				"numbers": []map[string]interface{}{{"phone": "+15555550100"}},
			})
		},
	})
	out, err := client.Voice().SearchNumbers(context.Background(), &SearchNumbersOptions{
		AreaCode: "415", Country: "US", Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "area_code=415") ||
		!strings.Contains(gotQuery, "country=US") ||
		!strings.Contains(gotQuery, "limit=5") {
		t.Errorf("query: %q", gotQuery)
	}
	if len(out) != 1 || out[0]["phone"] != "+15555550100" {
		t.Errorf("numbers parse: %v", out)
	}
}

func TestVoice_InstallPack(t *testing.T) {
	var gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice/marketplace/packs/pack-1/install": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			jsonResponse(w, 200, map[string]interface{}{"installed": true})
		},
	})
	out, err := client.Voice().InstallPack(context.Background(), "pack-1")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/voice/marketplace/packs/pack-1/install" {
		t.Errorf("path: %s", gotPath)
	}
	if out["installed"] != true {
		t.Errorf("response: %v", out)
	}
}

// ----------------------------------------------------------------------------
// Calls + speaker
// ----------------------------------------------------------------------------

func TestVoice_EndCall(t *testing.T) {
	var gotMethod, gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice/calls/call-1/end": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(204)
		},
	})
	if err := client.Voice().EndCall(context.Background(), "call-1"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/voice/calls/call-1/end" {
		t.Errorf("end-call route: %s %s", gotMethod, gotPath)
	}
}

func TestVoice_AddWords(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice/speaker/sp-1/words": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{})
		},
	})
	if err := client.Voice().AddWords(context.Background(), "sp-1",
		[]string{"sopressata", "calabrese"}); err != nil {
		t.Fatal(err)
	}
	words, ok := gotBody["words"].([]interface{})
	if !ok || len(words) != 2 || words[0] != "sopressata" {
		t.Errorf("words: %v", gotBody["words"])
	}
}

// ----------------------------------------------------------------------------
// Edge pipeline
// ----------------------------------------------------------------------------

func TestVoice_ProcessAudio_Base64(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice/process": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{
				"transcript":  "hi",
				"audio_url":   "https://r2/x",
				"pipeline_ms": 250,
			})
		},
	})
	bytes := []byte{0x10, 0x20, 0x30}
	expected := base64.StdEncoding.EncodeToString(bytes)
	out, err := client.Voice().ProcessAudio(context.Background(), ProcessAudioRequest{
		AudioBytes: bytes, Language: "en", AgentID: "a1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["audio"] != expected {
		t.Errorf("audio: %v want %v", gotBody["audio"], expected)
	}
	if gotBody["language"] != "en" || gotBody["agent_id"] != "a1" {
		t.Errorf("body: %v", gotBody)
	}
	if out["transcript"] != "hi" {
		t.Errorf("response: %v", out)
	}
}

func TestVoice_GetVoiceWebSocketURL(t *testing.T) {
	client := NewClient(Config{
		AppID:   "test",
		APIKey:  "k",
		BaseURL: "https://api.example.com/v1",
	})
	gotNoSession := client.Voice().GetVoiceWebSocketURL("")
	wantNoSession := "wss://api.example.com/v1/ws/voice"
	if gotNoSession != wantNoSession {
		t.Errorf("no-session: got %q, want %q", gotNoSession, wantNoSession)
	}
	gotWithSession := client.Voice().GetVoiceWebSocketURL("sess-1")
	if !strings.HasPrefix(gotWithSession, "wss://api.example.com/v1/ws/voice?session_id=") {
		t.Errorf("with-session: got %q", gotWithSession)
	}
	if !strings.Contains(gotWithSession, "sess-1") {
		t.Errorf("session id missing: %q", gotWithSession)
	}
}

// ----------------------------------------------------------------------------
// Caller profiles
// ----------------------------------------------------------------------------

func TestVoice_ListCallerProfiles_DefaultPagination(t *testing.T) {
	var gotQuery string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/caller-profiles": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			jsonResponse(w, 200, map[string]interface{}{"profiles": []interface{}{}})
		},
	})
	if _, err := client.Voice().ListCallerProfiles(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "limit=50") || !strings.Contains(gotQuery, "offset=0") {
		t.Errorf("default pagination: %q", gotQuery)
	}
}

func TestVoice_RecordCallerOrder_Path(t *testing.T) {
	var gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/caller-profiles/+15551234567/orders": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			jsonResponse(w, 201, map[string]interface{}{"order_id": "o1"})
		},
	})
	out, err := client.Voice().RecordCallerOrder(context.Background(), "+15551234567",
		map[string]interface{}{"total": 1599})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/caller-profiles/+15551234567/orders" {
		t.Errorf("path: %s", gotPath)
	}
	if out["order_id"] != "o1" {
		t.Errorf("response: %v", out)
	}
}

// ----------------------------------------------------------------------------
// Test agent + provisioning + workflow templates
// ----------------------------------------------------------------------------

func TestVoice_TestAgent_DefaultsScenarioCount(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/test": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{"score": 90})
		},
	})
	if _, err := client.Voice().TestAgent(context.Background(),
		TestAgentRequest{TenantID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if v, _ := gotBody["scenario_count"].(float64); v != 5 {
		t.Errorf("default scenario_count: %v", gotBody["scenario_count"])
	}
}

func TestVoice_TestAgent_RespectsExplicitCount(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/test": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{})
		},
	})
	if _, err := client.Voice().TestAgent(context.Background(),
		TestAgentRequest{TenantID: "t1", ScenarioCount: 12}); err != nil {
		t.Fatal(err)
	}
	if v, _ := gotBody["scenario_count"].(float64); v != 12 {
		t.Errorf("scenario_count: %v", gotBody["scenario_count"])
	}
}

func TestVoice_GetProvisioningStatus_PassesJobID(t *testing.T) {
	var gotQuery string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/ether/voice/agents/a1/provisioning-status": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			jsonResponse(w, 200, map[string]interface{}{"status": "running"})
		},
	})
	out, err := client.Voice().GetProvisioningStatus(context.Background(), "a1", "job-9")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "job_id=job-9") {
		t.Errorf("query: %q", gotQuery)
	}
	if out["status"] != "running" {
		t.Errorf("response: %v", out)
	}
}

func TestVoice_GetConfig_PropagatesNotFound(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-agents/configs/missing": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 404, map[string]interface{}{
				"error": map[string]interface{}{"code": "NOT_FOUND", "message": "no agent"},
			})
		},
	})
	_, err := client.Voice().GetConfig(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected OlympusAPIError, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Errorf("expected 404, got %d", apiErr.StatusCode)
	}
}

// ----------------------------------------------------------------------------
// Service accessors
// ----------------------------------------------------------------------------

func TestWave2Accessors(t *testing.T) {
	c := NewClient(Config{AppID: "test", APIKey: "k", BaseURL: "http://ignored"})
	if c.Identity() == nil || c.Identity() != c.Identity() {
		t.Error("Identity() accessor")
	}
	if c.SmartHome() == nil || c.SmartHome() != c.SmartHome() {
		t.Error("SmartHome() accessor")
	}
	if c.SMS() == nil || c.SMS() != c.SMS() {
		t.Error("SMS() accessor")
	}
}
