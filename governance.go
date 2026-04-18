package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// GovernanceService is the policy exception framework surface.
//
// Narrow scope — two policy keys at launch:
//   - "session_ttl_role_ceiling": extend role TTL for a specific app+role.
//   - "grace_policy_category":    override the whole-app grace policy.
//
// No approve/deny/revoke here — those are Cockpit-only actions requiring
// platform_admin JWT. SDK callers file + list + get status.
//
// Part of olympus-cloud-gcp#3254 / #3259 / §17.7 of APP-SCOPED-PERMISSIONS.md.
type GovernanceService struct {
	http *httpClient
}

// ExceptionRequest is a policy exception record.
type ExceptionRequest struct {
	ExceptionID    string                 `json:"exception_id"`
	AppID          string                 `json:"app_id"`
	TenantID       string                 `json:"tenant_id,omitempty"`
	PolicyKey      string                 `json:"policy_key"`
	RequestedValue map[string]interface{} `json:"requested_value"`
	Justification  string                 `json:"justification"`
	RiskTier       string                 `json:"risk_tier"`
	RiskScore      float64                `json:"risk_score"`
	RiskRationale  string                 `json:"risk_rationale"`
	Status         string                 `json:"status"`
	ExpiresAt      string                 `json:"expires_at"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	ReviewerID     string                 `json:"reviewer_id,omitempty"`
	ReviewedAt     string                 `json:"reviewed_at,omitempty"`
	ReviewerNotes  string                 `json:"reviewer_notes,omitempty"`
	RevokedAt      string                 `json:"revoked_at,omitempty"`
	RevokeReason   string                 `json:"revoke_reason,omitempty"`
}

// RequestExceptionParams controls RequestException.
//
// Justification must be >= 100 chars; platform validator rejects shorter.
type RequestExceptionParams struct {
	PolicyKey      string                 // "session_ttl_role_ceiling" | "grace_policy_category"
	RequestedValue map[string]interface{}
	Justification  string
	TenantID       string
}

// ListExceptionsParams controls ListExceptions.
type ListExceptionsParams struct {
	AppID  string
	Status string
}

// RequestException files a new policy exception.
//
// Platform auto-scores and routes low-risk to auto_approved, medium/high to
// pending_review.
func (s *GovernanceService) RequestException(ctx context.Context, p RequestExceptionParams) (*ExceptionRequest, error) {
	body := map[string]interface{}{
		"policy_key":      p.PolicyKey,
		"requested_value": p.RequestedValue,
		"justification":   p.Justification,
	}
	if p.TenantID != "" {
		body["tenant_id"] = p.TenantID
	}
	raw, err := s.http.post(ctx, "/api/v1/platform/exceptions", body)
	if err != nil {
		return nil, err
	}
	var out ExceptionRequest
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListExceptions returns exceptions for this tenant, optionally filtered.
func (s *GovernanceService) ListExceptions(ctx context.Context, p ListExceptionsParams) ([]ExceptionRequest, error) {
	q := url.Values{}
	if p.AppID != "" {
		q.Set("app_id", p.AppID)
	}
	if p.Status != "" {
		q.Set("status", p.Status)
	}
	raw, err := s.http.get(ctx, "/api/v1/platform/exceptions", q)
	if err != nil {
		return nil, err
	}
	rowsRaw, _ := raw["exceptions"].([]interface{})
	out := make([]ExceptionRequest, 0, len(rowsRaw))
	for _, row := range rowsRaw {
		rowMap, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		var ex ExceptionRequest
		if err := remarshal(rowMap, &ex); err == nil {
			out = append(out, ex)
		}
	}
	return out, nil
}

// GetException fetches a single exception by ID.
func (s *GovernanceService) GetException(ctx context.Context, exceptionID string) (*ExceptionRequest, error) {
	path := fmt.Sprintf("/api/v1/platform/exceptions/%s", url.PathEscape(exceptionID))
	raw, err := s.http.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out ExceptionRequest
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
