package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// EnterpriseContextService wraps the Enterprise Context API (Company 360)
// that assembles complete tenant context for AI agents in a single call.
//
// Used by voice Workers, chat agents, and Pantheon agents during live
// interactions. Cached for 5 minutes per (tenant_id, location_id).
// Designed for <2s P95 assembly via parallel Spanner queries.
//
// Issue #2993
type EnterpriseContextService struct {
	http *httpClient
}

// EnterpriseContextOptions holds optional parameters for context retrieval.
type EnterpriseContextOptions struct {
	// AgentType specifies the requesting agent: "voice", "chat", "pantheon", "workflow".
	// Defaults to "voice" if empty.
	AgentType string
	// CallerPhone is the caller's phone number for profile lookup.
	CallerPhone string
}

// Get retrieves the full Company 360 context for a tenant and location.
//
// Returns brand info, locations, menu, specials, FAQs, upsells, inventory,
// caller profile, and graph relationships in a single response.
func (s *EnterpriseContextService) Get(ctx context.Context, tenantID, locationID string, opts *EnterpriseContextOptions) (map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.AgentType != "" {
			q.Set("agent_type", opts.AgentType)
		}
		if opts.CallerPhone != "" {
			q.Set("caller_phone", opts.CallerPhone)
		}
	}

	path := fmt.Sprintf("/enterprise-context/%s/%s", tenantID, locationID)
	return s.http.get(ctx, path, q)
}

// GetDefault retrieves enterprise context for the default location.
func (s *EnterpriseContextService) GetDefault(ctx context.Context, tenantID string, opts *EnterpriseContextOptions) (map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.AgentType != "" {
			q.Set("agent_type", opts.AgentType)
		}
		if opts.CallerPhone != "" {
			q.Set("caller_phone", opts.CallerPhone)
		}
	}

	path := fmt.Sprintf("/enterprise-context/%s", tenantID)
	return s.http.get(ctx, path, q)
}
