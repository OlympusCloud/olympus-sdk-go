package olympus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIdentity_GetOrCreateFromFirebase_HappyPath(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/identities": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{
				"id":           "id-1",
				"firebase_uid": "fb-uid-1",
				"email":        "u@example.com",
				"created_at":   "2026-04-19T00:00:00Z",
				"updated_at":   "2026-04-19T00:00:00Z",
			})
		},
	})

	out, err := client.Identity().GetOrCreateFromFirebase(context.Background(), GetOrCreateFromFirebaseRequest{
		FirebaseUID: "fb-uid-1",
		Email:       "u@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: %s", gotMethod)
	}
	if gotPath != "/platform/identities" {
		t.Errorf("path: %s", gotPath)
	}
	if gotBody["firebase_uid"] != "fb-uid-1" {
		t.Errorf("firebase_uid not sent: %v", gotBody)
	}
	if out.ID != "id-1" || out.Email != "u@example.com" {
		t.Errorf("response not parsed: %+v", out)
	}
}

func TestIdentity_GetOrCreateFromFirebase_RequiresUID(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	_, err := client.Identity().GetOrCreateFromFirebase(context.Background(),
		GetOrCreateFromFirebaseRequest{})
	if err == nil {
		t.Fatal("expected error for missing firebase_uid, got nil")
	}
}

func TestIdentity_LinkToTenant_PostsExpectedBody(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/identities/links": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{})
		},
	})
	if err := client.Identity().LinkToTenant(context.Background(),
		"olympus-1", "tenant-1", "cust-1"); err != nil {
		t.Fatal(err)
	}
	if gotBody["olympus_id"] != "olympus-1" ||
		gotBody["tenant_id"] != "tenant-1" ||
		gotBody["commerce_customer_id"] != "cust-1" {
		t.Errorf("link body wrong: %v", gotBody)
	}
}

func TestIdentity_LinkToTenant_PropagatesError(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/identities/links": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 403, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "FORBIDDEN",
					"message": "not allowed",
				},
			})
		},
	})
	err := client.Identity().LinkToTenant(context.Background(), "o", "t", "c")
	if err == nil {
		t.Fatal("expected error from 403, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected OlympusAPIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("expected 403, got %d", apiErr.StatusCode)
	}
}

func TestIdentity_AgeVerification_RouteShape(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		method   string
		invoke   func(s *IdentityService) error
		wantBody map[string]interface{}
	}{
		{
			name:   "ScanID",
			path:   "/identity/scan-id",
			method: http.MethodPost,
			invoke: func(s *IdentityService) error {
				_, err := s.ScanID(context.Background(), "+15551234567", []byte{0x01, 0x02})
				return err
			},
			wantBody: map[string]interface{}{
				"phone": "+15551234567",
			},
		},
		{
			name:   "VerifyPassphrase",
			path:   "/identity/verify-passphrase",
			method: http.MethodPost,
			invoke: func(s *IdentityService) error {
				_, err := s.VerifyPassphrase(context.Background(), "+15551234567", "secret")
				return err
			},
			wantBody: map[string]interface{}{
				"phone":      "+15551234567",
				"passphrase": "secret",
			},
		},
		{
			name:   "SetPassphrase",
			path:   "/identity/set-passphrase",
			method: http.MethodPost,
			invoke: func(s *IdentityService) error {
				_, err := s.SetPassphrase(context.Background(), "+15551234567", "secret")
				return err
			},
			wantBody: map[string]interface{}{
				"phone":      "+15551234567",
				"passphrase": "secret",
			},
		},
		{
			name:   "CreateUploadSession",
			path:   "/identity/create-upload-session",
			method: http.MethodPost,
			invoke: func(s *IdentityService) error {
				_, err := s.CreateUploadSession(context.Background())
				return err
			},
		},
		{
			name:   "CheckVerificationStatus",
			path:   "/identity/status/+15551234567",
			method: http.MethodGet,
			invoke: func(s *IdentityService) error {
				_, err := s.CheckVerificationStatus(context.Background(), "+15551234567")
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			var gotBody map[string]interface{}
			_, client := testServer(t, map[string]http.HandlerFunc{
				tc.path: func(w http.ResponseWriter, r *http.Request) {
					gotMethod = r.Method
					gotPath = r.URL.Path
					if r.Body != nil {
						_ = json.NewDecoder(r.Body).Decode(&gotBody)
					}
					jsonResponse(w, 200, map[string]interface{}{})
				},
			})
			if err := tc.invoke(client.Identity()); err != nil {
				t.Fatalf("invoke: %v", err)
			}
			if gotMethod != tc.method {
				t.Errorf("method: got %s, want %s", gotMethod, tc.method)
			}
			if gotPath != tc.path {
				t.Errorf("path: got %s, want %s", gotPath, tc.path)
			}
			for k, v := range tc.wantBody {
				if gotBody[k] != v {
					t.Errorf("body[%s] = %v, want %v", k, gotBody[k], v)
				}
			}
		})
	}
}

// --------------------------------------------------------------------------
// Identity invite wrappers (#3403 §4.2)
// --------------------------------------------------------------------------

