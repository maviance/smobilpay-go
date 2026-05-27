package smobilpay

import (
	"errors"
	"fmt"
	"testing"
)

func TestAPIError_Error_includesStatusAndRespCode(t *testing.T) {
	e := &APIError{HTTPStatus: 498, RespCode: 41001, DevMsg: "quote expired"}
	got := e.Error()
	want := "smobilpay: API error (HTTP 498, respCode 41001): quote expired"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_Error_omitsRespCodeWhenZero(t *testing.T) {
	e := &APIError{HTTPStatus: 503, RawBody: "Service Unavailable"}
	got := e.Error()
	want := "smobilpay: API error (HTTP 503): Service Unavailable"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_errorsAs(t *testing.T) {
	wrapped := fmt.Errorf("call failed: %w", &APIError{HTTPStatus: 400, RespCode: 40001})
	var apiErr *APIError
	if !errors.As(wrapped, &apiErr) {
		t.Fatal("errors.As(*APIError) returned false")
	}
	if apiErr.RespCode != 40001 {
		t.Errorf("RespCode = %d, want 40001", apiErr.RespCode)
	}
}

func TestAuthError_Error_includesOAuthError(t *testing.T) {
	e := &AuthError{HTTPStatus: 401, OAuthError: "invalid_client", Message: "bad creds"}
	got := e.Error()
	want := "smobilpay: auth error (HTTP 401, oauth error invalid_client): bad creds"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAuthError_Unwrap(t *testing.T) {
	cause := errors.New("network down")
	e := &AuthError{Message: "transport failure", Cause: cause}
	if !errors.Is(e, cause) {
		t.Error("errors.Is did not unwrap to cause")
	}
}

func TestTransportError_Error_includesOp(t *testing.T) {
	e := &TransportError{Op: "GET /v2/ping", Cause: errors.New("dial timeout")}
	got := e.Error()
	want := "smobilpay: transport error on GET /v2/ping: dial timeout"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestTransportError_Unwrap(t *testing.T) {
	cause := errors.New("eof")
	e := &TransportError{Op: "POST /v2/quotestd", Cause: cause}
	if !errors.Is(e, cause) {
		t.Error("errors.Is did not unwrap to cause")
	}
}
