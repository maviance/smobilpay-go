package smobilpay

import (
	"context"
	"errors"

	"github.com/maviance/smobilpay-go/internal/apiclient"
)

// CustomerAccount is the result of GET /v2/validate — an account-lookup
// response reporting whether the supplied destination is recognized by
// the service and, when available, the associated customer name.
type CustomerAccount struct {
	Status      CustomerAccountStatus `json:"status"`
	Name        string                `json:"name,omitempty"`
	Destination string                `json:"destination"`
}

// CustomerAccountStatus distinguishes how the account was recognized.
type CustomerAccountStatus string

const (
	// CustomerAccountStatusUnknown: authenticity could be neither verified nor validated.
	CustomerAccountStatusUnknown CustomerAccountStatus = "UNKNOWN"
	// CustomerAccountStatusValidated: syntax confirmed internally (e.g. via regex).
	CustomerAccountStatusValidated CustomerAccountStatus = "VALIDATED"
	// CustomerAccountStatusVerified: cross-checked against the service provider.
	CustomerAccountStatusVerified CustomerAccountStatus = "VERIFIED"
)

// VerifyServiceNumber verifies that a service number is valid for the
// selected service (GET /v2/verify). Only meaningful for services that
// report IsVerifiable: true.
//
// All three arguments are required by the partner OpenAPI spec; an empty
// merchant or serviceNumber, or a non-positive serviceID, returns a
// client-side validation error without contacting the server.
func (a *AccountValidationAPI) VerifyServiceNumber(ctx context.Context, merchant string, serviceID int64, serviceNumber string) (bool, error) {
	if merchant == "" {
		return false, errors.New("smobilpay: VerifyServiceNumber requires merchant")
	}
	if serviceID <= 0 {
		return false, errors.New("smobilpay: VerifyServiceNumber requires serviceID > 0")
	}
	if serviceNumber == "" {
		return false, errors.New("smobilpay: VerifyServiceNumber requires serviceNumber")
	}
	q := apiclient.NewQuery().
		Add("merchant", merchant).
		Add("serviceid", serviceID).
		Add("serviceNumber", serviceNumber)
	var out bool
	if err := a.tr.Get(ctx, "/v2/verify", q, &out); err != nil {
		return false, err
	}
	return out, nil
}

// ValidateAccount validates an account by destination (typically an
// MSISDN or contract number) and retrieves the associated customer
// name, when available (GET /v2/validate).
//
// This is a restricted endpoint — access is granted only to partners
// who have cleared Maviance's internal validation and compliance review.
// Unauthorized callers receive HTTP 401 as an *APIError.
//
// Note: the partner OpenAPI spec uses the query parameter name
// `serviceId` (camelCase) for /v2/validate, distinct from the lowercase
// `serviceid` used by most other endpoints.
func (a *AccountValidationAPI) ValidateAccount(ctx context.Context, destination string, serviceID int64) (CustomerAccount, error) {
	if destination == "" {
		return CustomerAccount{}, errors.New("smobilpay: ValidateAccount requires destination")
	}
	if serviceID <= 0 {
		return CustomerAccount{}, errors.New("smobilpay: ValidateAccount requires serviceID > 0")
	}
	q := apiclient.NewQuery().
		Add("destination", destination).
		Add("serviceId", serviceID)
	var out CustomerAccount
	if err := a.tr.Get(ctx, "/v2/validate", q, &out); err != nil {
		return CustomerAccount{}, err
	}
	return out, nil
}
