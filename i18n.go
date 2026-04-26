// I18nService — error code i18n manifest consumer (issue #3637).
//
// Wraps `GET /v1/i18n/errors`, the centralised error code -> localized
// message manifest served by the Rust platform service. Consumers use it
// to render user-friendly translations of platform errors that arrive in
// the `{ error: { code, message } }` envelope without bundling per-app
// translations.
//
//	manifest, err := client.I18n().Errors(ctx, "en")
//	msg, err := client.I18n().Localize(ctx, "ORDER_NOT_FOUND", "es")
//
//	// Or localize a typed error directly:
//	if apiErr, ok := err.(*OlympusAPIError); ok {
//	    msg, _ := client.I18n().LocalizeError(ctx, apiErr, "fr")
//	    showSnackbar(msg)
//	}
//
// Caching: the manifest is identical for every caller, so we cache the
// parsed result in-memory for 1h (matches the backend
// `Cache-Control: public, max-age=3600`). Concurrent cold callers share
// a single in-flight request via a hand-rolled singleflight pattern
// (mutex + chan) so we never issue duplicate requests for the same
// payload.
//
// The `locale` argument on Errors is intentionally accepted-but-ignored
// at the network layer: the backend always ships every locale in one
// payload. We keep it on the public surface for API symmetry with
// Localize.

package olympus

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
	"time"
)

// ErrorManifestEntry is one row in the error manifest.
//
// Code is the canonical UPPER_SNAKE error code emitted in the
// `{ error: { code, message } }` envelope (e.g. ORDER_NOT_FOUND,
// VALIDATION_ERROR). Messages maps a locale string (BCP-47-ish, e.g.
// "en", "es", "fr") to the human-readable translation.
type ErrorManifestEntry struct {
	Code     string            `json:"code"`
	Messages map[string]string `json:"messages"`
}

// ErrorManifest is the top-level shape served by `GET /v1/i18n/errors`.
//
// The manifest is identical for every caller — the response is cached
// at the edge for 1 hour and the SDK caches the parsed result in-memory
// for the same window.
type ErrorManifest struct {
	// Version is the schema version. Bumped on breaking shape changes
	// (additive code changes stay at "1.0").
	Version string `json:"version"`
	// Locales the manifest carries human-authored translations for.
	// Apps should fall back to "en" when their preferred locale isn't
	// listed.
	Locales []string `json:"locales"`
	// Errors is one entry per canonical error code.
	Errors []ErrorManifestEntry `json:"errors"`
}

// EntryFor looks up a manifest entry by canonical code. Returns nil
// when the code is not in the manifest.
func (m *ErrorManifest) EntryFor(code string) *ErrorManifestEntry {
	if m == nil {
		return nil
	}
	for i := range m.Errors {
		if m.Errors[i].Code == code {
			return &m.Errors[i]
		}
	}
	return nil
}

// MessageFor picks the message for locale, falling back to "en". Returns
// "" if neither is present (caller should fall back to the raw code).
func (e *ErrorManifestEntry) MessageFor(locale string) string {
	if e == nil {
		return ""
	}
	if msg, ok := e.Messages[locale]; ok && msg != "" {
		return msg
	}
	if msg, ok := e.Messages["en"]; ok {
		return msg
	}
	return ""
}

// I18nCacheTTL is the cache duration for the parsed manifest. Must match
// the backend `Cache-Control: max-age=3600`.
const I18nCacheTTL = 1 * time.Hour

// I18nService fetches + caches + localizes against the platform error
// manifest at `GET /v1/i18n/errors`.
type I18nService struct {
	http *httpClient

	mu        sync.Mutex
	cached    *ErrorManifest
	expiresAt time.Time
	// inflight is the broadcast channel for the in-progress fetch.
	// Concurrent cold callers receive the same result on the same chan;
	// we never issue two parallel requests for the same payload.
	inflight chan i18nResult
}

type i18nResult struct {
	manifest *ErrorManifest
	err      error
}

