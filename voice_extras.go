package olympus

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// This file extends VoiceService (defined in voice.go) with the full dart
// parity surface added in Wave 2 of the SDK 1.0 campaign (#3216).
//
// The V2-005 cascade resolver methods (GetEffectiveConfig, GetPipeline)
// stay in voice.go; everything below — agent CRUD, personas, ambiance,
// templates, conversations, analytics, campaigns, phone numbers,
// marketplace voices, calls, speaker profiles, voice profiles, edge
// pipeline, caller profiles, escalation config, agent testing — was
// ported here.

// ---------------------------------------------------------------------------
// Agents (CRUD + V2-005 alias methods)
// ---------------------------------------------------------------------------

// ListConfigsOptions filters ListConfigs.
type ListConfigsOptions struct {
	Page     int
	Limit    int
	TenantID string
}

// ListConfigs lists all voice agent configurations.
func (s *VoiceService) ListConfigs(ctx context.Context, opts *ListConfigsOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			q.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.TenantID != "" {
			q.Set("tenant_id", opts.TenantID)
		}
	}
	raw, err := s.http.get(ctx, "/voice-agents/configs", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "configs"), nil
}

// GetConfig returns a single voice agent configuration.
func (s *VoiceService) GetConfig(ctx context.Context, configID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice-agents/configs/%s", configID), nil)
}

// CreateConfig creates a new voice agent configuration.
func (s *VoiceService) CreateConfig(ctx context.Context, config map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/voice-agents/configs", config)
}

// UpdateConfig updates an existing voice agent configuration.
func (s *VoiceService) UpdateConfig(ctx context.Context, configID string, config map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, fmt.Sprintf("/voice-agents/configs/%s", configID), config)
}

// DeleteConfig deletes a voice agent configuration.
func (s *VoiceService) DeleteConfig(ctx context.Context, configID string) error {
	return s.http.del(ctx, fmt.Sprintf("/voice-agents/configs/%s", configID))
}

// ListAgents is an alias for ListConfigs.
func (s *VoiceService) ListAgents(ctx context.Context, opts *ListConfigsOptions) ([]map[string]interface{}, error) {
	return s.ListConfigs(ctx, opts)
}

// GetAgent is an alias for GetConfig.
func (s *VoiceService) GetAgent(ctx context.Context, agentID string) (map[string]interface{}, error) {
	return s.GetConfig(ctx, agentID)
}

// DeleteAgent is an alias for DeleteConfig.
func (s *VoiceService) DeleteAgent(ctx context.Context, agentID string) error {
	return s.DeleteConfig(ctx, agentID)
}

// CreateAgentRequest holds the fields for CreateAgent.
type CreateAgentRequest struct {
	FromTemplateID  string                   `json:"from_template_id,omitempty"`
	Name            string                   `json:"name,omitempty"`
	VoiceID         string                   `json:"voice_id,omitempty"`
	Persona         string                   `json:"persona,omitempty"`
	Greeting        string                   `json:"greeting,omitempty"`
	PhoneNumber     string                   `json:"phone_number,omitempty"`
	LocationID      string                   `json:"location_id,omitempty"`
	AmbianceConfig  map[string]interface{}   `json:"ambiance_config,omitempty"`
	VoiceOverrides  map[string]interface{}   `json:"voice_overrides,omitempty"`
	BusinessHours   map[string]interface{}   `json:"business_hours,omitempty"`
	EscalationRules []map[string]interface{} `json:"escalation_rules,omitempty"`
}

// CreateAgent creates a new voice agent. Mirrors the dart self-service
// agent CRUD surface.
func (s *VoiceService) CreateAgent(ctx context.Context, req CreateAgentRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	if req.FromTemplateID != "" {
		body["from_template_id"] = req.FromTemplateID
	}
	if req.Name != "" {
		body["name"] = req.Name
	}
	if req.VoiceID != "" {
		body["voice_id"] = req.VoiceID
	}
	if req.Persona != "" {
		body["persona"] = req.Persona
	}
	if req.Greeting != "" {
		body["greeting"] = req.Greeting
	}
	if req.PhoneNumber != "" {
		body["phone_number"] = req.PhoneNumber
	}
	if req.LocationID != "" {
		body["location_id"] = req.LocationID
	}
	if req.AmbianceConfig != nil {
		body["ambiance_config"] = req.AmbianceConfig
	}
	if req.VoiceOverrides != nil {
		body["voice_overrides"] = req.VoiceOverrides
	}
	if req.BusinessHours != nil {
		body["business_hours"] = req.BusinessHours
	}
	if req.EscalationRules != nil {
		body["escalation_rules"] = req.EscalationRules
	}
	return s.http.post(ctx, "/voice-agents/configs", body)
}

