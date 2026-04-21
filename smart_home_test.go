package olympus

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSmartHome_ListPlatforms(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/smart-home/platforms": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method: %s", r.Method)
			}
			jsonResponse(w, 200, map[string]interface{}{
				"platforms": []map[string]interface{}{
					{"id": "p1", "name": "Hue"},
					{"id": "p2", "name": "SmartThings"},
				},
			})
		},
	})
	out, err := client.SmartHome().ListPlatforms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0]["id"] != "p1" {
		t.Errorf("platforms parse: %v", out)
	}
}

func TestSmartHome_ListDevices_QueryFilters(t *testing.T) {
	var gotQuery string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/smart-home/devices": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			jsonResponse(w, 200, map[string]interface{}{"devices": []interface{}{}})
		},
	})
	_, err := client.SmartHome().ListDevices(context.Background(), &ListDevicesOptions{
		PlatformID: "hue", RoomID: "living",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(gotQuery, "platform_id=hue") || !contains(gotQuery, "room_id=living") {
		t.Errorf("missing filters: %q", gotQuery)
	}
}

func TestSmartHome_ControlDevice(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/smart-home/devices/dev-1/control": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			jsonResponse(w, 200, map[string]interface{}{"ok": true})
		},
	})
	out, err := client.SmartHome().ControlDevice(context.Background(), "dev-1", map[string]interface{}{
		"power":      "on",
		"brightness": 75,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/smart-home/devices/dev-1/control" {
		t.Errorf("path: %s", gotPath)
	}
	if gotBody["power"] != "on" {
		t.Errorf("body.power: %v", gotBody["power"])
	}
	if out["ok"] != true {
		t.Errorf("response: %v", out)
	}
}

func TestSmartHome_DeleteScene(t *testing.T) {
	var gotMethod, gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/smart-home/scenes/scene-1": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(204)
		},
	})
	if err := client.SmartHome().DeleteScene(context.Background(), "scene-1"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/smart-home/scenes/scene-1" {
		t.Errorf("delete scene route: %s %s", gotMethod, gotPath)
	}
}

func TestSmartHome_ActivateScene(t *testing.T) {
	var gotMethod, gotPath string
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/smart-home/scenes/scene-1/activate": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			jsonResponse(w, 200, map[string]interface{}{"activated": true})
		},
	})
	out, err := client.SmartHome().ActivateScene(context.Background(), "scene-1")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/smart-home/scenes/scene-1/activate" {
		t.Errorf("activate scene route: %s %s", gotMethod, gotPath)
	}
	if out["activated"] != true {
		t.Errorf("response: %v", out)
	}
}

func TestSmartHome_ListAutomations_FallsBackToData(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/smart-home/automations": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "a1"},
				},
			})
		},
	})
	out, err := client.SmartHome().ListAutomations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0]["id"] != "a1" {
		t.Errorf("automations parse via data fallback: %v", out)
	}
}

func TestSmartHome_GetDevice_PropagatesNotFound(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/smart-home/devices/missing": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 404, map[string]interface{}{
				"error": map[string]interface{}{"code": "NOT_FOUND", "message": "no such device"},
			})
		},
	})
	_, err := client.SmartHome().GetDevice(context.Background(), "missing")
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

// contains is a tiny helper to avoid importing strings in only one test.
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
