// Package olympus provides a typed Go SDK for the Olympus Cloud Platform API.
//
// Create a single client per application:
//
//	client := olympus.NewClient(olympus.Config{
//	    AppID:  "com.my-restaurant",
//	    APIKey: "oc_live_...",
//	})
//
//	// Authenticate
//	session, err := client.Auth().Login(ctx, "user@example.com", "password")
//
//	// Create an order
//	order, err := client.Commerce().CreateOrder(ctx, olympus.CreateOrderRequest{
//	    Items:  []olympus.OrderItem{{CatalogID: "burger-01", Qty: 2, Price: 1299}},
//	    Source: "pos",
//	})
//
//	// Ask AI
//	resp, err := client.AI().Query(ctx, "What sold best this week?", nil)
package olympus

// OlympusClient is the main entry point for the Olympus Cloud SDK.
// It provides typed access to 19 platform services.
type OlympusClient struct {
	config *Config
	http   *httpClient

	auth        *AuthService
	commerce    *CommerceService
	ai          *AIService
	pay         *PayService
	notify      *NotifyService
	events      *EventsService
	data        *DataService
	storage     *StorageService
	marketplace *MarketplaceService
	billing     *BillingService
	gating      *GatingService
	devices     *DevicesService
	observe     *ObserveService
	creator     *CreatorService
	platform    *PlatformService
	developer   *DeveloperService
	business    *BusinessService
	maximus     *MaximusService
	pos         *POSService
}

// NewClient creates a new Olympus Cloud SDK client with the given configuration.
func NewClient(cfg Config) *OlympusClient {
	h := newHTTPClient(&cfg)
	return &OlympusClient{
		config: &cfg,
		http:   h,
	}
}

// Auth returns the authentication and user management service.
func (c *OlympusClient) Auth() *AuthService {
	if c.auth == nil {
		c.auth = &AuthService{http: c.http}
	}
	return c.auth
}

// Commerce returns the orders, catalog, and commerce operations service.
func (c *OlympusClient) Commerce() *CommerceService {
	if c.commerce == nil {
		c.commerce = &CommerceService{http: c.http}
	}
	return c.commerce
}

// AI returns the AI inference, agents, embeddings, and NLP service.
func (c *OlympusClient) AI() *AIService {
	if c.ai == nil {
		c.ai = &AIService{http: c.http}
	}
	return c.ai
}

// Pay returns the payments, refunds, balance, and payouts service.
func (c *OlympusClient) Pay() *PayService {
	if c.pay == nil {
		c.pay = &PayService{http: c.http}
	}
	return c.pay
}

// Notify returns the push, SMS, email, and notification service.
func (c *OlympusClient) Notify() *NotifyService {
	if c.notify == nil {
		c.notify = &NotifyService{http: c.http}
	}
	return c.notify
}

// Events returns the real-time events and webhook management service.
func (c *OlympusClient) Events() *EventsService {
	if c.events == nil {
		c.events = &EventsService{http: c.http}
	}
	return c.events
}

// Data returns the data query, CRUD, and search service.
func (c *OlympusClient) Data() *DataService {
	if c.data == nil {
		c.data = &DataService{http: c.http}
	}
	return c.data
}

// Storage returns the file storage service.
func (c *OlympusClient) Storage() *StorageService {
	if c.storage == nil {
		c.storage = &StorageService{http: c.http}
	}
	return c.storage
}

// Marketplace returns the app marketplace and installation service.
func (c *OlympusClient) Marketplace() *MarketplaceService {
	if c.marketplace == nil {
		c.marketplace = &MarketplaceService{http: c.http}
	}
	return c.marketplace
}

// Billing returns the subscription billing and usage service.
func (c *OlympusClient) Billing() *BillingService {
	if c.billing == nil {
		c.billing = &BillingService{http: c.http}
	}
	return c.billing
}

// Gating returns the feature gating and policy evaluation service.
func (c *OlympusClient) Gating() *GatingService {
	if c.gating == nil {
		c.gating = &GatingService{http: c.http}
	}
	return c.gating
}

// Devices returns the device management (MDM) service.
func (c *OlympusClient) Devices() *DevicesService {
	if c.devices == nil {
		c.devices = &DevicesService{http: c.http}
	}
	return c.devices
}

// Observe returns the client-side observability service.
func (c *OlympusClient) Observe() *ObserveService {
	if c.observe == nil {
		c.observe = &ObserveService{http: c.http}
	}
	return c.observe
}

// Creator returns the creator platform service (posts, media, AI content).
// v0.3.0 — Issue #2839
func (c *OlympusClient) Creator() *CreatorService {
	if c.creator == nil {
		c.creator = &CreatorService{http: c.http}
	}
	return c.creator
}

// Platform returns the tenant lifecycle service (signup/cleanup workflows).
// v0.3.0 — Issues #2845, #2846
func (c *OlympusClient) Platform() *PlatformService {
	if c.platform == nil {
		c.platform = &PlatformService{http: c.http}
	}
	return c.platform
}

// Developer returns the developer platform service (API keys, DevBox, deploys).
// v0.3.0 — Issues #2834, #2835, #2828, #2829
func (c *OlympusClient) Developer() *DeveloperService {
	if c.developer == nil {
		c.developer = &DeveloperService{http: c.http}
	}
	return c.developer
}

// Business returns the business data access service (revenue, staff, insights).
// v0.3.0 — Issue #2570
func (c *OlympusClient) Business() *BusinessService {
	if c.business == nil {
		c.business = &BusinessService{http: c.http}
	}
	return c.business
}

// Maximus returns the consumer AI assistant service (voice, calendar, email).
// v0.3.0 — Issues #2567, #2568, #2571
func (c *OlympusClient) Maximus() *MaximusService {
	if c.maximus == nil {
		c.maximus = &MaximusService{http: c.http}
	}
	return c.maximus
}

// POS returns the POS voice order integration service.
// v0.3.0 — Issue #2453
func (c *OlympusClient) POS() *POSService {
	if c.pos == nil {
		c.pos = &POSService{http: c.http}
	}
	return c.pos
}

// Config returns the active SDK configuration.
func (c *OlympusClient) Config() *Config {
	return c.config
}

// HTTPClient returns the underlying HTTP client for advanced usage.
func (c *OlympusClient) HTTPClient() *httpClient {
	return c.http
}
