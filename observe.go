package olympus

import (
	"context"
	"fmt"
	"time"
)

// ObserveService handles client-side observability: event logging, error reporting,
// tracing, and user identification.
//
// Routes: /monitoring/client/*, /analytics/*.
type ObserveService struct {
	http *httpClient
}

// LogEvent logs a custom analytics event with properties.
func (s *ObserveService) LogEvent(ctx context.Context, name string, properties map[string]interface{}) error {
	_, err := s.http.post(ctx, "/monitoring/client/events", map[string]interface{}{
		"event":      name,
		"properties": properties,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
	return err
}

// LogError reports a client-side error.
func (s *ObserveService) LogError(ctx context.Context, errMsg string, errContext map[string]interface{}) error {
	body := map[string]interface{}{
		"error":     errMsg,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if errContext != nil {
		body["context"] = errContext
	}

	_, err := s.http.post(ctx, "/monitoring/client/errors", body)
	return err
}

// TraceSpan represents a client-side trace span that can be ended to report
// its duration.
type TraceSpan struct {
	Name      string
	TraceID   string
	StartedAt time.Time
	endedAt   *time.Time
	http      *httpClient
}

// End closes the span and reports its duration to the server.
func (span *TraceSpan) End(ctx context.Context) error {
	now := time.Now().UTC()
	span.endedAt = &now
	duration := now.Sub(span.StartedAt)

	_, err := span.http.post(ctx, "/monitoring/client/traces", map[string]interface{}{
		"trace_id":    span.TraceID,
		"name":        span.Name,
		"duration_ms": duration.Milliseconds(),
		"started_at":  span.StartedAt.Format(time.RFC3339),
		"ended_at":    now.Format(time.RFC3339),
	})
	return err
}

// Elapsed returns the time since the span started, or the total duration if ended.
func (span *TraceSpan) Elapsed() time.Duration {
	if span.endedAt != nil {
		return span.endedAt.Sub(span.StartedAt)
	}
	return time.Since(span.StartedAt)
}

// StartTrace starts a client-side trace span. Call End on the returned TraceSpan
// to close the span and report its duration.
func (s *ObserveService) StartTrace(ctx context.Context, name string) *TraceSpan {
	now := time.Now().UTC()
	traceID := fmt.Sprintf("%d-%d", now.UnixMilli(), hashString(name))

	return &TraceSpan{
		Name:      name,
		TraceID:   traceID,
		StartedAt: now,
		http:      s.http,
	}
}

// SetUser identifies the current user for analytics attribution.
func (s *ObserveService) SetUser(ctx context.Context, userID string, properties map[string]interface{}) error {
	body := map[string]interface{}{
		"user_id": userID,
	}
	if properties != nil {
		body["properties"] = properties
	}

	_, err := s.http.post(ctx, "/monitoring/client/identify", body)
	return err
}

