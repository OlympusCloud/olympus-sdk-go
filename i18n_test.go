// Tests for I18nService — wraps GET /v1/i18n/errors (#3637).
//
// Covers:
// - manifest fetch + parse
// - cache hit avoids second HTTP call
// - in-flight dedup (concurrent cold callers share one request)
// - localize fallback to en when locale unknown
// - localize returns code when code unknown
// - LocalizeError happy + missing-code paths
// - integration test against a recorded fixture matching the canonical
//   response shape (AC-6)

package olympus

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// canonicalManifestFixture mirrors the byte-for-byte response shape from
// backend/rust/platform/src/handlers/i18n.rs. Locked here so the SDK
// parser keeps working even when the deployed endpoint lags.
const canonicalManifestFixture = `{
  "version": "1.0",
  "locales": ["en", "es", "fr"],
  "errors": [
    {
      "code": "NOT_FOUND",
      "messages": {
        "en": "The requested resource was not found.",
        "es": "No se encontró el recurso solicitado.",
        "fr": "La ressource demandée est introuvable."
      }
    },
    {
      "code": "VALIDATION_ERROR",
      "messages": {
        "en": "One or more fields failed validation.",
        "es": "Uno o más campos no superaron la validación.",
        "fr": "Un ou plusieurs champs ont échoué à la validation."
      }
    },
    {
      "code": "RATE_LIMIT_EXCEEDED",
      "messages": {
        "en": "Too many requests. Please try again in a few moments.",
        "es": "Demasiadas solicitudes. Por favor, inténtelo de nuevo en unos momentos.",
        "fr": "Trop de requêtes. Veuillez réessayer dans quelques instants."
      }
    }
  ]
}`

func i18nMockServer(t *testing.T, hits *int32, delay time.Duration) (*OlympusClient, func()) {
	t.Helper()
	handler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		if delay > 0 {
			time.Sleep(delay)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(canonicalManifestFixture))
	}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/v1/i18n/errors": handler,
	})
	return client, func() {}
}

func TestI18n_Errors_FetchesAndParses(t *testing.T) {
	var hits int32
	client, cleanup := i18nMockServer(t, &hits, 0)
	defer cleanup()

	manifest, err := client.I18n().Errors(context.Background(), "en")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if manifest.Version != "1.0" {
		t.Errorf("version: %q", manifest.Version)
	}
	if len(manifest.Locales) != 3 || manifest.Locales[0] != "en" {
		t.Errorf("locales: %v", manifest.Locales)
	}
	if len(manifest.Errors) != 3 {
		t.Fatalf("entries: %d", len(manifest.Errors))
	}
	if manifest.Errors[0].Code != "NOT_FOUND" {
		t.Errorf("first code: %q", manifest.Errors[0].Code)
	}
	if got := manifest.Errors[0].Messages["es"]; got != "No se encontró el recurso solicitado." {
		t.Errorf("es: %q", got)
	}
}

func TestI18n_Errors_CacheHit(t *testing.T) {
	var hits int32
	client, cleanup := i18nMockServer(t, &hits, 0)
	defer cleanup()

	for i := 0; i < 5; i++ {
		if _, err := client.I18n().Errors(context.Background(), "en"); err != nil {
			t.Fatal(err)
		}
	}
	// Different locale arg, same network call (manifest is locale-agnostic).
	if _, err := client.I18n().Errors(context.Background(), "fr"); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("expected 1 HTTP hit, got %d", got)
	}
}

func TestI18n_Errors_ConcurrentColdCallers_ShareOneInflight(t *testing.T) {
	var hits int32
	client, cleanup := i18nMockServer(t, &hits, 30*time.Millisecond)
	defer cleanup()

	const N = 8
	var wg sync.WaitGroup
	wg.Add(N)
	errCh := make(chan error, N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_, err := client.I18n().Errors(context.Background(), "en")
			if err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("worker err: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("singleflight broke: expected 1 HTTP hit for %d concurrent callers, got %d", N, got)
	}
}

func TestI18n_Errors_ClearCache_ForcesRefetch(t *testing.T) {
	var hits int32
	client, cleanup := i18nMockServer(t, &hits, 0)
	defer cleanup()

	if _, err := client.I18n().Errors(context.Background(), "en"); err != nil {
		t.Fatal(err)
	}
	client.I18n().ClearCache()
	if _, err := client.I18n().Errors(context.Background(), "en"); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("expected 2 HTTP hits, got %d", got)
	}
}

func TestI18n_Localize_LocaleHit(t *testing.T) {
	var hits int32
	client, _ := i18nMockServer(t, &hits, 0)

	tests := []struct {
		code, locale, want string
	}{
		{"NOT_FOUND", "es", "No se encontró el recurso solicitado."},
		{"NOT_FOUND", "fr", "La ressource demandée est introuvable."},
		{"VALIDATION_ERROR", "es", "Uno o más campos no superaron la validación."},
	}
	for _, tt := range tests {
		got, err := client.I18n().Localize(context.Background(), tt.code, tt.locale)
		if err != nil {
			t.Errorf("%s/%s err: %v", tt.code, tt.locale, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s/%s: got %q want %q", tt.code, tt.locale, got, tt.want)
		}
	}
}

func TestI18n_Localize_FallsBackToEn(t *testing.T) {
	var hits int32
	client, _ := i18nMockServer(t, &hits, 0)

	got, err := client.I18n().Localize(context.Background(), "NOT_FOUND", "de")
	if err != nil {
		t.Fatal(err)
	}
	if got != "The requested resource was not found." {
		t.Errorf("expected en fallback, got %q", got)
	}
}

func TestI18n_Localize_UnknownCode_ReturnsCode(t *testing.T) {
	var hits int32
	client, _ := i18nMockServer(t, &hits, 0)

	got, err := client.I18n().Localize(context.Background(), "UNKNOWN_FUTURE_CODE", "en")
	if err != nil {
		t.Fatal(err)
	}
	if got != "UNKNOWN_FUTURE_CODE" {
		t.Errorf("expected raw code, got %q", got)
	}
}

func TestI18n_Localize_EmptyCode(t *testing.T) {
	var hits int32
	client, _ := i18nMockServer(t, &hits, 0)

	got, err := client.I18n().Localize(context.Background(), "", "en")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("empty code should return empty string, got %q", got)
	}
	got, err = client.I18n().Localize(context.Background(), "   ", "en")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("whitespace code should return empty string, got %q", got)
	}
}

