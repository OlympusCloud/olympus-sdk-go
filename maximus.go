package olympus

import (
	"context"
	"net/url"
	"time"
)

// MaximusService wraps the Maximus consumer AI assistant endpoints:
// voice queries, calendar, email, subscription billing.
//
// v0.3.0 — Issues #2567, #2568, #2571
type MaximusService struct {
	http *httpClient
}

// VoiceQuery processes a voice query and returns AI response + TTS config.
func (s *MaximusService) VoiceQuery(ctx context.Context, text string) (map[string]interface{}, error) {
	return s.http.post(ctx, "/maximus/voice/query", map[string]interface{}{"text": text})
}

// GetWakeWordConfig returns the wake word detection config.
func (s *MaximusService) GetWakeWordConfig(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/maximus/voice/wake-word/config", nil)
}

// AdaptSpeaker submits a voice sample for speaker adaptation.
func (s *MaximusService) AdaptSpeaker(ctx context.Context, sample map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/maximus/voice/speaker/adapt", sample)
}

// ListCalendarEvents returns calendar events in a date range.
func (s *MaximusService) ListCalendarEvents(ctx context.Context, start, end *time.Time) (map[string]interface{}, error) {
	q := url.Values{}
	if start != nil {
		q.Set("start", start.Format(time.RFC3339))
	}
	if end != nil {
		q.Set("end", end.Format(time.RFC3339))
	}
	return s.http.get(ctx, "/maximus/calendar/events", q)
}

// CreateCalendarEvent creates a calendar event.
func (s *MaximusService) CreateCalendarEvent(ctx context.Context, event map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/maximus/calendar/events", event)
}

// DeleteCalendarEvent deletes a calendar event.
func (s *MaximusService) DeleteCalendarEvent(ctx context.Context, eventID string) error {
	return s.http.del(ctx, "/maximus/calendar/events/"+eventID)
}

// SyncCalendar triggers calendar sync with Google/Outlook.
func (s *MaximusService) SyncCalendar(ctx context.Context, provider string) (map[string]interface{}, error) {
	return s.http.post(ctx, "/maximus/calendar/sync", map[string]interface{}{"provider": provider})
}

// ListInbox returns inbox messages.
func (s *MaximusService) ListInbox(ctx context.Context, limit int, label string) (map[string]interface{}, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", string(rune(limit)))
	}
	if label != "" {
		q.Set("label", label)
	}
	return s.http.get(ctx, "/maximus/email/inbox", q)
}

// GetEmailThread returns a full email thread.
func (s *MaximusService) GetEmailThread(ctx context.Context, threadID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/maximus/email/threads/"+threadID, nil)
}

// SendEmail sends an email.
func (s *MaximusService) SendEmail(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/maximus/email/send", body)
}

// ListPlans returns available Maximus subscription plans.
// Plans: free, pro ($9.99), premium ($19.99), business ($29.99).
func (s *MaximusService) ListPlans(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/maximus/plans", nil)
}

// GetUsage returns current usage for a tenant's Maximus subscription.
func (s *MaximusService) GetUsage(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/maximus/usage/"+tenantID, nil)
}

// Subscribe subscribes to a Maximus plan.
func (s *MaximusService) Subscribe(ctx context.Context, planID string) (map[string]interface{}, error) {
	return s.http.post(ctx, "/maximus/subscribe", map[string]interface{}{"plan_id": planID})
}
