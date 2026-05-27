package smobilpay

import (
	"context"
	"errors"
	"time"

	"github.com/maviance/smobilpay-go/internal/apiclient"
)

// Ping is the response from GET /v2/ping — an authenticated round-trip
// probe that echoes server time, version, request nonce, and the public
// access key used to authenticate.
type Ping struct {
	Time    time.Time `json:"time"`
	Version string    `json:"version"`
	Nonce   string    `json:"nonce"`
	Key     string    `json:"key"`
}

// Account is the authenticated agent's profile, returned by GET /v2/account.
type Account struct {
	Balance            float64 `json:"balance"`
	Currency           string  `json:"currency"`
	Key                string  `json:"key"`
	AgentID            string  `json:"agentId"`
	AgentName          string  `json:"agentName"`
	AgentAddress       string  `json:"agentAddress"`
	AgentPhoneNumber   string  `json:"agentPhonenumber"`
	CompanyName        string  `json:"companyName"`
	CompanyAddress     string  `json:"companyAddress"`
	CompanyPhoneNumber string  `json:"companyPhonenumber"`
	LimitMax           float64 `json:"limitMax"`
	LimitRemaining     float64 `json:"limitRemaining"`
}

// Commission is the optional commission record on PaymentStatus.
type Commission struct {
	Earnings *float64 `json:"earnings"`
	Currency string   `json:"currency"`
}

// PaymentStatus is the current state of a previously-issued payment
// collection. ServiceID is a string here per the partner spec (it is an
// int64 elsewhere — e.g. Service.ServiceID).
type PaymentStatus struct {
	PTN            string            `json:"ptn"`
	ServiceID      string            `json:"serviceid"`
	Merchant       string            `json:"merchant"`
	Timestamp      time.Time         `json:"timestamp"`
	ReceiptNumber  string            `json:"receiptNumber"`
	VeriCode       string            `json:"veriCode"`
	ClearingDate   Date              `json:"clearingDate"`
	TRID           string            `json:"trid"`
	PriceLocalCur  *float64          `json:"priceLocalCur"`
	PriceSystemCur *float64          `json:"priceSystemCur"`
	LocalCur       string            `json:"localCur"`
	SystemCur      string            `json:"systemCur"`
	PIN            string            `json:"pin,omitempty"`
	Status         PaymentStatusType `json:"status"`
	PayItemID      string            `json:"payItemId"`
	PayItemDescr   string            `json:"payItemDescr,omitempty"`
	ErrorCode      int               `json:"errorCode"`
	Tag            string            `json:"tag,omitempty"`
	Commission     *Commission       `json:"commission,omitempty"`
}

// Ping returns the result of GET /v2/ping. Used as an auth probe.
func (v *VerifyAPI) Ping(ctx context.Context) (Ping, error) {
	var out Ping
	err := v.tr.Get(ctx, "/v2/ping", apiclient.NewQuery(), &out)
	return out, err
}

// Account returns the authenticated agent's profile (GET /v2/account).
func (v *VerifyAPI) Account(ctx context.Context) (Account, error) {
	var out Account
	err := v.tr.Get(ctx, "/v2/account", apiclient.NewQuery(), &out)
	return out, err
}

// VerifyTransaction looks up the live status of a previously-issued
// collection by PTN, by caller TRID, or both. At least one of the two
// must be non-empty.
func (v *VerifyAPI) VerifyTransaction(ctx context.Context, ptn, trid string) ([]PaymentStatus, error) {
	if ptn == "" && trid == "" {
		return nil, errors.New("smobilpay: VerifyTransaction requires ptn or trid")
	}
	q := apiclient.NewQuery().Add("ptn", ptn).Add("trid", trid)
	var out []PaymentStatus
	err := v.tr.Get(ctx, "/v2/verifytx", q, &out)
	return out, err
}

// HistoryByPtn returns the history record for a specific PTN (GET /v2/historystd).
func (v *VerifyAPI) HistoryByPtn(ctx context.Context, ptn string) ([]PaymentStatus, error) {
	if ptn == "" {
		return nil, errors.New("smobilpay: HistoryByPtn requires ptn")
	}
	q := apiclient.NewQuery().Add("ptn", ptn)
	var out []PaymentStatus
	err := v.tr.Get(ctx, "/v2/historystd", q, &out)
	return out, err
}

// HistoryByTrid returns history records for a caller TRID.
func (v *VerifyAPI) HistoryByTrid(ctx context.Context, trid string) ([]PaymentStatus, error) {
	if trid == "" {
		return nil, errors.New("smobilpay: HistoryByTrid requires trid")
	}
	q := apiclient.NewQuery().Add("trid", trid)
	var out []PaymentStatus
	err := v.tr.Get(ctx, "/v2/historystd", q, &out)
	return out, err
}

// HistoryByDateRange returns history records over an inclusive date
// range. The dates are sent as YYYY-MM-DD (UTC calendar date); any
// time-of-day component on from/to is silently dropped on the wire.
// This matches the partner spec (format: date) and the Java client.
func (v *VerifyAPI) HistoryByDateRange(ctx context.Context, from, to time.Time) ([]PaymentStatus, error) {
	if from.IsZero() || to.IsZero() {
		return nil, errors.New("smobilpay: HistoryByDateRange requires both from and to")
	}
	if to.Before(from) {
		return nil, errors.New("smobilpay: HistoryByDateRange: 'to' is before 'from'")
	}
	q := apiclient.NewQuery().
		Add("timestamp_from", from.UTC().Format("2006-01-02")).
		Add("timestamp_to", to.UTC().Format("2006-01-02"))
	var out []PaymentStatus
	err := v.tr.Get(ctx, "/v2/historystd", q, &out)
	return out, err
}
