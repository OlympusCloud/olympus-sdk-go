package olympus

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSMS_Send_VoicePlatform(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice/sms/send": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{"message_id": "m-1"})
		},
	})
	out, err := client.SMS().Send(context.Background(), SendRequest{
		ConfigID: "cfg-1", To: "+15551234567", Body: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/voice/sms/send" {
		t.Errorf("path: %s", gotPath)
	}
	if gotBody["config_id"] != "cfg-1" || gotBody["to"] != "+15551234567" || gotBody["body"] != "hello" {
		t.Errorf("body: %v", gotBody)
	}
	if out["message_id"] != "m-1" {
		t.Errorf("message_id missing: %v", out)
	}
}

func TestSMS_GetConversations_PaginationQuery(t *testing.T) {
	var gotQuery string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice/sms/conversations/+15551234567": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			jsonResponse(w, 200, map[string]interface{}{
				"conversations": []map[string]interface{}{{"id": "c1"}},
			})
		},
	})
	out, err := client.SMS().GetConversations(context.Background(), "+15551234567",
		&GetConversationsOptions{Limit: 25, Offset: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(gotQuery, "limit=25") || !contains(gotQuery, "offset=50") {
		t.Errorf("query missing pagination: %q", gotQuery)
	}
	if len(out) != 1 || out[0]["id"] != "c1" {
		t.Errorf("conversations parse: %v", out)
	}
}

func TestSMS_SendViaCpaas_WithoutWebhook(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/cpaas/messages/sms": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{"id": "msg-1"})
		},
	})
	_, err := client.SMS().SendViaCpaas(context.Background(), SendViaCpaasRequest{
		From: "+15550001111", To: "+15550002222", Body: "hi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["from"] != "+15550001111" || gotBody["to"] != "+15550002222" || gotBody["body"] != "hi" {
		t.Errorf("body: %v", gotBody)
	}
	if _, ok := gotBody["webhook_url"]; ok {
		t.Errorf("webhook_url should be omitted when empty: %v", gotBody)
	}
}

func TestSMS_SendViaCpaas_WithWebhook(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/cpaas/messages/sms": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{})
		},
	})
	_, err := client.SMS().SendViaCpaas(context.Background(), SendViaCpaasRequest{
		From: "+15550001111", To: "+15550002222", Body: "hi",
		WebhookURL: "https://hooks/me",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["webhook_url"] != "https://hooks/me" {
		t.Errorf("webhook_url not sent: %v", gotBody)
	}
}

func TestSMS_GetStatus(t *testing.T) {
	var gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/cpaas/messages/msg-1": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			jsonResponse(w, 200, map[string]interface{}{
				"id":     "msg-1",
				"status": "delivered",
			})
		},
	})
	out, err := client.SMS().GetStatus(context.Background(), "msg-1")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/cpaas/messages/msg-1" {
		t.Errorf("path: %s", gotPath)
	}
	if out["status"] != "delivered" {
		t.Errorf("status: %v", out["status"])
	}
}

func TestSMS_GetStatus_PropagatesError(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/cpaas/messages/missing": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 404, map[string]interface{}{
				"error": map[string]interface{}{"code": "NOT_FOUND", "message": "gone"},
			})
		},
	})
	_, err := client.SMS().GetStatus(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*OlympusAPIError)
	if !ok {
		t.Fatalf("expected OlympusAPIError, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Errorf("expected 404, got %d", apiErr.StatusCode)
	}
}
