// Package apiclient — error types are declared here (not in the root
// smobilpay package) because internal/apiclient is the lowest layer
// and the root smobilpay package re-exports them as type aliases.
package apiclient

import "fmt"

// APIError is returned when a Smobilpay endpoint responds with a non-2xx
// HTTP status. RespCode is the canonical machine identifier (e.g. 41004
// for a voucher catalog inconsistency) — match on RespCode for
// programmatic handling. The full catalog is issued during onboarding.
type APIError struct {
	HTTPStatus int
	RespCode   int
	DevMsg     string
	UsrMsg     string
	Link       string
	RawBody    string
}

func (e *APIError) Error() string {
	if e.RespCode != 0 {
		return fmt.Sprintf("smobilpay: API error (HTTP %d, respCode %d): %s",
			e.HTTPStatus, e.RespCode, e.message())
	}
	return fmt.Sprintf("smobilpay: API error (HTTP %d): %s", e.HTTPStatus, e.message())
}

func (e *APIError) message() string {
	if e.DevMsg != "" {
		return e.DevMsg
	}
	if e.UsrMsg != "" {
		return e.UsrMsg
	}
	return e.RawBody
}

// AuthError is returned when OAuth 2.0 token issuance or refresh fails.
// OAuthError is the value of the "error" field in the RFC 6749 error
// envelope when present (e.g. "invalid_client").
type AuthError struct {
	HTTPStatus int
	OAuthError string
	Message    string
	Cause      error
}

func (e *AuthError) Error() string {
	if e.OAuthError != "" {
		return fmt.Sprintf("smobilpay: auth error (HTTP %d, oauth error %s): %s",
			e.HTTPStatus, e.OAuthError, e.Message)
	}
	return fmt.Sprintf("smobilpay: auth error (HTTP %d): %s", e.HTTPStatus, e.Message)
}

func (e *AuthError) Unwrap() error { return e.Cause }

// TransportError is returned when an HTTP call cannot be completed
// (network failure, timeout, malformed URI, unparsable response body).
type TransportError struct {
	Op    string
	Cause error
}

func (e *TransportError) Error() string {
	return fmt.Sprintf("smobilpay: transport error on %s: %v", e.Op, e.Cause)
}

func (e *TransportError) Unwrap() error { return e.Cause }
