package smobilpay

import "github.com/maviance/smobilpay-go/internal/apiclient"

// APIError is returned when a Smobilpay endpoint responds with a non-2xx
// HTTP status.
//
// Match on RespCode (the canonical machine identifier, e.g. 41004 for a
// voucher catalog inconsistency) for programmatic handling — the full
// error catalog is issued to partners during onboarding. DevMsg carries
// a verbose plain-language description for integrators; UsrMsg is a
// user-safe summary; Link points to documentation when available.
// RawBody is the unparsed response body, useful when the server
// returned a non-envelope payload.
type APIError = apiclient.APIError

// AuthError is returned when OAuth 2.0 token issuance or refresh fails.
//
// OAuthError is the value of the "error" field in the RFC 6749 error
// envelope when present (e.g. "invalid_client"). Message carries a
// human-readable description of what went wrong. Cause is the wrapped
// transport-level error when applicable; use errors.Unwrap or
// errors.Is to traverse the chain.
type AuthError = apiclient.AuthError

// TransportError is returned when an HTTP call cannot be completed —
// network failure, timeout, malformed URI, or unparsable response body.
//
// Op identifies the operation that failed in the form "METHOD /path"
// (e.g. "GET /v2/ping"). Cause is the underlying error; use errors.Is
// or errors.As to introspect.
type TransportError = apiclient.TransportError
