package smobilpay

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestMasterdata_Merchants(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/merchant" {
			http.Error(w, "bad", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"merchant":"ENEO","name":"ENEO","description":"d","country":"CMR","status":"Active","logo":"u","logoHash":"h"}]`)
	})
	got, err := c.Masterdata.Merchants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Merchant != "ENEO" || got[0].Status != MerchantStatusActive {
		t.Errorf("got %+v", got)
	}
}

func TestMasterdata_Services(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":10039,"merchant":"ENEO","title":"ENEO","description":"d","category":"c","country":"CMR","localCur":"XAF","type":"SEARCHABLE_BILL","status":"Active","isReqCustomerName":false,"isReqCustomerAddress":false,"isReqCustomerNumber":false,"isReqServiceNumber":true,"isVerifiable":false}]`)
	})
	got, err := c.Masterdata.Services(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ServiceID != 10039 || got[0].Type != ServiceTypeSearchableBill {
		t.Errorf("got %+v", got)
	}
}

func TestMasterdata_Cashouts_sendsServiceID(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "serviceid=20053" {
			t.Errorf("query = %q, want serviceid=20053", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":20053,"merchant":"MOMO","payItemId":"PI","amountType":"FIXED","localCur":"XAF","name":"N"}]`)
	})
	got, err := c.Masterdata.Cashouts(context.Background(), 20053)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PayItemID() != "PI" {
		t.Errorf("got %+v", got)
	}
}

func TestMasterdata_Cashouts_omitsZeroServiceID(t *testing.T) {
	var capturedQuery string
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	if _, err := c.Masterdata.Cashouts(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if capturedQuery != "" {
		t.Errorf("expected no query for zero serviceID, got %q", capturedQuery)
	}
}

func TestPaymentItem_interfaceSatisfied(t *testing.T) {
	// Compile-time check: every concrete catalog type must satisfy
	// PaymentItem. The asserts blow up at compile time if not.
	var (
		_ PaymentItem = (*Cashout)(nil)
		_ PaymentItem = (*Cashin)(nil)
		_ PaymentItem = (*Topup)(nil)
		_ PaymentItem = (*Product)(nil)
		_ PaymentItem = (*Bill)(nil)
		_ PaymentItem = (*Subscription)(nil)
	)
}

func TestMasterdata_Products(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/product" || r.URL.RawQuery != "serviceid=90006" {
			t.Errorf("path=%q, query=%q", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":90006,"merchant":"CANAL","payItemId":"PI-PROD","amountType":"FIXED","localCur":"XAF","name":"Canal+"}]`)
	})
	got, err := c.Masterdata.Products(context.Background(), 90006)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PayItemID() != "PI-PROD" {
		t.Errorf("got %+v", got)
	}
}

func TestMasterdata_Vouchers(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/voucher" || r.URL.RawQuery != "serviceid=90041" {
			t.Errorf("path=%q, query=%q", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":90041,"merchant":"ENEOP","payItemId":"PI-VOU","amountType":"CUSTOM","localCur":"XAF","name":"ENEO prepaid"}]`)
	})
	got, err := c.Masterdata.Vouchers(context.Background(), 90041)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PayItemID() != "PI-VOU" {
		t.Errorf("got %+v", got)
	}
}

func TestMasterdata_Topups(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/topup" || r.URL.RawQuery != "serviceid=20051" {
			t.Errorf("path=%q, query=%q", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":20051,"merchant":"CMMTN","payItemId":"PI-TOPUP","amountType":"CUSTOM","localCur":"XAF","name":"MTN airtime"}]`)
	})
	got, err := c.Masterdata.Topups(context.Background(), 20051)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PayItemID() != "PI-TOPUP" {
		t.Errorf("got %+v", got)
	}
}

func TestMasterdata_Cashins(t *testing.T) {
	c := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/cashin" || r.URL.RawQuery != "serviceid=50052" {
			t.Errorf("path=%q, query=%q", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":50052,"merchant":"CMORANGEMOMO","payItemId":"PI-CASHIN","amountType":"CUSTOM","localCur":"XAF","name":"Orange Cash-In"}]`)
	})
	got, err := c.Masterdata.Cashins(context.Background(), 50052)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PayItemID() != "PI-CASHIN" {
		t.Errorf("got %+v", got)
	}
}

func TestPaymentItem_accessorsReturnEmbeddedValues(t *testing.T) {
	amount := 500.0
	opt := 1.5
	item := Cashout{paymentItemBase{
		ServiceIDValue:      99,
		MerchantValue:       "MERCH",
		PayItemIDValue:      "PI-99",
		PayItemDescrValue:   "desc",
		AmountTypeValue:     AmountTypeFixed,
		LocalCurValue:       "XAF",
		NameValue:           "Item",
		AmountLocalCurValue: &amount,
		DescriptionValue:    "description",
		OptStrgValue:        "opt-string",
		OptNmbValue:         &opt,
	}}
	// Exercise every PaymentItem method through the interface to ensure
	// the embedded paymentItemBase methods are reachable on concrete types.
	var p PaymentItem = &item
	if p.ServiceID() != 99 {
		t.Errorf("ServiceID = %d", p.ServiceID())
	}
	if p.Merchant() != "MERCH" {
		t.Errorf("Merchant = %q", p.Merchant())
	}
	if p.PayItemID() != "PI-99" {
		t.Errorf("PayItemID = %q", p.PayItemID())
	}
	if p.PayItemDescr() != "desc" {
		t.Errorf("PayItemDescr = %q", p.PayItemDescr())
	}
	if p.AmountType() != AmountTypeFixed {
		t.Errorf("AmountType = %q", p.AmountType())
	}
	if p.LocalCur() != "XAF" {
		t.Errorf("LocalCur = %q", p.LocalCur())
	}
	if p.Name() != "Item" {
		t.Errorf("Name = %q", p.Name())
	}
	if p.AmountLocalCur() == nil || *p.AmountLocalCur() != 500.0 {
		t.Errorf("AmountLocalCur = %v", p.AmountLocalCur())
	}
	if p.Description() != "description" {
		t.Errorf("Description = %q", p.Description())
	}
	if p.OptStrg() != "opt-string" {
		t.Errorf("OptStrg = %q", p.OptStrg())
	}
	if p.OptNmb() == nil || *p.OptNmb() != 1.5 {
		t.Errorf("OptNmb = %v", p.OptNmb())
	}
}