// UpdateAgentRequest holds the mutable fields supported by UpdateAgent.
type UpdateAgentRequest struct {
	Name            string                   `json:"name,omitempty"`
	VoiceID         string                   `json:"voice_id,omitempty"`
	Persona         string                   `json:"persona,omitempty"`
	Greeting        string                   `json:"greeting,omitempty"`
	AmbianceConfig  map[string]interface{}   `json:"ambiance_config,omitempty"`
	VoiceOverrides  map[string]interface{}   `json:"voice_overrides,omitempty"`
	BusinessHours   map[string]interface{}   `json:"business_hours,omitempty"`
	EscalationRules []map[string]interface{} `json:"escalation_rules,omitempty"`
	IsActive        *bool                    `json:"is_active,omitempty"`
}

// UpdateAgent updates mutable fields on an existing agent.
func (s *VoiceService) UpdateAgent(ctx context.Context, agentID string, req UpdateAgentRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	if req.Name != "" {
		body["name"] = req.Name
	}
	if req.VoiceID != "" {
		body["voice_id"] = req.VoiceID
	}
	if req.Persona != "" {
		body["persona"] = req.Persona
	}
	if req.Greeting != "" {
		body["greeting"] = req.Greeting
	}
	if req.AmbianceConfig != nil {
		body["ambiance_config"] = req.AmbianceConfig
	}
	if req.VoiceOverrides != nil {
		body["voice_overrides"] = req.VoiceOverrides
	}
	if req.BusinessHours != nil {
		body["business_hours"] = req.BusinessHours
	}
	if req.EscalationRules != nil {
		body["escalation_rules"] = req.EscalationRules
	}
	if req.IsActive != nil {
		body["is_active"] = *req.IsActive
	}
	return s.http.put(ctx, fmt.Sprintf("/voice-agents/configs/%s", agentID), body)
}

