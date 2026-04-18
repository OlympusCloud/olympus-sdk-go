package olympus

import (
	"context"
	"fmt"
)

// ConnectService handles marketing-funnel + pre-conversion lead capture.
//
// Routes: /connect/*, /leads.
//
// Issue #3108 — the /leads endpoint is intentionally unauthenticated so
// marketing surfaces can POST leads before the user signs up. Idempotency is
// email-based over a 1h window.
type ConnectService struct {
	http *httpClient
}

// UTM holds the standard UTM tracking parameters captured from a landing page.
type UTM struct {
	Source   string `json:"source,omitempty"`
	Medium   string `json:"medium,omitempty"`
	Campaign string `json:"campaign,omitempty"`
	Term     string `json:"term,omitempty"`
	Content  string `json:"content,omitempty"`
}

// CreateLeadRequest is the payload for creating a pre-conversion lead.
type CreateLeadRequest struct {
	// Email is used for idempotency dedup (1h window per lead).
	Email    string                 `json:"email"`
	Name     string                 `json:"name,omitempty"`
	Phone    string                 `json:"phone,omitempty"`
	Company  string                 `json:"company,omitempty"`
	Source   string                 `json:"source,omitempty"`
	UTM      *UTM                   `json:"utm,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CreateLeadResponse is the shape returned by POST /leads.
type CreateLeadResponse struct {
	LeadID    string `json:"lead_id"`
	Status    string `json:"status"` // "created" or "deduped"
	CreatedAt string `json:"created_at"`
}

// CreateLead posts a pre-conversion lead. Safe to retry — deduplicates on email.
//
// Backing endpoint: POST /api/v1/leads (Python).
func (s *ConnectService) CreateLead(ctx context.Context, req CreateLeadRequest) (*CreateLeadResponse, error) {
	if req.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	raw, err := s.http.post(ctx, "/leads", req)
	if err != nil {
		return nil, err
	}
	out := &CreateLeadResponse{}
	// The Python endpoint historically returns snake_case; fall back to
	// camelCase keys for cross-SDK test fixture compatibility.
	if v, ok := raw["lead_id"].(string); ok {
		out.LeadID = v
	} else if v, ok := raw["leadId"].(string); ok {
		out.LeadID = v
	}
	if v, ok := raw["status"].(string); ok {
		out.Status = v
	}
	if v, ok := raw["created_at"].(string); ok {
		out.CreatedAt = v
	} else if v, ok := raw["createdAt"].(string); ok {
		out.CreatedAt = v
	}
	return out, nil
}
