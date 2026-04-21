package olympus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const sdkVersion = "go/" + Version

// StaleCatalogHandler fires when the server returns X-Olympus-Catalog-Stale: true
// (§4.7 rolling-window signal). Consumers should schedule a background token
// refresh at a randomized 0–15 min offset.
type StaleCatalogHandler func()

// httpClient is the internal HTTP transport used by all service methods.
//
// tokenMu guards mutable token + handler fields so SetAccessToken / SetAppToken /
// OnCatalogStale are safe to call concurrently with in-flight requests.
type httpClient struct {
	config  *Config
	client  *http.Client
	baseURL string

	tokenMu        sync.RWMutex
	accessToken    string
	appToken       string
	onStaleCatalog StaleCatalogHandler
}

func newHTTPClient(cfg *Config) *httpClient {
	return &httpClient{
		config:  cfg,
		baseURL: cfg.effectiveBaseURL(),
		client: &http.Client{
			Timeout: cfg.effectiveTimeout(),
		},
	}
}

// SetAccessToken sets the user-scoped access token. When set, it takes
// precedence over the API key for authentication.
func (h *httpClient) SetAccessToken(token string) {
	h.tokenMu.Lock()
	h.accessToken = token
	h.tokenMu.Unlock()
}

// ClearAccessToken removes the user-scoped access token, reverting to API key auth.
func (h *httpClient) ClearAccessToken() {
	h.tokenMu.Lock()
	h.accessToken = ""
	h.tokenMu.Unlock()
}

// SetAppToken sets the App JWT (attached as X-App-Token on every request per §4.5).
// Called after /auth/app-tokens/mint completes.
func (h *httpClient) SetAppToken(token string) {
	h.tokenMu.Lock()
	h.appToken = token
	h.tokenMu.Unlock()
}

// ClearAppToken clears the App JWT.
func (h *httpClient) ClearAppToken() {
	h.tokenMu.Lock()
	h.appToken = ""
	h.tokenMu.Unlock()
}

// GetAccessToken returns the current access token (used by OlympusClient to
// decode JWT claims for the scope bitset fast-path).
func (h *httpClient) GetAccessToken() string {
	h.tokenMu.RLock()
	defer h.tokenMu.RUnlock()
	return h.accessToken
}

// OnCatalogStale registers a handler for the X-Olympus-Catalog-Stale header
// (§4.7). Pass nil to clear.
func (h *httpClient) OnCatalogStale(handler StaleCatalogHandler) {
	h.tokenMu.Lock()
	h.onStaleCatalog = handler
	h.tokenMu.Unlock()
}

func (h *httpClient) get(ctx context.Context, path string, query url.Values) (map[string]interface{}, error) {
	return h.doJSON(ctx, http.MethodGet, path, query, nil)
}

func (h *httpClient) post(ctx context.Context, path string, body interface{}) (map[string]interface{}, error) {
	return h.doJSON(ctx, http.MethodPost, path, nil, body)
}

func (h *httpClient) put(ctx context.Context, path string, body interface{}) (map[string]interface{}, error) {
	return h.doJSON(ctx, http.MethodPut, path, nil, body)
}

func (h *httpClient) patch(ctx context.Context, path string, body interface{}) (map[string]interface{}, error) {
	return h.doJSON(ctx, http.MethodPatch, path, nil, body)
}

