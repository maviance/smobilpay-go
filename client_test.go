package smobilpay

import (
	"strings"
	"testing"
)

func TestNew_initializesAllAPIGroups(t *testing.T) {
	cfg, err := NewConfig(
		WithBaseURL("https://api.example.invalid"),
		WithCredentials("pub", "sec"),
	)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Verify == nil {
		t.Error("Verify is nil")
	}
	if c.Masterdata == nil {
		t.Error("Masterdata is nil")
	}
	if c.Initiate == nil {
		t.Error("Initiate is nil")
	}
	if c.Confirm == nil {
		t.Error("Confirm is nil")
	}
	if c.AccountValidation == nil {
		t.Error("AccountValidation is nil")
	}
	if c.Tokens == nil {
		t.Error("Tokens is nil")
	}
}

func TestNew_rejectsInvalidConfig(t *testing.T) {
	// Zero Config has no BaseURL; that's the input we must reject.
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error on empty Config")
	}
	if !strings.Contains(err.Error(), "BaseURL") &&
		!strings.Contains(err.Error(), "Credentials") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestClient_Config_returnsConfiguredValues(t *testing.T) {
	cfg, err := NewConfig(
		WithBaseURL("https://api.example.invalid"),
		WithCredentials("pub", "sec"),
		WithAPIVersion("3.2.0"),
	)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := c.Config()
	if got.BaseURL != "https://api.example.invalid" {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, "https://api.example.invalid")
	}
	if got.PublicKey != "pub" {
		t.Errorf("PublicKey = %q, want %q", got.PublicKey, "pub")
	}
	if got.SecretKey != "sec" {
		t.Errorf("SecretKey = %q, want %q", got.SecretKey, "sec")
	}
	if got.APIVersion != "3.2.0" {
		t.Errorf("APIVersion = %q, want %q", got.APIVersion, "3.2.0")
	}
}
