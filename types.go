package smobilpay

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Date is a date-only value tolerant of the multiple wire formats emitted
// by the Smobilpay API. UnmarshalJSON accepts, in order of preference:
//
//   - "YYYY-MM-DD"                  — the canonical date-only form.
//   - "YYYY-MM-DDTHH:MM:SSZ"        — RFC 3339 with UTC offset.
//   - "YYYY-MM-DDTHH:MM:SS±HH:MM"   — RFC 3339 with a non-UTC offset.
//   - "YYYY-MM-DDTHH:MM:SS"         — defensive fallback for naive datetimes.
//
// JSON null decodes into a zero-valued Date. Invalid or non-string input
// returns an error.
//
// MarshalJSON emits "YYYY-MM-DD" in whatever timezone the underlying
// time.Time was parsed in — so a "2024-01-15T00:30:00+01:00" round-trips
// back to "2024-01-15", not the UTC-shifted "2024-01-14". A zero Date
// marshals to JSON null.
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
	if string(b) == "null" {
		return nil
	}
	if len(b) == 0 {
		return errors.New("smobilpay: Date.UnmarshalJSON: empty input")
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

// PaymentItem is the common interface implemented by every catalog
// payment-item type — Cashout, Cashin, Topup, Product (covers both
// /v2/product and /v2/voucher), Bill, and Subscription. These all
// share the shape returned by the masterdata and lookup endpoints and
// can be uniformly fed into a QuoteRequest.
//
// PaymentStatus (from /v2/historystd and /v2/verifytx) is intentionally
// NOT a PaymentItem: it reports the state of a completed transaction,
// not a quotable catalog entry, and its serviceid is a string per the
// partner spec rather than int64.
//
// PayItemID is the value passed to QuoteRequest to obtain pricing.
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

// ---------------------------------------------------------------------------
// Lenient JSON helpers.
//
// The Smobilpay acceptance server diverges from its own OpenAPI spec by
// serializing several numeric fields as quoted JSON strings (e.g. an
// account's limitMax comes through as "100000000.00", and serviceid is
// "20053" across most catalog endpoints). The Service masterdata also
// emits the isReq* flags as JSON numbers 0/1 rather than booleans.
//
// These unexported helper types absorb both the spec-correct native
// form AND the actual wire form. They are plugged into the affected
// DTOs via custom UnmarshalJSON methods (alias-shadow pattern) so the
// public field types stay clean.
// ---------------------------------------------------------------------------

// lenientInt64 decodes either a JSON integer or a quoted string holding an
// integer. Empty string and JSON null decode to 0.
type lenientInt64 int64

func (l *lenientInt64) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*l = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*l = 0
			return nil
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("smobilpay: lenientInt64: cannot parse %q: %w", s, err)
		}
		*l = lenientInt64(v)
		return nil
	}
	var v int64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*l = lenientInt64(v)
	return nil
}

// lenientFloat64 decodes either a JSON number or a quoted string holding
// a numeric value. Empty string and JSON null decode to 0.
type lenientFloat64 float64

func (l *lenientFloat64) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*l = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*l = 0
			return nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("smobilpay: lenientFloat64: cannot parse %q: %w", s, err)
		}
		*l = lenientFloat64(v)
		return nil
	}
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*l = lenientFloat64(v)
	return nil
}

// lenientFloat64Ptr decodes the same forms as lenientFloat64 plus JSON
// null, surfacing the result as a *float64 (nil for null/missing).
type lenientFloat64Ptr struct{ V *float64 }

func (l *lenientFloat64Ptr) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		l.V = nil
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			l.V = nil
			return nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("smobilpay: lenientFloat64Ptr: cannot parse %q: %w", s, err)
		}
		l.V = &v
		return nil
	}
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	l.V = &v
	return nil
}

// lenientBool decodes booleans from native JSON booleans, JSON numbers
// (0=false, non-zero=true), or quoted strings ("true"/"false"/"1"/"0"/"").
type lenientBool bool

func (l *lenientBool) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*l = false
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		switch s {
		case "", "0", "false", "f", "False", "FALSE":
			*l = false
		case "1", "true", "t", "True", "TRUE":
			*l = true
		default:
			return fmt.Errorf("smobilpay: lenientBool: cannot parse %q", s)
		}
		return nil
	}
	// Try number (0/1) first; fall through to bool.
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*l = lenientBool(n != 0)
		return nil
	}
	var v bool
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*l = lenientBool(v)
	return nil
}
