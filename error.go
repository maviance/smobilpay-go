package smobilpay

import "github.com/maviance/smobilpay-go/internal/apiclient"

// APIError is returned when a Smobilpay endpoint responds with a non-2xx
// HTTP status. See [apiclient.APIError] for fields.
type APIError = apiclient.APIError

// AuthError is returned when OAuth 2.0 token issuance or refresh fails.
// See [apiclient.AuthError] for fields.
type AuthError = apiclient.AuthError

// TransportError is returned when an HTTP call cannot be completed.
// See [apiclient.TransportError] for fields.
type TransportError = apiclient.TransportError
