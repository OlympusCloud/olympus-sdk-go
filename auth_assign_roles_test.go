package olympus

// Tests for AuthService.AssignRoles + ListTeammates wrappers (W12-1 /
// olympus-cloud-gcp#3599 / olympus-sdk-dart#45 fanout). Mirrors the
// canonical Dart contract: POST /platform/users/{id}/roles/assign with
// snake_case body returning void; GET /platform/teammates returning
// OlympusTeammate[].

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// AssignRoles — request shape, success, and error mapping
// ---------------------------------------------------------------------------

func TestAuthService_AssignRoles_RequestShape(t *testing.T) {
	var capturedBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/users/u-1/roles/assign": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &capturedBody); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			jsonResponse(w, 200, map[string]interface{}{
				"ok":       true,
				"audit_id": "aud-3599-0001",
			})
		},
	})

	err := client.Auth().AssignRoles(context.Background(), AssignRolesRequest{
		UserID:       "u-1",
		TenantID:     "t-1",
		GrantScopes:  []string{"commerce.order.write@tenant", "commerce.order.write@tenant"},
		RevokeScopes: []string{"platform.policy.write@tenant"},
		Note:         "rotating ops on-call",
	})
	if err != nil {
		t.Fatalf("AssignRoles failed: %v", err)
	}

	if capturedBody["tenant_id"] != "t-1" {
		t.Errorf("tenant_id: %v", capturedBody["tenant_id"])
	}
	gs, _ := capturedBody["grant_scopes"].([]interface{})
	if len(gs) != 1 || gs[0] != "commerce.order.write@tenant" {
		t.Errorf("grant_scopes (want deduped+sorted): %v", gs)
	}
	rs, _ := capturedBody["revoke_scopes"].([]interface{})
	if len(rs) != 1 || rs[0] != "platform.policy.write@tenant" {
		t.Errorf("revoke_scopes: %v", rs)
	}
	if capturedBody["note"] != "rotating ops on-call" {
		t.Errorf("note: %v", capturedBody["note"])
	}
}

func TestAuthService_AssignRoles_OmitsNoteWhenEmpty(t *testing.T) {
	var capturedBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/users/u-2/roles/assign": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &capturedBody)
			jsonResponse(w, 200, map[string]interface{}{})
		},
	})

	err := client.Auth().AssignRoles(context.Background(), AssignRolesRequest{
		UserID:       "u-2",
		TenantID:     "t-1",
		GrantScopes:  []string{"a.b.c@tenant"},
		RevokeScopes: nil,
	})
	if err != nil {
		t.Fatalf("AssignRoles failed: %v", err)
	}
	if _, ok := capturedBody["note"]; ok {
		t.Errorf("note should be omitted when empty, got: %v", capturedBody["note"])
	}
	rs, _ := capturedBody["revoke_scopes"].([]interface{})
	if len(rs) != 0 {
		t.Errorf("expected empty revoke_scopes array, got %v", rs)
	}
}

func TestAuthService_AssignRoles_400ValidationError(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/users/u-4/roles/assign": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 400, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "ROLES_VALIDATION_ERROR",
					"message": "grant_scopes and revoke_scopes cannot both be empty",
				},
			})
		},
	})

	err := client.Auth().AssignRoles(context.Background(), AssignRolesRequest{
		UserID:       "u-4",
		TenantID:     "t-1",
		GrantScopes:  nil,
		RevokeScopes: nil,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected *OlympusAPIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("status: %d", apiErr.StatusCode)
	}
	if apiErr.Code != "ROLES_VALIDATION_ERROR" {
		t.Errorf("code: %q", apiErr.Code)
	}
}