func (h *httpClient) del(ctx context.Context, path string) error {
	_, err := h.doJSON(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// doJSON executes an HTTP request with JSON body/response, auth headers, and retry logic.
func (h *httpClient) doJSON(ctx context.Context, method, path string, query url.Values, body interface{}) (map[string]interface{}, error) {
	fullURL := h.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("olympus-sdk: failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	maxRetries := h.config.effectiveMaxRetries()
	baseDelay := h.config.effectiveRetryBaseDelay()

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: baseDelay * 2^(attempt-1)
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// Reset body reader for retry
			if body != nil {
				data, _ := json.Marshal(body)
				bodyReader = bytes.NewReader(data)
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("olympus-sdk: failed to create request: %w", err)
		}

		h.applyHeaders(req)

		resp, err := h.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("olympus-sdk: request failed: %w", err)
			// Retry on network errors
			continue
		}

		result, parseErr := h.handleResponse(resp)
		if parseErr != nil {
			if apiErr, ok := parseErr.(*OlympusAPIError); ok {
				// Retry on 429 and 5xx
				if apiErr.IsRateLimited() || apiErr.IsServerError() {
					lastErr = apiErr
					continue
				}
			}
			return nil, parseErr
		}

		return result, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("olympus-sdk: request failed after %d retries", maxRetries)
}

// applyHeaders sets authentication and SDK headers on a request.
func (h *httpClient) applyHeaders(req *http.Request) {
	h.tokenMu.RLock()
	access := h.accessToken
	app := h.appToken
	h.tokenMu.RUnlock()

	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	} else if h.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.config.APIKey)
	}

	// App JWT (§4.5 dual-JWT flow) — attached when set via SetAppToken.
	// Platform-shell tokens (Portal/Cockpit) have no app context and omit.
	if app != "" {
		req.Header.Set("X-App-Token", app)
	}

	req.Header.Set("X-App-Id", h.config.AppID)
	req.Header.Set("X-SDK-Version", sdkVersion)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

// checkStaleCatalog fires the stale-catalog handler when the response carries
// the X-Olympus-Catalog-Stale: true header (§4.7 rolling window).
func (h *httpClient) checkStaleCatalog(resp *http.Response) {
	if resp == nil || resp.StatusCode >= 400 {
		return
	}
	if resp.Header.Get("X-Olympus-Catalog-Stale") != "true" {
		return
	}
	h.tokenMu.RLock()
	handler := h.onStaleCatalog
	h.tokenMu.RUnlock()
	if handler == nil {
		return
	}
	// Swallow handler panics — the caller's request was successful.
	defer func() {
		_ = recover()
	}()
	handler()
}

// handleResponse reads the HTTP response and returns parsed JSON or an error.
func (h *httpClient) handleResponse(resp *http.Response) (map[string]interface{}, error) {
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("olympus-sdk: failed to read response body: %w", err)
	}

	// Fire stale-catalog handler BEFORE returning the body (§4.7).
	h.checkStaleCatalog(resp)

	if resp.StatusCode >= 400 {
		return nil, h.parseError(resp.StatusCode, respBody, resp.Header)
	}

	// 204 No Content
	if resp.StatusCode == 204 || len(respBody) == 0 {
		return map[string]interface{}{}, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("olympus-sdk: failed to parse response JSON: %w", err)
	}

	return result, nil
}

// parseError attempts to extract a structured API error from the response body.
func (h *httpClient) parseError(statusCode int, body []byte, headers http.Header) error {
	apiErr := &OlympusAPIError{
		StatusCode: statusCode,
		Code:       "UNKNOWN",
		Message:    "Unknown error",
	}

	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		apiErr.Message = http.StatusText(statusCode)
		return apiErr
	}

	var envelope errorResponse
	envelopeHit := false
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error != nil {
		apiErr.Code = envelope.Error.Code
		apiErr.Message = envelope.Error.Message
		apiErr.RequestID = envelope.Error.RequestID
		envelopeHit = true
	}

	// Try flat error structure (only when envelope didn't match)
	if !envelopeHit {
		var flat map[string]interface{}
		if err := json.Unmarshal(body, &flat); err == nil {
			if code, ok := flat["code"].(string); ok {
				apiErr.Code = code
			}
			if msg, ok := flat["message"].(string); ok {
				apiErr.Message = msg
			}
			if reqID, ok := flat["request_id"].(string); ok {
				apiErr.RequestID = reqID
			}
		}
	}

	// Fallback: use raw body as message
	if apiErr.Message == "Unknown error" {
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			apiErr.Message = msg
		}
	}

	// Route recognized app-scoped codes to typed error subclasses.
	if typed := routeAppScopedError(apiErr, body, headers); typed != nil {
		return typed
	}

	return apiErr
}
