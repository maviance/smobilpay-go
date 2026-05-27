package smobilpay

import (
	"context"

	"github.com/maviance/smobilpay-go/internal/apiclient"
)

// Merchant is a merchant supported by the system. Every Service is
// assigned to a merchant.
type Merchant struct {
	Merchant    string         `json:"merchant"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Country     string         `json:"country"`
	Status      MerchantStatus `json:"status"`
	Logo        string         `json:"logo,omitempty"`
	LogoHash    string         `json:"logoHash,omitempty"`
	Category    string         `json:"category,omitempty"` // deprecated; use categories on services
}

// Service is a service offered by a merchant. The IsReq* flags drive
// which CollectionRequest fields are required.
type Service struct {
	ServiceID            int64         `json:"serviceid"`
	Merchant             string        `json:"merchant"`
	Title                string        `json:"title"`
	Description          string        `json:"description"`
	Category             string        `json:"category"`
	Country              string        `json:"country"`
	LocalCur             string        `json:"localCur"`
	Type                 ServiceType   `json:"type"`
	Status               ServiceStatus `json:"status"`
	IsReqCustomerName    bool          `json:"isReqCustomerName"`
	IsReqCustomerAddress bool          `json:"isReqCustomerAddress"`
	IsReqCustomerNumber  bool          `json:"isReqCustomerNumber"`
	IsReqServiceNumber   bool          `json:"isReqServiceNumber"`
	IsVerifiable         bool          `json:"isVerifiable"`
	LabelCustomerNumber  []I18nText    `json:"labelCustomerNumber,omitempty"`
	LabelServiceNumber   []I18nText    `json:"labelServiceNumber,omitempty"`
	Hint                 []I18nText    `json:"hint,omitempty"`
	ValidationMask       string        `json:"validationMask,omitempty"`
	Denomination         *int          `json:"denomination,omitempty"`
}

// paymentItemBase carries the fields shared by every catalog item. The
// "Value" suffix avoids the field-vs-method collision when embedded
// concrete types implement the PaymentItem interface.
type paymentItemBase struct {
	ServiceIDValue      int64      `json:"serviceid"`
	MerchantValue       string     `json:"merchant"`
	PayItemIDValue      string     `json:"payItemId"`
	PayItemDescrValue   string     `json:"payItemDescr,omitempty"`
	AmountTypeValue     AmountType `json:"amountType"`
	LocalCurValue       string     `json:"localCur"`
	NameValue           string     `json:"name"`
	AmountLocalCurValue *float64   `json:"amountLocalCur,omitempty"`
	DescriptionValue    string     `json:"description,omitempty"`
	OptStrgValue        string     `json:"optStrg,omitempty"`
	OptNmbValue         *float64   `json:"optNmb,omitempty"`
}

// PaymentItem interface implementations.
func (p paymentItemBase) ServiceID() int64       { return p.ServiceIDValue }
func (p paymentItemBase) Merchant() string       { return p.MerchantValue }
func (p paymentItemBase) PayItemID() string      { return p.PayItemIDValue }
func (p paymentItemBase) PayItemDescr() string   { return p.PayItemDescrValue }
func (p paymentItemBase) AmountType() AmountType { return p.AmountTypeValue }
func (p paymentItemBase) LocalCur() string       { return p.LocalCurValue }
func (p paymentItemBase) Name() string           { return p.NameValue }
func (p paymentItemBase) AmountLocalCur() *float64 {
	return p.AmountLocalCurValue
}
func (p paymentItemBase) Description() string { return p.DescriptionValue }
func (p paymentItemBase) OptStrg() string     { return p.OptStrgValue }
func (p paymentItemBase) OptNmb() *float64    { return p.OptNmbValue }

// Cashout is a cash-out item — a collection from a customer's mobile
// wallet. Money flows OUT of the customer's wallet into the partner's
// balance.
type Cashout struct{ paymentItemBase }

// Cashin is a cash-in item — a disbursement into a recipient's mobile
// wallet. Money flows INTO the recipient's wallet from the partner's
// balance.
type Cashin struct{ paymentItemBase }

// Topup is a top-up package.
type Topup struct{ paymentItemBase }

// Product is a purchasable product (or voucher, when fetched from
// /v2/voucher). For voucher purchases the digital code is delivered on
// CollectionResponse.PIN.
type Product struct{ paymentItemBase }

// Merchants returns every merchant in the system (GET /v2/merchant).
func (m *MasterdataAPI) Merchants(ctx context.Context) ([]Merchant, error) {
	var out []Merchant
	err := m.tr.Get(ctx, "/v2/merchant", apiclient.NewQuery(), &out)
	return out, err
}

// Services returns every service in the system (GET /v2/service).
func (m *MasterdataAPI) Services(ctx context.Context) ([]Service, error) {
	var out []Service
	err := m.tr.Get(ctx, "/v2/service", apiclient.NewQuery(), &out)
	return out, err
}

// Products returns products, optionally filtered by serviceID (zero
// means no filter; the parameter is omitted from the request).
func (m *MasterdataAPI) Products(ctx context.Context, serviceID int64) ([]Product, error) {
	var out []Product
	err := m.tr.Get(ctx, "/v2/product",
		apiclient.NewQuery().Add("serviceid", serviceID), &out)
	return out, err
}

// Vouchers returns voucher products, optionally filtered by serviceID.
// The digital code is returned on CollectionResponse.PIN on a successful
// collection against the resulting payItemId.
func (m *MasterdataAPI) Vouchers(ctx context.Context, serviceID int64) ([]Product, error) {
	var out []Product
	err := m.tr.Get(ctx, "/v2/voucher",
		apiclient.NewQuery().Add("serviceid", serviceID), &out)
	return out, err
}

// Topups returns top-up packages, optionally filtered by serviceID.
func (m *MasterdataAPI) Topups(ctx context.Context, serviceID int64) ([]Topup, error) {
	var out []Topup
	err := m.tr.Get(ctx, "/v2/topup",
		apiclient.NewQuery().Add("serviceid", serviceID), &out)
	return out, err
}

// Cashins returns cash-in packages, optionally filtered by serviceID.
func (m *MasterdataAPI) Cashins(ctx context.Context, serviceID int64) ([]Cashin, error) {
	var out []Cashin
	err := m.tr.Get(ctx, "/v2/cashin",
		apiclient.NewQuery().Add("serviceid", serviceID), &out)
	return out, err
}

// Cashouts returns cash-out packages, optionally filtered by serviceID.
func (m *MasterdataAPI) Cashouts(ctx context.Context, serviceID int64) ([]Cashout, error) {
	var out []Cashout
	err := m.tr.Get(ctx, "/v2/cashout",
		apiclient.NewQuery().Add("serviceid", serviceID), &out)
	return out, err
}