// Errors fetches the full error manifest. The response is cached for
// I18nCacheTTL after the first call. Concurrent cold callers share a
// single HTTP request.
//
// The locale argument is decorative — the backend always ships every
// locale in one payload. Use Localize when you only need a single
// translated string.
func (s *I18nService) Errors(ctx context.Context, locale string) (*ErrorManifest, error) {
	_ = locale // Decorative; manifest is locale-agnostic.

	s.mu.Lock()
	if s.cached != nil && time.Now().Before(s.expiresAt) {
		m := s.cached
		s.mu.Unlock()
		return m, nil
	}
	if s.inflight != nil {
		// Another goroutine is fetching. Wait on its broadcast chan.
		ch := s.inflight
		s.mu.Unlock()
		select {
		case res, ok := <-ch:
			if !ok {
				// Channel closed without a value (shouldn't happen, but
				// be defensive). Re-enter to retry.
				return s.Errors(ctx, locale)
			}
			return res.manifest, res.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// We are the first caller. Mark in-flight, fetch, then broadcast.
	ch := make(chan i18nResult, 1)
	s.inflight = ch
	s.mu.Unlock()

	manifest, err := s.fetch(ctx)
	res := i18nResult{manifest: manifest, err: err}

	s.mu.Lock()
	if err == nil && manifest != nil {
		s.cached = manifest
		s.expiresAt = time.Now().Add(I18nCacheTTL)
	}
	s.inflight = nil
	s.mu.Unlock()

	// Broadcast: any goroutine waiting on this chan receives the result,
	// then close so late receivers don't block forever (a buffered
	// 1-slot chan + close pattern lets every receiver get the value
	// because closed-with-value chans deliver to all receivers).
	ch <- res
	close(ch)

	return manifest, err
}

func (s *I18nService) fetch(ctx context.Context) (*ErrorManifest, error) {
	raw, err := s.http.getRaw(ctx, "/v1/i18n/errors", url.Values{})
	if err != nil {
		return nil, err
	}
	var manifest ErrorManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// Localize resolves code to a human-readable string in locale, falling
// back to "en" and finally to the raw code itself when neither is
// present. Triggers a manifest fetch on first call (or after the 1h
// cache expires). Empty/whitespace code short-circuits to "".
func (s *I18nService) Localize(ctx context.Context, code, locale string) (string, error) {
	if trimmed := trimSpace(code); trimmed == "" {
		return "", nil
	}
	manifest, err := s.Errors(ctx, locale)
	if err != nil {
		return "", err
	}
	entry := manifest.EntryFor(code)
	if entry == nil {
		return code, nil
	}
	if msg := entry.MessageFor(locale); msg != "" {
		return msg, nil
	}
	return code, nil
}

// LocalizeError localizes the Code carried by an OlympusAPIError.
//
// Returns the localized message when the code is in the manifest;
// otherwise falls back to err.Message (the server-provided English
// string) and finally to the raw error code. Network errors during the
// manifest fetch are returned as the second return value.
func (s *I18nService) LocalizeError(
	ctx context.Context,
	apiErr *OlympusAPIError,
	locale string,
) (string, error) {
	if apiErr == nil {
		return "", nil
	}
	if apiErr.Code == "" {
		return apiErr.Message, nil
	}
	manifest, err := s.Errors(ctx, locale)
	if err != nil {
		return "", err
	}
	entry := manifest.EntryFor(apiErr.Code)
	if entry == nil {
		if apiErr.Message != "" {
			return apiErr.Message, nil
		}
		return apiErr.Code, nil
	}
	if msg := entry.MessageFor(locale); msg != "" {
		return msg, nil
	}
	if apiErr.Message != "" {
		return apiErr.Message, nil
	}
	return apiErr.Code, nil
}

// ClearCache drops any cached manifest. Useful for tests and for tenants
// that have flipped a manifest version mid-session.
func (s *I18nService) ClearCache() {
	s.mu.Lock()
	s.cached = nil
	s.expiresAt = time.Time{}
	s.mu.Unlock()
}

// trimSpace is a small helper avoiding a strings package import for one
// call site. Equivalent to strings.TrimSpace for the ASCII whitespace
// our error-code inputs ever contain.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end {
		c := s[start]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		start++
	}
	for end > start {
		c := s[end-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		end--
	}
	return s[start:end]
}
