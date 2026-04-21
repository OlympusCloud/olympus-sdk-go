package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// SmartHomeService is the smart-home integration surface — platforms,
// devices, rooms, scenes, automations, and device control.
//
// Routes: /smart-home/*.
type SmartHomeService struct {
	http *httpClient
}

// ---------------------------------------------------------------------------
// Platforms
// ---------------------------------------------------------------------------

// ListPlatforms returns connected smart-home platforms (Hue, SmartThings,
// HomeKit, ...).
func (s *SmartHomeService) ListPlatforms(ctx context.Context) ([]map[string]interface{}, error) {
	raw, err := s.http.get(ctx, "/smart-home/platforms", nil)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "platforms"), nil
}

// ---------------------------------------------------------------------------
// Devices
// ---------------------------------------------------------------------------

// ListDevicesOptions filters ListDevices.
type ListDevicesOptions struct {
	PlatformID string
	RoomID     string
}

// ListDevices returns smart-home devices across connected platforms.
func (s *SmartHomeService) ListDevices(ctx context.Context, opts *ListDevicesOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.PlatformID != "" {
			q.Set("platform_id", opts.PlatformID)
		}
		if opts.RoomID != "" {
			q.Set("room_id", opts.RoomID)
		}
	}
	raw, err := s.http.get(ctx, "/smart-home/devices", q)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "devices"), nil
}

// GetDevice returns details for a single smart-home device.
func (s *SmartHomeService) GetDevice(ctx context.Context, deviceID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/smart-home/devices/%s", deviceID), nil)
}

// ControlDevice sends a control command to a device (e.g. on/off, brightness,
// color). The command shape is device-driver specific.
func (s *SmartHomeService) ControlDevice(ctx context.Context, deviceID string, command map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/smart-home/devices/%s/control", deviceID), command)
}

// ---------------------------------------------------------------------------
// Rooms
// ---------------------------------------------------------------------------

// ListRooms returns rooms with their associated devices.
func (s *SmartHomeService) ListRooms(ctx context.Context) ([]map[string]interface{}, error) {
	raw, err := s.http.get(ctx, "/smart-home/rooms", nil)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "rooms"), nil
}

// ---------------------------------------------------------------------------
// Scenes (v0.3.0 — Issue #2569)
// ---------------------------------------------------------------------------

// ListScenes returns automation scenes (e.g. "Good morning", "Movie night").
func (s *SmartHomeService) ListScenes(ctx context.Context) ([]map[string]interface{}, error) {
	raw, err := s.http.get(ctx, "/smart-home/scenes", nil)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "scenes"), nil
}

// ActivateScene activates a scene by ID.
func (s *SmartHomeService) ActivateScene(ctx context.Context, sceneID string) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/smart-home/scenes/%s/activate", sceneID), nil)
}

// CreateScene creates a new scene with devices and actions.
func (s *SmartHomeService) CreateScene(ctx context.Context, scene map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/smart-home/scenes", scene)
}

// DeleteScene deletes a scene.
func (s *SmartHomeService) DeleteScene(ctx context.Context, sceneID string) error {
	return s.http.del(ctx, fmt.Sprintf("/smart-home/scenes/%s", sceneID))
}

// ---------------------------------------------------------------------------
// Automations
// ---------------------------------------------------------------------------

// ListAutomations returns automation rules (trigger-action).
func (s *SmartHomeService) ListAutomations(ctx context.Context) ([]map[string]interface{}, error) {
	raw, err := s.http.get(ctx, "/smart-home/automations", nil)
	if err != nil {
		return nil, err
	}
	return extractList(raw, "automations"), nil
}

// CreateAutomation creates a new automation rule.
func (s *SmartHomeService) CreateAutomation(ctx context.Context, automation map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/smart-home/automations", automation)
}

// DeleteAutomation deletes an automation rule.
func (s *SmartHomeService) DeleteAutomation(ctx context.Context, automationID string) error {
	return s.http.del(ctx, fmt.Sprintf("/smart-home/automations/%s", automationID))
}

// Helper extractList is defined in admin_ether.go and reused here.