// CloneAgentRequest holds the fields for CloneAgent.
type CloneAgentRequest struct {
	NewName     string `json:"new_name,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	LocationID  string `json:"location_id,omitempty"`
}

// CloneAgent clones an existing agent.
func (s *VoiceService) CloneAgent(ctx context.Context, agentID string, req CloneAgentRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	if req.NewName != "" {
		body["new_name"] = req.NewName
	}
	if req.PhoneNumber != "" {
		body["phone_number"] = req.PhoneNumber
	}
	if req.LocationID != "" {
		body["location_id"] = req.LocationID
	}
	return s.http.post(ctx, fmt.Sprintf("/voice-agents/configs/%s/clone", agentID), body)
}

// PreviewAgentVoiceRequest holds the fields for PreviewAgentVoice.
type PreviewAgentVoiceRequest struct {
	SampleText     string                 `json:"sample_text"`
	VoiceID        string                 `json:"voice_id,omitempty"`
	VoiceOverrides map[string]interface{} `json:"voice_overrides,omitempty"`
}

// PreviewAgentVoice generates a TTS preview clip for an agent.
func (s *VoiceService) PreviewAgentVoice(ctx context.Context, agentID string, req PreviewAgentVoiceRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{"sample_text": req.SampleText}
	if req.VoiceID != "" {
		body["voice_id"] = req.VoiceID
	}
	if req.VoiceOverrides != nil {
		body["voice_overrides"] = req.VoiceOverrides
	}
	return s.http.post(ctx, fmt.Sprintf("/voice-agents/configs/%s/preview", agentID), body)
}

// ListGeminiVoices lists the catalog of available Gemini Live voices.
func (s *VoiceService) ListGeminiVoices(ctx context.Context, language string) ([]map[string]interface{}, error) {
	q := url.Values{}
	if language != "" {
		q.Set("language", language)
	}
	raw, err := s.http.get(ctx, "/voice/voices", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "voices"), nil
}

// ---------------------------------------------------------------------------
// Voice Pool (v0.8.0 — #232)
// ---------------------------------------------------------------------------

// GetPool returns the voice pool (persona rotation) for an agent.
func (s *VoiceService) GetPool(ctx context.Context, agentID string) ([]interface{}, error) {
	resp, err := s.http.get(ctx, fmt.Sprintf("/voice-agents/%s/pool", agentID), nil)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"pool", "entries", "data"} {
		if items, ok := resp[key].([]interface{}); ok {
			return items, nil
		}
	}
	return []interface{}{}, nil
}

// AddToPool adds a persona to an agent's voice pool.
func (s *VoiceService) AddToPool(ctx context.Context, agentID string, entry map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/voice-agents/%s/pool", agentID), entry)
}

// RemoveFromPool removes a persona from an agent's voice pool.
func (s *VoiceService) RemoveFromPool(ctx context.Context, agentID, entryID string) error {
	return s.http.del(ctx, fmt.Sprintf("/voice-agents/%s/pool/%s", agentID, entryID))
}

// ---------------------------------------------------------------------------
// Schedule (v0.8.0 — #232)
// ---------------------------------------------------------------------------

// GetSchedule returns the operating schedule for an agent.
func (s *VoiceService) GetSchedule(ctx context.Context, agentID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice-agents/%s/schedule", agentID), nil)
}

// UpdateSchedule updates the operating schedule for an agent.
func (s *VoiceService) UpdateSchedule(ctx context.Context, agentID string, request map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, fmt.Sprintf("/voice-agents/%s/schedule", agentID), request)
}

// ---------------------------------------------------------------------------
// Provisioning Wizard (v0.6.0 — Issue #84)
// ---------------------------------------------------------------------------

// ProvisionAgentRequest holds the fields for ProvisionAgent.
type ProvisionAgentRequest struct {
	AgentID      string                 `json:"-"`
	TenantID     string                 `json:"tenant_id"`
	VoiceName    string                 `json:"voice_name"`
	Profile      map[string]interface{} `json:"profile"`
	GreetingText string                 `json:"greeting_text"`
}

// ProvisionAgent starts the provisioning wizard for a new agent.
func (s *VoiceService) ProvisionAgent(ctx context.Context, req ProvisionAgentRequest) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/ether/voice/agents/%s/provision-wizard", req.AgentID), map[string]interface{}{
		"tenant_id":     req.TenantID,
		"voice_name":    req.VoiceName,
		"profile":       req.Profile,
		"greeting_text": req.GreetingText,
	})
}

// GetProvisioningStatus returns the current status of a provisioning job.
func (s *VoiceService) GetProvisioningStatus(ctx context.Context, agentID, jobID string) (map[string]interface{}, error) {
	q := url.Values{}
	q.Set("job_id", jobID)
	return s.http.get(ctx, fmt.Sprintf("/ether/voice/agents/%s/provisioning-status", agentID), q)
}

// ---------------------------------------------------------------------------
// Persona library
// ---------------------------------------------------------------------------

// ListPersonasOptions filters ListPersonas.
type ListPersonasOptions struct {
	Category    string
	Industry    string
	PremiumOnly *bool
}

// ListPersonas lists curated voice personas.
func (s *VoiceService) ListPersonas(ctx context.Context, opts *ListPersonasOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Category != "" {
			q.Set("category", opts.Category)
		}
		if opts.Industry != "" {
			q.Set("industry", opts.Industry)
		}
		if opts.PremiumOnly != nil {
			q.Set("premium_only", strconv.FormatBool(*opts.PremiumOnly))
		}
	}
	raw, err := s.http.get(ctx, "/voice/personas", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "personas"), nil
}

// GetPersona returns a single persona by ID or slug.
func (s *VoiceService) GetPersona(ctx context.Context, idOrSlug string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice/personas/%s", idOrSlug), nil)
}

// ApplyPersonaToAgent applies a persona to an existing agent.
func (s *VoiceService) ApplyPersonaToAgent(ctx context.Context, agentID, personaIDOrSlug string) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/voice-agents/configs/%s/apply-persona", agentID), map[string]interface{}{
		"persona": personaIDOrSlug,
	})
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

// ListAgentTemplates lists voice agent templates.
func (s *VoiceService) ListAgentTemplates(ctx context.Context, scope string) ([]map[string]interface{}, error) {
	q := url.Values{}
	if scope != "" {
		q.Set("scope", scope)
	}
	raw, err := s.http.get(ctx, "/voice-agents/templates", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "templates"), nil
}

// ListTemplates lists available agent templates (no filter).
func (s *VoiceService) ListTemplates(ctx context.Context) ([]map[string]interface{}, error) {
	return s.ListAgentTemplates(ctx, "")
}

// InstantiateAgentTemplateRequest holds the fields for InstantiateAgentTemplate.
type InstantiateAgentTemplateRequest struct {
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number,omitempty"`
	LocationID  string `json:"location_id,omitempty"`
}

// InstantiateAgentTemplate instantiates a new agent from an existing template.
func (s *VoiceService) InstantiateAgentTemplate(ctx context.Context, templateID string, req InstantiateAgentTemplateRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{"name": req.Name}
	if req.PhoneNumber != "" {
		body["phone_number"] = req.PhoneNumber
	}
	if req.LocationID != "" {
		body["location_id"] = req.LocationID
	}
	return s.http.post(ctx, fmt.Sprintf("/voice-agents/templates/%s/instantiate", templateID), body)
}

