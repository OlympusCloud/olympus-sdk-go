package olympus

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// VoiceOrdersService wraps the voice order placement API.
//
// AI voice agents collect orders by phone. This service validates prices
// against the menu, stores orders in Spanner, and prepares them for POS
// push and SMS confirmation.
//
// Issue #2999
type VoiceOrdersService struct {
	http *httpClient
}

// VoiceOrderItem represents a single item in a voice order.
type VoiceOrderItem struct {
	MenuItemID          string   `json:"menu_item_id"`
	Name                string   `json:"name"`
	Quantity            int      `json:"quantity"`
	UnitPrice           float64  `json:"unit_price"`
	Modifiers           []string `json:"modifiers,omitempty"`
	SpecialInstructions string   `json:"special_instructions,omitempty"`
}

// CreateVoiceOrderRequest holds the parameters for creating a voice order.
type CreateVoiceOrderRequest struct {
	LocationID      string                 `json:"location_id"`
	Items           []VoiceOrderItem       `json:"items"`
	Fulfillment     string                 `json:"fulfillment,omitempty"`
	DeliveryAddress string                 `json:"delivery_address,omitempty"`
	PaymentMethod   string                 `json:"payment_method,omitempty"`
	CallerPhone     string                 `json:"caller_phone,omitempty"`
	CallerName      string                 `json:"caller_name,omitempty"`
	CallSID         string                 `json:"call_sid,omitempty"`
	AgentID         string                 `json:"agent_id,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ListVoiceOrdersOptions holds optional filters for listing voice orders.
type ListVoiceOrdersOptions struct {
	CallerPhone string
	Status      string
	LocationID  string
	Limit       int
}

// Create places a new voice order, validating item prices against the menu.
func (s *VoiceOrdersService) Create(ctx context.Context, req CreateVoiceOrderRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"location_id": req.LocationID,
		"items":       req.Items,
	}
	if req.Fulfillment != "" {
		body["fulfillment"] = req.Fulfillment
	}
	if req.DeliveryAddress != "" {
		body["delivery_address"] = req.DeliveryAddress
	}
	if req.PaymentMethod != "" {
		body["payment_method"] = req.PaymentMethod
	}
	if req.CallerPhone != "" {
		body["caller_phone"] = req.CallerPhone
	}
	if req.CallerName != "" {
		body["caller_name"] = req.CallerName
	}
	if req.CallSID != "" {
		body["call_sid"] = req.CallSID
	}
	if req.AgentID != "" {
		body["agent_id"] = req.AgentID
	}
	if req.Metadata != nil {
		body["metadata"] = req.Metadata
	}

	return s.http.post(ctx, "/voice-orders", body)
}

// Get retrieves a voice order by ID.
func (s *VoiceOrdersService) Get(ctx context.Context, orderID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/voice-orders/%s", orderID), nil)
}

// List returns voice orders with optional filters.
func (s *VoiceOrdersService) List(ctx context.Context, opts *ListVoiceOrdersOptions) (map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.CallerPhone != "" {
			q.Set("caller_phone", opts.CallerPhone)
		}
		if opts.Status != "" {
			q.Set("status", opts.Status)
		}
		if opts.LocationID != "" {
			q.Set("location_id", opts.LocationID)
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}

	return s.http.get(ctx, "/voice-orders", q)
}

// PushToPOS pushes a voice order to the tenant's POS system (Toast/Square/Clover).
// Updates pos_push_status and transitions the order to "confirmed".
func (s *VoiceOrdersService) PushToPOS(ctx context.Context, orderID string) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/voice-orders/%s/push-pos", orderID), nil)
}
