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

import "sync"

// OlympusClient is the main entry point for the Olympus Cloud SDK.
// It provides typed access to 22 platform services.
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
	maximus            *MaximusService
	pos                *POSService
	agentWorkflows     *AgentWorkflowsService
	enterpriseContext  *EnterpriseContextService
	messages           *MessagesService
	voiceOrders        *VoiceOrdersService
	adminEther         *AdminEtherService
	adminCpaas         *AdminCpaasService
	adminBilling       *AdminBillingService
	adminGating        *AdminGatingService
	tuning             *TuningService
	voice              *VoiceService
	connect            *ConnectService
	consent            *ConsentService
	governance         *GovernanceService

	// Cached decoded JWT claims (lazy; invalidated on token change).
	// Protected by cacheMu since *OlympusClient is shared across goroutines.
	cacheMu              sync.RWMutex
	cachedClaims         map[string]interface{}
	cachedClaimsForToken string
	cachedBitset         []byte
	cachedBitsetForToken string
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

// AgentWorkflows returns the AI Agent Workflow Orchestration service (#2915).
// Provides tenant-scoped multi-agent DAG pipelines with cron/event triggers,
// capability routing, and billing. Distinct from marketplace workflows.
func (c *OlympusClient) AgentWorkflows() *AgentWorkflowsService {
	if c.agentWorkflows == nil {
		c.agentWorkflows = &AgentWorkflowsService{http: c.http}
	}
	return c.agentWorkflows
}

// EnterpriseContext returns the Company 360 context service for AI agents.
// Issues #2993
func (c *OlympusClient) EnterpriseContext() *EnterpriseContextService {
	if c.enterpriseContext == nil {
		c.enterpriseContext = &EnterpriseContextService{http: c.http}
	}
	return c.enterpriseContext
}

// Messages returns the message queue and department routing service.
// Issue #2997
func (c *OlympusClient) Messages() *MessagesService {
	if c.messages == nil {
		c.messages = &MessagesService{http: c.http}
	}
	return c.messages
}

// VoiceOrders returns the voice order placement service.
// Issue #2999
func (c *OlympusClient) VoiceOrders() *VoiceOrdersService {
	if c.voiceOrders == nil {
		c.voiceOrders = &VoiceOrdersService{http: c.http}
	}
	return c.voiceOrders
}

// AdminEther returns the Ether AI model catalog admin service.
// Provides CRUD for models and tiers, plus hot-reload of the catalog cache.
func (c *OlympusClient) AdminEther() *AdminEtherService {
	if c.adminEther == nil {
		c.adminEther = &AdminEtherService{http: c.http}
	}
	return c.adminEther
}

// AdminCpaas returns the CPaaS provider configuration and health admin service.
// Controls Telnyx-primary / Twilio-fallback routing and circuit-breaker health.
func (c *OlympusClient) AdminCpaas() *AdminCpaasService {
	if c.adminCpaas == nil {
		c.adminCpaas = &AdminCpaasService{http: c.http}
	}
	return c.adminCpaas
}

// AdminBilling returns the billing plan catalog and usage metering admin service.
// Manages the global plan catalog, add-ons, minute packs, and usage recording.
func (c *OlympusClient) AdminBilling() *AdminBillingService {
	if c.adminBilling == nil {
		c.adminBilling = &AdminBillingService{http: c.http}
	}
	return c.adminBilling
}

// AdminGating returns the feature flag and gating admin service.
// Manages feature definitions, plan-level assignments, resource limits, and evaluation.
func (c *OlympusClient) AdminGating() *AdminGatingService {
	if c.adminGating == nil {
		c.adminGating = &AdminGatingService{http: c.http}
	}
	return c.adminGating
}

// Tuning returns the AI tuning jobs, persona generation, and chaos audio service.
// Manages model fine-tuning lifecycle, synthetic personas, and noise simulation.
func (c *OlympusClient) Tuning() *TuningService {
	if c.tuning == nil {
		c.tuning = &TuningService{http: c.http}
	}
	return c.tuning
}

// Voice returns the Voice Agent V2 cascade resolver + voice-agent service.
// v0.4.0 — Issues #3162 (V2-005).
func (c *OlympusClient) Voice() *VoiceService {
	if c.voice == nil {
		c.voice = &VoiceService{http: c.http}
	}
	return c.voice
}

// Connect returns the marketing-funnel + pre-conversion lead capture service.
// v0.4.0 — Issue #3108.
func (c *OlympusClient) Connect() *ConnectService {
	if c.connect == nil {
		c.connect = &ConnectService{http: c.http}
	}
	return c.connect
}

// Consent returns the app-scoped permissions consent service (v2.0.0).
//
// See docs/platform/APP-SCOPED-PERMISSIONS.md §6. Part of epic #3234.
func (c *OlympusClient) Consent() *ConsentService {
	if c.consent == nil {
		c.consent = &ConsentService{http: c.http}
	}
	return c.consent
}

