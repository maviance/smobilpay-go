// Package main implements the Smobilpay Go smoke-test runner.
//
// The runner consumes a JSON config file whose schema is drop-in compatible
// with the Java and Node.js clients' smoke-test.json — so a single config can
// drive all three runners and let us diff their output.
package main

import "errors"

// SmokeConfig is the JSON schema for the smoke-test config file —
// identical to the Java client's SmokeTestConfig and the Node.js client's
// smoke-test.json so a single JSON file works for every runner.
//
// Each per-flow pointer field is optional: set it to null (or omit it) to
// skip that scenario. Collection-style flows can additionally opt into a
// real /v2/collectstd call by setting Collect=true and the customer fields
// on the relevant block — see smoke-test.example.json for the canonical
// example.
type SmokeConfig struct {
	BaseURL    string `json:"baseUrl"`
	PublicKey  string `json:"publicKey"`
	SecretKey  string `json:"secretKey"`
	APIVersion string `json:"apiVersion,omitempty"`

	Cashout      *CashoutCfg      `json:"cashout,omitempty"`
	Bill         *BillCfg         `json:"bill,omitempty"`
	Topup        *TopupCfg        `json:"topup,omitempty"`
	Voucher      *VoucherCfg      `json:"voucher,omitempty"`
	Product      *ProductCfg      `json:"product,omitempty"`
	Subscription *SubscriptionCfg `json:"subscription,omitempty"`
	Cashin       *CashinCfg       `json:"cashin,omitempty"`
	Verify       *VerifyCfg       `json:"verify,omitempty"`
	// Validation drives the account-validation scenario. The Go field is named
	// Validation (not Validate) to avoid a name clash with the Validate()
	// method below; the JSON key remains "validate" so the schema stays
	// drop-in compatible with the Java/Node configs.
	Validation *ValidateCfg `json:"validate,omitempty"`
}

// CashoutCfg drives the cashout (collection) scenario — money flows OUT of
// the customer wallet.
type CashoutCfg struct {
	ServiceID int64 `json:"serviceId"`
	Amount    int   `json:"amount"`

	// Collect opt-in fields.
	Collect              bool   `json:"collect,omitempty"`
	CustomerPhoneNumber  string `json:"customerPhonenumber,omitempty"`
	CustomerEmailAddress string `json:"customerEmailaddress,omitempty"`
	ServiceNumber        string `json:"serviceNumber,omitempty"`
	CustomerName         string `json:"customerName,omitempty"`
	CustomerAddress      string `json:"customerAddress,omitempty"`
	CustomerNumber       string `json:"customerNumber,omitempty"`
	TRID                 string `json:"trid,omitempty"`
	Tag                  string `json:"tag,omitempty"`
	CallbackURL          string `json:"callbackUrl,omitempty"`
	CData                string `json:"cdata,omitempty"`
}

// BillCfg drives the bill payment scenario. ServiceNumber is required for
// SEARCHABLE_BILL discovery, so it is declared block-side (not as a
// collect-only field).
type BillCfg struct {
	Merchant      string `json:"merchant"`
	ServiceID     int64  `json:"serviceId"`
	ServiceNumber string `json:"serviceNumber"`

	// Collect opt-in fields.
	Collect              bool   `json:"collect,omitempty"`
	CustomerPhoneNumber  string `json:"customerPhonenumber,omitempty"`
	CustomerEmailAddress string `json:"customerEmailaddress,omitempty"`
	CustomerName         string `json:"customerName,omitempty"`
	CustomerAddress      string `json:"customerAddress,omitempty"`
	CustomerNumber       string `json:"customerNumber,omitempty"`
	TRID                 string `json:"trid,omitempty"`
	Tag                  string `json:"tag,omitempty"`
	CallbackURL          string `json:"callbackUrl,omitempty"`
	CData                string `json:"cdata,omitempty"`
}

// TopupCfg drives the airtime top-up scenario.
type TopupCfg struct {
	ServiceID int64 `json:"serviceId"`
	Amount    int   `json:"amount"`

	// Collect opt-in fields.
	Collect              bool   `json:"collect,omitempty"`
	CustomerPhoneNumber  string `json:"customerPhonenumber,omitempty"`
	CustomerEmailAddress string `json:"customerEmailaddress,omitempty"`
	ServiceNumber        string `json:"serviceNumber,omitempty"`
	CustomerName         string `json:"customerName,omitempty"`
	CustomerAddress      string `json:"customerAddress,omitempty"`
	CustomerNumber       string `json:"customerNumber,omitempty"`
	TRID                 string `json:"trid,omitempty"`
	Tag                  string `json:"tag,omitempty"`
	CallbackURL          string `json:"callbackUrl,omitempty"`
	CData                string `json:"cdata,omitempty"`
}

