package smobilpay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCollectionRequest_validation(t *testing.T) {
	cases := []struct {
		name string
		req  CollectionRequest
		want string
	}{
		{"missing quoteId", CollectionRequest{CustomerPhoneNumber: "p", CustomerEmailAddress: "e"}, "quoteId"},
		{"missing phone", CollectionRequest{QuoteID: "q", CustomerEmailAddress: "e"}, "customerPhonenumber"},
		{"missing email", CollectionRequest{QuoteID: "q", CustomerPhoneNumber: "p"}, "customerEmailaddress"},
		{
			"tag too long",
			CollectionRequest{QuoteID: "q", CustomerPhoneNumber: "p", CustomerEmailAddress: "e",
				Tag: strings.Repeat("x", 51)},
			"tag",
		},
		{
			"callback too long",
			CollectionRequest{QuoteID: "q", CustomerPhoneNumber: "p", CustomerEmailAddress: "e",
				CallbackURL: "https://" + strings.Repeat("x", 248)},
			"callbackUrl",
		},
	}
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.Confirm.Collect(context.Background(), tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestCollectionRequest_marshalOmitsNil(t *testing.T) {
	req := CollectionRequest{
		QuoteID:              "q-1",
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "c@example.com",
	}
	b, _ := json.Marshal(req)
	got := string(b)
	if !strings.Contains(got, `"quoteId":"q-1"`) {
		t.Errorf("marshal: missing quoteId in %s", got)
	}
	if strings.Contains(got, "customerName") || strings.Contains(got, "trid") {
		t.Errorf("marshal: emitted nil field in %s", got)
	}
}

func TestConfirm_Collect(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v2/collectstd" {
			http.Error(w, "bad", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ptn":"P-1","timestamp":"2024-01-15T10:35:00Z","agentBalance":900.5,"receiptNumber":"r","veriCode":"v","priceLocalCur":510,"priceSystemCur":510,"localCur":"XAF","systemCur":"XAF","status":"PENDING","payItemId":"PI"}`)
	})
	resp, err := c.Confirm.Collect(context.Background(), CollectionRequest{
		QuoteID:              "q-1",
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "c@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PTN != "P-1" || resp.Status != PaymentStatusPending {
		t.Errorf("got %+v", resp)
	}
}

func TestConfirm_Collect_serializesAllFields(t *testing.T) {
	var captured map[string]any
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v2/collectstd" {
			http.Error(w, "bad", 400)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("body unmarshal: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ptn":"P-1","timestamp":"2024-01-15T10:35:00Z","receiptNumber":"r","veriCode":"v","localCur":"XAF","systemCur":"XAF","status":"PENDING"}`)
	})

	req := CollectionRequest{
		QuoteID:              "q-1",
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "c@example.com",
		CustomerName:         "Jane Doe",
		CustomerAddress:      "123 Test St",
		CustomerNumber:       "CUST-1",
		ServiceNumber:        "SVC-1",
		TRID:                 "TRID-1",
		Tag:                  "tag-1",
		CallbackURL:          "https://example.com/callback",
		CData:                "extra-data",
	}
	if _, err := c.Confirm.Collect(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	wantKeys := []string{
		"quoteId",
		"customerPhonenumber",
		"customerEmailaddress",
		"customerName",
		"customerAddress",
		"customerNumber",
		"serviceNumber",
		"trid",
		"tag",
		"callbackUrl",
		"cdata",
	}
	for _, k := range wantKeys {
		if _, ok := captured[k]; !ok {
			t.Errorf("body missing key %q (got %+v)", k, captured)
		}
	}
}
