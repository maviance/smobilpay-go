package smobilpay

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

// DefaultAPIVersion is the default value of the x-api-version request header.
const DefaultAPIVersion = "3.0.0"

// Config is the immutable configuration for a Client. Construct via
// NewConfig + functional options.
type Config struct {
	BaseURL          string
	PublicKey        string
	SecretKey        string
	APIVersion       string
	RequestTimeout   time.Duration
	TokenRefreshSkew time.Duration
	HTTPClient       *http.Client
}

// Option mutates a Config during construction.
type Option func(*Config)

// NewConfig validates and finalises a Config. Required: WithBaseURL and
// WithCredentials. Returns an error if either is missing.
func NewConfig(opts ...Option) (Config, error) {
	cfg := Config{
		APIVersion:       DefaultAPIVersion,
		RequestTimeout:   30 * time.Second,
		TokenRefreshSkew: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/")
	if cfg.BaseURL == "" {
		return Config{}, errors.New("smobilpay: WithBaseURL is required")
	}
	if cfg.PublicKey == "" || cfg.SecretKey == "" {
		return Config{}, errors.New("smobilpay: WithCredentials is required (publicKey and secretKey both non-empty)")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.RequestTimeout}
	}
	return cfg, nil
}

// WithBaseURL sets the partner base URL (issued during onboarding).
func WithBaseURL(baseURL string) Option { return func(c *Config) { c.BaseURL = baseURL } }

// WithCredentials sets the OAuth 2.0 client_credentials pair.
func WithCredentials(publicKey, secretKey string) Option {
	return func(c *Config) {
		c.PublicKey = publicKey
		c.SecretKey = secretKey
	}
}

// WithAPIVersion overrides the value of the x-api-version request header.
// Default: DefaultAPIVersion ("3.0.0").
func WithAPIVersion(version string) Option {
	return func(c *Config) { c.APIVersion = version }
}

// WithRequestTimeout sets the per-request timeout. Default: 30s.
func WithRequestTimeout(d time.Duration) Option {
	return func(c *Config) { c.RequestTimeout = d }
}

// WithTokenRefreshSkew sets the lead time at which a cached token is
// treated as expired so it can be re-minted before the next request
// fails. Default: 30s.
func WithTokenRefreshSkew(d time.Duration) Option {
	return func(c *Config) { c.TokenRefreshSkew = d }
}

// WithHTTPClient injects a custom *http.Client (e.g. for proxy or custom
// TLS). The injected client's own Timeout field governs per-request
// deadlines; the value set via WithRequestTimeout is NOT applied to an
// injected client. Pass an http.Client whose Timeout is 0 to disable
// per-request timeouts entirely.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) { c.HTTPClient = client }
}