func TestIdentity_Invite_PostsExpectedPayload(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/identity/invite": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{
				"id":          "invite-1",
				"token":       "invite-jwt-xyz",
				"email":       "newhire@pizza.test",
				"role":        "staff",
				"location_id": "loc-42",
				"tenant_id":   "tenant-1",
				"expires_at":  "2026-04-28T10:00:00Z",
				"status":      "pending",
				"created_at":  "2026-04-21T10:00:00Z",
			})
		},
	})
	out, err := client.Identity().Invite(context.Background(), InviteCreateRequest{
		Email:      "newhire@pizza.test",
		Role:       "staff",
		LocationID: "loc-42",
		Message:    "Welcome to the team!",
		TTLSeconds: 86400 * 7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["email"] != "newhire@pizza.test" {
		t.Errorf("email not sent: %v", gotBody)
	}
	if gotBody["role"] != "staff" {
		t.Errorf("role not sent: %v", gotBody)
	}
	if gotBody["location_id"] != "loc-42" {
		t.Errorf("location_id not sent: %v", gotBody)
	}
	if gotBody["message"] != "Welcome to the team!" {
		t.Errorf("message not sent: %v", gotBody)
	}
	// JSON numbers decode as float64.
	if v, _ := gotBody["ttl_seconds"].(float64); v != 86400*7 {
		t.Errorf("ttl_seconds not sent: %v", gotBody)
	}
	if out.ID != "invite-1" || out.Token != "invite-jwt-xyz" {
		t.Errorf("invite not parsed: %+v", out)
	}
	if out.Status != InviteStatusPending {
		t.Errorf("status not parsed: %v", out.Status)
	}
	if out.ExpiresAt == nil || out.CreatedAt == nil {
		t.Errorf("timestamps not parsed: %+v", out)
	}
}

func TestIdentity_Invite_ValidatesRequired(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	cases := []struct {
		name string
		req  InviteCreateRequest
	}{
		{"no email", InviteCreateRequest{Role: "staff"}},
		{"no role", InviteCreateRequest{Email: "a@b.test"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.Identity().Invite(context.Background(), tc.req)
			if err == nil {
				t.Fatalf("expected client-side validation error")
			}
			if !strings.Contains(err.Error(), "olympus-sdk:") {
				t.Errorf("expected SDK-authored error, got: %v", err)
			}
		})
	}
}

func TestIdentity_Invite_ValidationErrorPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/identity/invite": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 422, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "VALIDATION",
					"message": "role 'godmode' not allowed on invite surface",
				},
			})
		},
	})
	_, err := client.Identity().Invite(context.Background(), InviteCreateRequest{
		Email: "a@b.test", Role: "godmode",
	})
	if err == nil {
		t.Fatal("expected 422 error, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected *OlympusAPIError, got %T", err)
	}
	if apiErr.StatusCode != 422 {
		t.Errorf("expected 422, got %d", apiErr.StatusCode)
	}
}

func TestIdentity_AcceptInvite_InstallsSession(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/identity/invites/invite-jwt-xyz/accept": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			// Mirrors VerifyFirebaseTokenResponse: top-level token fields
			// plus nested `user` (see rust-auth models.rs:366/386).
			jsonResponse(w, 200, map[string]interface{}{
				"access_token":  "accepted-access",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "accepted-refresh",
				"user": map[string]interface{}{
					"id":        "user-42",
					"tenant_id": "tenant-1",
					"email":     "newhire@pizza.test",
					"roles":     []string{"staff"},
				},
			})
		},
	})
	session, err := client.Identity().AcceptInvite(context.Background(), "invite-jwt-xyz", "firebase-id-token-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/identity/invites/invite-jwt-xyz/accept" {
		t.Errorf("path: %s", gotPath)
	}
	if gotBody["firebase_id_token"] != "firebase-id-token-abc" {
		t.Errorf("firebase_id_token not sent: %v", gotBody)
	}
	if session.AccessToken != "accepted-access" {
		t.Errorf("access token not parsed: %+v", session)
	}
	if session.RefreshToken != "accepted-refresh" {
		t.Errorf("refresh token not parsed: %+v", session)
	}
	if got := client.HTTPClient().GetAccessToken(); got != "accepted-access" {
		t.Errorf("access token not installed on http client: got %q", got)
	}
	if session.ExpiresIn != 3600 {
		t.Errorf("expires_in not parsed: got %d", session.ExpiresIn)
	}
	// UserID/TenantID/Roles nest under `user` in the real backend shape —
	// parseAuthSession reads only top-level fields, so those are expected
	// empty here. SDK callers who need the user block should call /auth/me
	// after AcceptInvite.
}

func TestIdentity_AcceptInvite_RequiresBothTokens(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	_, err := client.Identity().AcceptInvite(context.Background(), "", "firebase-token")
	if err == nil {
		t.Error("expected error for empty invite token")
	}
	_, err = client.Identity().AcceptInvite(context.Background(), "invite-token", "")
	if err == nil {
		t.Error("expected error for empty firebase token")
	}
}

