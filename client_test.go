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
	// Zero Config has no BaseURL, no credentials, and no HTTPClient.
	// The error message must enumerate every missing field.
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error on empty Config")
	}
	msg := err.Error()
	for _, want := range []string{"BaseURL", "Credentials", "HTTPClient"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q: %v", want, err)
		}
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
