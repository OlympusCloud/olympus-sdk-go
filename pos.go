package olympus

import "context"

// POSService wraps POS voice order integration endpoints. Supports
// Square, Toast, and Clover POS systems (auto-detected from tenant).
//
// v0.3.0 — Issue #2453
type POSService struct {
	http *httpClient
}

// SubmitVoiceOrder submits a voice-parsed order to the tenant's POS system.
func (s *POSService) SubmitVoiceOrder(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/pos/voice-order", body)
}

// SyncMenu triggers a menu sync from POS to voice AI knowledge base.
func (s *POSService) SyncMenu(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	return s.http.post(ctx, "/pos/"+tenantID+"/sync-menu", nil)
}

// GetOrderStatus returns the status of a voice-submitted order.
func (s *POSService) GetOrderStatus(ctx context.Context, orderID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/pos/voice-orders/"+orderID, nil)
}
