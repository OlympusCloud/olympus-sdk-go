package olympus

import (
	"encoding/json"
	"sort"
	"time"
)

// --------------------------------------------------------------------------
// Auth models
// --------------------------------------------------------------------------

// AuthSession represents an authenticated session returned after login.
type AuthSession struct {
	AccessToken  string   `json:"access_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int      `json:"expires_in"`
	RefreshToken string   `json:"refresh_token,omitempty"`
	UserID       string   `json:"user_id,omitempty"`
	TenantID     string   `json:"tenant_id,omitempty"`
	Roles        []string `json:"roles,omitempty"`
	// AppScopes lists the canonical scope strings granted to the session
	// (decoded from the JWT `app_scopes` claim). Format: "auth.session.read@user".
	// Populated by parseAuthSession when the server returns the list in the
	// login/refresh payload and re-hydrated from the token in AuthService.
	AppScopes []string `json:"app_scopes,omitempty"`
}

// User represents a platform user.
type User struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Name      string     `json:"name,omitempty"`
	Roles     []string   `json:"roles,omitempty"`
	TenantID  string     `json:"tenant_id,omitempty"`
	Status    string     `json:"status,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// APIKey represents an API key for programmatic access.
type APIKey struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Key       string     `json:"key,omitempty"` // Only present on creation.
	Scopes    []string   `json:"scopes,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// OlympusTeammate is a teammate listed by the Auth/Platform service.
//
// Returned by AuthService.ListTeammates. Mirrors the canonical Dart contract
// shipped in olympus-sdk-dart#45 (W12-1 / olympus-cloud-gcp#3599).
// AssignedScopes is a set so callers can do membership checks in O(1).
type OlympusTeammate struct {
	UserID         string              `json:"user_id"`
	DisplayName    string              `json:"display_name"`
	Role           string              `json:"role"`
	AssignedScopes map[string]struct{} `json:"-"`
}

// MarshalJSON serialises AssignedScopes as a sorted string array on the wire.
func (t OlympusTeammate) MarshalJSON() ([]byte, error) {
	scopes := make([]string, 0, len(t.AssignedScopes))
	for s := range t.AssignedScopes {
		scopes = append(scopes, s)
	}
	sort.Strings(scopes)
	type alias struct {
		UserID         string   `json:"user_id"`
		DisplayName    string   `json:"display_name"`
		Role           string   `json:"role"`
		AssignedScopes []string `json:"assigned_scopes"`
	}
	return json.Marshal(alias{
		UserID:         t.UserID,
		DisplayName:    t.DisplayName,
		Role:           t.Role,
		AssignedScopes: scopes,
	})
}

// --------------------------------------------------------------------------
// Commerce models
// --------------------------------------------------------------------------

// Order represents an order in the commerce system.
type Order struct {
	ID         string      `json:"id"`
	Status     string      `json:"status"`
	Items      []OrderItem `json:"items,omitempty"`
	Source     string      `json:"source,omitempty"`
	TableID    string      `json:"table_id,omitempty"`
	CustomerID string      `json:"customer_id,omitempty"`
	Subtotal   int         `json:"subtotal,omitempty"`
	Tax        int         `json:"tax,omitempty"`
	Total      int         `json:"total,omitempty"`
	CreatedAt  *time.Time  `json:"created_at,omitempty"`
	UpdatedAt  *time.Time  `json:"updated_at,omitempty"`
}

// OrderItem represents a single line item within an order.
type OrderItem struct {
	CatalogID string          `json:"catalog_id"`
	Qty       int             `json:"qty"`
	Price     int             `json:"price"` // Price in cents.
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Modifiers []OrderModifier `json:"modifiers,omitempty"`
	Notes     string          `json:"notes,omitempty"`
}

// OrderModifier represents a modifier applied to an order item.
type OrderModifier struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price,omitempty"`
}

