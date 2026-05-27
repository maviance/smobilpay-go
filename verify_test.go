package smobilpay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newMockClient stands up an httptest server that always responds to the
// OAuth token mint and routes all other paths to handler.
func newMockClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"jwt-X","token_type":"Bearer","expires_in":3600}`)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	cfg, err := NewConfig(WithBaseURL(srv.URL), WithCredentials("pub", "sec"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestVerify_Ping(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v2/ping" {
			http.Error(w, "bad", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"time":"2024-01-15T10:30:00Z","version":"3.0.0","nonce":"n1","key":"k1"}`)
	})
	p, err := c.Verify.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if p.Version != "3.0.0" || p.Nonce != "n1" || p.Key != "k1" {
		t.Errorf("got %+v", p)
	}
	if y, m, d := p.Time.Date(); y != 2024 || m != time.January || d != 15 {
		t.Errorf("time = %v", p.Time)
	}
}

func TestVerify_Account(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"balance":1000.5,"currency":"XAF","key":"k","agentId":"a1","agentName":"A","agentAddress":"x","agentPhonenumber":"237699999999","companyName":"C","companyAddress":"y","companyPhonenumber":"237699999998","limitMax":10000,"limitRemaining":9000}`)
	})
	a, err := c.Verify.Account(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a.Currency != "XAF" || a.Balance != 1000.5 || a.AgentID != "a1" {
		t.Errorf("got %+v", a)
	}
}

func TestVerify_VerifyTransaction_requiresOneParam(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.Verify.VerifyTransaction(context.Background(), "", ""); err == nil {
		t.Error("expected error when both ptn and trid are empty")
	}
}

func TestVerify_VerifyTransaction_byPtn(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "ptn=P-1" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"ptn":"P-1","serviceid":"20053","merchant":"M","timestamp":"2024-01-15T10:30:00Z","receiptNumber":"r","veriCode":"v","trid":"t","priceLocalCur":500,"priceSystemCur":500,"localCur":"XAF","systemCur":"XAF","status":"SUCCESS","payItemId":"pi","errorCode":0}]`)
	})
	got, err := c.Verify.VerifyTransaction(context.Background(), "P-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PTN != "P-1" || got[0].Status != PaymentStatusSuccess {
		t.Errorf("got %+v", got)
	}
}

func TestVerify_HistoryByDateRange_validatesOrder(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	from := time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	if _, err := c.Verify.HistoryByDateRange(context.Background(), from, to); err == nil {
		t.Error("expected error when to < from")
	}
}

func TestVerify_HistoryByDateRange_sendsTimestamps(t *testing.T) {
	var captured string
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	from := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 5, 31, 0, 0, 0, 0, time.UTC)
	if _, err := c.Verify.HistoryByDateRange(context.Background(), from, to); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(captured, "timestamp_from=2024-05-01") ||
		!strings.Contains(captured, "timestamp_to=2024-05-31") {
		t.Errorf("query = %q", captured)
	}
}

func TestVerify_HistoryByPtn_requiresPtn(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.Verify.HistoryByPtn(context.Background(), ""); err == nil {
		t.Error("expected error on empty ptn")
	}
}

func TestVerify_HistoryByPtn_sendsPtn(t *testing.T) {
	var captured string
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	if _, err := c.Verify.HistoryByPtn(context.Background(), "P-7"); err != nil {
		t.Fatal(err)
	}
	if captured != "ptn=P-7" {
		t.Errorf("query = %q, want ptn=P-7", captured)
	}
}

func TestVerify_HistoryByTrid_requiresTrid(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.Verify.HistoryByTrid(context.Background(), ""); err == nil {
		t.Error("expected error on empty trid")
	}
}

func TestVerify_HistoryByTrid_sendsTrid(t *testing.T) {
	var captured string
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	if _, err := c.Verify.HistoryByTrid(context.Background(), "ORDER-1"); err != nil {
		t.Fatal(err)
	}
	if captured != "trid=ORDER-1" {
		t.Errorf("query = %q, want trid=ORDER-1", captured)
	}
}

// Ensure PaymentStatus.ServiceID stays a string (per partner spec) even
// though Service.ServiceID elsewhere is int64.
func TestPaymentStatus_ServiceIDIsString(t *testing.T) {
	raw := `{"ptn":"P","serviceid":"99","merchant":"M","timestamp":"2024-01-01T00:00:00Z","receiptNumber":"r","veriCode":"v","trid":"t","priceLocalCur":1,"priceSystemCur":1,"localCur":"X","systemCur":"X","status":"SUCCESS","payItemId":"p","errorCode":0}`
	var s PaymentStatus
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	if s.ServiceID != "99" {
		t.Errorf("ServiceID = %q", s.ServiceID)
	}
}
