package olympus

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// SMSService is the SMS messaging surface — send outbound SMS, retrieve
// conversation history, and query message delivery status via the CPaaS
// abstraction layer.
//
// Two route families:
//   - /voice/sms/*      — voice-platform SMS (send, conversations)
//   - /cpaas/messages/* — unified CPaaS messaging (SMS, MMS, status)
//
// Routes: /voice/sms/send, /voice/sms/conversations/{phone},
// /cpaas/messages/sms, /cpaas/messages/{id}.
type SMSService struct {
	http *httpClient
}

// ---------------------------------------------------------------------------
// Voice SMS (tenant-scoped)
// ---------------------------------------------------------------------------

// SendRequest holds the parameters for Send.
type SendRequest struct {
	ConfigID string `json:"config_id"`
	To       string `json:"to"`
	Body     string `json:"body"`
}

// Send sends an outbound SMS through a voice agent config. ConfigID
// identifies the voice agent config (and its assigned phone number); To is
// the E.164 destination; Body is the message text.
func (s *SMSService) Send(ctx context.Context, req SendRequest) (map[string]interface{}, error) {
	return s.http.post(ctx, "/voice/sms/send", map[string]interface{}{
		"config_id": req.ConfigID,
		"to":        req.To,
		"body":      req.Body,
	})
}

// GetConversationsOptions filters GetConversations.
type GetConversationsOptions struct {
	Limit  int
	Offset int
}

// GetConversations lists threaded SMS conversations for a phone number.
func (s *SMSService) GetConversations(ctx context.Context, phone string, opts *GetConversationsOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			q.Set("offset", strconv.Itoa(opts.Offset))
		}
	}
	raw, err := s.http.get(ctx, fmt.Sprintf("/voice/sms/conversations/%s", phone), q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "conversations"), nil
}

// ---------------------------------------------------------------------------
// CPaaS Messaging (provider-abstracted)
// ---------------------------------------------------------------------------

// SendViaCpaasRequest holds the parameters for SendViaCpaas.
type SendViaCpaasRequest struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Body       string `json:"body"`
	WebhookURL string `json:"webhook_url,omitempty"`
}

// SendViaCpaas sends an SMS via the unified CPaaS layer (Telnyx primary,
// Twilio fallback). From and To are E.164 phone numbers. Returns the message
// resource with provider-assigned ID and delivery status.
func (s *SMSService) SendViaCpaas(ctx context.Context, req SendViaCpaasRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"from": req.From,
		"to":   req.To,
		"body": req.Body,
	}
	if req.WebhookURL != "" {
		body["webhook_url"] = req.WebhookURL
	}
	return s.http.post(ctx, "/cpaas/messages/sms", body)
}

// GetStatus returns the delivery status and metadata of a sent message.
func (s *SMSService) GetStatus(ctx context.Context, messageID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/cpaas/messages/%s", messageID), nil)
}
