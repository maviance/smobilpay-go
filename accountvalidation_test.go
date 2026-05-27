package smobilpay

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestVerifyServiceNumber_validates(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.AccountValidation.VerifyServiceNumber(context.Background(), "", 1, "n"); err == nil {
		t.Error("expected error on empty merchant")
	}
	if _, err := c.AccountValidation.VerifyServiceNumber(context.Background(), "M", 1, ""); err == nil {
		t.Error("expected error on empty serviceNumber")
	}
}

func TestVerifyServiceNumber_emptyMerchant(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.AccountValidation.VerifyServiceNumber(context.Background(), "", 1001, "203157530")
	if err == nil {
		t.Fatal("expected error on empty merchant")
	}
}

func TestVerifyServiceNumber_emptyServiceNumber(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.AccountValidation.VerifyServiceNumber(context.Background(), "ENEO", 1001, "")
	if err == nil {
		t.Fatal("expected error on empty serviceNumber")
	}
}

func TestVerifyServiceNumber_zeroServiceID(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.AccountValidation.VerifyServiceNumber(context.Background(), "ENEO", 0, "203157530")
	if err == nil {
		t.Fatal("expected error on zero serviceID")
	}
}

func TestVerifyServiceNumber_true(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v2/verify" {
			http.Error(w, "bad", 400)
			return
		}
		want := "merchant=ENEO&serviceid=1001&serviceNumber=203157530"
		if r.URL.RawQuery != want {
			t.Errorf("query = %q, want %q", r.URL.RawQuery, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `true`)
	})
	ok, err := c.AccountValidation.VerifyServiceNumber(context.Background(), "ENEO", 1001, "203157530")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestVerifyServiceNumber_false(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `false`)
	})
	ok, err := c.AccountValidation.VerifyServiceNumber(context.Background(), "ENEO", 1001, "203157530")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false")
	}
}

func TestValidateAccount(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v2/validate" {
			http.Error(w, "bad", 400)
			return
		}
		want := "destination=677389120&serviceId=20053"
		if r.URL.RawQuery != want {
			t.Errorf("query = %q, want %q", r.URL.RawQuery, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"VERIFIED","name":"Jane","destination":"677389120"}`)
	})
	ca, err := c.AccountValidation.ValidateAccount(context.Background(), "677389120", 20053)
	if err != nil {
		t.Fatal(err)
	}
	if ca.Status != CustomerAccountStatusVerified {
		t.Errorf("status = %q, want %q", ca.Status, CustomerAccountStatusVerified)
	}
	if ca.Name != "Jane" {
		t.Errorf("name = %q, want Jane", ca.Name)
	}
	if ca.Destination != "677389120" {
		t.Errorf("destination = %q", ca.Destination)
	}
}

func TestValidateAccount_validates(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.AccountValidation.ValidateAccount(context.Background(), "", 1); err == nil {
		t.Error("expected error on empty destination")
	}
}

func TestValidateAccount_zeroServiceID(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.AccountValidation.ValidateAccount(context.Background(), "677389120", 0)
	if err == nil {
		t.Fatal("expected error on zero serviceID")
	}
}

func TestValidateAccount_status_validated_unknown(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    CustomerAccountStatus
	}{
		{"unknown", `{"status":"UNKNOWN","destination":"677389120"}`, CustomerAccountStatusUnknown},
		{"validated", `{"status":"VALIDATED","destination":"677389120"}`, CustomerAccountStatusValidated},
		{"verified", `{"status":"VERIFIED","name":"Jane","destination":"677389120"}`, CustomerAccountStatusVerified},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := tc.payload
			c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, payload)
			})
			ca, err := c.AccountValidation.ValidateAccount(context.Background(), "677389120", 20053)
			if err != nil {
				t.Fatal(err)
			}
			if ca.Status != tc.want {
				t.Errorf("status = %q, want %q", ca.Status, tc.want)
			}
		})
	}
}
