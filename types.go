package smobilpay

import (
	"errors"
	"fmt"
	"time"
)

// Date is a date-only value tolerant of the multiple wire formats emitted
// by the Smobilpay API: "YYYY-MM-DD", "YYYY-MM-DDTHH:MM:SSZ", and
// "YYYY-MM-DDTHH:MM:SS±HH:MM". MarshalJSON emits the canonical
// "YYYY-MM-DD" form; a zero Date marshals to JSON null.
type Date struct {
	time.Time
}

var dateLayouts = []string{
	"2006-01-02",
	time.RFC3339,
	"2006-01-02T15:04:05",
}

// UnmarshalJSON tries each accepted layout in order.
func (d *Date) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return errors.New("smobilpay: Date must be a JSON string")
	}
	s := string(b[1 : len(b)-1])
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			d.Time = t
			return nil
		}
	}
	return fmt.Errorf("smobilpay: cannot parse %q as Date", s)
}

// MarshalJSON emits the canonical YYYY-MM-DD form; a zero value emits null.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Format("2006-01-02") + `"`), nil
}

// ServiceType drives which masterdata endpoint produces the matching
// payment items for a service.
type ServiceType string

const (
	ServiceTypeSearchableBill    ServiceType = "SEARCHABLE_BILL"
	ServiceTypeNonSearchableBill ServiceType = "NON_SEARCHABLE_BILL"
	ServiceTypeProduct           ServiceType = "PRODUCT"
	ServiceTypeTopup             ServiceType = "TOPUP"
	ServiceTypeSubscription      ServiceType = "SUBSCRIPTION"
	ServiceTypeCashin            ServiceType = "CASHIN"
	ServiceTypeCashout           ServiceType = "CASHOUT"
	ServiceTypeVoucher           ServiceType = "VOUCHER"
)

// AmountType describes how the payment amount is determined for a
// payment item.
//
//   - FIXED   — must be paid in full at AmountLocalCur.
//   - CUSTOM  — caller chooses the amount.
//   - PARTIAL — amount may be less than AmountLocalCur.
//   - OVERPAY — amount may exceed AmountLocalCur (subject to regulation).
type AmountType string

const (
	AmountTypeFixed   AmountType = "FIXED"
	AmountTypeCustom  AmountType = "CUSTOM"
	AmountTypePartial AmountType = "PARTIAL"
	AmountTypeOverpay AmountType = "OVERPAY"
)

// BillType classifies a Bill.
type BillType string

const (
	BillTypeRegular BillType = "REGULAR"
	BillTypeOverdue BillType = "OVERDUE"
)

// MerchantStatus reports a merchant's availability. The server emits
// mixed-case values; they are preserved verbatim.
type MerchantStatus string

const (
	MerchantStatusActive   MerchantStatus = "Active"
	MerchantStatusInactive MerchantStatus = "Inactive"
)

// ServiceStatus reports a service's availability. The server emits
// mixed-case values; they are preserved verbatim.
type ServiceStatus string

const (
	ServiceStatusActive   ServiceStatus = "Active"
	ServiceStatusInactive ServiceStatus = "Inactive"
)

// PaymentStatusType is the payment processing status. With request
// header x-api-version: 3.0.0, the server rewrites SUCCESS to PENDING on
// CollectionResponse.Status; poll VerifyTransaction or wait for the
// callback webhook for the final status.
type PaymentStatusType string

const (
	PaymentStatusReversed PaymentStatusType = "REVERSED"
	PaymentStatusPending  PaymentStatusType = "PENDING"
	PaymentStatusErrored  PaymentStatusType = "ERRORED"
	PaymentStatusSuccess  PaymentStatusType = "SUCCESS"
)

// PaymentItem is the common interface implemented by every payment-item
// type returned by the masterdata and lookup endpoints (Cashout, Cashin,
// Topup, Product, Bill, Subscription). PayItemID is the value passed to
// QuoteRequest to obtain pricing.
type PaymentItem interface {
	ServiceID() int64
	Merchant() string
	PayItemID() string
	PayItemDescr() string
	AmountType() AmountType
	LocalCur() string
	Name() string
	AmountLocalCur() *float64
	Description() string
	OptStrg() string
	OptNmb() *float64
}

// I18nText is a localized text entry used for service hints and field
// labels.
type I18nText struct {
	Language  string `json:"language"`
	LocalText string `json:"localText"`
}
