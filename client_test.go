package olympus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testServer creates a test HTTP server that routes requests to handler functions.
func testServer(t *testing.T, handlers map[string]http.HandlerFunc) (*httptest.Server, *OlympusClient) {
	t.Helper()

	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := NewClient(Config{
		AppID:      "test-app",
		APIKey:     "test-key-123",
		BaseURL:    server.URL,
		MaxRetries: 0, // No retries in tests.
	})

	return server, client
}

// jsonResponse writes a JSON response to the test response writer.
func jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func TestNewClient(t *testing.T) {
	client := NewClient(Config{
		AppID:  "com.test-app",
		APIKey: "oc_test_abc",
	})

	if client.Config().AppID != "com.test-app" {
		t.Errorf("expected AppID 'com.test-app', got '%s'", client.Config().AppID)
	}
	if client.Config().APIKey != "oc_test_abc" {
		t.Errorf("expected APIKey 'oc_test_abc', got '%s'", client.Config().APIKey)
	}
}

func TestClientServiceAccessors(t *testing.T) {
	client := NewClient(Config{AppID: "test", APIKey: "key"})

	// All 13 service accessors should return non-nil and be stable (cached).
	if client.Auth() == nil {
		t.Error("Auth() returned nil")
	}
	if client.Auth() != client.Auth() {
		t.Error("Auth() not cached")
	}
	if client.Commerce() == nil {
		t.Error("Commerce() returned nil")
	}
	if client.AI() == nil {
		t.Error("AI() returned nil")
	}
	if client.Pay() == nil {
		t.Error("Pay() returned nil")
	}
	if client.Notify() == nil {
		t.Error("Notify() returned nil")
	}
	if client.Events() == nil {
		t.Error("Events() returned nil")
	}
	if client.Data() == nil {
		t.Error("Data() returned nil")
	}
	if client.Storage() == nil {
		t.Error("Storage() returned nil")
	}
	if client.Marketplace() == nil {
		t.Error("Marketplace() returned nil")
	}
	if client.Billing() == nil {
		t.Error("Billing() returned nil")
	}
	if client.Gating() == nil {
		t.Error("Gating() returned nil")
	}
	if client.Devices() == nil {
		t.Error("Devices() returned nil")
	}
	if client.Observe() == nil {
		t.Error("Observe() returned nil")
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{AppID: "test", APIKey: "key"}

	if cfg.effectiveBaseURL() != "https://api.olympuscloud.ai/api/v1" {
		t.Errorf("unexpected default base URL: %s", cfg.effectiveBaseURL())
	}
	if cfg.effectiveTimeout() != 30*time.Second {
		t.Errorf("unexpected default timeout: %v", cfg.effectiveTimeout())
	}
	if cfg.effectiveMaxRetries() != 3 {
		t.Errorf("unexpected default max retries: %d", cfg.effectiveMaxRetries())
	}
	if cfg.effectiveRetryBaseDelay() != 500*time.Millisecond {
		t.Errorf("unexpected default retry base delay: %v", cfg.effectiveRetryBaseDelay())
	}
}

func TestConfigEnvironmentURLs(t *testing.T) {
	tests := []struct {
		env  Environment
		want string
	}{
		{EnvProduction, "https://api.olympuscloud.ai/api/v1"},
		{EnvStaging, "https://staging.api.olympuscloud.ai/api/v1"},
		{EnvDevelopment, "https://dev.api.olympuscloud.ai/api/v1"},
		{EnvSandbox, "https://sandbox.api.olympuscloud.ai/api/v1"},
	}

	for _, tt := range tests {
		cfg := Config{Environment: tt.env}
		if got := cfg.effectiveBaseURL(); got != tt.want {
			t.Errorf("Environment %s: got URL %s, want %s", tt.env, got, tt.want)
		}
	}
}

func TestConfigCustomBaseURL(t *testing.T) {
	cfg := Config{
		BaseURL:     "https://custom.api.example.com",
		Environment: EnvProduction,
	}
	if got := cfg.effectiveBaseURL(); got != "https://custom.api.example.com" {
		t.Errorf("custom BaseURL not used: got %s", got)
	}
}

func TestConfigCustomTimeout(t *testing.T) {
	cfg := Config{Timeout: 10 * time.Second}
	if got := cfg.effectiveTimeout(); got != 10*time.Second {
		t.Errorf("custom timeout not used: got %v", got)
	}
}

func TestAuthHeaders(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/auth/me": func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			appID := r.Header.Get("X-App-Id")
			sdkVer := r.Header.Get("X-SDK-Version")

			if auth != "Bearer test-key-123" {
				t.Errorf("expected Bearer test-key-123, got %s", auth)
			}
			if appID != "test-app" {
				t.Errorf("expected X-App-Id test-app, got %s", appID)
			}
			if sdkVer != "go/0.1.0" {
				t.Errorf("expected X-SDK-Version go/0.1.0, got %s", sdkVer)
			}

			jsonResponse(w, 200, map[string]interface{}{
				"id":    "user-1",
				"email": "test@example.com",
			})
		},
	})

	ctx := context.Background()
	_, err := client.Auth().Me(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAccessTokenOverridesAPIKey(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/auth/me": func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer user-token-xyz" {
				t.Errorf("expected Bearer user-token-xyz, got %s", auth)
			}
			jsonResponse(w, 200, map[string]interface{}{
				"id":    "user-1",
				"email": "test@example.com",
			})
		},
	})

	client.HTTPClient().SetAccessToken("user-token-xyz")
	ctx := context.Background()
	_, err := client.Auth().Me(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIErrorParsing(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/auth/me": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 401, map[string]interface{}{
				"error": map[string]interface{}{
					"code":       "UNAUTHORIZED",
					"message":    "Invalid token",
					"request_id": "req-abc-123",
				},
			})
		},
	})

	ctx := context.Background()
	_, err := client.Auth().Me(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected OlympusAPIError, got %T", err)
	}

	if apiErr.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", apiErr.Code)
	}
	if apiErr.Message != "Invalid token" {
		t.Errorf("expected message 'Invalid token', got '%s'", apiErr.Message)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("expected status 401, got %d", apiErr.StatusCode)
	}
	if apiErr.RequestID != "req-abc-123" {
		t.Errorf("expected request_id 'req-abc-123', got '%s'", apiErr.RequestID)
	}
	if !apiErr.IsUnauthorized() {
		t.Error("expected IsUnauthorized() to be true")
	}
}

