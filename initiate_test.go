package smobilpay

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestInitiate_Bills_requiresParams(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.Initiate.Bills(context.Background(), "", 0, ""); err == nil {
		t.Error("expected error on empty merchant")
	}
	if _, err := c.Initiate.Bills(context.Background(), "M", 0, ""); err == nil {
		t.Error("expected error on empty serviceNumber")
	}
}

func TestInitiate_Bills_emptyMerchant(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.Initiate.Bills(context.Background(), "", 10039, "203157530")
	if err == nil {
		t.Fatal("expected error on empty merchant")
	}
	if !strings.Contains(err.Error(), "merchant") {
		t.Errorf("error = %v, want one mentioning merchant", err)
	}
}

func TestInitiate_Bills_emptyServiceNumber(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.Initiate.Bills(context.Background(), "ENEO", 10039, "")
	if err == nil {
		t.Fatal("expected error on empty serviceNumber")
	}
	if !strings.Contains(err.Error(), "serviceNumber") {
		t.Errorf("error = %v, want one mentioning serviceNumber", err)
	}
}

func TestInitiate_Bills(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		want := "merchant=ENEO&serviceid=10039&serviceNumber=203157530"
		if r.URL.RawQuery != want {
			t.Errorf("query = %q, want %q", r.URL.RawQuery, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":10039,"merchant":"ENEO","payItemId":"PI","amountType":"FIXED","localCur":"XAF","name":"N","amountLocalCur":1234.5,"billType":"REGULAR","payOrder":1,"serviceNumber":"203157530","billDate":"2024-01-15","billDueDate":"2024-02-15"}]`)
	})
	got, err := c.Initiate.Bills(context.Background(), "ENEO", 10039, "203157530")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].BillType != BillTypeRegular || got[0].PayOrder != 1 {
		t.Errorf("got %+v", got)
	}
	if y, m, _ := got[0].BillDate.Date(); y != 2024 || m != time.January {
		t.Errorf("BillDate = %v", got[0].BillDate)
	}
}

func TestInitiate_Subscriptions_requiresOneOfNumberOrCustomer(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.Initiate.Subscriptions(context.Background(), "M", 1, "", ""); err == nil {
		t.Error("expected error when both serviceNumber and customerNumber empty")
	}
	if _, err := c.Initiate.Subscriptions(context.Background(), "", 1, "DEC-1", ""); err == nil {
		t.Error("expected error when merchant empty")
	}
}

func TestInitiate_Subscriptions_serviceNumberOnly(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "serviceNumber=DEC-1") ||
			strings.Contains(r.URL.RawQuery, "customerNumber=") {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":5000,"merchant":"CMSABC","payItemId":"PI","amountType":"FIXED","localCur":"XAF","name":"N","serviceNumber":"DEC-1"}]`)
	})
	got, err := c.Initiate.Subscriptions(context.Background(), "CMSABC", 5000, "DEC-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ServiceNumber != "DEC-1" {
		t.Errorf("got %+v", got)
	}
}

func TestInitiate_Subscriptions_customerNumberOnly(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "customerNumber=CUST-7") ||
			strings.Contains(r.URL.RawQuery, "serviceNumber=") {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":5000,"merchant":"CMSABC","payItemId":"PI","amountType":"FIXED","localCur":"XAF","name":"N","serviceNumber":"DEC-2","customerNumber":"CUST-7"}]`)
	})
	got, err := c.Initiate.Subscriptions(context.Background(), "CMSABC", 5000, "", "CUST-7")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].CustomerNumber != "CUST-7" {
		t.Errorf("got %+v", got)
	}
}

func TestInitiate_Subscriptions_bothNumbers(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "serviceNumber=DEC-1") ||
			!strings.Contains(r.URL.RawQuery, "customerNumber=CUST-7") {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	if _, err := c.Initiate.Subscriptions(context.Background(), "CMSABC", 5000, "DEC-1", "CUST-7"); err != nil {
		t.Fatal(err)
	}
}

func TestInitiate_Quote_validatesAmount(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.Initiate.Quote(context.Background(), QuoteRequest{Amount: 0, PayItemID: "X"}); err == nil {
		t.Error("expected error on amount<1")
	}
	if _, err := c.Initiate.Quote(context.Background(), QuoteRequest{Amount: 1, PayItemID: ""}); err == nil {
		t.Error("expected error on empty PayItemID")
	}
}

func TestInitiate_Quote_zeroAmount(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.Initiate.Quote(context.Background(), QuoteRequest{Amount: 0, PayItemID: "PI"})
	if err == nil {
		t.Fatal("expected error on amount=0")
	}
	if !strings.Contains(err.Error(), "amount") {
		t.Errorf("error = %v, want one mentioning amount", err)
	}
}

func TestInitiate_Quote_negativeAmount(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.Initiate.Quote(context.Background(), QuoteRequest{Amount: -5, PayItemID: "PI"})
	if err == nil {
		t.Fatal("expected error on negative amount")
	}
	if !strings.Contains(err.Error(), "amount") {
		t.Errorf("error = %v, want one mentioning amount", err)
	}
}

func TestInitiate_Quote_emptyPayItemID(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.Initiate.Quote(context.Background(), QuoteRequest{Amount: 100, PayItemID: ""})
	if err == nil {
		t.Fatal("expected error on empty PayItemID")
	}
	if !strings.Contains(err.Error(), "payItemId") {
		t.Errorf("error = %v, want one mentioning payItemId", err)
	}
}

func TestInitiate_Bills_zeroServiceID(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.Initiate.Bills(context.Background(), "ENEO", 0, "203157530")
	if err == nil {
		t.Fatal("expected error on serviceID=0")
	}
	if !strings.Contains(err.Error(), "serviceID") {
		t.Errorf("err = %v, want substring 'serviceID'", err)
	}
}

func TestInitiate_Subscriptions_zeroServiceID(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	_, err := c.Initiate.Subscriptions(context.Background(), "CMSABC", 0, "DEC-1", "")
	if err == nil {
		t.Fatal("expected error on serviceID=0")
	}
	if !strings.Contains(err.Error(), "serviceID") {
		t.Errorf("err = %v, want substring 'serviceID'", err)
	}
}

func TestInitiate_Quote(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v2/quotestd" {
			http.Error(w, "bad", 400)
			return
		}
		b, _ := io.ReadAll(r.Body)
		if string(b) != `{"amount":500,"payItemId":"PI"}` {
			t.Errorf("body = %s", b)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"quoteId":"q-1","expiresAt":"2024-01-15T10:35:00Z","payItemId":"PI","amountLocalCur":500,"priceLocalCur":510,"localCur":"XAF","systemCur":"XAF"}`)
	})
	q, err := c.Initiate.Quote(context.Background(), QuoteRequest{Amount: 500, PayItemID: "PI"})
	if err != nil {
		t.Fatal(err)
	}
	if q.QuoteID != "q-1" || q.PayItemID != "PI" {
		t.Errorf("got %+v", q)
	}
}