// CatalogItem represents a menu item, product, or other catalog entry.
type CatalogItem struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Price       int               `json:"price"` // Price in cents.
	Description string            `json:"description,omitempty"`
	Category    string            `json:"category,omitempty"`
	CategoryID  string            `json:"category_id,omitempty"`
	ImageURL    string            `json:"image_url,omitempty"`
	Modifiers   []CatalogModifier `json:"modifiers,omitempty"`
	Available   *bool             `json:"available,omitempty"`
	CreatedAt   *time.Time        `json:"created_at,omitempty"`
	UpdatedAt   *time.Time        `json:"updated_at,omitempty"`
}

// CatalogModifier represents a modifier definition within a catalog item.
type CatalogModifier struct {
	ID       string                  `json:"id"`
	Name     string                  `json:"name"`
	Price    int                     `json:"price,omitempty"`
	Required *bool                   `json:"required,omitempty"`
	Options  []CatalogModifierOption `json:"options,omitempty"`
}

// CatalogModifierOption represents an individual option within a modifier group.
type CatalogModifierOption struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price,omitempty"`
}

// --------------------------------------------------------------------------
// AI models
// --------------------------------------------------------------------------

// AIResponse represents a response from an AI query or chat completion.
type AIResponse struct {
	Content      string `json:"content"`
	Model        string `json:"model,omitempty"`
	Tier         string `json:"tier,omitempty"`
	TokensUsed   int    `json:"tokens_used,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
	RequestID    string `json:"request_id,omitempty"`
}

// AgentResult represents the result of invoking a LangGraph agent.
type AgentResult struct {
	Output     string      `json:"output"`
	AgentName  string      `json:"agent_name,omitempty"`
	Steps      []AgentStep `json:"steps,omitempty"`
	TokensUsed int         `json:"tokens_used,omitempty"`
	RequestID  string      `json:"request_id,omitempty"`
}

// AgentStep represents a single step executed by an agent.
type AgentStep struct {
	Action      string `json:"action"`
	Observation string `json:"observation,omitempty"`
	Thought     string `json:"thought,omitempty"`
}

// AgentTask represents an asynchronous agent task with status tracking.
type AgentTask struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	AgentName   string     `json:"agent_name,omitempty"`
	Task        string     `json:"task,omitempty"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// IsCompleted returns true if the task has completed successfully.
func (t *AgentTask) IsCompleted() bool { return t.Status == "completed" }

// IsFailed returns true if the task has failed.
func (t *AgentTask) IsFailed() bool { return t.Status == "failed" }

// IsPending returns true if the task is still running.
func (t *AgentTask) IsPending() bool { return t.Status == "pending" || t.Status == "running" }

// Classification represents a text classification result.
type Classification struct {
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	Scores     map[string]float64 `json:"scores,omitempty"`
}

// SentimentResult represents a sentiment analysis result.
type SentimentResult struct {
	Sentiment string            `json:"sentiment"` // positive, negative, neutral, mixed
	Score     float64           `json:"score"`
	Aspects   []AspectSentiment `json:"aspects,omitempty"`
}

// AspectSentiment represents sentiment for a specific aspect of text.
type AspectSentiment struct {
	Aspect    string  `json:"aspect"`
	Sentiment string  `json:"sentiment"`
	Score     float64 `json:"score"`
}

// ChatMessage represents a message in a multi-turn conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// --------------------------------------------------------------------------
// Payment models
// --------------------------------------------------------------------------

// Payment represents a completed or pending payment.
type Payment struct {
	ID                    string     `json:"id"`
	Status                string     `json:"status"`
	OrderID               string     `json:"order_id,omitempty"`
	Amount                int        `json:"amount,omitempty"` // Amount in cents.
	Currency              string     `json:"currency,omitempty"`
	Method                string     `json:"method,omitempty"`
	StripePaymentIntentID string     `json:"stripe_payment_intent_id,omitempty"`
	CreatedAt             *time.Time `json:"created_at,omitempty"`
}

// Refund represents a refund issued against a payment.
type Refund struct {
	ID        string     `json:"id"`
	PaymentID string     `json:"payment_id"`
	Status    string     `json:"status"`
	Amount    int        `json:"amount,omitempty"` // Amount in cents; 0 = full refund.
	Reason    string     `json:"reason,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// Balance represents account balance information.
type Balance struct {
	Available int    `json:"available"` // Available balance in cents.
	Pending   int    `json:"pending"`   // Pending balance in cents.
	Currency  string `json:"currency,omitempty"`
}

// Total returns the sum of available and pending balances.
func (b *Balance) Total() int { return b.Available + b.Pending }

// Payout represents a payout to an external bank account.
type Payout struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Amount      int        `json:"amount,omitempty"` // Amount in cents.
	Currency    string     `json:"currency,omitempty"`
	Destination string     `json:"destination,omitempty"`
	Method      string     `json:"method,omitempty"` // "standard" or "instant"
	ArrivalDate *time.Time `json:"arrival_date,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
}

// TerminalReader represents a physical card reader registered via Stripe Terminal.
type TerminalReader struct {
	ID           string `json:"id"`
	DeviceType   string `json:"device_type,omitempty"`
	Label        string `json:"label,omitempty"`
	LocationID   string `json:"location_id,omitempty"`
	SerialNumber string `json:"serial_number,omitempty"`
	Status       string `json:"status,omitempty"` // "online" or "offline"
	IPAddress    string `json:"ip_address,omitempty"`
}

// TerminalPayment represents the result of presenting a payment to a reader.
type TerminalPayment struct {
	ID              string     `json:"id"`
	Status          string     `json:"status"`
	Amount          int        `json:"amount,omitempty"` // Amount in cents.
	Currency        string     `json:"currency,omitempty"`
	ReaderID        string     `json:"reader_id,omitempty"`
	PaymentIntentID string     `json:"payment_intent_id,omitempty"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
}

// --------------------------------------------------------------------------
// Billing models
// --------------------------------------------------------------------------

// Plan represents a billing plan (Ember, Spark, Blaze, Inferno, Olympus).
type Plan struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Tier         string   `json:"tier,omitempty"`
	MonthlyPrice int      `json:"monthly_price,omitempty"` // In cents.
	AnnualPrice  int      `json:"annual_price,omitempty"`  // In cents.
	MaxLocations int      `json:"max_locations,omitempty"`
	MaxAgents    int      `json:"max_agents,omitempty"`
	AICredits    int      `json:"ai_credits,omitempty"`
	VoiceMinutes int      `json:"voice_minutes,omitempty"`
	Features     []string `json:"features,omitempty"`
	Status       string   `json:"status,omitempty"`
}

// UsageReport represents tenant resource usage for a billing period.
type UsageReport struct {
	Period            string `json:"period,omitempty"`
	AICreditsUsed     int    `json:"ai_credits_used,omitempty"`
	AICreditsLimit    int    `json:"ai_credits_limit,omitempty"`
	VoiceMinutesUsed  int    `json:"voice_minutes_used,omitempty"`
	VoiceMinutesLimit int    `json:"voice_minutes_limit,omitempty"`
	StorageUsedMB     int    `json:"storage_used_mb,omitempty"`
	StorageLimitMB    int    `json:"storage_limit_mb,omitempty"`
	APICallsCount     int    `json:"api_calls_count,omitempty"`
	LocationCount     int    `json:"location_count,omitempty"`
	AgentCount        int    `json:"agent_count,omitempty"`
}

// Invoice represents a billing invoice.
type Invoice struct {
	ID          string            `json:"id"`
	Status      string            `json:"status,omitempty"`
	Amount      int               `json:"amount,omitempty"` // In cents.
	Currency    string            `json:"currency,omitempty"`
	PeriodStart *time.Time        `json:"period_start,omitempty"`
	PeriodEnd   *time.Time        `json:"period_end,omitempty"`
	PaidAt      *time.Time        `json:"paid_at,omitempty"`
	PDFURL      string            `json:"pdf_url,omitempty"`
	LineItems   []InvoiceLineItem `json:"line_items,omitempty"`
}

// InvoiceLineItem represents a single line item on an invoice.
type InvoiceLineItem struct {
	Description string `json:"description"`
	Amount      int    `json:"amount,omitempty"` // In cents.
	Quantity    int    `json:"quantity,omitempty"`
}

// --------------------------------------------------------------------------
// Marketplace models
// --------------------------------------------------------------------------

// MarketplaceApp represents an app listed on the Olympus Marketplace.
type MarketplaceApp struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
	Category     string     `json:"category,omitempty"`
	Industry     string     `json:"industry,omitempty"`
	Developer    string     `json:"developer,omitempty"`
	IconURL      string     `json:"icon_url,omitempty"`
	Rating       float64    `json:"rating,omitempty"`
	InstallCount int        `json:"install_count,omitempty"`
	Pricing      string     `json:"pricing,omitempty"`
	CreatedAt    *time.Time `json:"created_at,omitempty"`
}

// Installation represents an installed marketplace app instance.
type Installation struct {
	ID          string                 `json:"id"`
	AppID       string                 `json:"app_id"`
	AppName     string                 `json:"app_name,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	InstalledAt *time.Time             `json:"installed_at,omitempty"`
}

// --------------------------------------------------------------------------
// Device models
// --------------------------------------------------------------------------

// Device represents a managed device enrolled via MDM.
type Device struct {
	ID         string     `json:"id"`
	Name       string     `json:"name,omitempty"`
	Status     string     `json:"status,omitempty"`
	Profile    string     `json:"profile,omitempty"`
	Platform   string     `json:"platform,omitempty"`
	OSVersion  string     `json:"os_version,omitempty"`
	AppVersion string     `json:"app_version,omitempty"`
	LocationID string     `json:"location_id,omitempty"`
	LastSeen   *time.Time `json:"last_seen,omitempty"`
	EnrolledAt *time.Time `json:"enrolled_at,omitempty"`
}

// --------------------------------------------------------------------------
// Common models
// --------------------------------------------------------------------------

// Notification represents a notification entry.
type Notification struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type,omitempty"`
	Title     string                 `json:"title,omitempty"`
	Body      string                 `json:"body,omitempty"`
	Read      bool                   `json:"read"`
	Data      map[string]interface{} `json:"data,omitempty"`
	CreatedAt *time.Time             `json:"created_at,omitempty"`
}

// WebhookRegistration represents a registered webhook endpoint.
type WebhookRegistration struct {
	ID        string     `json:"id"`
	URL       string     `json:"url"`
	Events    []string   `json:"events"`
	Secret    string     `json:"secret,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// SearchResult represents a generic search result from data or AI search.
type SearchResult struct {
	ID       string                 `json:"id"`
	Score    float64                `json:"score"`
	Content  string                 `json:"content,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PolicyResult represents a policy evaluation result from the gating service.
type PolicyResult struct {
	Allowed bool        `json:"allowed"`
	Value   interface{} `json:"value,omitempty"`
	Reason  string      `json:"reason,omitempty"`
}

// FeatureFlag represents a feature flag for the tenant.
type FeatureFlag struct {
	Key     string      `json:"key"`
	Enabled bool        `json:"enabled"`
	Value   interface{} `json:"value,omitempty"`
}

// PaginatedResponse is a generic paginated list response.
type PaginatedResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// Pagination holds pagination metadata.
type Pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// HasNextPage returns true if there are more pages.
func (p *Pagination) HasNextPage() bool { return p.Page < p.TotalPages }

// HasPreviousPage returns true if there are preceding pages.
func (p *Pagination) HasPreviousPage() bool { return p.Page > 1 }
