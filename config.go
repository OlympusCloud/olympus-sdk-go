package olympus

import "time"

// Environment represents the target Olympus Cloud environment.
type Environment string

const (
	// EnvProduction is the production environment.
	EnvProduction Environment = "production"
	// EnvStaging is the staging environment.
	EnvStaging Environment = "staging"
	// EnvDevelopment is the development environment.
	EnvDevelopment Environment = "development"
	// EnvSandbox is the sandbox environment for testing.
	EnvSandbox Environment = "sandbox"
)

// baseURLs maps each environment to its default API base URL.
var baseURLs = map[Environment]string{
	EnvProduction:  "https://api.olympuscloud.ai/api/v1",
	EnvStaging:     "https://staging.api.olympuscloud.ai/api/v1",
	EnvDevelopment: "https://dev.api.olympuscloud.ai/api/v1",
	EnvSandbox:     "https://sandbox.api.olympuscloud.ai/api/v1",
}

// Config holds the SDK configuration.
type Config struct {
	// AppID is the application identifier (e.g., "com.my-restaurant").
	AppID string

	// APIKey is the API key for authentication.
	APIKey string

	// BaseURL overrides the default base URL for the environment.
	// If empty, the URL is derived from Environment.
	BaseURL string

	// Environment selects the target environment. Defaults to EnvProduction.
	Environment Environment

	// Timeout is the HTTP request timeout. Defaults to 30 seconds.
	Timeout time.Duration

	// MaxRetries is the maximum number of retry attempts for retryable errors.
	// Defaults to 3.
	MaxRetries int

	// RetryBaseDelay is the base delay for exponential backoff. Defaults to 500ms.
	RetryBaseDelay time.Duration
}

// effectiveBaseURL returns the base URL to use, preferring an explicit override.
func (c *Config) effectiveBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	if url, ok := baseURLs[c.Environment]; ok {
		return url
	}
	return baseURLs[EnvProduction]
}

// effectiveTimeout returns the timeout to use.
func (c *Config) effectiveTimeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 30 * time.Second
}

// effectiveMaxRetries returns the max retry count.
func (c *Config) effectiveMaxRetries() int {
	if c.MaxRetries > 0 {
		return c.MaxRetries
	}
	return 3
}

// effectiveRetryBaseDelay returns the retry base delay.
func (c *Config) effectiveRetryBaseDelay() time.Duration {
	if c.RetryBaseDelay > 0 {
		return c.RetryBaseDelay
	}
	return 500 * time.Millisecond
}
