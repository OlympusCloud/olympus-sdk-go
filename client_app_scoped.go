package olympus

// This file completes the *OlympusClient surface that ships with the
// app-scoped permissions v2.0 work in PR #2 (consent + governance + typed
// errors). The associated test files (app_scoped_permissions_test.go,
// concurrent_cache_test.go) reference these methods directly on
// *OlympusClient — adding them here unblocks `go test ./...` so the Wave 2
// CI gate is green.
//
// References:
//   - docs/platform/APP-SCOPED-PERMISSIONS.md §6 — consent surface
//   - docs/platform/APP-SCOPED-PERMISSIONS.md §4.5 — dual-JWT flow
//   - docs/platform/APP-SCOPED-PERMISSIONS.md §4.7 — stale-catalog rolling
//     window
//
// All token/bitset state is held on the underlying httpClient + the cache
// fields already declared on OlympusClient. These accessors are the public
// surface; the http transport and JWT parser do the actual work.

// SetAccessToken sets the user-scoped bearer token used for auth on every
// subsequent request. Also invalidates the JWT-claim and scope-bitset
// caches so HasScopeBit / IsAppScoped reflect the new token immediately.
func (c *OlympusClient) SetAccessToken(token string) {
	c.http.SetAccessToken(token)
	c.cacheMu.Lock()
	c.cachedClaims = nil
	c.cachedClaimsForToken = ""
	c.cachedBitset = nil
	c.cachedBitsetForToken = ""
	c.cacheMu.Unlock()
}

// ClearAccessToken removes the user-scoped access token and invalidates the
// JWT caches.
func (c *OlympusClient) ClearAccessToken() {
	c.http.ClearAccessToken()
	c.cacheMu.Lock()
	c.cachedClaims = nil
	c.cachedClaimsForToken = ""
	c.cachedBitset = nil
	c.cachedBitsetForToken = ""
	c.cacheMu.Unlock()
}

// SetAppToken sets the App JWT (attached as X-App-Token on every request
// per §4.5).
func (c *OlympusClient) SetAppToken(token string) {
	c.http.SetAppToken(token)
}

// ClearAppToken clears the App JWT.
func (c *OlympusClient) ClearAppToken() {
	c.http.ClearAppToken()
}

// OnCatalogStale registers a handler that fires when the gateway returns
// X-Olympus-Catalog-Stale: true (§4.7). Pass nil to clear.
func (c *OlympusClient) OnCatalogStale(handler StaleCatalogHandler) {
	c.http.OnCatalogStale(handler)
}

// IsAppScoped reports whether the current access token carries an
// app-scoped session (i.e. has app_id + app_scopes_bitset claims). Returns
// false when there's no token, the token isn't a JWT, or the token is a
// platform-shell token.
func (c *OlympusClient) IsAppScoped() bool {
	claims := c.currentClaims()
	if claims == nil {
		return false
	}
	if _, ok := claims["app_id"].(string); !ok {
		return false
	}
	if _, ok := claims["app_scopes_bitset"].(string); !ok {
		return false
	}
	return true
}

// HasScopeBit returns true iff the bit at position bitID is set in the
// access token's app_scopes_bitset claim. Returns false for negative bitID,
// out-of-range bitID, missing tokens, or platform-shell tokens.
//
// The decoded bitset is cached and invalidated on SetAccessToken.
func (c *OlympusClient) HasScopeBit(bitID int) bool {
	if bitID < 0 {
		return false
	}
	bitset := c.currentBitset()
	if bitset == nil {
		return false
	}
	byteIdx := bitID / 8
	if byteIdx >= len(bitset) {
		return false
	}
	return bitset[byteIdx]&(1<<(bitID%8)) != 0
}

// currentClaims returns the lazily-decoded JWT claims for the active
// access token, caching them by token. Returns nil if no token is set or
// the token doesn't parse.
func (c *OlympusClient) currentClaims() map[string]interface{} {
	token := c.http.GetAccessToken()
	if token == "" {
		return nil
	}
	c.cacheMu.RLock()
	if c.cachedClaimsForToken == token && c.cachedClaims != nil {
		out := c.cachedClaims
		c.cacheMu.RUnlock()
		return out
	}
	c.cacheMu.RUnlock()

	claims := parseJWTPayload(token)
	if claims == nil {
		return nil
	}
	c.cacheMu.Lock()
	c.cachedClaims = claims
	c.cachedClaimsForToken = token
	c.cacheMu.Unlock()
	return claims
}

// currentBitset returns the lazily-decoded app_scopes_bitset bytes for the
// active access token, caching them by token. Returns nil when there's no
// token, no bitset claim, or the bitset can't be decoded.
func (c *OlympusClient) currentBitset() []byte {
	token := c.http.GetAccessToken()
	if token == "" {
		return nil
	}
	c.cacheMu.RLock()
	if c.cachedBitsetForToken == token && c.cachedBitset != nil {
		out := c.cachedBitset
		c.cacheMu.RUnlock()
		return out
	}
	c.cacheMu.RUnlock()

	claims := c.currentClaims()
	if claims == nil {
		return nil
	}
	encoded, ok := claims["app_scopes_bitset"].(string)
	if !ok || encoded == "" {
		return nil
	}
	decoded, err := base64URLDecode(encoded)
	if err != nil {
		return nil
	}
	c.cacheMu.Lock()
	c.cachedBitset = decoded
	c.cachedBitsetForToken = token
	c.cacheMu.Unlock()
	return decoded
}
