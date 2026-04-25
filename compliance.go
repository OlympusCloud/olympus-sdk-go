package olympus

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// ComplianceService is the cross-app compliance surface.
//
// Currently wraps the dram-shop compliance ledger (#3316) used by both
// BarOS and PizzaOS to write and read events at alcohol-serving venues.
// Future verticals will hook in via the same `vertical_extensions` JSON
// payload shape so the platform schema doesn't churn per-app.
type ComplianceService struct {
	http *httpClient
}

// DramShopEventType enumerates the permitted dram-shop event types.
//
// Any other value will be rejected by the server with a 400. The values
// here mirror the pinned `VALID_EVENT_TYPES` list in the Rust handler at
// backend/rust/platform/src/handlers/dram_shop.rs.
const (
	DramShopEventIDCheckPassed    = "id_check_passed"
	DramShopEventIDCheckFailed    = "id_check_failed"
	DramShopEventServiceRefused   = "service_refused"
	DramShopEventOverServeWarning = "over_serve_warning"
	DramShopEventIncidentFiled    = "incident_filed"
)

// RecordDramShopEventParams controls RecordDramShopEvent. Only LocationID
// and EventType are required; everything else is server-tolerant.
//
// VerticalExtensions is the per-app JSON payload — pass anything the
// future BAC estimator should consume (e.g. PizzaOS food-weighting).
type RecordDramShopEventParams struct {
	LocationID         string                 `json:"location_id"`
	EventType          string                 `json:"event_type"`
	CustomerRef        string                 `json:"customer_ref,omitempty"`
	StaffUserID        string                 `json:"staff_user_id,omitempty"`
	EstimatedBAC       *float64               `json:"estimated_bac,omitempty"`
	BACInputs          map[string]interface{} `json:"bac_inputs,omitempty"`
	VerticalExtensions map[string]interface{} `json:"vertical_extensions,omitempty"`
	Notes              string                 `json:"notes,omitempty"`
	OccurredAt         *time.Time             `json:"-"`
}

// DramShopEvent is a single row from `platform_dram_shop_events` (#3316).
type DramShopEvent struct {
	EventID            string                 `json:"event_id"`
	TenantID           string                 `json:"tenant_id"`
	LocationID         string                 `json:"location_id"`
	EventType          string                 `json:"event_type"`
	CustomerRef        *string                `json:"customer_ref"`
	StaffUserID        *string                `json:"staff_user_id"`
	EstimatedBAC       *float64               `json:"estimated_bac"`
	BACInputs          map[string]interface{} `json:"bac_inputs"`
	VerticalExtensions map[string]interface{} `json:"vertical_extensions"`
	Notes              *string                `json:"notes"`
	OccurredAt         time.Time              `json:"occurred_at"`
	CreatedAt          time.Time              `json:"created_at"`
}

// DramShopEventList is the response envelope for ListDramShopEvents.
type DramShopEventList struct {
	Events        []DramShopEvent `json:"events"`
	TotalReturned int             `json:"total_returned"`
}

// ListDramShopEventsParams controls ListDramShopEvents. All fields are
// optional. Limit is server-clamped to 1..=500 (default 100).
type ListDramShopEventsParams struct {
	LocationID string
	From       *time.Time
	To         *time.Time
	EventType  string
	Limit      int
}

// ListDramShopRulesParams controls ListDramShopRules. With AppID set,
// returns app-specific overrides PLUS platform defaults; without, returns
// platform defaults only.
type ListDramShopRulesParams struct {
	JurisdictionCode string
	AppID            string
	RuleType         string
}

// DramShopRule is a single row from `platform_dram_shop_rules` (#3316).
type DramShopRule struct {
	TenantID         string                 `json:"tenant_id"`
	RuleID           string                 `json:"rule_id"`
	JurisdictionCode string                 `json:"jurisdiction_code"`
	RuleType         string                 `json:"rule_type"`
	RulePayload      map[string]interface{} `json:"rule_payload"`
	EffectiveFrom    time.Time              `json:"effective_from"`
	EffectiveUntil   *time.Time             `json:"effective_until"`
	OverrideAppID    *string                `json:"override_app_id"`
	Notes            *string                `json:"notes"`
	CreatedAt        *time.Time             `json:"created_at"`
}

// RecordDramShopEvent appends a compliance event (#3316).
//
// The server stamps server-side `event_id`, `tenant_id`, and `created_at`.
// Returns the persisted row.
func (s *ComplianceService) RecordDramShopEvent(ctx context.Context, p RecordDramShopEventParams) (*DramShopEvent, error) {
	body := map[string]interface{}{
		"location_id": p.LocationID,
		"event_type":  p.EventType,
	}
	if p.CustomerRef != "" {
		body["customer_ref"] = p.CustomerRef
	}
	if p.StaffUserID != "" {
		body["staff_user_id"] = p.StaffUserID
	}
	if p.EstimatedBAC != nil {
		body["estimated_bac"] = *p.EstimatedBAC
	}
	if p.BACInputs != nil {
		body["bac_inputs"] = p.BACInputs
	}
	if p.VerticalExtensions != nil {
		body["vertical_extensions"] = p.VerticalExtensions
	}
	if p.Notes != "" {
		body["notes"] = p.Notes
	}
	if p.OccurredAt != nil {
		body["occurred_at"] = p.OccurredAt.UTC().Format(time.RFC3339Nano)
	}

	raw, err := s.http.post(ctx, "/platform/compliance/dram-shop-events", body)
	if err != nil {
		return nil, err
	}
	var out DramShopEvent
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDramShopEvents queries the audit ledger with optional filters (#3316).
func (s *ComplianceService) ListDramShopEvents(ctx context.Context, p ListDramShopEventsParams) (*DramShopEventList, error) {
	q := url.Values{}
	if p.LocationID != "" {
		q.Set("location_id", p.LocationID)
	}
	if p.From != nil {
		q.Set("from", p.From.UTC().Format(time.RFC3339))
	}
	if p.To != nil {
		q.Set("to", p.To.UTC().Format(time.RFC3339))
	}
	if p.EventType != "" {
		q.Set("event_type", p.EventType)
	}
	if p.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", p.Limit))
	}

	raw, err := s.http.get(ctx, "/platform/compliance/dram-shop-events", q)
	if err != nil {
		return nil, err
	}
	var out DramShopEventList
	if err := remarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDramShopRules returns currently-effective dram-shop rules (#3316).
//
// With AppID set, the server returns rules for that app's vertical
// override PLUS the platform default rules. Without AppID, returns
// platform defaults only.
func (s *ComplianceService) ListDramShopRules(ctx context.Context, p ListDramShopRulesParams) ([]DramShopRule, error) {
	q := url.Values{}
	if p.JurisdictionCode != "" {
		q.Set("jurisdiction_code", p.JurisdictionCode)
	}
	if p.AppID != "" {
		q.Set("app_id", p.AppID)
	}
	if p.RuleType != "" {
		q.Set("rule_type", p.RuleType)
	}

	raw, err := s.http.get(ctx, "/platform/compliance/dram-shop-rules", q)
	if err != nil {
		return nil, err
	}
	rulesRaw, _ := raw["rules"].([]interface{})
	out := make([]DramShopRule, 0, len(rulesRaw))
	for _, row := range rulesRaw {
		rowMap, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		var r DramShopRule
		if err := remarshal(rowMap, &r); err == nil {
			out = append(out, r)
		}
	}
	return out, nil
}
