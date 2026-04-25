package olympus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// ConsentService is the app-scoped permissions consent surface.
//
// Surface matches §6 of docs/platform/APP-SCOPED-PERMISSIONS.md. Every
// method hits a platform endpoint; no client-side state. The fast-path
// bitset check lives on *OlympusClient directly (HasScopeBit).
//
// Part of olympus-cloud-gcp#3254 / #3234 epic.
type ConsentService struct {
	http *httpClient
}

// ConsentPrompt is the server-rendered consent copy + stable hash.
// The PromptHash must be echoed back on Grant calls so the server can
// verify the user saw the current catalog copy.
//
// Shape matches GET /platform/consent-prompt (#3242).
type ConsentPrompt struct {
	AppID         string `json:"app_id"`
	Scope         string `json:"scope"`
	PromptText    string `json:"prompt_text"`
	PromptHash    string `json:"prompt_hash"`
	IsDestructive bool   `json:"is_destructive"`
	RequiresMFA   bool   `json:"requires_mfa"`
	AppMayRequest bool   `json:"app_may_request"`
}

// Grant represents a row from platform_app_tenant_grants or
// platform_app_user_grants.
type Grant struct {
	TenantID  string `json:"tenant_id"`
	AppID     string `json:"app_id"`
	Scope     string `json:"scope"`
	GrantedAt string `json:"granted_at"`
	GrantedBy string `json:"granted_by,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Source    string `json:"source"`
	RevokedAt string `json:"revoked_at,omitempty"`
}

// ListGrantedParams controls ListGranted.
type ListGrantedParams struct {
	AppID    string
	TenantID string // optional — defaults to JWT tenant_id
	Holder   string // "tenant" (default) | "user"
}

// DescribeParams controls Describe.
type DescribeParams struct {
	AppID string
	Scope string
}

// GrantParams controls Grant.
//
// Tenant scopes require tenant_admin role; user scopes require the caller's
// own JWT. For holder="user", PromptHash MUST match the current server copy
// (fetched via Describe).
type GrantParams struct {
	AppID      string
	Scope      string
	Holder     string // "tenant" | "user"
	TenantID   string
	UserID     string
	PromptHash string
}

// RevokeParams controls Revoke.
type RevokeParams struct {
	AppID  string
	Scope  string
	Holder string // "tenant" | "user"
}

// ListGranted returns active (non-revoked) grants for an app + tenant (or user).
func (s *ConsentService) ListGranted(ctx context.Context, p ListGrantedParams) ([]Grant, error) {
	path := fmt.Sprintf("/api/v1/platform/apps/%s/%s", url.PathEscape(p.AppID), grantPathSuffix(p.Holder))
	q := url.Values{}
	if p.TenantID != "" {
		q.Set("tenant_id", p.TenantID)
	}
	raw, err := s.http.get(ctx, path, q)
	if err != nil {
		return nil, err
	}
	return parseGrantsList(raw), nil
}

// Describe returns the consent prompt + hash for a scope.
//
// Call BEFORE Grant with holder="user" so the returned PromptHash can be
// echoed back as proof that what the user saw matches what the server stores.
func (s *ConsentService) Describe(ctx context.Context, p DescribeParams) (*ConsentPrompt, error) {
	q := url.Values{}
	q.Set("app_id", p.AppID)
	q.Set("scope", p.Scope)
	raw, err := s.http.get(ctx, "/platform/consent-prompt", q)
	if err != nil {
		return nil, err
	}
	var prompt ConsentPrompt
	if err := remarshal(raw, &prompt); err != nil {
		return nil, err
	}
	return &prompt, nil
}

// Grant a scope. For holder="user", PromptHash must match the current copy.
func (s *ConsentService) Grant(ctx context.Context, p GrantParams) (*Grant, error) {
	path := fmt.Sprintf("/api/v1/platform/apps/%s/%s", url.PathEscape(p.AppID), grantPathSuffix(p.Holder))
	body := map[string]interface{}{"scope": p.Scope}
	if p.TenantID != "" {
		body["tenant_id"] = p.TenantID
	}
	if p.UserID != "" {
		body["user_id"] = p.UserID
	}
	if p.PromptHash != "" {
		body["consent_prompt_hash"] = p.PromptHash
	}
	raw, err := s.http.post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out Grant
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Revoke a scope (soft-delete — sets revoked_at).
func (s *ConsentService) Revoke(ctx context.Context, p RevokeParams) error {
	path := fmt.Sprintf("/api/v1/platform/apps/%s/%s/%s",
		url.PathEscape(p.AppID), grantPathSuffix(p.Holder), url.PathEscape(p.Scope))
	return s.http.del(ctx, path)
}

func grantPathSuffix(holder string) string {
	if holder == "user" {
		return "user-grants"
	}
	return "tenant-grants"
}

// parseGrantsList converts the server's {grants: [...]} envelope into Grant slice.
func parseGrantsList(raw map[string]interface{}) []Grant {
	grantsRaw, _ := raw["grants"].([]interface{})
	out := make([]Grant, 0, len(grantsRaw))
	for _, row := range grantsRaw {
		rowMap, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		var g Grant
		if err := remarshal(rowMap, &g); err == nil {
			out = append(out, g)
		}
	}
	return out
}

// remarshal converts a map[string]interface{} to a typed struct by going
// through JSON. Convenient for services that want typed results from the
// http client's raw-map return.
func remarshal(raw map[string]interface{}, out interface{}) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