// VoucherCfg drives the voucher purchase scenario.
type VoucherCfg struct {
	ServiceID int64 `json:"serviceId"`
	Amount    int   `json:"amount"`

	// Collect opt-in fields.
	Collect              bool   `json:"collect,omitempty"`
	CustomerPhoneNumber  string `json:"customerPhonenumber,omitempty"`
	CustomerEmailAddress string `json:"customerEmailaddress,omitempty"`
	ServiceNumber        string `json:"serviceNumber,omitempty"`
	CustomerName         string `json:"customerName,omitempty"`
	CustomerAddress      string `json:"customerAddress,omitempty"`
	CustomerNumber       string `json:"customerNumber,omitempty"`
	TRID                 string `json:"trid,omitempty"`
	Tag                  string `json:"tag,omitempty"`
	CallbackURL          string `json:"callbackUrl,omitempty"`
	CData                string `json:"cdata,omitempty"`
}

// ProductCfg drives the product purchase scenario. Amount is omitempty
// because catalog products (e.g. Canal+ decoders) have a fixed price.
type ProductCfg struct {
	ServiceID int64 `json:"serviceId"`
	Amount    int   `json:"amount,omitempty"`

	// Collect opt-in fields.
	Collect              bool   `json:"collect,omitempty"`
	CustomerPhoneNumber  string `json:"customerPhonenumber,omitempty"`
	CustomerEmailAddress string `json:"customerEmailaddress,omitempty"`
	ServiceNumber        string `json:"serviceNumber,omitempty"`
	CustomerName         string `json:"customerName,omitempty"`
	CustomerAddress      string `json:"customerAddress,omitempty"`
	CustomerNumber       string `json:"customerNumber,omitempty"`
	TRID                 string `json:"trid,omitempty"`
	Tag                  string `json:"tag,omitempty"`
	CallbackURL          string `json:"callbackUrl,omitempty"`
	CData                string `json:"cdata,omitempty"`
}

// SubscriptionCfg drives the subscription top-up scenario. Both
// ServiceNumber and CustomerNumber are declared at the block level because
// discovery needs them; the same names from the collect block would
// conflict, so collect-only customer fields stay below.
type SubscriptionCfg struct {
	Merchant       string `json:"merchant"`
	ServiceID      int64  `json:"serviceId"`
	ServiceNumber  string `json:"serviceNumber,omitempty"`
	CustomerNumber string `json:"customerNumber,omitempty"`
	Amount         int    `json:"amount,omitempty"`

	// Collect opt-in fields (ServiceNumber + CustomerNumber declared above).
	Collect              bool   `json:"collect,omitempty"`
	CustomerPhoneNumber  string `json:"customerPhonenumber,omitempty"`
	CustomerEmailAddress string `json:"customerEmailaddress,omitempty"`
	CustomerName         string `json:"customerName,omitempty"`
	CustomerAddress      string `json:"customerAddress,omitempty"`
	TRID                 string `json:"trid,omitempty"`
	Tag                  string `json:"tag,omitempty"`
	CallbackURL          string `json:"callbackUrl,omitempty"`
	CData                string `json:"cdata,omitempty"`
}

// CashinCfg drives the cashin (disbursement) scenario — money flows INTO
// the recipient wallet.
type CashinCfg struct {
	ServiceID int64 `json:"serviceId"`
	Amount    int   `json:"amount"`

	// Collect opt-in fields.
	Collect              bool   `json:"collect,omitempty"`
	CustomerPhoneNumber  string `json:"customerPhonenumber,omitempty"`
	CustomerEmailAddress string `json:"customerEmailaddress,omitempty"`
	ServiceNumber        string `json:"serviceNumber,omitempty"`
	CustomerName         string `json:"customerName,omitempty"`
	CustomerAddress      string `json:"customerAddress,omitempty"`
	CustomerNumber       string `json:"customerNumber,omitempty"`
	TRID                 string `json:"trid,omitempty"`
	Tag                  string `json:"tag,omitempty"`
	CallbackURL          string `json:"callbackUrl,omitempty"`
	CData                string `json:"cdata,omitempty"`
}

// VerifyCfg drives the pre-payment serviceNumber verification scenario.
// Verification is not a collection flow, so no collect fields apply.
type VerifyCfg struct {
	Merchant      string `json:"merchant"`
	ServiceID     int64  `json:"serviceId"`
	ServiceNumber string `json:"serviceNumber"`
}

// ValidateCfg drives the account-validation scenario (GET /v2/validate).
// Validation is read-only, so no collect fields apply.
type ValidateCfg struct {
	Destination string `json:"destination"`
	ServiceID   int64  `json:"serviceId"`
}

// Validate enforces that the required top-level credentials are present.
// Per-flow blocks are not validated here — they are checked at scenario
// dispatch time.
func (c SmokeConfig) Validate() error {
	if c.BaseURL == "" {
		return errors.New("smoketest: missing required field 'baseUrl'")
	}
	if c.PublicKey == "" {
		return errors.New("smoketest: missing required field 'publicKey'")
	}
	if c.SecretKey == "" {
		return errors.New("smoketest: missing required field 'secretKey'")
	}
	return nil
}