// Governance returns the policy exception framework service (v2.0.0).
//
// Narrow scope: session_ttl_role_ceiling and grace_policy_category only.
// See §17 of APP-SCOPED-PERMISSIONS.md.
func (c *OlympusClient) Governance() *GovernanceService {
	if c.governance == nil {
		c.governance = &GovernanceService{http: c.http}
	}
	return c.governance
}

// ============================================================================
// App-scoped token management (§4.5 dual-JWT flow)
// ============================================================================

// SetAppToken attaches the App JWT obtained from /auth/app-tokens/mint.
// Forwarded on every request as X-App-Token alongside the user JWT
// Authorization header per the dual-JWT flow (§4.5).
func (c *OlympusClient) SetAppToken(token string) {
	c.http.SetAppToken(token)
	c.invalidateBitsetCache()
}

// ClearAppToken clears the App JWT (e.g. on logout).
func (c *OlympusClient) ClearAppToken() {
	c.http.ClearAppToken()
	c.invalidateBitsetCache()
}

// SetAccessToken replaces the active access token. Invalidates cached
// bitset decode.
func (c *OlympusClient) SetAccessToken(token string) {
	c.http.SetAccessToken(token)
	c.invalidateBitsetCache()
}

// OnCatalogStale registers a handler fired when the server returns
// X-Olympus-Catalog-Stale: true (§4.7 rolling window). Consumers should
// schedule a background token refresh at a randomized 0–15 min offset.
func (c *OlympusClient) OnCatalogStale(handler StaleCatalogHandler) {
	c.http.OnCatalogStale(handler)
}

// HasScopeBit is the fast-path constant-time bitmask check against the
// decoded app_scopes_bitset claim in the current access token.
//
// Returns false when no token is set, for platform-shell tokens without a
// bitset, for negative bitID, or when bitID is out of range. Used by SDK
// service methods to fail-fast with a typed ScopeDeniedError BEFORE making
// an HTTP call.
func (c *OlympusClient) HasScopeBit(bitID int) bool {
	if bitID < 0 {
		return false
	}
	bitset := c.decodeBitsetOnce()
	if bitset == nil {
		return false
	}
	byteIdx := bitID / 8
	bitIdx := bitID % 8
	if byteIdx >= len(bitset) {
		return false
	}
	return bitset[byteIdx]&(1<<bitIdx) != 0
}

// IsAppScoped returns true when the current access token carries an app_id
// claim (minted via /auth/app-tokens/mint for an app-scoped session).
func (c *OlympusClient) IsAppScoped() bool {
	claims := c.decodedClaims()
	if claims == nil {
		return false
	}
	_, ok := claims["app_id"].(string)
	return ok
}

func (c *OlympusClient) invalidateBitsetCache() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cachedClaims = nil
	c.cachedClaimsForToken = ""
	c.cachedBitset = nil
	c.cachedBitsetForToken = ""
}

// decodedClaims returns the parsed JWT claims for the current access token,
// caching the decode across calls. Safe to call from multiple goroutines.
func (c *OlympusClient) decodedClaims() map[string]interface{} {
	token := c.http.GetAccessToken()
	if token == "" {
		return nil
	}

	// Fast path: read-locked hit.
	c.cacheMu.RLock()
	if c.cachedClaimsForToken == token && c.cachedClaims != nil {
		claims := c.cachedClaims
		c.cacheMu.RUnlock()
		return claims
	}
	c.cacheMu.RUnlock()

	// Miss: parse + take write lock to commit.
	claims := parseJWTPayload(token)
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	// Double-check under the write lock — another goroutine may have raced us.
	if c.cachedClaimsForToken == token && c.cachedClaims != nil {
		return c.cachedClaims
	}
	if claims != nil {
		c.cachedClaims = claims
		c.cachedClaimsForToken = token
	}
	return claims
}

// decodeBitsetOnce returns the decoded scope bitset for the current access
// token. Safe to call from multiple goroutines.
func (c *OlympusClient) decodeBitsetOnce() []byte {
	token := c.http.GetAccessToken()
	if token == "" {
		return nil
	}

	// Fast path.
	c.cacheMu.RLock()
	if c.cachedBitsetForToken == token && c.cachedBitset != nil {
		bs := c.cachedBitset
		c.cacheMu.RUnlock()
		return bs
	}
	c.cacheMu.RUnlock()

	// Miss: read claims (which is also cached under the same lock) then decode.
	claims := c.decodedClaims()
	if claims == nil {
		return nil
	}
	bitsetStr, _ := claims["app_scopes_bitset"].(string)

	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if c.cachedBitsetForToken == token && c.cachedBitset != nil {
		return c.cachedBitset
	}
	if bitsetStr == "" {
		c.cachedBitset = []byte{}
		c.cachedBitsetForToken = token
		return c.cachedBitset
	}
	decoded, err := base64URLDecode(bitsetStr)
	if err != nil {
		return nil
	}
	c.cachedBitset = decoded
	c.cachedBitsetForToken = token
	return decoded
}

// Config returns the active SDK configuration.
func (c *OlympusClient) Config() *Config {
	return c.config
}

// HTTPClient returns the underlying HTTP client for advanced usage.
func (c *OlympusClient) HTTPClient() *httpClient {
	return c.http
}
