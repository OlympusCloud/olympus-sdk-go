package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// NotifyService handles push, SMS, email, Slack, and in-app notifications.
//
// Wraps the Olympus Notification service via the Go API Gateway.
// Routes: /notifications/*, /messaging/*.
type NotifyService struct {
	http *httpClient
}

// Push sends a push notification to a user's device(s).
func (s *NotifyService) Push(ctx context.Context, userID, title, body string) error {
	_, err := s.http.post(ctx, "/notifications/push", map[string]interface{}{
		"user_id": userID,
		"title":   title,
		"body":    body,
	})
	return err
}

// SMS sends an SMS message.
func (s *NotifyService) SMS(ctx context.Context, phone, message string) error {
	_, err := s.http.post(ctx, "/messaging/sms", map[string]interface{}{
		"phone":   phone,
		"message": message,
	})
	return err
}

// Email sends an email.
func (s *NotifyService) Email(ctx context.Context, to, subject, html string) error {
	_, err := s.http.post(ctx, "/messaging/email", map[string]interface{}{
		"to":      to,
		"subject": subject,
		"html":    html,
	})
	return err
}

// Slack sends a Slack message to a channel.
func (s *NotifyService) Slack(ctx context.Context, channel, message string) error {
	_, err := s.http.post(ctx, "/messaging/slack", map[string]interface{}{
		"channel": channel,
		"message": message,
	})
	return err
}

// Chat sends an in-app chat message to a user.
func (s *NotifyService) Chat(ctx context.Context, userID, message string) error {
	_, err := s.http.post(ctx, "/notifications/chat", map[string]interface{}{
		"user_id": userID,
		"message": message,
	})
	return err
}

// ListOptions holds optional filters for listing notifications.
type ListNotificationsOptions struct {
	Limit     int
	UnreadOnly bool
}

// List returns notifications for the current user.
func (s *NotifyService) List(ctx context.Context, opts *ListNotificationsOptions) ([]Notification, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Limit > 0 {
			q.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		if opts.UnreadOnly {
			q.Set("unread_only", "true")
		}
	}

	resp, err := s.http.get(ctx, "/notifications", q)
	if err != nil {
		return nil, err
	}

	return parseSlice(resp, "notifications", parseNotification), nil
}

// MarkRead marks a notification as read.
func (s *NotifyService) MarkRead(ctx context.Context, notificationID string) error {
	_, err := s.http.patch(ctx, fmt.Sprintf("/notifications/%s", notificationID), map[string]interface{}{
		"read": true,
	})
	return err
}
