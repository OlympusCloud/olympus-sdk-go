package olympus

import (
	"context"
	"encoding/json"
	"fmt"
)

// VoiceService handles Voice Agent V2 cascade resolver endpoints (V2-005) and
// adjacent voice-agent operations.
//
// Wraps the Olympus Python voice-agent service via the Go API Gateway
// wildcard proxy (backend/go/cmd/api/routes_analytics.go:370).
// Routes: /voice-agents/*.
type VoiceService struct {
	http *httpClient
}

// VoiceDefaultsRung is a single rung of the voice-defaults cascade.
//
// Each rung (platform, app, tenant, agent) is either nil (no override at that
// scope) or populated with whatever subset of fields was configured there.
type VoiceDefaultsRung struct {
	Pipeline                string                 `json:"pipeline,omitempty"`
	PipelineConfig          map[string]interface{} `json:"pipelineConfig,omitempty"`
	TierOverride            *string                `json:"tierOverride,omitempty"`
	LogLevel                string                 `json:"logLevel,omitempty"`
	DebugTranscriptsEnabled *bool                  `json:"debugTranscriptsEnabled,omitempty"`
	V2ShadowEnabled         *bool                  `json:"v2ShadowEnabled,omitempty"`
	V2PrimaryEnabled        *bool                  `json:"v2PrimaryEnabled,omitempty"`
}

// VoiceDefaultsCascade holds the four rungs of the voice-defaults cascade in
// ascending-specificity order: platform → app → tenant → agent.
type VoiceDefaultsCascade struct {
	Platform *VoiceDefaultsRung `json:"platform"`
	App      *VoiceDefaultsRung `json:"app"`
	Tenant   *VoiceDefaultsRung `json:"tenant"`
	Agent    *VoiceDefaultsRung `json:"agent"`
}

// VoiceEffectiveConfig is the full merged view returned by
// GET /api/v1/voice-agents/configs/{id}/effective-config (V2-005).
type VoiceEffectiveConfig struct {
	AgentID                 string                 `json:"agentId"`
	TenantID                string                 `json:"tenantId"`
	Pipeline                string                 `json:"pipeline"`
	PipelineConfig          map[string]interface{} `json:"pipelineConfig"`
	TierOverride            *string                `json:"tierOverride,omitempty"`
	LogLevel                string                 `json:"logLevel"`
	DebugTranscriptsEnabled bool                   `json:"debugTranscriptsEnabled"`
	V2ShadowEnabled         bool                   `json:"v2ShadowEnabled"`
	V2PrimaryEnabled        bool                   `json:"v2PrimaryEnabled"`

	// Telephony bindings (present if the agent has an assigned phone number).
	TelephonyProvider   string `json:"telephonyProvider,omitempty"`
	ProviderAccountRef  string `json:"providerAccountRef,omitempty"`
	PreferredCodec      string `json:"preferredCodec,omitempty"`
	PreferredSampleRate int    `json:"preferredSampleRate,omitempty"`
	HDAudioEnabled      *bool  `json:"hdAudioEnabled,omitempty"`
	WebhookPathOverride string `json:"webhookPathOverride,omitempty"`
	V2Routed            *bool  `json:"v2Routed,omitempty"`

	VoiceDefaults  VoiceDefaultsCascade `json:"voiceDefaults"`
	ResolvedAt     string               `json:"resolvedAt"`
	CascadeVersion string               `json:"cascadeVersion"`
}

// VoicePipeline is the canonical pipeline-only subset returned by
// GET /api/v1/voice-agents/configs/{id}/pipeline (V2-005).
type VoicePipeline struct {
	AgentID        string                 `json:"agentId"`
	Pipeline       string                 `json:"pipeline"`
	PipelineConfig map[string]interface{} `json:"pipelineConfig"`
	ResolvedAt     string                 `json:"resolvedAt"`
	CascadeVersion string                 `json:"cascadeVersion"`
}

// GetEffectiveConfig resolves the effective voice-agent configuration after
// cascading platform → app → tenant → agent voice defaults.
//
// Backing endpoint: GET /api/v1/voice-agents/configs/{id}/effective-config
// (Python cascade resolver — V2-005, issue #3162).
func (s *VoiceService) GetEffectiveConfig(ctx context.Context, agentID string) (*VoiceEffectiveConfig, error) {
	path := fmt.Sprintf("/voice-agents/configs/%s/effective-config", agentID)
	raw, err := s.http.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	// The httpClient returns map[string]interface{}; round-trip through JSON
	// to populate the typed struct.
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal effective-config response: %w", err)
	}
	var cfg VoiceEffectiveConfig
	if err := json.Unmarshal(buf, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal effective-config response: %w", err)
	}
	return &cfg, nil
}

// GetPipeline resolves only the pipeline view of an agent's configuration.
// Cheaper than GetEffectiveConfig when callers only need the pipeline name +
// config (agent runtimes, provisioners).
//
// Backing endpoint: GET /api/v1/voice-agents/configs/{id}/pipeline.
func (s *VoiceService) GetPipeline(ctx context.Context, agentID string) (*VoicePipeline, error) {
	path := fmt.Sprintf("/voice-agents/configs/%s/pipeline", agentID)
	raw, err := s.http.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal pipeline response: %w", err)
	}
	var pipe VoicePipeline
	if err := json.Unmarshal(buf, &pipe); err != nil {
		return nil, fmt.Errorf("unmarshal pipeline response: %w", err)
	}
	return &pipe, nil
}
