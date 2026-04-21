package olympus

import (
	"context"
	"encoding/json"
	"net/http"
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
