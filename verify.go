package smobilpay

import (
	"context"
	"encoding/json"
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

// UnmarshalJSON decodes Account, tolerating the acceptance server's
// habit of serializing balance / limitMax / limitRemaining as quoted
// strings rather than native JSON numbers (in violation of the
// partner OpenAPI spec).
func (a *Account) UnmarshalJSON(data []byte) error {
	type Alias Account
	aux := struct {
		Balance        lenientFloat64 `json:"balance"`
		LimitMax       lenientFloat64 `json:"limitMax"`
		LimitRemaining lenientFloat64 `json:"limitRemaining"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	a.Balance = float64(aux.Balance)
	a.LimitMax = float64(aux.LimitMax)
	a.LimitRemaining = float64(aux.LimitRemaining)
	return nil
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

// UnmarshalJSON decodes PaymentStatus, tolerating the acceptance
// server's habit of serializing priceLocalCur and priceSystemCur as
// quoted strings.
func (p *PaymentStatus) UnmarshalJSON(data []byte) error {
	type Alias PaymentStatus
	aux := struct {
		PriceLocalCur  lenientFloat64Ptr `json:"priceLocalCur"`
		PriceSystemCur lenientFloat64Ptr `json:"priceSystemCur"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	p.PriceLocalCur = aux.PriceLocalCur.V
	p.PriceSystemCur = aux.PriceSystemCur.V
	return nil
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
// range. from is sent as YYYY-MM-DDT00:00:00Z (UTC start-of-day) and
// to as YYYY-MM-DDT23:59:59Z (UTC end-of-day); the server requires
// ISO-8601 datetime (the spec's "format: date" is misleading — a
// date-only string is rejected with respCode 40302).
func (v *VerifyAPI) HistoryByDateRange(ctx context.Context, from, to time.Time) ([]PaymentStatus, error) {
	if from.IsZero() || to.IsZero() {
		return nil, errors.New("smobilpay: HistoryByDateRange requires both from and to")
	}
	if to.Before(from) {
		return nil, errors.New("smobilpay: HistoryByDateRange: 'to' is before 'from'")
	}
	fromTS := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	toTS := time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 0, time.UTC).Format(time.RFC3339)
	q := apiclient.NewQuery().
		Add("timestamp_from", fromTS).
		Add("timestamp_to", toTS)
	var out []PaymentStatus
	err := v.tr.Get(ctx, "/v2/historystd", q, &out)
	return out, err
}
