package smobilpay

import (
	"context"
	"fmt"
	"net/mail"
	"regexp"
	"time"
)

var phoneDigitsOnly = regexp.MustCompile(`^[0-9]+$`)

// CollectionRequest is the body for POST /v2/collectstd.
//
// QuoteID (from a prior Initiate.Quote call; UUID format),
// CustomerPhoneNumber (digits only, no leading +, E.164 numeric form
// e.g. "237699999999"), and CustomerEmailAddress (valid email per
// RFC 5322) are required. All other fields are required only when
// the chosen Service sets the corresponding IsReq* flag.
//
// Tag <= 50 chars, CallbackURL <= 255 chars.
type CollectionRequest struct {
	QuoteID              string `json:"quoteId"`
	CustomerPhoneNumber  string `json:"customerPhonenumber"`
	CustomerEmailAddress string `json:"customerEmailaddress"`
	CustomerName         string `json:"customerName,omitempty"`
	CustomerAddress      string `json:"customerAddress,omitempty"`
	CustomerNumber       string `json:"customerNumber,omitempty"`
	ServiceNumber        string `json:"serviceNumber,omitempty"`
	TRID                 string `json:"trid,omitempty"`
	Tag                  string `json:"tag,omitempty"`
	CallbackURL          string `json:"callbackUrl,omitempty"`
	CData                string `json:"cdata,omitempty"`
}

func (r CollectionRequest) validate() error {
	if r.QuoteID == "" {
		return fmt.Errorf("smobilpay: CollectionRequest: quoteId is required")
	}
	if r.CustomerPhoneNumber == "" {
		return fmt.Errorf("smobilpay: CollectionRequest: customerPhonenumber is required")
	}
	if !phoneDigitsOnly.MatchString(r.CustomerPhoneNumber) {
		return fmt.Errorf("smobilpay: CollectionRequest: customerPhonenumber must contain digits only (got %q)", r.CustomerPhoneNumber)
	}
	if r.CustomerEmailAddress == "" {
		return fmt.Errorf("smobilpay: CollectionRequest: customerEmailaddress is required")
	}
	if _, err := mail.ParseAddress(r.CustomerEmailAddress); err != nil {
		return fmt.Errorf("smobilpay: CollectionRequest: customerEmailaddress is not a valid email address: %w", err)
	}
	if len(r.Tag) > 50 {
		return fmt.Errorf("smobilpay: CollectionRequest: tag must be <= 50 chars (got %d)", len(r.Tag))
	}
	if len(r.CallbackURL) > 255 {
		return fmt.Errorf("smobilpay: CollectionRequest: callbackUrl must be <= 255 chars (got %d)", len(r.CallbackURL))
	}
	return nil
}

// CollectionResponse confirms a payment collection. With request
// x-api-version: 3.0.0, a SUCCESS status is rewritten to PENDING
// server-side; poll Verify.VerifyTransaction or wait for the callback
// webhook to learn the final status.
type CollectionResponse struct {
	PTN            string            `json:"ptn"`
	Timestamp      time.Time         `json:"timestamp"`
	AgentBalance   *float64          `json:"agentBalance,omitempty"`
	ReceiptNumber  string            `json:"receiptNumber"`
	VeriCode       string            `json:"veriCode"`
	PriceLocalCur  *float64          `json:"priceLocalCur,omitempty"`
	PriceSystemCur *float64          `json:"priceSystemCur,omitempty"`
	LocalCur       string            `json:"localCur"`
	SystemCur      string            `json:"systemCur"`
	TRID           string            `json:"trid,omitempty"`
	PIN            string            `json:"pin,omitempty"`
	Status         PaymentStatusType `json:"status"`
	PayItemID      string            `json:"payItemId,omitempty"`
	PayItemDescr   string            `json:"payItemDescr,omitempty"`
	Tag            string            `json:"tag,omitempty"`
}

// Collect executes a payment collection against a valid (unexpired)
// quote. A 498 APIError indicates the quote has expired; re-quote
// before retrying.
func (c *ConfirmAPI) Collect(ctx context.Context, req CollectionRequest) (CollectionResponse, error) {
	if err := req.validate(); err != nil {
		return CollectionResponse{}, err
	}
	var out CollectionResponse
	err := c.tr.Post(ctx, "/v2/collectstd", req, &out)
	return out, err
}