// PublishAgentAsTemplateRequest holds the fields for PublishAgentAsTemplate.
type PublishAgentAsTemplateRequest struct {
	Scope       string `json:"scope"`
	Description string `json:"description,omitempty"`
}

// PublishAgentAsTemplate publishes the current agent as a template.
func (s *VoiceService) PublishAgentAsTemplate(ctx context.Context, agentID string, req PublishAgentAsTemplateRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{"scope": req.Scope}
	if req.Description != "" {
		body["description"] = req.Description
	}
	return s.http.post(ctx, fmt.Sprintf("/voice-agents/configs/%s/publish-template", agentID), body)
}

// ---------------------------------------------------------------------------
// Background ambiance
// ---------------------------------------------------------------------------

// ListAmbianceLibrary lists the curated library of ambient beds.
func (s *VoiceService) ListAmbianceLibrary(ctx context.Context, category string) ([]map[string]interface{}, error) {
	q := url.Values{}
	if category != "" {
		q.Set("category", category)
	}
	raw, err := s.http.get(ctx, "/voice/ambiance/library", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "beds"), nil
}

// UploadAmbianceBedRequest holds the fields for UploadAmbianceBed.
type UploadAmbianceBedRequest struct {
	Name        string `json:"name"`
	AudioBytes  []byte `json:"-"`
	TimeOfDay   string `json:"time_of_day,omitempty"`
	Description string `json:"description,omitempty"`
}

// UploadAmbianceBed uploads a custom ambient bed. Audio is base64-encoded
// for parity with the dart SDK transport.
func (s *VoiceService) UploadAmbianceBed(ctx context.Context, req UploadAmbianceBedRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"name":         req.Name,
		"audio_base64": base64.StdEncoding.EncodeToString(req.AudioBytes),
	}
	if req.TimeOfDay != "" {
		body["time_of_day"] = req.TimeOfDay
	}
	if req.Description != "" {
		body["description"] = req.Description
	}
	return s.http.post(ctx, "/voice/ambiance/upload", body)
}

// UpdateAgentAmbianceRequest holds the fields for UpdateAgentAmbiance.
type UpdateAgentAmbianceRequest struct {
	Enabled           *bool             `json:"enabled,omitempty"`
	Intensity         *float64          `json:"intensity,omitempty"`
	DefaultR2Key      string            `json:"default_r2_key,omitempty"`
	TimeOfDayVariants map[string]string `json:"time_of_day_variants,omitempty"`
}

// UpdateAgentAmbiance updates an agent's ambiance configuration.
func (s *VoiceService) UpdateAgentAmbiance(ctx context.Context, agentID string, req UpdateAgentAmbianceRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	if req.Enabled != nil {
		body["enabled"] = *req.Enabled
	}
	if req.Intensity != nil {
		body["intensity"] = *req.Intensity
	}
	if req.DefaultR2Key != "" {
		body["default_r2_key"] = req.DefaultR2Key
	}
	if req.TimeOfDayVariants != nil {
		body["time_of_day_variants"] = req.TimeOfDayVariants
	}
	return s.http.patch(ctx, fmt.Sprintf("/voice-agents/configs/%s/ambiance", agentID), body)
}

// UpdateAgentVoiceOverridesRequest holds the fields for
// UpdateAgentVoiceOverrides.
type UpdateAgentVoiceOverridesRequest struct {
	Pitch           *float64 `json:"pitch,omitempty"`
	Speed           *float64 `json:"speed,omitempty"`
	Warmth          *float64 `json:"warmth,omitempty"`
	RegionalDialect string   `json:"regional_dialect,omitempty"`
}

// UpdateAgentVoiceOverrides updates an agent's voice tuning overrides.
func (s *VoiceService) UpdateAgentVoiceOverrides(ctx context.Context, agentID string, req UpdateAgentVoiceOverridesRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	if req.Pitch != nil {
		body["pitch"] = *req.Pitch
	}
	if req.Speed != nil {
		body["speed"] = *req.Speed
	}
	if req.Warmth != nil {
		body["warmth"] = *req.Warmth
	}
	if req.RegionalDialect != "" {
		body["regional_dialect"] = req.RegionalDialect
	}
	return s.http.patch(ctx, fmt.Sprintf("/voice-agents/configs/%s/voice-overrides", agentID), body)
}

// ---------------------------------------------------------------------------
// Workflow Templates
// ---------------------------------------------------------------------------

// ListWorkflowTemplatesOptions filters ListWorkflowTemplates.
type ListWorkflowTemplatesOptions struct {
	Page  int
	Limit int
}

