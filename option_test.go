package smobilpay

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewConfig_defaults(t *testing.T) {
	cfg, err := NewConfig(
		WithBaseURL("https://api.example.invalid"),
		WithCredentials("pub", "sec"),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.APIVersion != "3.0.0" {
		t.Errorf("APIVersion = %q", cfg.APIVersion)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v", cfg.RequestTimeout)
	}
	if cfg.TokenRefreshSkew != 30*time.Second {
		t.Errorf("TokenRefreshSkew = %v", cfg.TokenRefreshSkew)
	}
	if cfg.HTTPClient == nil {
		t.Error("HTTPClient should be defaulted")
	}
}

func TestNewConfig_trimsTrailingSlash(t *testing.T) {
	cfg, _ := NewConfig(
		WithBaseURL("https://api.example.invalid/"),
		WithCredentials("pub", "sec"),
	)
	if strings.HasSuffix(cfg.BaseURL, "/") {
		t.Errorf("trailing slash not trimmed: %q", cfg.BaseURL)
	}
}

func TestNewConfig_missingBaseURL(t *testing.T) {
	if _, err := NewConfig(WithCredentials("pub", "sec")); err == nil {
		t.Error("expected error when baseURL missing")
	}
}

func TestNewConfig_missingCredentials(t *testing.T) {
	if _, err := NewConfig(WithBaseURL("https://api.example.invalid")); err == nil {
		t.Error("expected error when credentials missing")
	}
}

func TestNewConfig_emptyPublicKey(t *testing.T) {
	if _, err := NewConfig(
		WithBaseURL("https://api.example.invalid"),
		WithCredentials("", "sec"),
	); err == nil {
		t.Error("expected error on empty publicKey")
	}
}

func TestNewConfig_overrides(t *testing.T) {
	hc := &http.Client{Timeout: time.Second}
	cfg, err := NewConfig(
		WithBaseURL("https://api.example.invalid"),
		WithCredentials("pub", "sec"),
		WithAPIVersion("3.1.0"),
		WithRequestTimeout(10*time.Second),
		WithTokenRefreshSkew(5*time.Second),
		WithHTTPClient(hc),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.APIVersion != "3.1.0" {
		t.Errorf("APIVersion = %q", cfg.APIVersion)
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Errorf("RequestTimeout = %v", cfg.RequestTimeout)
	}
	if cfg.TokenRefreshSkew != 5*time.Second {
		t.Errorf("TokenRefreshSkew = %v", cfg.TokenRefreshSkew)
	}
	if cfg.HTTPClient != hc {
		t.Error("HTTPClient override not applied")
	}
}
