package olympus

import "fmt"

// OlympusAPIError represents a structured error returned by the Olympus Cloud API.
type OlympusAPIError struct {
	// Code is the machine-readable error code (e.g., "UNAUTHORIZED", "NOT_FOUND").
	Code string `json:"code"`

	// Message is the human-readable error description.
	Message string `json:"message"`

	// StatusCode is the HTTP status code from the response.
	StatusCode int `json:"status_code"`

	// RequestID is the server-assigned request identifier for debugging.
	RequestID string `json:"request_id,omitempty"`
}

// Error implements the error interface.
func (e *OlympusAPIError) Error() string {
	return fmt.Sprintf("OlympusAPIError(%s): %s [status=%d, reqId=%s]",
		e.Code, e.Message, e.StatusCode, e.RequestID)
}

// IsNotFound returns true if the error represents a 404 response.
func (e *OlympusAPIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsUnauthorized returns true if the error represents a 401 response.
func (e *OlympusAPIError) IsUnauthorized() bool {
	return e.StatusCode == 401
}

// IsForbidden returns true if the error represents a 403 response.
func (e *OlympusAPIError) IsForbidden() bool {
	return e.StatusCode == 403
}

// IsRateLimited returns true if the error represents a 429 response.
func (e *OlympusAPIError) IsRateLimited() bool {
	return e.StatusCode == 429
}

// IsServerError returns true if the error represents a 5xx response.
func (e *OlympusAPIError) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// errorResponse represents the JSON error envelope from the API.
type errorResponse struct {
	Error *errorDetail `json:"error,omitempty"`
}

type errorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}