// ListWorkflowTemplates lists all workflow templates for the current tenant.
func (s *VoiceService) ListWorkflowTemplates(ctx context.Context, opts *ListWorkflowTemplatesOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			q.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	raw, err := s.http.get(ctx, "/voice/workflow-templates", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "templates"), nil
}

// CreateWorkflowTemplate creates a new workflow template.
func (s *VoiceService) CreateWorkflowTemplate(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/voice/workflow-templates", request)
}

// GetWorkflowTemplate returns a single workflow template by ID.
func (s *VoiceService) GetWorkflowTemplate(ctx context.Context, id string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice/workflow-templates/%s", id), nil)
}

// DeleteWorkflowTemplate deletes a workflow template.
func (s *VoiceService) DeleteWorkflowTemplate(ctx context.Context, id string) error {
	return s.http.del(ctx, fmt.Sprintf("/voice/workflow-templates/%s", id))
}

// CreateWorkflowInstance instantiates a workflow from a template.
func (s *VoiceService) CreateWorkflowInstance(ctx context.Context, templateID string, params map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/voice/workflow-templates/%s/instances", templateID), params)
}

// ---------------------------------------------------------------------------
// Voicemail (v0.8.0 — #232)
// ---------------------------------------------------------------------------

// ListVoicemailsOptions filters ListVoicemails.
type ListVoicemailsOptions struct {
	CallerPhone string
	Page        int
	Limit       int
}

// ListVoicemails lists voicemails for the tenant.
func (s *VoiceService) ListVoicemails(ctx context.Context, opts *ListVoicemailsOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.CallerPhone != "" {
			q.Set("caller_phone", opts.CallerPhone)
		}
		if opts.Page > 0 {
			q.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	raw, err := s.http.get(ctx, "/voice/voicemails", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "voicemails"), nil
}

// UpdateVoicemail updates a voicemail (mark as read, resolve).
func (s *VoiceService) UpdateVoicemail(ctx context.Context, id string, data map[string]interface{}) (map[string]interface{}, error) {
	return s.http.patch(ctx, fmt.Sprintf("/voice/voicemails/%s", id), data)
}

// GetVoicemailAudioURL returns a signed URL for a voicemail audio recording.
func (s *VoiceService) GetVoicemailAudioURL(ctx context.Context, id string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice/voicemails/%s/audio", id), nil)
}

// ---------------------------------------------------------------------------
// Conversations
// ---------------------------------------------------------------------------

// ListConversationsOptions filters ListConversations.
type ListConversationsOptions struct {
	AgentID  string
	Status   string
	Page     int
	Limit    int
	TenantID string
}

// ListConversations lists voice conversations with optional filters.
func (s *VoiceService) ListConversations(ctx context.Context, opts *ListConversationsOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.AgentID != "" {
			q.Set("agent_id", opts.AgentID)
		}
		if opts.Status != "" {
			q.Set("status", opts.Status)
		}
		if opts.Page > 0 {
			q.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.TenantID != "" {
			q.Set("tenant_id", opts.TenantID)
		}
	}
	raw, err := s.http.get(ctx, "/voice-agents/conversations", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "conversations"), nil
}

// GetConversation returns a single conversation with its transcript and metadata.
func (s *VoiceService) GetConversation(ctx context.Context, conversationID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice-agents/conversations/%s", conversationID), nil)
}

// ListVoiceMessagesOptions filters ListMessages on the VoiceService.
//
// Distinct from ListMessagesOptions on MessagesService — that filters the
// department message queue; this filters per-department voice transcripts.
type ListVoiceMessagesOptions struct {
	Department string
	Page       int
	Limit      int
}

// ListMessages lists department messages.
func (s *VoiceService) ListMessages(ctx context.Context, opts *ListVoiceMessagesOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Department != "" {
			q.Set("department", opts.Department)
		}
		if opts.Page > 0 {
			q.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	raw, err := s.http.get(ctx, "/voice/messages", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "messages"), nil
}

// ---------------------------------------------------------------------------
// Analytics
// ---------------------------------------------------------------------------

// GetAnalyticsOptions filters GetAnalytics.
type GetAnalyticsOptions struct {
	AgentID string
	From    string
	To      string
}

// GetAnalytics returns voice analytics (call volume, duration, sentiment, ...).
func (s *VoiceService) GetAnalytics(ctx context.Context, opts *GetAnalyticsOptions) (map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.AgentID != "" {
			q.Set("agent_id", opts.AgentID)
		}
		if opts.From != "" {
			q.Set("from", opts.From)
		}
		if opts.To != "" {
			q.Set("to", opts.To)
		}
	}
	return s.http.get(ctx, "/voice-agents/analytics", q)
}

// ---------------------------------------------------------------------------
// Campaigns
// ---------------------------------------------------------------------------

// ListCampaignsOptions filters ListCampaigns.
type ListCampaignsOptions struct {
	Page  int
	Limit int
}

// ListCampaigns lists outbound voice campaigns.
func (s *VoiceService) ListCampaigns(ctx context.Context, opts *ListCampaignsOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			q.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	raw, err := s.http.get(ctx, "/voice-agents/campaigns", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "campaigns"), nil
}

// GetCampaign returns a single campaign by ID.
func (s *VoiceService) GetCampaign(ctx context.Context, campaignID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice-agents/campaigns/%s", campaignID), nil)
}

// CreateCampaign creates a new outbound campaign.
func (s *VoiceService) CreateCampaign(ctx context.Context, campaign map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/voice-agents/campaigns", campaign)
}

// UpdateCampaign updates an existing campaign.
func (s *VoiceService) UpdateCampaign(ctx context.Context, campaignID string, campaign map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, fmt.Sprintf("/voice-agents/campaigns/%s", campaignID), campaign)
}

