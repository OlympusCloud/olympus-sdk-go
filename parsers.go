package olympus

import (
	"math"
	"time"
)

// --------------------------------------------------------------------------
// Generic helpers
// --------------------------------------------------------------------------

// getString safely extracts a string value from a JSON map.
func getString(data map[string]interface{}, key string) string {
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

// getStringOr tries multiple keys and returns the first non-empty match.
func getStringOr(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := data[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// getInt safely extracts an int from a JSON number.
func getInt(data map[string]interface{}, key string) int {
	switch v := data[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

// getFloat64 safely extracts a float64 from a JSON number.
func getFloat64(data map[string]interface{}, key string) float64 {
	switch v := data[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0.0
}

// getBool safely extracts a bool from a JSON value.
func getBool(data map[string]interface{}, key string) bool {
	if v, ok := data[key].(bool); ok {
		return v
	}
	return false
}

// getBoolPtr returns a *bool or nil if the key is not present.
func getBoolPtr(data map[string]interface{}, key string) *bool {
	if v, ok := data[key].(bool); ok {
		return &v
	}
	return nil
}

// getTime parses an ISO 8601 timestamp string from a JSON map.
func getTime(data map[string]interface{}, key string) *time.Time {
	if v, ok := data[key].(string); ok && v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return &t
		}
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return &t
		}
	}
	return nil
}

// getStringSlice extracts a []string from a JSON array.
func getStringSlice(data map[string]interface{}, key string) []string {
	items, ok := data[key].([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// getMap safely extracts a nested map.
func getMap(data map[string]interface{}, key string) map[string]interface{} {
	if m, ok := data[key].(map[string]interface{}); ok {
		return m
	}
	return nil
}

// getMapStringFloat64 extracts a map[string]float64 from a JSON map.
func getMapStringFloat64(data map[string]interface{}, key string) map[string]float64 {
	raw, ok := data[key].(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]float64, len(raw))
	for k, v := range raw {
		if f, ok := v.(float64); ok {
			result[k] = f
		}
	}
	return result
}

// toFloat64Slice converts a []interface{} of numbers to []float64.
func toFloat64Slice(items []interface{}) []float64 {
	result := make([]float64, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case float64:
			result = append(result, v)
		case int:
			result = append(result, float64(v))
		}
	}
	return result
}

// hashString produces a simple numeric hash for generating trace IDs.
func hashString(s string) int64 {
	var h int64
	for _, c := range s {
		h = h*31 + int64(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// parseSlice extracts a typed slice from a JSON response.
func parseSlice[T any](data map[string]interface{}, key string, parser func(map[string]interface{}) *T) []T {
	items, ok := data[key].([]interface{})
	if !ok {
		return nil
	}
	result := make([]T, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, *parser(m))
		}
	}
	return result
}

// --------------------------------------------------------------------------
// Model parsers
// --------------------------------------------------------------------------

func parseAuthSession(data map[string]interface{}) *AuthSession {
	session := &AuthSession{
		AccessToken:  getString(data, "access_token"),
		TokenType:    getStringOr(data, "token_type"),
		ExpiresIn:    getInt(data, "expires_in"),
		RefreshToken: getString(data, "refresh_token"),
		UserID:       getString(data, "user_id"),
		TenantID:     getString(data, "tenant_id"),
		Roles:        getStringSlice(data, "roles"),
		AppScopes:    getStringSlice(data, "app_scopes"),
	}
	// If the server didn't include app_scopes in the envelope, fall back to
	// the JWT `app_scopes` claim so HasScope works uniformly.
	if len(session.AppScopes) == 0 && session.AccessToken != "" {
		if claims := parseJWTPayload(session.AccessToken); claims != nil {
			if raw, ok := claims["app_scopes"].([]interface{}); ok {
				out := make([]string, 0, len(raw))
				for _, v := range raw {
					if s, ok := v.(string); ok {
						out = append(out, s)
					}
				}
				session.AppScopes = out
			}
		}
	}
	return session
}

func parseUser(data map[string]interface{}) *User {
	return &User{
		ID:        getString(data, "id"),
		Email:     getString(data, "email"),
		Name:      getString(data, "name"),
		Roles:     getStringSlice(data, "roles"),
		TenantID:  getString(data, "tenant_id"),
		Status:    getString(data, "status"),
		CreatedAt: getTime(data, "created_at"),
		UpdatedAt: getTime(data, "updated_at"),
	}
}

func parseAPIKey(data map[string]interface{}) *APIKey {
	return &APIKey{
		ID:        getString(data, "id"),
		Name:      getString(data, "name"),
		Key:       getString(data, "key"),
		Scopes:    getStringSlice(data, "scopes"),
		CreatedAt: getTime(data, "created_at"),
		ExpiresAt: getTime(data, "expires_at"),
	}
}

func parseOrder(data map[string]interface{}) *Order {
	var items []OrderItem
	if rawItems, ok := data["items"].([]interface{}); ok {
		for _, raw := range rawItems {
			if m, ok := raw.(map[string]interface{}); ok {
				items = append(items, *parseOrderItem(m))
			}
		}
	}

	return &Order{
		ID:         getString(data, "id"),
		Status:     getStringOr(data, "status"),
		Items:      items,
		Source:     getString(data, "source"),
		TableID:    getString(data, "table_id"),
		CustomerID: getString(data, "customer_id"),
		Subtotal:   getInt(data, "subtotal"),
		Tax:        getInt(data, "tax"),
		Total:      getInt(data, "total"),
		CreatedAt:  getTime(data, "created_at"),
		UpdatedAt:  getTime(data, "updated_at"),
	}
}

func parseOrderItem(data map[string]interface{}) *OrderItem {
	var modifiers []OrderModifier
	if rawMods, ok := data["modifiers"].([]interface{}); ok {
		for _, raw := range rawMods {
			if m, ok := raw.(map[string]interface{}); ok {
				modifiers = append(modifiers, OrderModifier{
					ID:    getString(m, "id"),
					Name:  getString(m, "name"),
					Price: getInt(m, "price"),
				})
			}
		}
	}

	return &OrderItem{
		CatalogID: getStringOr(data, "catalog_id", "menu_item_id"),
		Qty:       intOr(getInt(data, "qty"), getInt(data, "quantity"), 1),
		Price:     getInt(data, "price"),
		ID:        getString(data, "id"),
		Name:      getString(data, "name"),
		Modifiers: modifiers,
		Notes:     getString(data, "notes"),
	}
}

func parseCatalogItem(data map[string]interface{}) *CatalogItem {
	return &CatalogItem{
		ID:          getString(data, "id"),
		Name:        getString(data, "name"),
		Price:       getInt(data, "price"),
		Description: getString(data, "description"),
		Category:    getString(data, "category"),
		CategoryID:  getString(data, "category_id"),
		ImageURL:    getString(data, "image_url"),
		Available:   getBoolPtr(data, "available"),
		CreatedAt:   getTime(data, "created_at"),
		UpdatedAt:   getTime(data, "updated_at"),
	}
}

func parseAIResponse(data map[string]interface{}) *AIResponse {
	tokensUsed := getInt(data, "tokens_used")
	if tokensUsed == 0 {
		if usage, ok := data["usage"].(map[string]interface{}); ok {
			tokensUsed = getInt(usage, "total_tokens")
		}
	}

	return &AIResponse{
		Content:      getStringOr(data, "content", "response", "text"),
		Model:        getString(data, "model"),
		Tier:         getString(data, "tier"),
		TokensUsed:   tokensUsed,
		FinishReason: getString(data, "finish_reason"),
		RequestID:    getString(data, "request_id"),
	}
}

func parseAgentResult(data map[string]interface{}) *AgentResult {
	var steps []AgentStep
	if rawSteps, ok := data["steps"].([]interface{}); ok {
		for _, raw := range rawSteps {
			if m, ok := raw.(map[string]interface{}); ok {
				steps = append(steps, AgentStep{
					Action:      getString(m, "action"),
					Observation: getString(m, "observation"),
					Thought:     getString(m, "thought"),
				})
			}
		}
	}

	return &AgentResult{
		Output:     getStringOr(data, "output", "result"),
		AgentName:  getString(data, "agent_name"),
		Steps:      steps,
		TokensUsed: getInt(data, "tokens_used"),
		RequestID:  getString(data, "request_id"),
	}
}

func parseAgentTask(data map[string]interface{}) *AgentTask {
	return &AgentTask{
		ID:          getStringOr(data, "id", "task_id"),
		Status:      getStringOr(data, "status"),
		AgentName:   getStringOr(data, "agent_name", "agent"),
		Task:        getString(data, "task"),
		Result:      getString(data, "result"),
		Error:       getString(data, "error"),
		CreatedAt:   getTime(data, "created_at"),
		CompletedAt: getTime(data, "completed_at"),
	}
}

func parseClassification(data map[string]interface{}) *Classification {
	return &Classification{
		Label:      getStringOr(data, "label", "category"),
		Confidence: math.Max(getFloat64(data, "confidence"), getFloat64(data, "score")),
		Scores:     getMapStringFloat64(data, "scores"),
	}
}

func parseSentimentResult(data map[string]interface{}) *SentimentResult {
	var aspects []AspectSentiment
	if rawAspects, ok := data["aspects"].([]interface{}); ok {
		for _, raw := range rawAspects {
			if m, ok := raw.(map[string]interface{}); ok {
				aspects = append(aspects, AspectSentiment{
					Aspect:    getString(m, "aspect"),
					Sentiment: getString(m, "sentiment"),
					Score:     getFloat64(m, "score"),
				})
			}
		}
	}

	return &SentimentResult{
		Sentiment: getStringOr(data, "sentiment"),
		Score:     getFloat64(data, "score"),
		Aspects:   aspects,
	}
}

func parsePayment(data map[string]interface{}) *Payment {
	return &Payment{
		ID:                    getStringOr(data, "id", "payment_id"),
		Status:                getStringOr(data, "status"),
		OrderID:               getString(data, "order_id"),
		Amount:                getInt(data, "amount"),
		Currency:              getString(data, "currency"),
		Method:                getStringOr(data, "method", "payment_method"),
		StripePaymentIntentID: getString(data, "stripe_payment_intent_id"),
		CreatedAt:             getTime(data, "created_at"),
	}
}

func parseRefund(data map[string]interface{}) *Refund {
	return &Refund{
		ID:        getStringOr(data, "id", "refund_id"),
		PaymentID: getString(data, "payment_id"),
		Status:    getStringOr(data, "status"),
		Amount:    getInt(data, "amount"),
		Reason:    getString(data, "reason"),
		CreatedAt: getTime(data, "created_at"),
	}
}

func parseBalance(data map[string]interface{}) *Balance {
	return &Balance{
		Available: getInt(data, "available"),
		Pending:   getInt(data, "pending"),
		Currency:  getString(data, "currency"),
	}
}

func parsePayout(data map[string]interface{}) *Payout {
	return &Payout{
		ID:          getStringOr(data, "id", "payout_id"),
		Status:      getStringOr(data, "status"),
		Amount:      getInt(data, "amount"),
		Currency:    getString(data, "currency"),
		Destination: getString(data, "destination"),
		Method:      getString(data, "method"),
		ArrivalDate: getTime(data, "arrival_date"),
		CreatedAt:   getTime(data, "created_at"),
	}
}

func parseTerminalReader(data map[string]interface{}) *TerminalReader {
	return &TerminalReader{
		ID:           getString(data, "id"),
		DeviceType:   getString(data, "device_type"),
		Label:        getString(data, "label"),
		LocationID:   getStringOr(data, "location", "location_id"),
		SerialNumber: getString(data, "serial_number"),
		Status:       getString(data, "status"),
		IPAddress:    getString(data, "ip_address"),
	}
}

func parseTerminalPayment(data map[string]interface{}) *TerminalPayment {
	return &TerminalPayment{
		ID:              getString(data, "id"),
		Status:          getStringOr(data, "status"),
		Amount:          getInt(data, "amount"),
		Currency:        getString(data, "currency"),
		ReaderID:        getString(data, "reader_id"),
		PaymentIntentID: getStringOr(data, "payment_intent_id", "payment_intent"),
		CreatedAt:       getTime(data, "created_at"),
	}
}

func parsePlan(data map[string]interface{}) *Plan {
	return &Plan{
		ID:           getStringOr(data, "id", "plan_id"),
		Name:         getString(data, "name"),
		Tier:         getString(data, "tier"),
		MonthlyPrice: getInt(data, "monthly_price"),
		AnnualPrice:  getInt(data, "annual_price"),
		MaxLocations: getInt(data, "max_locations"),
		MaxAgents:    getInt(data, "max_agents"),
		AICredits:    getInt(data, "ai_credits"),
		VoiceMinutes: getInt(data, "voice_minutes"),
		Features:     getStringSlice(data, "features"),
		Status:       getString(data, "status"),
	}
}

func parseUsageReport(data map[string]interface{}) *UsageReport {
	return &UsageReport{
		Period:            getString(data, "period"),
		AICreditsUsed:     getInt(data, "ai_credits_used"),
		AICreditsLimit:    getInt(data, "ai_credits_limit"),
		VoiceMinutesUsed:  getInt(data, "voice_minutes_used"),
		VoiceMinutesLimit: getInt(data, "voice_minutes_limit"),
		StorageUsedMB:     getInt(data, "storage_used_mb"),
		StorageLimitMB:    getInt(data, "storage_limit_mb"),
		APICallsCount:     getInt(data, "api_calls_count"),
		LocationCount:     getInt(data, "location_count"),
		AgentCount:        getInt(data, "agent_count"),
	}
}

func parseInvoice(data map[string]interface{}) *Invoice {
	var lineItems []InvoiceLineItem
	if rawItems, ok := data["line_items"].([]interface{}); ok {
		for _, raw := range rawItems {
			if m, ok := raw.(map[string]interface{}); ok {
				lineItems = append(lineItems, InvoiceLineItem{
					Description: getString(m, "description"),
					Amount:      getInt(m, "amount"),
					Quantity:    getInt(m, "quantity"),
				})
			}
		}
	}

	return &Invoice{
		ID:          getStringOr(data, "id", "invoice_id"),
		Status:      getString(data, "status"),
		Amount:      getInt(data, "amount"),
		Currency:    getString(data, "currency"),
		PeriodStart: getTime(data, "period_start"),
		PeriodEnd:   getTime(data, "period_end"),
		PaidAt:      getTime(data, "paid_at"),
		PDFURL:      getString(data, "pdf_url"),
		LineItems:   lineItems,
	}
}

func parseMarketplaceApp(data map[string]interface{}) *MarketplaceApp {
	return &MarketplaceApp{
		ID:           getString(data, "id"),
		Name:         getString(data, "name"),
		Description:  getString(data, "description"),
		Category:     getString(data, "category"),
		Industry:     getString(data, "industry"),
		Developer:    getString(data, "developer"),
		IconURL:      getString(data, "icon_url"),
		Rating:       getFloat64(data, "rating"),
		InstallCount: getInt(data, "install_count"),
		Pricing:      getString(data, "pricing"),
		CreatedAt:    getTime(data, "created_at"),
	}
}

func parseInstallation(data map[string]interface{}) *Installation {
	return &Installation{
		ID:          getStringOr(data, "id", "installation_id"),
		AppID:       getString(data, "app_id"),
		AppName:     getString(data, "app_name"),
		Status:      getString(data, "status"),
		Config:      getMap(data, "config"),
		InstalledAt: getTime(data, "installed_at"),
	}
}

func parseDevice(data map[string]interface{}) *Device {
	return &Device{
		ID:         getStringOr(data, "id", "device_id"),
		Name:       getString(data, "name"),
		Status:     getString(data, "status"),
		Profile:    getString(data, "profile"),
		Platform:   getString(data, "platform"),
		OSVersion:  getString(data, "os_version"),
		AppVersion: getString(data, "app_version"),
		LocationID: getString(data, "location_id"),
		LastSeen:   getTime(data, "last_seen"),
		EnrolledAt: getTime(data, "enrolled_at"),
	}
}

func parseWebhookRegistration(data map[string]interface{}) *WebhookRegistration {
	return &WebhookRegistration{
		ID:        getString(data, "id"),
		URL:       getString(data, "url"),
		Events:    getStringSlice(data, "events"),
		Secret:    getString(data, "secret"),
		CreatedAt: getTime(data, "created_at"),
	}
}

func parseSearchResult(data map[string]interface{}) *SearchResult {
	return &SearchResult{
		ID:       getString(data, "id"),
		Score:    getFloat64(data, "score"),
		Content:  getString(data, "content"),
		Metadata: getMap(data, "metadata"),
	}
}

func parsePolicyResult(data map[string]interface{}) *PolicyResult {
	return &PolicyResult{
		Allowed: getBool(data, "allowed"),
		Value:   data["value"],
		Reason:  getString(data, "reason"),
	}
}

func parseFeatureFlag(data map[string]interface{}) *FeatureFlag {
	return &FeatureFlag{
		Key:     getString(data, "key"),
		Enabled: getBool(data, "enabled"),
		Value:   data["value"],
	}
}

func parseNotification(data map[string]interface{}) *Notification {
	return &Notification{
		ID:        getString(data, "id"),
		Type:      getString(data, "type"),
		Title:     getString(data, "title"),
		Body:      getString(data, "body"),
		Read:      getBool(data, "read"),
		Data:      getMap(data, "data"),
		CreatedAt: getTime(data, "created_at"),
	}
}

func parsePagination(data map[string]interface{}) Pagination {
	if p, ok := data["pagination"].(map[string]interface{}); ok {
		return Pagination{
			Page:       intOr(getInt(p, "page"), 0, 1),
			PerPage:    intOr(getInt(p, "per_page"), 0, 20),
			Total:      getInt(p, "total"),
			TotalPages: getInt(p, "total_pages"),
		}
	}
	return Pagination{
		Page:       intOr(getInt(data, "page"), 0, 1),
		PerPage:    intOr(getInt(data, "per_page"), 0, 20),
		Total:      getInt(data, "total"),
		TotalPages: getInt(data, "total_pages"),
	}
}

// intOr returns the first non-zero value, or the last one.
func intOr(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	if len(vals) > 0 {
		return vals[len(vals)-1]
	}
	return 0
}