func TestAPIErrorNotFound(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/commerce/orders/missing": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 404, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "NOT_FOUND",
					"message": "Order not found",
				},
			})
		},
	})

	ctx := context.Background()
	_, err := client.Commerce().GetOrder(ctx, "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected OlympusAPIError, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Error("expected IsNotFound() to be true")
	}
}

func TestOlympusAPIErrorInterface(t *testing.T) {
	err := &OlympusAPIError{
		Code:       "RATE_LIMITED",
		Message:    "Too many requests",
		StatusCode: 429,
		RequestID:  "req-xyz",
	}

	if !err.IsRateLimited() {
		t.Error("expected IsRateLimited() true")
	}
	if err.IsForbidden() {
		t.Error("expected IsForbidden() false")
	}
	if err.IsServerError() {
		t.Error("expected IsServerError() false")
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error() returned empty string")
	}

	serverErr := &OlympusAPIError{StatusCode: 500}
	if !serverErr.IsServerError() {
		t.Error("expected IsServerError() true for 500")
	}
}

func TestHTTPClient204NoContent(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/storage/objects/test.txt": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(204)
		},
	})

	ctx := context.Background()
	err := client.Storage().Delete(ctx, "test.txt")
	if err != nil {
		t.Fatalf("unexpected error on 204: %v", err)
	}
}
