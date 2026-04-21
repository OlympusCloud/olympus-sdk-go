package olympus

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestVoiceOrders_Create(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-orders": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{"order_id": "vo-1"})
		},
	})
	out, err := client.VoiceOrders().Create(context.Background(), CreateVoiceOrderRequest{
		LocationID: "loc-1",
		Items: []VoiceOrderItem{
			{MenuItemID: "m1", Name: "Margherita", Quantity: 1, UnitPrice: 14.99},
		},
		Fulfillment: "pickup",
		CallerPhone: "+15551234567",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/voice-orders" {
		t.Errorf("path: %s", gotPath)
	}
	if gotBody["location_id"] != "loc-1" {
		t.Errorf("location_id: %v", gotBody["location_id"])
	}
	if gotBody["fulfillment"] != "pickup" {
		t.Errorf("fulfillment: %v", gotBody["fulfillment"])
	}
	if out["order_id"] != "vo-1" {
		t.Errorf("response: %v", out)
	}
}

func TestVoiceOrders_Create_OmitsEmptyOptional(t *testing.T) {
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-orders": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 201, map[string]interface{}{})
		},
	})
	_, err := client.VoiceOrders().Create(context.Background(), CreateVoiceOrderRequest{
		LocationID: "loc-1",
		Items:      []VoiceOrderItem{{MenuItemID: "m1", Quantity: 1, UnitPrice: 1.99}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"fulfillment", "delivery_address", "payment_method",
		"caller_phone", "caller_name", "call_sid", "agent_id", "metadata"} {
		if _, ok := gotBody[k]; ok {
			t.Errorf("unexpected field %s in body: %v", k, gotBody)
		}
	}
}

func TestVoiceOrders_Get(t *testing.T) {
	var gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-orders/vo-1": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			jsonResponse(w, 200, map[string]interface{}{"order_id": "vo-1"})
		},
	})
	out, err := client.VoiceOrders().Get(context.Background(), "vo-1")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/voice-orders/vo-1" {
		t.Errorf("path: %s", gotPath)
	}
	if out["order_id"] != "vo-1" {
		t.Errorf("response: %v", out)
	}
}

func TestVoiceOrders_List_Filters(t *testing.T) {
	var gotQuery string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-orders": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			jsonResponse(w, 200, map[string]interface{}{"orders": []interface{}{}})
		},
	})
	if _, err := client.VoiceOrders().List(context.Background(), &ListVoiceOrdersOptions{
		CallerPhone: "+15551234567", Status: "confirmed", LocationID: "loc-1", Limit: 10,
	}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"caller_phone=", "status=confirmed", "location_id=loc-1", "limit=10",
	} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query missing %q: %s", want, gotQuery)
		}
	}
}

func TestVoiceOrders_PushToPOS(t *testing.T) {
	var gotMethod, gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-orders/vo-1/push-pos": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			jsonResponse(w, 200, map[string]interface{}{"status": "pushed"})
		},
	})
	out, err := client.VoiceOrders().PushToPOS(context.Background(), "vo-1")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/voice-orders/vo-1/push-pos" {
		t.Errorf("push-pos route: %s %s", gotMethod, gotPath)
	}
	if out["status"] != "pushed" {
		t.Errorf("response: %v", out)
	}
}

func TestVoiceOrders_Get_PropagatesNotFound(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/voice-orders/missing": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 404, map[string]interface{}{
				"error": map[string]interface{}{"code": "NOT_FOUND", "message": "no order"},
			})
		},
	})
	_, err := client.VoiceOrders().Get(context.Background(), "missing")
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