func TestIdentity_AcceptInvite_EmailMismatchPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/identity/invites/invite-jwt-xyz/accept": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 403, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "FORBIDDEN",
					"message": "firebase email 'other@b.test' does not match invite email 'newhire@pizza.test'",
				},
			})
		},
	})
	_, err := client.Identity().AcceptInvite(context.Background(), "invite-jwt-xyz", "firebase-id-token-abc")
	if err == nil {
		t.Fatal("expected 403 error, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok || !apiErr.IsForbidden() {
		t.Fatalf("expected forbidden, got %T: %v", err, err)
	}
}

func TestIdentity_ListInvites_BareArray(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/identity/invites": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = io.WriteString(w, `[
				{"id":"i-1","email":"a@b.test","role":"staff","tenant_id":"t-1","status":"pending","expires_at":"2026-04-28T10:00:00Z","created_at":"2026-04-21T10:00:00Z"},
				{"id":"i-2","email":"c@d.test","role":"manager","tenant_id":"t-1","status":"accepted","expires_at":"2026-04-28T10:00:00Z","created_at":"2026-04-21T10:00:00Z","accepted_at":"2026-04-21T11:00:00Z"}
			]`)
		},
	})
	out, err := client.Identity().ListInvites(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 invites, got %d", len(out))
	}
	if out[0].Status != InviteStatusPending {
		t.Errorf("first invite status: %v", out[0].Status)
	}
	if out[1].Status != InviteStatusAccepted || out[1].AcceptedAt == nil {
		t.Errorf("second invite not fully parsed: %+v", out[1])
	}
	// Listed invites MUST NOT include the token (server stores only the hash).
	for _, inv := range out {
		if inv.Token != "" {
			t.Errorf("token leaked on list: %+v", inv)
		}
	}
}

func TestIdentity_ListInvites_EnvelopeItemsResponse(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/identity/invites": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"id": "i-1", "email": "a@b.test", "role": "staff",
						"tenant_id": "t-1", "status": "pending",
					},
				},
			})
		},
	})
	out, err := client.Identity().ListInvites(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].ID != "i-1" {
		t.Errorf("envelope items not parsed: %+v", out)
	}
}

func TestIdentity_RevokeInvite_PostsToID(t *testing.T) {
	var gotPath, gotMethod string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/identity/invites/invite-1/revoke": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotMethod = r.Method
			jsonResponse(w, 200, map[string]interface{}{
				"id":        "invite-1",
				"email":     "newhire@pizza.test",
				"role":      "staff",
				"tenant_id": "tenant-1",
				"status":    "revoked",
			})
		},
	})
	out, err := client.Identity().RevokeInvite(context.Background(), "invite-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/identity/invites/invite-1/revoke" {
		t.Errorf("path: %s", gotPath)
	}
	if out.Status != InviteStatusRevoked {
		t.Errorf("expected revoked status, got %v", out.Status)
	}
}

func TestIdentity_RevokeInvite_RequiresInviteID(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	_, err := client.Identity().RevokeInvite(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty invite id")
	}
}

func TestIdentity_RemoveFromTenant_PostsUserIDAndReason(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/identity/remove_from_tenant": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{
				"tenant_id":  "tenant-1",
				"user_id":    "user-42",
				"removed_at": "2026-04-21T12:00:00Z",
			})
		},
	})
	out, err := client.Identity().RemoveFromTenant(context.Background(), "user-42", "no-show")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["user_id"] != "user-42" {
		t.Errorf("user_id not sent: %v", gotBody)
	}
	if gotBody["reason"] != "no-show" {
		t.Errorf("reason not sent: %v", gotBody)
	}
	if out.TenantID != "tenant-1" || out.UserID != "user-42" || out.RemovedAt == nil {
		t.Errorf("result not parsed: %+v", out)
	}
}

func TestIdentity_RemoveFromTenant_OmitsEmptyReason(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/identity/remove_from_tenant": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{
				"tenant_id":  "tenant-1",
				"user_id":    "user-42",
				"removed_at": "2026-04-21T12:00:00Z",
			})
		},
	})
	_, err := client.Identity().RemoveFromTenant(context.Background(), "user-42", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, has := gotBody["reason"]; has {
		t.Errorf("reason should be omitted when empty: %v", gotBody)
	}
}

func TestIdentity_RemoveFromTenant_RequiresUserID(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{})
	_, err := client.Identity().RemoveFromTenant(context.Background(), "", "no-show")
	if err == nil {
		t.Fatal("expected error for empty user id")
	}
}

func TestIdentity_RemoveFromTenant_SelfRemovalPropagates(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/identity/remove_from_tenant": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 400, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "BAD_REQUEST",
					"message": "cannot remove yourself from the tenant; transfer admin first",
				},
			})
		},
	})
	_, err := client.Identity().RemoveFromTenant(context.Background(), "self-user-id", "")
	if err == nil {
		t.Fatal("expected 400 error, got nil")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok || apiErr.StatusCode != 400 {
		t.Fatalf("expected 400, got %T: %v", err, err)
	}
}