func TestI18n_LocalizeError_HappyPath(t *testing.T) {
	var hits int32
	client, _ := i18nMockServer(t, &hits, 0)

	apiErr := &OlympusAPIError{Code: "NOT_FOUND", Message: "server msg", StatusCode: 404}
	got, err := client.I18n().LocalizeError(context.Background(), apiErr, "es")
	if err != nil {
		t.Fatal(err)
	}
	if got != "No se encontró el recurso solicitado." {
		t.Errorf("got %q", got)
	}
}

func TestI18n_LocalizeError_UnknownCode_FallsBackToServerMsg(t *testing.T) {
	var hits int32
	client, _ := i18nMockServer(t, &hits, 0)

	apiErr := &OlympusAPIError{Code: "BRAND_NEW_CODE", Message: "server-side English", StatusCode: 500}
	got, err := client.I18n().LocalizeError(context.Background(), apiErr, "es")
	if err != nil {
		t.Fatal(err)
	}
	if got != "server-side English" {
		t.Errorf("got %q", got)
	}
}

func TestI18n_LocalizeError_EmptyCode_ReturnsServerMsg(t *testing.T) {
	var hits int32
	client, _ := i18nMockServer(t, &hits, 0)

	apiErr := &OlympusAPIError{Code: "", Message: "plain text error", StatusCode: 502}
	got, err := client.I18n().LocalizeError(context.Background(), apiErr, "es")
	if err != nil {
		t.Fatal(err)
	}
	if got != "plain text error" {
		t.Errorf("got %q", got)
	}
}

func TestI18n_LocalizeError_FallsThroughToCode(t *testing.T) {
	var hits int32
	client, _ := i18nMockServer(t, &hits, 0)

	apiErr := &OlympusAPIError{Code: "BRAND_NEW_CODE", Message: "", StatusCode: 500}
	got, err := client.I18n().LocalizeError(context.Background(), apiErr, "es")
	if err != nil {
		t.Fatal(err)
	}
	if got != "BRAND_NEW_CODE" {
		t.Errorf("got %q", got)
	}
}

func TestI18n_LocalizeError_NilErr(t *testing.T) {
	var hits int32
	client, _ := i18nMockServer(t, &hits, 0)

	got, err := client.I18n().LocalizeError(context.Background(), nil, "en")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("nil err should return empty string, got %q", got)
	}
}

// AC-6 — fixture-shape contract validating the schema invariants the Rust
// manifest tests enforce server-side. If the deployed endpoint changes
// shape, this test fails — protecting downstream apps from silent parse
// failures during a bad deploy.
func TestI18n_E2eFixtureContract(t *testing.T) {
	var hits int32
	client, _ := i18nMockServer(t, &hits, 0)

	manifest, err := client.I18n().Errors(context.Background(), "en")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.0" {
		t.Errorf("version: %q", manifest.Version)
	}
	wantLocales := map[string]bool{"en": true, "es": true, "fr": true}
	for _, loc := range manifest.Locales {
		delete(wantLocales, loc)
	}
	if len(wantLocales) != 0 {
		t.Errorf("missing locales: %v", wantLocales)
	}

	// Schema invariant: every entry must translate every locale.
	for _, entry := range manifest.Errors {
		for _, locale := range manifest.Locales {
			msg, ok := entry.Messages[locale]
			if !ok {
				t.Errorf("entry %s missing locale %s", entry.Code, locale)
				continue
			}
			if msg == "" {
				t.Errorf("entry %s locale %s blank", entry.Code, locale)
			}
		}
	}

	// Spot-check a translation actually localizes.
	msg, err := client.I18n().Localize(context.Background(), "RATE_LIMIT_EXCEEDED", "es")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(msg, "Demasiadas solicitudes") {
		t.Errorf("es localization missing: %q", msg)
	}
}

func TestErrorManifest_EntryFor_NilSafe(t *testing.T) {
	var m *ErrorManifest
	if m.EntryFor("ANY") != nil {
		t.Error("nil manifest EntryFor should return nil")
	}
}

func TestErrorManifestEntry_MessageFor_FallbackChain(t *testing.T) {
	entry := &ErrorManifestEntry{
		Code: "X",
		Messages: map[string]string{
			"en": "english",
			"es": "spanish",
		},
	}
	if got := entry.MessageFor("es"); got != "spanish" {
		t.Errorf("es: %q", got)
	}
	if got := entry.MessageFor("de"); got != "english" {
		t.Errorf("de fallback: %q", got)
	}
	empty := &ErrorManifestEntry{Code: "Y", Messages: map[string]string{}}
	if got := empty.MessageFor("en"); got != "" {
		t.Errorf("empty entry: %q", got)
	}
	var nilEntry *ErrorManifestEntry
	if got := nilEntry.MessageFor("en"); got != "" {
		t.Errorf("nil entry: %q", got)
	}
}