func TestAuthService_AssignRoles_403Forbidden(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/users/u-5/roles/assign": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 403, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "INSUFFICIENT_PERMISSIONS",
					"message": "caller lacks platform.founder.roles.assign@tenant",
				},
			})
		},
	})

	err := client.Auth().AssignRoles(context.Background(), AssignRolesRequest{
		UserID:      "u-5",
		TenantID:    "t-1",
		GrantScopes: []string{"x@tenant"},
	})
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected *OlympusAPIError, got %T: %v", err, err)
	}
	if !apiErr.IsForbidden() {
		t.Errorf("IsForbidden=false")
	}
	if apiErr.Code != "INSUFFICIENT_PERMISSIONS" {
		t.Errorf("code: %q", apiErr.Code)
	}
}

func TestAuthService_AssignRoles_404NotFound(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/users/missing/roles/assign": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 404, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "USER_NOT_FOUND",
					"message": "user is not a member of this tenant",
				},
			})
		},
	})

	err := client.Auth().AssignRoles(context.Background(), AssignRolesRequest{
		UserID:      "missing",
		TenantID:    "t-1",
		GrantScopes: []string{"x@tenant"},
	})
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected *OlympusAPIError, got %T: %v", err, err)
	}
	if !apiErr.IsNotFound() {
		t.Errorf("IsNotFound=false")
	}
	if apiErr.Code != "USER_NOT_FOUND" {
		t.Errorf("code: %q", apiErr.Code)
	}
}

// ---------------------------------------------------------------------------
// ListTeammates — query, parse, optional filter
// ---------------------------------------------------------------------------

func TestAuthService_ListTeammates_WithTenantID(t *testing.T) {
	var capturedQuery string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/teammates": func(w http.ResponseWriter, r *http.Request) {
			capturedQuery = r.URL.RawQuery
			jsonResponse(w, 200, map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"user_id":         "u-1",
						"display_name":    "Alice",
						"role":            "tenant_admin",
						"assigned_scopes": []interface{}{"commerce.order.write@tenant"},
					},
				},
			})
		},
	})

	out, err := client.Auth().ListTeammates(context.Background(), ListTeammatesOptions{TenantID: "t-1"})
	if err != nil {
		t.Fatalf("ListTeammates failed: %v", err)
	}
	if !strings.Contains(capturedQuery, "tenant_id=t-1") {
		t.Errorf("expected tenant_id=t-1 in query, got %q", capturedQuery)
	}
	if len(out) != 1 {
		t.Fatalf("len: %d", len(out))
	}
	if out[0].UserID != "u-1" || out[0].DisplayName != "Alice" || out[0].Role != "tenant_admin" {
		t.Errorf("unexpected teammate: %+v", out[0])
	}
	if _, ok := out[0].AssignedScopes["commerce.order.write@tenant"]; !ok {
		t.Errorf("missing assigned scope: %+v", out[0].AssignedScopes)
	}
}

func TestAuthService_ListTeammates_OmitsTenantIDWhenEmpty(t *testing.T) {
	var capturedQuery string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/teammates": func(w http.ResponseWriter, r *http.Request) {
			capturedQuery = r.URL.RawQuery
			jsonResponse(w, 200, map[string]interface{}{"data": []interface{}{}})
		},
	})

	_, err := client.Auth().ListTeammates(context.Background(), ListTeammatesOptions{})
	if err != nil {
		t.Fatalf("ListTeammates failed: %v", err)
	}
	if capturedQuery != "" {
		t.Errorf("expected empty query, got %q", capturedQuery)
	}
}

func TestAuthService_ListTeammates_TolerantOfMissingFields(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/teammates": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"user_id": "u-3"},
				},
			})
		},
	})

	out, err := client.Auth().ListTeammates(context.Background(), ListTeammatesOptions{})
	if err != nil {
		t.Fatalf("ListTeammates failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len: %d", len(out))
	}
	if out[0].UserID != "u-3" || out[0].DisplayName != "" || out[0].Role != "" {
		t.Errorf("expected empty defaults, got %+v", out[0])
	}
	if len(out[0].AssignedScopes) != 0 {
		t.Errorf("expected empty assigned_scopes, got %v", out[0].AssignedScopes)
	}
}
