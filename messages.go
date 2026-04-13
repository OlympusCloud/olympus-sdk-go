package olympus

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// MessagesService wraps the message queue with department routing API.
//
// AI agents route messages to business departments (manager, catering,
// sales, lost-and-found, reservations) when they cannot fully handle a
// request. Notification dispatch via Twilio SMS + SendGrid email on create.
//
// Issue #2997
type MessagesService struct {
	http *httpClient
}

// CreateMessageRequest holds the parameters for queuing a message.
type CreateMessageRequest struct {
	Department string                 `json:"department"`
	Message    string                 `json:"message"`
	CallerPhone string               `json:"caller_phone,omitempty"`
	CallerName  string               `json:"caller_name,omitempty"`
	LocationID  string               `json:"location_id,omitempty"`
	Priority    string               `json:"priority,omitempty"`
	Source      string               `json:"source,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateMessageRequest holds the parameters for updating a message.
type UpdateMessageRequest struct {
	Status     string `json:"status,omitempty"`
	AssignedTo string `json:"assigned_to,omitempty"`
}

// ConfigureDepartmentRequest holds the parameters for configuring department routing.
type ConfigureDepartmentRequest struct {
	NotificationChannels    []string              `json:"notification_channels"`
	Recipients              []map[string]string   `json:"recipients"`
	EscalationAfterMinutes  int                   `json:"escalation_after_minutes,omitempty"`
	IsActive                bool                  `json:"is_active"`
	LocationID              string                `json:"location_id,omitempty"`
}

// ListMessagesOptions holds optional filters for listing messages.
type ListMessagesOptions struct {
	Department string
	Status     string
	LocationID string
	Limit      int
}

// QueueMessage creates a new message in the queue and triggers notification dispatch.
func (s *MessagesService) QueueMessage(ctx context.Context, req CreateMessageRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"department": req.Department,
		"message":    req.Message,
	}
	if req.CallerPhone != "" {
		body["caller_phone"] = req.CallerPhone
	}
	if req.CallerName != "" {
		body["caller_name"] = req.CallerName
	}
	if req.LocationID != "" {
		body["location_id"] = req.LocationID
	}
	if req.Priority != "" {
		body["priority"] = req.Priority
	}
	if req.Source != "" {
		body["source"] = req.Source
	}
	if req.Metadata != nil {
		body["metadata"] = req.Metadata
	}

	return s.http.post(ctx, "/messages/queue", body)
}

// List returns messages with optional filters.
func (s *MessagesService) List(ctx context.Context, opts *ListMessagesOptions) (map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Department != "" {
			q.Set("department", opts.Department)
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

	return s.http.get(ctx, "/messages", q)
}

// Update modifies a message's status or assignment.
func (s *MessagesService) Update(ctx context.Context, messageID string, req UpdateMessageRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	if req.Status != "" {
		body["status"] = req.Status
	}
	if req.AssignedTo != "" {
		body["assigned_to"] = req.AssignedTo
	}

	return s.http.patch(ctx, fmt.Sprintf("/messages/%s", messageID), body)
}

// ListDepartments returns configured departments with routing rules.
func (s *MessagesService) ListDepartments(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := s.http.get(ctx, "/messages/departments", nil)
	if err != nil {
		return nil, err
	}
	return extractMapSlice(resp, "departments", "data"), nil
}

// ConfigureDepartment sets routing configuration for a department.
func (s *MessagesService) ConfigureDepartment(ctx context.Context, department string, req ConfigureDepartmentRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"notification_channels":     req.NotificationChannels,
		"recipients":                req.Recipients,
		"escalation_after_minutes":  req.EscalationAfterMinutes,
		"is_active":                 req.IsActive,
	}
	if req.LocationID != "" {
		body["location_id"] = req.LocationID
	}

	return s.http.put(ctx, fmt.Sprintf("/messages/departments/%s", department), body)
}