// DeleteCampaign deletes a campaign.
func (s *VoiceService) DeleteCampaign(ctx context.Context, campaignID string) error {
	return s.http.del(ctx, fmt.Sprintf("/voice-agents/campaigns/%s", campaignID))
}

// ---------------------------------------------------------------------------
// Phone Numbers
// ---------------------------------------------------------------------------

// ListNumbersOptions filters ListNumbers.
type ListNumbersOptions struct {
	Page  int
	Limit int
}

// ListNumbers lists provisioned phone numbers.
func (s *VoiceService) ListNumbers(ctx context.Context, opts *ListNumbersOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			q.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	raw, err := s.http.get(ctx, "/voice/phone-numbers", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "numbers"), nil
}

// GetNumber returns details for a single phone number.
func (s *VoiceService) GetNumber(ctx context.Context, numberID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice/phone-numbers/%s", numberID), nil)
}

// ProvisionNumber provisions a new phone number.
func (s *VoiceService) ProvisionNumber(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/voice/phone-numbers/provision", request)
}

// ReleaseNumber releases a provisioned phone number.
func (s *VoiceService) ReleaseNumber(ctx context.Context, numberID string) error {
	return s.http.del(ctx, fmt.Sprintf("/voice/phone-numbers/%s", numberID))
}

// AssignNumber assigns a phone number to a voice agent.
func (s *VoiceService) AssignNumber(ctx context.Context, numberID, agentID string) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/voice/phone-numbers/%s/assign", numberID), map[string]interface{}{
		"agent_id": agentID,
	})
}

// SearchNumbersOptions filters SearchNumbers.
type SearchNumbersOptions struct {
	AreaCode string
	Contains string
	Country  string
	Limit    int
}

// SearchNumbers searches available phone numbers by area code or pattern.
func (s *VoiceService) SearchNumbers(ctx context.Context, opts *SearchNumbersOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.AreaCode != "" {
			q.Set("area_code", opts.AreaCode)
		}
		if opts.Contains != "" {
			q.Set("contains", opts.Contains)
		}
		if opts.Country != "" {
			q.Set("country", opts.Country)
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	raw, err := s.http.get(ctx, "/voice/phone-numbers/search", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "numbers"), nil
}

// PortNumber initiates a number port-in request.
func (s *VoiceService) PortNumber(ctx context.Context, portRequest map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/voice/phone-numbers/port", portRequest)
}

// GetPortStatus returns the status of a port-in request.
func (s *VoiceService) GetPortStatus(ctx context.Context, portID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice/phone-numbers/port/%s", portID), nil)
}

// CancelPort cancels a pending port-in request.
func (s *VoiceService) CancelPort(ctx context.Context, portID string) error {
	return s.http.del(ctx, fmt.Sprintf("/voice/phone-numbers/port/%s", portID))
}

// ---------------------------------------------------------------------------
// Marketplace (Voices & Packs)
// ---------------------------------------------------------------------------

// ListVoicesOptions filters ListVoices.
type ListVoicesOptions struct {
	Language string
	Gender   string
	Limit    int
}

// ListVoices lists available voices in the marketplace.
func (s *VoiceService) ListVoices(ctx context.Context, opts *ListVoicesOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Language != "" {
			q.Set("language", opts.Language)
		}
		if opts.Gender != "" {
			q.Set("gender", opts.Gender)
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	raw, err := s.http.get(ctx, "/voice/marketplace/voices", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "voices"), nil
}

// GetMyVoices returns voices installed for the current tenant.
func (s *VoiceService) GetMyVoices(ctx context.Context) ([]map[string]interface{}, error) {
	raw, err := s.http.get(ctx, "/voice/marketplace/my-voices", nil)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "voices"), nil
}

// ListPacksOptions filters ListPacks.
type ListPacksOptions struct {
	Limit int
}

// ListPacks lists voice packs (bundles of voices).
func (s *VoiceService) ListPacks(ctx context.Context, opts *ListPacksOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil && opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	raw, err := s.http.get(ctx, "/voice/marketplace/packs", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "packs"), nil
}

// GetPack returns a single voice pack by ID.
func (s *VoiceService) GetPack(ctx context.Context, packID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice/marketplace/packs/%s", packID), nil)
}

// InstallPack installs a voice pack for the current tenant.
func (s *VoiceService) InstallPack(ctx context.Context, packID string) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/voice/marketplace/packs/%s/install", packID), nil)
}

