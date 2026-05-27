package smobilpay

import (
	"context"
	"errors"
	"time"

	"github.com/maviance/smobilpay-go/internal/apiclient"
)

// Bill is a bill payment item returned by /v2/bill.
type Bill struct {
	paymentItemBase
	BillType       BillType `json:"billType"`
	PenaltyAmount  *float64 `json:"penaltyAmount,omitempty"`
	PayOrder       int      `json:"payOrder"`
	ServiceNumber  string   `json:"serviceNumber"`
	BillNumber     string   `json:"billNumber,omitempty"`
	CustomerNumber string   `json:"customerNumber,omitempty"`
	BillMonth      string   `json:"billMonth,omitempty"`
	BillYear       string   `json:"billYear,omitempty"`
	BillDate       Date     `json:"billDate,omitempty"`
	BillDueDate    Date     `json:"billDueDate,omitempty"`
}

// Subscription is a subscription payment item returned by /v2/subscription.
type Subscription struct {
	paymentItemBase
	ServiceNumber     string `json:"serviceNumber"`
	CustomerReference string `json:"customerReference,omitempty"`
	CustomerName      string `json:"customerName,omitempty"`
	CustomerNumber    string `json:"customerNumber,omitempty"`
	StartDate         Date   `json:"startDate,omitempty"`
	DueDate           Date   `json:"dueDate,omitempty"`
	EndDate           Date   `json:"endDate,omitempty"`
}

// QuoteRequest is the body for POST /v2/quotestd. Amount must be >= 1.
type QuoteRequest struct {
	Amount    int    `json:"amount"`
	PayItemID string `json:"payItemId"`
}

// QuoteResponse is the response from POST /v2/quotestd. QuoteID is the
// value to pass to Confirm.Collect. Honor ExpiresAt — quotes are
// short-lived; on HTTP 498 re-quote and retry.
type QuoteResponse struct {
	QuoteID        string    `json:"quoteId"`
	ExpiresAt      time.Time `json:"expiresAt"`
	PayItemID      string    `json:"payItemId"`
	AmountLocalCur *float64  `json:"amountLocalCur,omitempty"`
	PriceLocalCur  *float64  `json:"priceLocalCur,omitempty"`
	PriceSystemCur *float64  `json:"priceSystemCur,omitempty"`
	LocalCur       string    `json:"localCur"`
	SystemCur      string    `json:"systemCur"`
	Promotion      string    `json:"promotion,omitempty"`
}

// Bills searches bills for a service number. For SEARCHABLE_BILL
// services this may return multiple open bills; NON_SEARCHABLE_BILL
// always returns one.
func (i *InitiateAPI) Bills(ctx context.Context, merchant string, serviceID int64, serviceNumber string) ([]Bill, error) {
	if merchant == "" {
		return nil, errors.New("smobilpay: Bills requires merchant")
	}
	if serviceID <= 0 {
		return nil, errors.New("smobilpay: Bills requires serviceID > 0")
	}
	if serviceNumber == "" {
		return nil, errors.New("smobilpay: Bills requires serviceNumber")
	}
	q := apiclient.NewQuery().
		Add("merchant", merchant).
		Add("serviceid", serviceID).
		Add("serviceNumber", serviceNumber)
	var out []Bill
	err := i.tr.Get(ctx, "/v2/bill", q, &out)
	return out, err
}

// Subscriptions searches subscriptions by service number, customer
// number, or both. At least one of the two must be non-empty.
func (i *InitiateAPI) Subscriptions(ctx context.Context, merchant string, serviceID int64, serviceNumber, customerNumber string) ([]Subscription, error) {
	if merchant == "" {
		return nil, errors.New("smobilpay: Subscriptions requires merchant")
	}
	if serviceID <= 0 {
		return nil, errors.New("smobilpay: Subscriptions requires serviceID > 0")
	}
	if serviceNumber == "" && customerNumber == "" {
		return nil, errors.New("smobilpay: Subscriptions requires serviceNumber or customerNumber")
	}
	q := apiclient.NewQuery().
		Add("merchant", merchant).
		Add("serviceid", serviceID).
		Add("serviceNumber", serviceNumber).
		Add("customerNumber", customerNumber)
	var out []Subscription
	err := i.tr.Get(ctx, "/v2/subscription", q, &out)
	return out, err
}

// Quote requests a price quote for a payment collection. Quotes are
// short-lived; on HTTP 498 (APIError) re-quote before retrying.
func (i *InitiateAPI) Quote(ctx context.Context, req QuoteRequest) (QuoteResponse, error) {
	if req.Amount < 1 {
		return QuoteResponse{}, errors.New("smobilpay: Quote: amount must be >= 1")
	}
	if req.PayItemID == "" {
		return QuoteResponse{}, errors.New("smobilpay: Quote: payItemId is required")
	}
	var out QuoteResponse
	err := i.tr.Post(ctx, "/v2/quotestd", req, &out)
	return out, err
}
