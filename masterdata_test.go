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
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":20053,"merchant":"MOMO","payItemId":"PI","amountType":"FIXED","localCur":"XAF","name":"N"}]`)
	})
	got, err := c.Masterdata.Cashouts(context.Background(), 20053)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PayItemIDValue != "PI" {
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