// ---------------------------------------------------------------------------
// Calls
// ---------------------------------------------------------------------------

// EndCall ends an active call by ID.
func (s *VoiceService) EndCall(ctx context.Context, callID string) error {
	_, err := s.http.post(ctx, fmt.Sprintf("/voice/calls/%s/end", callID), nil)
	return err
}

// ---------------------------------------------------------------------------
// Speaker
// ---------------------------------------------------------------------------

// GetSpeakerProfile returns the speaker profile for a given speaker ID.
func (s *VoiceService) GetSpeakerProfile(ctx context.Context, speakerID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice/speaker/%s", speakerID), nil)
}

// EnrollSpeaker enrolls a new speaker for voice recognition.
func (s *VoiceService) EnrollSpeaker(ctx context.Context, enrollment map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/voice/speaker/enroll", enrollment)
}

// AddWords adds custom words or phrases for a speaker's vocabulary.
func (s *VoiceService) AddWords(ctx context.Context, speakerID string, words []string) error {
	_, err := s.http.post(ctx, fmt.Sprintf("/voice/speaker/%s/words", speakerID), map[string]interface{}{
		"words": words,
	})
	return err
}

// ---------------------------------------------------------------------------
// Profiles
// ---------------------------------------------------------------------------

// ListProfilesOptions filters ListProfiles.
type ListProfilesOptions struct {
	Page  int
	Limit int
}

// ListProfiles lists voice profiles for the tenant.
func (s *VoiceService) ListProfiles(ctx context.Context, opts *ListProfilesOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			q.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	raw, err := s.http.get(ctx, "/voice/profiles", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "profiles"), nil
}

// GetProfile returns a single voice profile by ID.
func (s *VoiceService) GetProfile(ctx context.Context, profileID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice/profiles/%s", profileID), nil)
}

// CreateProfile creates a new voice profile.
func (s *VoiceService) CreateProfile(ctx context.Context, profile map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/voice/profiles", profile)
}

// UpdateProfile updates an existing voice profile.
func (s *VoiceService) UpdateProfile(ctx context.Context, profileID string, profile map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, fmt.Sprintf("/voice/profiles/%s", profileID), profile)
}

// ---------------------------------------------------------------------------
// Edge Voice Pipeline (CF Container — STT→Ether→TTS)
// ---------------------------------------------------------------------------

// ProcessAudioRequest holds the fields for ProcessAudio.
type ProcessAudioRequest struct {
	AudioBytes []byte `json:"-"`
	Language   string `json:"language,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
	VoiceID    string `json:"voice_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
}

// ProcessAudio processes recorded audio through the full edge voice pipeline.
//
// Sends audio to the CF Container voice pipeline which runs:
// STT (Workers AI Whisper, FREE) → Ether classification → AI response → TTS.
//
// Returns {transcript, response, audio_url, pipeline_ms}.
func (s *VoiceService) ProcessAudio(ctx context.Context, req ProcessAudioRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"audio": base64.StdEncoding.EncodeToString(req.AudioBytes),
	}
	if req.Language != "" {
		body["language"] = req.Language
	}
	if req.AgentID != "" {
		body["agent_id"] = req.AgentID
	}
	if req.VoiceID != "" {
		body["voice_id"] = req.VoiceID
	}
	if req.SessionID != "" {
		body["session_id"] = req.SessionID
	}
	return s.http.post(ctx, "/voice/process", body)
}

