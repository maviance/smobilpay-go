package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleConfig = `{
  "baseUrl": "https://api.example.invalid",
  "publicKey": "pub",
  "secretKey": "sec",
  "apiVersion": "3.0.0",
  "cashout": { "serviceId": 20053, "amount": 500 },
  "bill": { "merchant": "ENEO", "serviceId": 10039, "serviceNumber": "203157530" },
  "voucher": null
}`

func TestSmokeConfig_parses(t *testing.T) {
	var cfg SmokeConfig
	if err := json.Unmarshal([]byte(sampleConfig), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.BaseURL != "https://api.example.invalid" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.PublicKey != "pub" || cfg.SecretKey != "sec" {
		t.Errorf("creds = %+v", cfg)
	}
	if cfg.APIVersion != "3.0.0" {
		t.Errorf("APIVersion = %q", cfg.APIVersion)
	}
	if cfg.Cashout == nil || cfg.Cashout.ServiceID != 20053 || cfg.Cashout.Amount != 500 {
		t.Errorf("Cashout = %+v", cfg.Cashout)
	}
	if cfg.Bill == nil || cfg.Bill.Merchant != "ENEO" {
		t.Errorf("Bill = %+v", cfg.Bill)
	}
	if cfg.Bill == nil || cfg.Bill.ServiceNumber != "203157530" {
		t.Errorf("Bill.ServiceNumber = %+v", cfg.Bill)
	}
	if cfg.Voucher != nil {
		t.Errorf("Voucher should be nil for skip")
	}
}

func TestSmokeConfig_validate_requiresFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  SmokeConfig
		want string
	}{
		{"missing baseUrl", SmokeConfig{PublicKey: "p", SecretKey: "s"}, "baseUrl"},
		{"missing publicKey", SmokeConfig{BaseURL: "u", SecretKey: "s"}, "publicKey"},
		{"missing secretKey", SmokeConfig{BaseURL: "u", PublicKey: "p"}, "secretKey"},
		{"happy", SmokeConfig{BaseURL: "u", PublicKey: "p", SecretKey: "s"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.want == "" && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("err = %v, want substring %q", err, tc.want)
				}
			}
		})
	}
}

func TestSmokeConfig_parsesCollectOpts(t *testing.T) {
	const cfgJSON = `{
	  "baseUrl": "https://api.example.invalid",
	  "publicKey": "pub",
	  "secretKey": "sec",
	  "cashin": {
	    "serviceId": 50052,
	    "amount": 1000,
	    "collect": true,
	    "customerPhonenumber": "699999999",
	    "customerEmailaddress": "acceptance@maviance.test",
	    "serviceNumber": "699999999"
	  }
	}`
	var cfg SmokeConfig
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Cashin == nil {
		t.Fatalf("Cashin block missing")
	}
	if cfg.Cashin.ServiceID != 50052 {
		t.Errorf("ServiceID = %d, want 50052", cfg.Cashin.ServiceID)
	}
	if cfg.Cashin.Amount != 1000 {
		t.Errorf("Amount = %d, want 1000", cfg.Cashin.Amount)
	}
	if !cfg.Cashin.Collect {
		t.Errorf("Collect = false, want true")
	}
	if cfg.Cashin.CustomerPhoneNumber != "699999999" {
		t.Errorf("CustomerPhoneNumber = %q, want 699999999", cfg.Cashin.CustomerPhoneNumber)
	}
	if cfg.Cashin.CustomerEmailAddress != "acceptance@maviance.test" {
		t.Errorf("CustomerEmailAddress = %q, want acceptance@maviance.test", cfg.Cashin.CustomerEmailAddress)
	}
	if cfg.Cashin.ServiceNumber != "699999999" {
		t.Errorf("ServiceNumber = %q, want 699999999", cfg.Cashin.ServiceNumber)
	}
}
