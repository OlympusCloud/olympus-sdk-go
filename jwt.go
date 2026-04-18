package olympus

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)


// parseJWTPayload decodes the payload segment of a JWT into a map.
// Performs NO signature verification — that's the gateway's job. This helper
// exists purely to read claims client-side for the scope bitset fast-path.
// Returns nil on any parse error.
func parseJWTPayload(token string) map[string]interface{} {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	decoded, err := base64URLDecode(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil
	}
	return claims
}

// base64URLDecode handles Base64url-no-pad encoding used by JWT parts +
// the app_scopes_bitset claim. RawURLEncoding natively handles the unpadded
// form so no manual padding calculation is required.
func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