// GetVoiceWebSocketURL returns the WebSocket URL for streaming voice
// interaction. The WebSocket endpoint at /ws/voice accepts:
//   - {type: "audio", data: "<base64>"}  — audio chunks
//   - {type: "barge_in"}                  — interrupt current response
//   - {type: "ping"}                      — keepalive
//
// And responds with:
//   - {type: "transcript", text: "..."}                         — interim STT
//   - {type: "response",   text: "...", audio_url: "..."}       — AI response
//   - {type: "pong"}                                            — keepalive
//
// The returned URL substitutes wss:// for the configured https:// base.
func (s *VoiceService) GetVoiceWebSocketURL(sessionID string) string {
	base := strings.Replace(s.http.config.effectiveBaseURL(), "https://", "wss://", 1)
	if sessionID != "" {
		return fmt.Sprintf("%s/ws/voice?session_id=%s", base, url.QueryEscape(sessionID))
	}
	return base + "/ws/voice"
}

// PipelineHealth checks edge voice pipeline health.
func (s *VoiceService) PipelineHealth(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/voice/pipeline/health", nil)
}

// ---------------------------------------------------------------------------
// Caller Profiles (v0.3.0 — Issue #2868)
// ---------------------------------------------------------------------------

// GetCallerProfile looks up a caller profile by phone number for personalized
// voice AI. Returns preferences, order history, loyalty tier, and past
// interactions.
func (s *VoiceService) GetCallerProfile(ctx context.Context, phoneNumber string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/caller-profiles/%s", phoneNumber), nil)
}

// ListCallerProfilesOptions controls pagination on ListCallerProfiles.
// Limit defaults to 50 (per dart parity); Offset defaults to 0.
type ListCallerProfilesOptions struct {
	Limit  int
	Offset int
}

// ListCallerProfiles lists all caller profiles for the current tenant
// (paginated).
func (s *VoiceService) ListCallerProfiles(ctx context.Context, opts *ListCallerProfilesOptions) (map[string]interface{}, error) {
	limit := 50
	offset := 0
	if opts != nil {
		if opts.Limit > 0 {
			limit = opts.Limit
		}
		if opts.Offset > 0 {
			offset = opts.Offset
		}
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	return s.http.get(ctx, "/caller-profiles", q)
}

// UpsertCallerProfile creates or updates a caller profile.
func (s *VoiceService) UpsertCallerProfile(ctx context.Context, profile map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/caller-profiles", profile)
}

// DeleteCallerProfile deletes a caller profile.
func (s *VoiceService) DeleteCallerProfile(ctx context.Context, profileID string) error {
	return s.http.del(ctx, fmt.Sprintf("/caller-profiles/%s", profileID))
}

// RecordCallerOrder records an order for a caller (updates stats + loyalty
// points).
func (s *VoiceService) RecordCallerOrder(ctx context.Context, phoneNumber string, orderData map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/caller-profiles/%s/orders", phoneNumber), orderData)
}

// ---------------------------------------------------------------------------
// Escalation Config (v0.3.0 — Issue #2870)
// ---------------------------------------------------------------------------

// GetEscalationConfig returns voice agent escalation config (transfer
// targets, sentiment threshold).
func (s *VoiceService) GetEscalationConfig(ctx context.Context, agentID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice-agents/%s/escalation-config", agentID), nil)
}

// UpdateEscalationConfig updates voice agent escalation config.
func (s *VoiceService) UpdateEscalationConfig(ctx context.Context, agentID string, config map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, fmt.Sprintf("/voice-agents/%s/escalation-config", agentID), config)
}

// GetBusinessHours returns voice agent business hours.
func (s *VoiceService) GetBusinessHours(ctx context.Context, agentID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice-agents/%s/business-hours", agentID), nil)
}

// UpdateBusinessHours updates voice agent business hours.
func (s *VoiceService) UpdateBusinessHours(ctx context.Context, agentID string, hours map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, fmt.Sprintf("/voice-agents/%s/business-hours", agentID), hours)
}

// ---------------------------------------------------------------------------
// Agent Testing (v0.6.0 — Issue #170)
// ---------------------------------------------------------------------------

// TestAgentRequest holds the fields for TestAgent.
type TestAgentRequest struct {
	TenantID      string `json:"tenant_id"`
	ScenarioCount int    `json:"scenario_count"`
}

// TestAgent triggers an AI-to-AI test suite against a voice agent. The
// platform generates realistic caller scenarios, executes them against the
// agent, and returns a scorecard with transcripts and accuracy ratings.
func (s *VoiceService) TestAgent(ctx context.Context, req TestAgentRequest) (map[string]interface{}, error) {
	scenarioCount := req.ScenarioCount
	if scenarioCount == 0 {
		scenarioCount = 5
	}
	return s.http.post(ctx, "/voice-agents/test", map[string]interface{}{
		"tenant_id":      req.TenantID,
		"scenario_count": scenarioCount,
	})
}
