package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// DevicesService handles Mobile Device Management (MDM): enrollment, kiosk mode,
// updates, and wipe.
//
// Routes: /auth/devices/*, /platform/device-policies/*, /diagnostics/*.
type DevicesService struct {
	http *httpClient
}

// Enroll enrolls a device with a profile. Profile specifies the device role
// (e.g., "kiosk", "pos_terminal", "kds", "signage").
func (s *DevicesService) Enroll(ctx context.Context, deviceID, profile string) (*Device, error) {
	resp, err := s.http.post(ctx, "/auth/devices/register", map[string]interface{}{
		"device_id": deviceID,
		"profile":   profile,
	})
	if err != nil {
		return nil, err
	}
	return parseDevice(resp), nil
}

// SetKioskMode locks a device to a specific application.
func (s *DevicesService) SetKioskMode(ctx context.Context, deviceID, appID string) error {
	_, err := s.http.post(ctx, fmt.Sprintf("/platform/device-policies/%s/kiosk", deviceID), map[string]interface{}{
		"app_id":  appID,
		"enabled": true,
	})
	return err
}

// PushUpdate pushes an OTA update to a device group.
func (s *DevicesService) PushUpdate(ctx context.Context, deviceGroupID, version string) error {
	_, err := s.http.post(ctx, "/platform/device-policies/updates", map[string]interface{}{
		"device_group_id": deviceGroupID,
		"target_version":  version,
	})
	return err
}

// Wipe remote-wipes a device (factory reset).
func (s *DevicesService) Wipe(ctx context.Context, deviceID string) error {
	_, err := s.http.post(ctx, fmt.Sprintf("/platform/device-policies/%s/wipe", deviceID), nil)
	return err
}

// ListDevices lists enrolled devices for the tenant.
func (s *DevicesService) ListDevices(ctx context.Context, locationID string) ([]Device, error) {
	q := url.Values{}
	if locationID != "" {
		q.Set("location_id", locationID)
	}

	resp, err := s.http.get(ctx, "/diagnostics/devices", q)
	if err != nil {
		return nil, err
	}

	devices := parseSlice(resp, "devices", parseDevice)
	if len(devices) == 0 {
		devices = parseSlice(resp, "data", parseDevice)
	}
	return devices, nil
}

// GetDevice returns device details by ID.
func (s *DevicesService) GetDevice(ctx context.Context, deviceID string) (*Device, error) {
	resp, err := s.http.get(ctx, fmt.Sprintf("/diagnostics/devices/%s", deviceID), nil)
	if err != nil {
		return nil, err
	}
	return parseDevice(resp), nil
}
