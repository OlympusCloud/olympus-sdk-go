package olympus

import (
	"context"
	"fmt"
)

// EventsService handles real-time event subscriptions, webhooks, and event publishing.
//
// Routes: /events/*, /platform/tenants/me/webhooks/*.
type EventsService struct {
	http *httpClient
}

// Publish publishes an event to the platform event bus.
func (s *EventsService) Publish(ctx context.Context, eventType string, data map[string]interface{}) error {
	_, err := s.http.post(ctx, "/events/publish", map[string]interface{}{
		"event_type": eventType,
		"data":       data,
	})
	return err
}

// WebhookRegister registers a webhook endpoint for one or more event types.
func (s *EventsService) WebhookRegister(ctx context.Context, webhookURL string, events []string) (*WebhookRegistration, error) {
	resp, err := s.http.post(ctx, "/platform/tenants/me/webhooks", map[string]interface{}{
		"url":    webhookURL,
		"events": events,
	})
	if err != nil {
		return nil, err
	}
	return parseWebhookRegistration(resp), nil
}

// WebhookTest sends a test webhook payload for a given event type.
func (s *EventsService) WebhookTest(ctx context.Context, eventType string) error {
	_, err := s.http.post(ctx, "/platform/tenants/me/webhooks/test", map[string]interface{}{
		"event_type": eventType,
	})
	return err
}

// WebhookReplay replays a previously delivered event by its ID.
func (s *EventsService) WebhookReplay(ctx context.Context, eventID string) error {
	_, err := s.http.post(ctx, "/platform/tenants/me/webhooks/replay", map[string]interface{}{
		"event_id": eventID,
	})
	return err
}

// ListWebhooks returns all registered webhooks.
func (s *EventsService) ListWebhooks(ctx context.Context) ([]WebhookRegistration, error) {
	resp, err := s.http.get(ctx, "/platform/tenants/me/webhooks", nil)
	if err != nil {
		return nil, err
	}

	hooks := parseSlice(resp, "webhooks", parseWebhookRegistration)
	if len(hooks) == 0 {
		hooks = parseSlice(resp, "data", parseWebhookRegistration)
	}
	return hooks, nil
}

// WebhookDelete deletes a registered webhook.
func (s *EventsService) WebhookDelete(ctx context.Context, webhookID string) error {
	return s.http.del(ctx, fmt.Sprintf("/platform/tenants/me/webhooks/%s", webhookID))
}
