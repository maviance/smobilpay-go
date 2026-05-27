# Go Client Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild `github.com/maviance/smobilpay-go` from a 34-line HMAC helper into a full Smobilpay partner API v3.2.0 client (OAuth2-only), with parity to the Java client at `/root/s3p-clients/java`, 80%+ test coverage, a smoke-test harness that consumes the Java `smoke-test.json` schema, and a partner-facing README.

**Architecture:** Single root package `smobilpay` with API groups as fields on `Client` (`client.Verify.Ping(ctx)`, etc.). DTOs co-located with the API group that returns them. Internal `internal/apiclient/` package holds the HTTP transport, OAuth 2.0 token manager (with `singleflight` dedup), and query-string builder. Smoke-test runner under `cmd/smoketest/`. Modern Go SDK shape (anthropic-sdk-go / openai-go / stripe-go style).

**Tech Stack:** Go 1.22, `net/http`, `encoding/json`, `errors.As` typed errors, `golang.org/x/sync/singleflight` (only runtime dep), stdlib `testing` + `httptest`, GNU Make.

**Source of truth:** [OpenAPI partner spec](/root/php-smobilpay-s3p-api/apidocs/s3p_3.2.0_openapi_specs_partner.yml). Java client at `/root/s3p-clients/java` is the reference implementation; the smoke-test harness diffs Go vs Java output.

**Spec:** [docs/superpowers/specs/2026-05-27-go-client-rebuild-design.md](../specs/2026-05-27-go-client-rebuild-design.md)

---

## Task 1: Foundation — clean slate, go.mod, .gitignore

**Files:**
- Delete: `s3p/signature.go`, `s3p/signature_test.go`, `examples/main.go`
- Modify: `go.mod`
- Modify: `.gitignore`

- [ ] **Step 1: Delete old HMAC code and example**

```bash
git rm s3p/signature.go s3p/signature_test.go examples/main.go
rmdir s3p examples 2>/dev/null || true
```

- [ ] **Step 2: Rewrite `go.mod` for Go 1.22 + singleflight**

Overwrite `go.mod`:

```go
module github.com/maviance/smobilpay-go

go 1.22

require golang.org/x/sync v0.7.0
```

- [ ] **Step 3: Refresh `go.sum`**

Run: `go mod download golang.org/x/sync && go mod tidy`
Expected: `go.sum` created with `golang.org/x/sync` entry; no errors.

- [ ] **Step 4: Extend `.gitignore`**

Append to `.gitignore`:

```
# Build output
/build/
/cover.out
/coverage.html

# Local smoke-test credentials (example file is committed; real config is not).
/smoke-test.json

# Editor / OS noise
.DS_Store
.idea/
.vscode/
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: clean slate for client rebuild

Delete legacy HMAC signature helper and example; bump module to Go 1.22
and add singleflight runtime dependency."
```

---

## Task 2: Makefile + tools/normalize.sh

**Files:**
- Create: `Makefile`
- Create: `tools/normalize.sh` (executable)

- [ ] **Step 1: Write `Makefile`**

```makefile
GO          ?= go
COVERFILE   ?= cover.out
COVER_MIN   ?= 80.0
JAVA_DIR    ?= ../java
BUILD_DIR   ?= build

.PHONY: build test cover lint smoketest smoketest-compare tidy clean help

help:
	@echo "Targets:"
	@echo "  build              go build ./..."
	@echo "  test               go test -race ./..."
	@echo "  cover              go test with $(COVER_MIN)% coverage gate"
	@echo "  lint               gofmt + go vet"
	@echo "  smoketest          run cmd/smoketest against ./smoke-test.json"
	@echo "  smoketest-compare  diff Go vs Java smoke-test output (JAVA_DIR=$(JAVA_DIR))"
	@echo "  tidy               go mod tidy"
	@echo "  clean              remove build artifacts"

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

cover:
	$(GO) test -race -coverprofile=$(COVERFILE) ./...
	@total=$$($(GO) tool cover -func=$(COVERFILE) | awk '/^total:/ { print $$3 }' | tr -d '%'); \
	awk -v t=$$total -v m=$(COVER_MIN) 'BEGIN { \
		if (t+0 < m+0) { printf "coverage %.1f%% below %.1f%% gate\n", t, m; exit 1 } \
		else { printf "coverage %.1f%% (gate %.1f%%) OK\n", t, m } }'

lint:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "gofmt issues:"; echo "$$out"; exit 1; fi
	$(GO) vet ./...

smoketest:
	$(GO) run ./cmd/smoketest

smoketest-compare: | $(BUILD_DIR)
	$(GO) run ./cmd/smoketest > $(BUILD_DIR)/smoketest.go.txt
	cd $(JAVA_DIR) && ./gradlew runSmokeTest --console=plain \
		--args="$$PWD/smoke-test.json" > $(CURDIR)/$(BUILD_DIR)/smoketest.java.txt
	./tools/normalize.sh $(BUILD_DIR)/smoketest.go.txt   > $(BUILD_DIR)/smoketest.go.norm.txt
	./tools/normalize.sh $(BUILD_DIR)/smoketest.java.txt > $(BUILD_DIR)/smoketest.java.norm.txt
	@if diff -u $(BUILD_DIR)/smoketest.java.norm.txt $(BUILD_DIR)/smoketest.go.norm.txt; then \
		echo "Go and Java smoke-test outputs match (modulo redacted volatile fields)."; \
	else exit 1; fi

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BUILD_DIR) $(COVERFILE) coverage.html

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)
```

- [ ] **Step 2: Write `tools/normalize.sh`**

```bash
#!/usr/bin/env bash
# Normalize a smoketest output by redacting volatile fields so the Go and
# Java outputs are comparable line-for-line.
set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "usage: $0 <smoketest-output-file>" >&2
    exit 64
fi

sed -E \
    -e 's|^(\s+server time:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+nonce echo:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+quoteId:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+expiresAt:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+(first|forced)\s+bearer prefix:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+transactions:\s+)[0-9]+$|\1<REDACTED>|' \
    -e 's|^(\s+(services|merchants):\s+)[0-9]+$|\1<REDACTED>|' \
    -e 's|^(\s+-\s+[A-Z_]+:\s+)[0-9]+$|\1<REDACTED>|' \
    -e 's|^(\s+range:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+\.\.\.and\s+)[0-9]+(\s+more)$|\1<REDACTED>\2|' \
    -e 's|PTN-[0-9]+|PTN-<REDACTED>|g' \
    "$1"
```

- [ ] **Step 3: Make it executable**

```bash
chmod +x tools/normalize.sh
```

- [ ] **Step 4: Verify `make help`**

Run: `make help`
Expected: prints the target list with no errors.

- [ ] **Step 5: Commit**

```bash
git add Makefile tools/normalize.sh
git commit -m "chore: add Makefile and smoke-test diff normalizer"
```

---

## Task 3: Package overview (doc.go)

**Files:**
- Create: `doc.go`

- [ ] **Step 1: Write `doc.go`**

```go
// Package smobilpay is the Go client for the Smobilpay partner API (v3.2.0).
//
// The client covers every partner-facing endpoint a partner needs to move
// money in and out, sell value-added services, and drive a payment UI from
// the static catalog. Surface and semantics mirror the official Java client.
//
// # Authentication
//
// Every secured endpoint requires an OAuth 2.0 bearer minted at
// POST /oauth/token using the client_credentials grant. The client manages
// token issuance and caching automatically; see [Client.Tokens] for manual
// refresh.
//
// # Quick start
//
//	cfg, err := smobilpay.NewConfig(
//	    smobilpay.WithBaseURL("https://api.example.invalid"),
//	    smobilpay.WithCredentials(pub, sec),
//	)
//	if err != nil { /* invalid configuration */ }
//
//	client, err := smobilpay.New(cfg)
//	if err != nil { /* unable to initialise */ }
//
//	ping, err := client.Verify.Ping(context.Background())
//
// # Errors
//
// API errors are surfaced as *[APIError]; authentication errors as
// *[AuthError]; transport errors as *[TransportError]. Use errors.As to
// introspect.
//
// # Onboarding
//
// Base URL, partner credentials, and the full error catalog are issued by
// Maviance support during partner onboarding — contact support@smobilpay.com.
package smobilpay
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./...`
Expected: succeeds, no output.

- [ ] **Step 3: Commit**

```bash
git add doc.go
git commit -m "feat: package overview doc.go"
```

---

## Task 4: error.go — typed errors

> **Cycle note:** The root `smobilpay` package imports `internal/apiclient` (for the transport + token manager), and the transport needs to construct typed errors on every non-2xx. To avoid a circular import, the error **types live in `internal/apiclient`** as the source of truth, and the root `smobilpay` package re-exports them via type aliases. Users still write `*smobilpay.APIError`; the alias is transparent.

**Files:**
- Create: `internal/apiclient/error.go`
- Create: `error.go` (root — type aliases only)
- Create: `error_test.go` (root — exercises the aliases)

- [ ] **Step 1: Write failing tests (`error_test.go`)**

```go
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
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./...`
Expected: build failure ("APIError undefined", ...).

- [ ] **Step 3a: Implement `internal/apiclient/error.go` (source of truth)**

```go
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
```

- [ ] **Step 3b: Implement `error.go` (root — re-export aliases)**

```go
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
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: PASS for all 7 tests. The aliases mean the test file's
references to `*smobilpay.APIError` resolve to `*apiclient.APIError`
transparently.

- [ ] **Step 5: Commit**

```bash
git add internal/apiclient/error.go error.go error_test.go
git commit -m "feat: typed errors in internal/apiclient with root-package aliases"
```

---

## Task 5: types.go — enums, Date, PaymentItem interface

**Files:**
- Create: `types.go`
- Create: `types_test.go`

- [ ] **Step 1: Write failing tests (`types_test.go`)**

```go
package smobilpay

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDate_UnmarshalJSON_dateOnly(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"2024-01-15"`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if y, m, day := d.Date(); y != 2024 || m != time.January || day != 15 {
		t.Errorf("Date() = %d-%02d-%02d, want 2024-01-15", y, m, day)
	}
}

func TestDate_UnmarshalJSON_dateTimeZ(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"2024-01-15T10:30:00Z"`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if y, m, day := d.Date(); y != 2024 || m != time.January || day != 15 {
		t.Errorf("Date() = %d-%02d-%02d, want 2024-01-15", y, m, day)
	}
}

func TestDate_UnmarshalJSON_dateTimeOffset(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"2024-01-15T10:30:00+01:00"`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if y, m, day := d.Date(); y != 2024 || m != time.January || day != 15 {
		t.Errorf("Date() = %d-%02d-%02d, want 2024-01-15", y, m, day)
	}
}

func TestDate_UnmarshalJSON_null(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`null`), &d); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if !d.IsZero() {
		t.Error("null should leave Date zero-valued")
	}
}

func TestDate_UnmarshalJSON_bad(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"not-a-date"`), &d); err == nil {
		t.Error("expected error on bad date")
	}
}

func TestDate_MarshalJSON_canonical(t *testing.T) {
	d := Date{time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC)}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `"2024-01-15"` {
		t.Errorf("marshal = %s, want \"2024-01-15\"", b)
	}
}

func TestDate_MarshalJSON_zero(t *testing.T) {
	var d Date
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal zero: %v", err)
	}
	if string(b) != `null` {
		t.Errorf("zero marshals to %s, want null", b)
	}
}

func TestServiceType_constants(t *testing.T) {
	cases := []struct {
		got, want ServiceType
	}{
		{ServiceTypeSearchableBill, "SEARCHABLE_BILL"},
		{ServiceTypeNonSearchableBill, "NON_SEARCHABLE_BILL"},
		{ServiceTypeProduct, "PRODUCT"},
		{ServiceTypeTopup, "TOPUP"},
		{ServiceTypeSubscription, "SUBSCRIPTION"},
		{ServiceTypeCashin, "CASHIN"},
		{ServiceTypeCashout, "CASHOUT"},
		{ServiceTypeVoucher, "VOUCHER"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("ServiceType = %q, want %q", c.got, c.want)
		}
	}
}

func TestMerchantStatus_jsonRoundtrip(t *testing.T) {
	// Server emits mixed-case "Active" / "Inactive" — preserve verbatim.
	for _, raw := range []string{`"Active"`, `"Inactive"`} {
		var s MerchantStatus
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			t.Fatalf("unmarshal %s: %v", raw, err)
		}
		b, _ := json.Marshal(s)
		if string(b) != raw {
			t.Errorf("roundtrip %s = %s", raw, b)
		}
	}
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./...`
Expected: build failure (undefined Date, ServiceType, etc.).

- [ ] **Step 3: Implement `types.go`**

```go
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
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: PASS for all types_test cases.

- [ ] **Step 5: Commit**

```bash
git add types.go types_test.go
git commit -m "feat: enums, lenient Date type, PaymentItem interface, I18nText"
```

---

## Task 6: option.go — Config and functional options

**Files:**
- Create: `option.go`
- Create: `option_test.go`

- [ ] **Step 1: Write failing tests (`option_test.go`)**

```go
package smobilpay

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewConfig_defaults(t *testing.T) {
	cfg, err := NewConfig(
		WithBaseURL("https://api.example.invalid"),
		WithCredentials("pub", "sec"),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.APIVersion != "3.0.0" {
		t.Errorf("APIVersion = %q", cfg.APIVersion)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v", cfg.RequestTimeout)
	}
	if cfg.TokenRefreshSkew != 30*time.Second {
		t.Errorf("TokenRefreshSkew = %v", cfg.TokenRefreshSkew)
	}
	if cfg.HTTPClient == nil {
		t.Error("HTTPClient should be defaulted")
	}
}

func TestNewConfig_trimsTrailingSlash(t *testing.T) {
	cfg, _ := NewConfig(
		WithBaseURL("https://api.example.invalid/"),
		WithCredentials("pub", "sec"),
	)
	if strings.HasSuffix(cfg.BaseURL, "/") {
		t.Errorf("trailing slash not trimmed: %q", cfg.BaseURL)
	}
}

func TestNewConfig_missingBaseURL(t *testing.T) {
	if _, err := NewConfig(WithCredentials("pub", "sec")); err == nil {
		t.Error("expected error when baseURL missing")
	}
}

func TestNewConfig_missingCredentials(t *testing.T) {
	if _, err := NewConfig(WithBaseURL("https://api.example.invalid")); err == nil {
		t.Error("expected error when credentials missing")
	}
}

func TestNewConfig_emptyPublicKey(t *testing.T) {
	if _, err := NewConfig(
		WithBaseURL("https://api.example.invalid"),
		WithCredentials("", "sec"),
	); err == nil {
		t.Error("expected error on empty publicKey")
	}
}

func TestNewConfig_overrides(t *testing.T) {
	hc := &http.Client{Timeout: time.Second}
	cfg, err := NewConfig(
		WithBaseURL("https://api.example.invalid"),
		WithCredentials("pub", "sec"),
		WithAPIVersion("3.1.0"),
		WithRequestTimeout(10*time.Second),
		WithTokenRefreshSkew(5*time.Second),
		WithHTTPClient(hc),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.APIVersion != "3.1.0" {
		t.Errorf("APIVersion = %q", cfg.APIVersion)
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Errorf("RequestTimeout = %v", cfg.RequestTimeout)
	}
	if cfg.TokenRefreshSkew != 5*time.Second {
		t.Errorf("TokenRefreshSkew = %v", cfg.TokenRefreshSkew)
	}
	if cfg.HTTPClient != hc {
		t.Error("HTTPClient override not applied")
	}
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./...`
Expected: build failure ("NewConfig undefined").

- [ ] **Step 3: Implement `option.go`**

```go
package smobilpay

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

// DefaultAPIVersion is the default value of the x-api-version request header.
const DefaultAPIVersion = "3.0.0"

// Config is the immutable configuration for a Client. Construct via
// NewConfig + functional options.
type Config struct {
	BaseURL          string
	PublicKey        string
	SecretKey        string
	APIVersion       string
	RequestTimeout   time.Duration
	TokenRefreshSkew time.Duration
	HTTPClient       *http.Client
}

// Option mutates a Config during construction.
type Option func(*Config)

// NewConfig validates and finalises a Config. Required: WithBaseURL and
// WithCredentials. Returns an error if either is missing.
func NewConfig(opts ...Option) (Config, error) {
	cfg := Config{
		APIVersion:       DefaultAPIVersion,
		RequestTimeout:   30 * time.Second,
		TokenRefreshSkew: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.BaseURL == "" {
		return Config{}, errors.New("smobilpay: WithBaseURL is required")
	}
	if cfg.PublicKey == "" || cfg.SecretKey == "" {
		return Config{}, errors.New("smobilpay: WithCredentials is required (publicKey and secretKey both non-empty)")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.RequestTimeout}
	}
	return cfg, nil
}

// WithBaseURL sets the partner base URL (issued during onboarding).
func WithBaseURL(url string) Option { return func(c *Config) { c.BaseURL = url } }

// WithCredentials sets the OAuth 2.0 client_credentials pair.
func WithCredentials(publicKey, secretKey string) Option {
	return func(c *Config) {
		c.PublicKey = publicKey
		c.SecretKey = secretKey
	}
}

// WithAPIVersion overrides the value of the x-api-version request header.
// Default: DefaultAPIVersion ("3.0.0").
func WithAPIVersion(version string) Option {
	return func(c *Config) { c.APIVersion = version }
}

// WithRequestTimeout sets the per-request timeout. Default: 30s.
func WithRequestTimeout(d time.Duration) Option {
	return func(c *Config) { c.RequestTimeout = d }
}

// WithTokenRefreshSkew sets the lead time at which a cached token is
// treated as expired so it can be re-minted before the next request
// fails. Default: 30s.
func WithTokenRefreshSkew(d time.Duration) Option {
	return func(c *Config) { c.TokenRefreshSkew = d }
}

// WithHTTPClient injects a custom *http.Client (e.g. for proxy or custom
// TLS). When supplied, its Timeout overrides RequestTimeout for
// per-request deadlines.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) { c.HTTPClient = client }
}
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: every option_test PASS.

- [ ] **Step 5: Commit**

```bash
git add option.go option_test.go
git commit -m "feat: Config + functional options"
```

---

## Task 7: internal/apiclient/query.go — Query builder

**Files:**
- Create: `internal/apiclient/query.go`
- Create: `internal/apiclient/query_test.go`

- [ ] **Step 1: Write failing tests (`internal/apiclient/query_test.go`)**

```go
package apiclient

import (
	"testing"
	"time"
)

func TestQuery_empty(t *testing.T) {
	q := NewQuery()
	if got := q.Encode(); got != "" {
		t.Errorf("empty Encode() = %q, want \"\"", got)
	}
	if !q.IsEmpty() {
		t.Error("IsEmpty() = false on empty builder")
	}
}

func TestQuery_skipsNilOrEmpty(t *testing.T) {
	q := NewQuery().
		Add("nilv", nil).
		Add("empty", "").
		Add("kept", "v")
	if got := q.Encode(); got != "kept=v" {
		t.Errorf("Encode() = %q, want %q", got, "kept=v")
	}
}

func TestQuery_skipsZeroInt64(t *testing.T) {
	q := NewQuery().Add("z", int64(0)).Add("n", int64(99))
	if got := q.Encode(); got != "n=99" {
		t.Errorf("Encode() = %q, want %q", got, "n=99")
	}
}

func TestQuery_keepsZeroIntWhenWrappedInPtr(t *testing.T) {
	zero := int64(0)
	q := NewQuery().Add("z", &zero)
	if got := q.Encode(); got != "z=0" {
		t.Errorf("Encode() = %q, want %q", got, "z=0")
	}
}

func TestQuery_encodesTimeISO8601(t *testing.T) {
	at := time.Date(2024, 5, 1, 12, 30, 0, 0, time.UTC)
	q := NewQuery().Add("ts", at)
	want := "ts=2024-05-01T12%3A30%3A00Z"
	if got := q.Encode(); got != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

func TestQuery_preservesInsertionOrder(t *testing.T) {
	q := NewQuery().
		Add("b", "2").
		Add("a", "1").
		Add("c", "3")
	if got := q.Encode(); got != "b=2&a=1&c=3" {
		t.Errorf("Encode() = %q, want %q", got, "b=2&a=1&c=3")
	}
}

func TestQuery_urlEncodesValues(t *testing.T) {
	q := NewQuery().Add("k", "a b&c")
	if got := q.Encode(); got != "k=a+b%26c" {
		t.Errorf("Encode() = %q", got)
	}
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./internal/apiclient`
Expected: build failure ("NewQuery undefined").

- [ ] **Step 3: Implement `internal/apiclient/query.go`**

```go
// Package apiclient holds the internal HTTP transport, OAuth 2.0 token
// manager, and query-string builder. Not part of the public API; only
// the smobilpay root package may depend on it.
package apiclient

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Query builds an ordered query string. nil values, empty strings, and
// zero int64 values are silently skipped — matching the Java client's
// QueryParams.add behaviour — so callers can pass optional parameters
// uniformly without nil-checking.
//
// Pass a pointer (*int64, *float64, *string) to force-include an
// otherwise-skippable zero value.
type Query struct {
	keys []string
	vals []string
}

// NewQuery returns an empty Query.
func NewQuery() *Query { return &Query{} }

// IsEmpty reports whether the query is empty.
func (q *Query) IsEmpty() bool { return len(q.keys) == 0 }

// Add appends key=value if value is non-empty after encoding rules.
// Returns q for chaining.
func (q *Query) Add(key string, value any) *Query {
	s, ok := encodeValue(value)
	if !ok {
		return q
	}
	q.keys = append(q.keys, key)
	q.vals = append(q.vals, s)
	return q
}

// Encode renders the query string (no leading "?"), URL-encoding values
// and preserving insertion order.
func (q *Query) Encode() string {
	if q.IsEmpty() {
		return ""
	}
	var b strings.Builder
	for i, k := range q.keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(k))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(q.vals[i]))
	}
	return b.String()
}

func encodeValue(value any) (string, bool) {
	switch v := value.(type) {
	case nil:
		return "", false
	case string:
		if v == "" {
			return "", false
		}
		return v, true
	case *string:
		if v == nil {
			return "", false
		}
		return *v, true
	case int:
		if v == 0 {
			return "", false
		}
		return strconv.Itoa(v), true
	case int64:
		if v == 0 {
			return "", false
		}
		return strconv.FormatInt(v, 10), true
	case *int64:
		if v == nil {
			return "", false
		}
		return strconv.FormatInt(*v, 10), true
	case float64:
		if v == 0 {
			return "", false
		}
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case *float64:
		if v == nil {
			return "", false
		}
		return strconv.FormatFloat(*v, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(v), true
	case time.Time:
		if v.IsZero() {
			return "", false
		}
		return v.UTC().Format(time.RFC3339), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: all query tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/apiclient/query.go internal/apiclient/query_test.go
git commit -m "feat: internal/apiclient.Query builder with skip-empty semantics"
```

---

## Task 8: internal/apiclient/oauth2.go — Token + Manager (singleflight)

> **Cycle note:** Tests for this package live in `package apiclient_test` (external test package) so they can import the root `smobilpay` package without creating an import cycle. The production code in `apiclient` never imports `smobilpay`.

**Files:**
- Create: `internal/apiclient/oauth2.go`
- Create: `internal/apiclient/oauth2_test.go`

- [ ] **Step 1: Write failing tests (`internal/apiclient/oauth2_test.go`)**

```go
package apiclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maviance/smobilpay-go/internal/apiclient"
)

// tokenPath is duplicated from the production code so we can reference
// it from the external test package without exporting it.
const tokenPath = "/oauth/token"

type oauthHandler struct {
	mu      sync.Mutex
	calls   int32
	respond func(w http.ResponseWriter, r *http.Request)
}

func (h *oauthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&h.calls, 1)
	h.respond(w, r)
}

func okTokenHandler(token string, expiresIn int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/oauth/token" {
			http.Error(w, "bad request", 400)
			return
		}
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   expiresIn,
		})
	}
}

func newTestManager(t *testing.T, srvURL string, clock func() time.Time) *apiclient.OAuth2Manager {
	t.Helper()
	return apiclient.NewOAuth2Manager(
		srvURL, "pub", "sec",
		5*time.Second,
		http.DefaultClient,
		clock,
	)
}

func TestOAuth2Manager_mints(t *testing.T) {
	h := &oauthHandler{respond: okTokenHandler("jwt-1", 3600)}
	srv := httptest.NewServer(h)
	defer srv.Close()

	now := time.Now()
	m := newTestManager(t, srv.URL, func() time.Time { return now })
	tok, err := m.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if tok != "jwt-1" {
		t.Errorf("token = %q, want jwt-1", tok)
	}
	if atomic.LoadInt32(&h.calls) != 1 {
		t.Errorf("server hits = %d, want 1", h.calls)
	}
}

func TestOAuth2Manager_caches(t *testing.T) {
	h := &oauthHandler{respond: okTokenHandler("jwt-1", 3600)}
	srv := httptest.NewServer(h)
	defer srv.Close()

	m := newTestManager(t, srv.URL, time.Now)
	for i := 0; i < 5; i++ {
		if _, err := m.AccessToken(context.Background()); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&h.calls); got != 1 {
		t.Errorf("server hits = %d, want 1 (cache hit)", got)
	}
}

func TestOAuth2Manager_refreshesNearExpiry(t *testing.T) {
	h := &oauthHandler{respond: okTokenHandler("jwt-1", 10)}
	srv := httptest.NewServer(h)
	defer srv.Close()

	now := time.Now()
	advance := int64(0)
	clock := func() time.Time {
		return now.Add(time.Duration(atomic.LoadInt64(&advance)) * time.Second)
	}
	m, _ := newTestManager(t, srv.URL, clock)
	if _, err := m.AccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	atomic.StoreInt64(&advance, 6) // skew=5 means token treated as expired
	h.respond = okTokenHandler("jwt-2", 10)
	tok, err := m.AccessToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok != "jwt-2" {
		t.Errorf("token = %q, want jwt-2", tok)
	}
	if atomic.LoadInt32(&h.calls) != 2 {
		t.Errorf("server hits = %d, want 2", h.calls)
	}
}

func TestOAuth2Manager_Refresh_forces(t *testing.T) {
	h := &oauthHandler{respond: okTokenHandler("jwt-1", 3600)}
	srv := httptest.NewServer(h)
	defer srv.Close()

	m := newTestManager(t, srv.URL, time.Now)
	if _, err := m.AccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.respond = okTokenHandler("jwt-2", 3600)
	tok, err := m.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok != "jwt-2" {
		t.Errorf("token = %q, want jwt-2", tok)
	}
}

func TestOAuth2Manager_authErrorOn401(t *testing.T) {
	h := &oauthHandler{respond: func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = fmt.Fprint(w, `{"error":"invalid_client","error_description":"bad creds"}`)
	}}
	srv := httptest.NewServer(h)
	defer srv.Close()

	m := newTestManager(t, srv.URL, time.Now)
	_, err := m.AccessToken(context.Background())
	var ae *apiclient.AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %v", err)
	}
	if ae.HTTPStatus != 401 || ae.OAuthError != "invalid_client" {
		t.Errorf("got %+v", ae)
	}
}

func TestOAuth2Manager_missingAccessToken(t *testing.T) {
	h := &oauthHandler{respond: func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"token_type":"Bearer","expires_in":3600}`)
	}}
	srv := httptest.NewServer(h)
	defer srv.Close()
	m := newTestManager(t, srv.URL, time.Now)
	if _, err := m.AccessToken(context.Background()); err == nil {
		t.Error("expected error on missing access_token")
	}
}

func TestOAuth2Manager_concurrentMintDedupes(t *testing.T) {
	calls := int32(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond) // hold the lock window open
		okTokenHandler("jwt-1", 3600)(w, r)
	}))
	defer srv.Close()

	m := newTestManager(t, srv.URL, time.Now)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := m.AccessToken(context.Background()); err != nil {
				t.Errorf("concurrent: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("server hits = %d, want 1 (singleflight dedupes)", got)
	}
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./internal/apiclient`
Expected: build failure ("OAuth2Manager undefined").

- [ ] **Step 3: Implement `internal/apiclient/oauth2.go`**

```go
package apiclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const tokenPath = "/oauth/token"
const grantBody = "grant_type=client_credentials"

// Token is a cached OAuth 2.0 bearer.
type Token struct {
	AccessToken string
	TokenType   string
	ExpiresAt   time.Time
}

// Expired reports whether the token will expire within skew of now.
func (t Token) Expired(now time.Time, skew time.Duration) bool {
	return !now.Add(skew).Before(t.ExpiresAt)
}

// OAuth2Manager mints, caches, and refreshes the client_credentials
// bearer used to authenticate every API call.
type OAuth2Manager struct {
	baseURL    string
	publicKey  string
	secretKey  string
	skew       time.Duration
	httpClient *http.Client
	clock      func() time.Time
	group      singleflight.Group

	mu     sync.RWMutex
	cached *Token
}

// NewOAuth2Manager returns an OAuth2Manager. clock defaults to time.Now
// when nil — pass a fake clock from tests. baseURL must not have a
// trailing slash (callers normalise upstream).
func NewOAuth2Manager(baseURL, publicKey, secretKey string, refreshSkew time.Duration, httpClient *http.Client, clock func() time.Time) *OAuth2Manager {
	if clock == nil {
		clock = time.Now
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OAuth2Manager{
		baseURL:    strings.TrimRight(baseURL, "/"),
		publicKey:  publicKey,
		secretKey:  secretKey,
		skew:       refreshSkew,
		httpClient: httpClient,
		clock:      clock,
	}
}

// AccessToken returns a valid bearer, minting one if the cache is empty
// or the cached token is within refreshSkew of expiry. Concurrent
// callers see a single in-flight mint via singleflight.
func (m *OAuth2Manager) AccessToken(ctx context.Context) (string, error) {
	if t := m.peek(); t != nil && !t.Expired(m.clock(), m.skew) {
		return t.AccessToken, nil
	}
	v, err, _ := m.group.Do("mint", func() (any, error) {
		// Double-check inside the singleflight: another goroutine may
		// have minted a token while we were queued.
		if t := m.peek(); t != nil && !t.Expired(m.clock(), m.skew) {
			return t.AccessToken, nil
		}
		t, err := m.mint(ctx)
		if err != nil {
			return "", err
		}
		m.set(t)
		return t.AccessToken, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// Refresh bypasses the cache and forces a fresh mint.
func (m *OAuth2Manager) Refresh(ctx context.Context) (string, error) {
	v, err, _ := m.group.Do("refresh", func() (any, error) {
		t, err := m.mint(ctx)
		if err != nil {
			return "", err
		}
		m.set(t)
		return t.AccessToken, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// Cached returns the current cached token, if any.
func (m *OAuth2Manager) Cached() (Token, bool) {
	if t := m.peek(); t != nil {
		return *t, true
	}
	return Token{}, false
}

func (m *OAuth2Manager) peek() *Token {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cached
}

func (m *OAuth2Manager) set(t *Token) {
	m.mu.Lock()
	m.cached = t
	m.mu.Unlock()
}

func (m *OAuth2Manager) mint(ctx context.Context) (*Token, error) {
	url := m.baseURL + tokenPath
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(grantBody))
	if err != nil {
		return nil, &TransportError{Op: "POST " + tokenPath, Cause: err}
	}
	creds := m.publicKey + ":" + m.secretKey
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	issuedAt := m.clock()
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, &TransportError{Op: "POST " + tokenPath, Cause: err}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		oauthErr := tryReadOAuthErrorCode(body)
		return nil, &AuthError{
			HTTPStatus: resp.StatusCode,
			OAuthError: oauthErr,
			Message:    fmt.Sprintf("OAuth token mint failed: %s", string(body)),
		}
	}
	return parseTokenResponse(body, issuedAt)
}

func parseTokenResponse(body []byte, issuedAt time.Time) (*Token, error) {
	var payload struct {
		AccessToken string  `json:"access_token"`
		TokenType   *string `json:"token_type"`
		ExpiresIn   *int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &AuthError{
			HTTPStatus: 200,
			Message:    "OAuth token response not valid JSON",
			Cause:      err,
		}
	}
	if payload.AccessToken == "" {
		return nil, &AuthError{HTTPStatus: 200, Message: "OAuth token response missing access_token"}
	}
	if payload.ExpiresIn == nil {
		return nil, &AuthError{HTTPStatus: 200, Message: "OAuth token response missing expires_in"}
	}
	tokenType := "Bearer"
	if payload.TokenType != nil && *payload.TokenType != "" {
		tokenType = *payload.TokenType
	}
	return &Token{
		AccessToken: payload.AccessToken,
		TokenType:   tokenType,
		ExpiresAt:   issuedAt.Add(time.Duration(*payload.ExpiresIn) * time.Second),
	}, nil
}

func tryReadOAuthErrorCode(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var p struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return ""
	}
	return p.Error
}
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: all OAuth tests PASS, including the singleflight dedup test (server hit count == 1 under 8 concurrent callers).

- [ ] **Step 5: Commit**

```bash
git add internal/apiclient/oauth2.go internal/apiclient/oauth2_test.go
git commit -m "feat: OAuth 2.0 client_credentials token manager with singleflight"
```

---

## Task 9: internal/apiclient/transport.go — HTTP transport

> **Cycle note:** Tests use `package apiclient_test` (external) for the same reason as Task 8.

**Files:**
- Create: `internal/apiclient/transport.go`
- Create: `internal/apiclient/transport_test.go`

- [ ] **Step 1: Write failing tests (`internal/apiclient/transport_test.go`)**

```go
package apiclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maviance/smobilpay-go/internal/apiclient"
)

type pingDTO struct {
	Time    string `json:"time"`
	Version string `json:"version"`
}

const tokenPath = "/oauth/token"

func okTokenHandler(token string, expiresIn int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":%q,"token_type":"Bearer","expires_in":%d}`, token, expiresIn)
	}
}

func newTestTransport(t *testing.T, srvURL string, tokenSrvURL string) *apiclient.Transport {
	t.Helper()
	mgr := apiclient.NewOAuth2Manager(tokenSrvURL, "pub", "sec", 30*time.Second, http.DefaultClient, time.Now)
	return apiclient.NewTransport(srvURL, "3.0.0", http.DefaultClient, mgr)
}

func TestTransport_GET_attachesHeaders(t *testing.T) {
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			okTokenHandler("jwt-X", 3600)(w, r)
			return
		}
		captured = r
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pingDTO{Time: "2024-01-01T00:00:00Z", Version: "3.0.0"})
	}))
	defer srv.Close()

	tr := newTestTransport(t, srv.URL, srv.URL)
	var out pingDTO
	if err := tr.Get(context.Background(), "/v2/ping", apiclient.NewQuery(), &out); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := captured.Header.Get("Authorization"); got != "Bearer jwt-X" {
		t.Errorf("Authorization = %q", got)
	}
	if got := captured.Header.Get("x-api-version"); got != "3.0.0" {
		t.Errorf("x-api-version = %q", got)
	}
	if got := captured.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
	if captured.URL.Path != "/v2/ping" {
		t.Errorf("Path = %q", captured.URL.Path)
	}
}

func TestTransport_GET_appendsQuery(t *testing.T) {
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			okTokenHandler("jwt-X", 3600)(w, r)
			return
		}
		captured = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "[]")
	}))
	defer srv.Close()

	tr := newTestTransport(t, srv.URL, srv.URL)
	q := apiclient.NewQuery().Add("serviceid", int64(20053))
	var out []map[string]any
	if err := tr.Get(context.Background(), "/v2/cashout", q, &out); err != nil {
		t.Fatal(err)
	}
	if captured.URL.RawQuery != "serviceid=20053" {
		t.Errorf("RawQuery = %q", captured.URL.RawQuery)
	}
}

func TestTransport_POST_serializesBody(t *testing.T) {
	var capturedBody string
	var capturedCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			okTokenHandler("jwt-X", 3600)(w, r)
			return
		}
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		capturedCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"quoteId":"abc"}`)
	}))
	defer srv.Close()

	tr := newTestTransport(t, srv.URL, srv.URL)
	body := map[string]any{"amount": 500, "payItemId": "X"}
	var out map[string]any
	if err := tr.Post(context.Background(), "/v2/quotestd", body, &out); err != nil {
		t.Fatal(err)
	}
	if capturedCT != "application/json" {
		t.Errorf("Content-Type = %q", capturedCT)
	}
	if capturedBody != `{"amount":500,"payItemId":"X"}` {
		t.Errorf("body = %s", capturedBody)
	}
}

func TestTransport_nonJSONResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			okTokenHandler("jwt-X", 3600)(w, r)
			return
		}
		_, _ = io.WriteString(w, "not json")
	}))
	defer srv.Close()

	tr := newTestTransport(t, srv.URL, srv.URL)
	var out pingDTO
	err := tr.Get(context.Background(), "/v2/ping", apiclient.NewQuery(), &out)
	var te *apiclient.TransportError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TransportError, got %v", err)
	}
}

func TestTransport_4xx_extractsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			okTokenHandler("jwt-X", 3600)(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(498)
		_, _ = fmt.Fprint(w, `{"respCode":41001,"devMsg":"quote expired","usrMsg":"u","link":"l"}`)
	}))
	defer srv.Close()

	tr := newTestTransport(t, srv.URL, srv.URL)
	var out pingDTO
	err := tr.Get(context.Background(), "/v2/ping", apiclient.NewQuery(), &out)
	var ae *apiclient.APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *APIError, got %v", err)
	}
	if ae.HTTPStatus != 498 || ae.RespCode != 41001 || ae.DevMsg != "quote expired" {
		t.Errorf("got %+v", ae)
	}
}

func TestTransport_4xx_bodyNotEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			okTokenHandler("jwt-X", 3600)(w, r)
			return
		}
		w.WriteHeader(503)
		_, _ = io.WriteString(w, "Service Unavailable")
	}))
	defer srv.Close()

	tr := newTestTransport(t, srv.URL, srv.URL)
	var out pingDTO
	err := tr.Get(context.Background(), "/v2/ping", apiclient.NewQuery(), &out)
	var ae *apiclient.APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *APIError, got %v", err)
	}
	if ae.HTTPStatus != 503 || ae.RespCode != 0 || ae.RawBody != "Service Unavailable" {
		t.Errorf("got %+v", ae)
	}
}

func TestTransport_normalizesTrailingSlash(t *testing.T) {
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			okTokenHandler("jwt-X", 3600)(w, r)
			return
		}
		captured = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{}")
	}))
	defer srv.Close()

	// Pass the trailing-slashed baseURL directly; NewTransport normalises.
	mgr := apiclient.NewOAuth2Manager(srv.URL+"/", "p", "s", 30*time.Second, http.DefaultClient, time.Now)
	tr := apiclient.NewTransport(srv.URL+"/", "3.0.0", http.DefaultClient, mgr)
	var out map[string]any
	if err := tr.Get(context.Background(), "v2/ping", apiclient.NewQuery(), &out); err != nil {
		t.Fatal(err)
	}
	if captured.URL.Path != "/v2/ping" {
		t.Errorf("path = %q", captured.URL.Path)
	}
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./internal/apiclient`
Expected: build failure ("Transport undefined").

- [ ] **Step 3: Implement `internal/apiclient/transport.go`**

```go
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Transport executes authenticated requests against the partner API.
type Transport struct {
	baseURL    string
	apiVersion string
	httpClient *http.Client
	tokens     *OAuth2Manager
}

// NewTransport builds a Transport. The OAuth2Manager is consulted on
// every call to attach the bearer header. baseURL trailing slashes
// are normalised.
func NewTransport(baseURL, apiVersion string, httpClient *http.Client, tokens *OAuth2Manager) *Transport {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Transport{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiVersion: apiVersion,
		httpClient: httpClient,
		tokens:     tokens,
	}
}

// Tokens exposes the bound token manager (for diagnostics / refresh).
func (t *Transport) Tokens() *OAuth2Manager { return t.tokens }

// Get executes an authenticated GET and decodes the JSON response body
// into out. Pass nil for out to discard the body.
func (t *Transport) Get(ctx context.Context, path string, q *Query, out any) error {
	req, err := t.newRequest(ctx, "GET", path, q, nil)
	if err != nil {
		return err
	}
	return t.do(req, out)
}

// Post executes an authenticated POST with body serialized as JSON and
// decodes the JSON response body into out.
func (t *Transport) Post(ctx context.Context, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return &TransportError{Op: "POST " + path, Cause: err}
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := t.newRequest(ctx, "POST", path, nil, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return t.do(req, out)
}

func (t *Transport) newRequest(ctx context.Context, method, path string, q *Query, body io.Reader) (*http.Request, error) {
	op := method + " " + path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := t.baseURL + path
	if q != nil && !q.IsEmpty() {
		url += "?" + q.Encode()
	}
	bearer, err := t.tokens.AccessToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, &TransportError{Op: op, Cause: err}
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("x-api-version", t.apiVersion)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (t *Transport) do(req *http.Request, out any) error {
	op := req.Method + " " + req.URL.Path
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return &TransportError{Op: op, Cause: err}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp.StatusCode, body)
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &TransportError{Op: op, Cause: fmt.Errorf("decode response: %w", err)}
	}
	return nil
}

func decodeAPIError(status int, body []byte) error {
	apiErr := &APIError{HTTPStatus: status, RawBody: string(body)}
	if len(body) > 0 {
		var env struct {
			RespCode int    `json:"respCode"`
			DevMsg   string `json:"devMsg"`
			UsrMsg   string `json:"usrMsg"`
			Link     string `json:"link"`
		}
		if err := json.Unmarshal(body, &env); err == nil && env.RespCode != 0 {
			apiErr.RespCode = env.RespCode
			apiErr.DevMsg = env.DevMsg
			apiErr.UsrMsg = env.UsrMsg
			apiErr.Link = env.Link
		}
	}
	return apiErr
}
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: every transport test PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/apiclient/transport.go internal/apiclient/transport_test.go
git commit -m "feat: HTTP transport with header injection and APIError envelope decoding"
```

---

## Task 10: client.go — Client facade

**Files:**
- Create: `client.go`
- Create: `client_test.go`

The API group structs (`VerifyAPI`, `MasterdataAPI`, etc.) are not yet
defined — they're added in Tasks 11–15. For now, the facade declares
the fields and `New()` wires them to a shared transport.

- [ ] **Step 1: Write failing tests (`client_test.go`)**

```go
package smobilpay

import (
	"strings"
	"testing"
)

func TestNew_initializesAllAPIGroups(t *testing.T) {
	cfg, err := NewConfig(
		WithBaseURL("https://api.example.invalid"),
		WithCredentials("pub", "sec"),
	)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Verify == nil {
		t.Error("Verify is nil")
	}
	if c.Masterdata == nil {
		t.Error("Masterdata is nil")
	}
	if c.Initiate == nil {
		t.Error("Initiate is nil")
	}
	if c.Confirm == nil {
		t.Error("Confirm is nil")
	}
	if c.AccountValidation == nil {
		t.Error("AccountValidation is nil")
	}
	if c.Tokens == nil {
		t.Error("Tokens is nil")
	}
}

func TestNew_rejectsInvalidConfig(t *testing.T) {
	// Zero Config has no BaseURL; that's the input we must reject.
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error on empty Config")
	}
	if !strings.Contains(err.Error(), "BaseURL") &&
		!strings.Contains(err.Error(), "Credentials") {
		t.Errorf("unexpected message: %v", err)
	}
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./...`
Expected: build failure ("New undefined").

- [ ] **Step 3: Implement `client.go`**

```go
package smobilpay

import (
	"errors"

	"github.com/maviance/smobilpay-go/internal/apiclient"
)

// Client is the top-level entry point for the Smobilpay partner API.
// Construct via New + a validated Config; the client is safe for
// concurrent use and intended to be reused for the lifetime of the
// application.
type Client struct {
	cfg       Config
	transport *apiclient.Transport

	Verify            *VerifyAPI
	Masterdata        *MasterdataAPI
	Initiate          *InitiateAPI
	Confirm           *ConfirmAPI
	AccountValidation *AccountValidationAPI
	Tokens            *apiclient.OAuth2Manager
}

// New constructs a Client. The Config must have been built via
// NewConfig; passing a zero Config returns an error.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" || cfg.PublicKey == "" || cfg.SecretKey == "" || cfg.HTTPClient == nil {
		return nil, errors.New("smobilpay: Config is incomplete; use NewConfig + WithBaseURL + WithCredentials")
	}
	tokens := apiclient.NewOAuth2Manager(
		cfg.BaseURL, cfg.PublicKey, cfg.SecretKey,
		cfg.TokenRefreshSkew, cfg.HTTPClient, nil)
	tr := apiclient.NewTransport(cfg.BaseURL, cfg.APIVersion, cfg.HTTPClient, tokens)
	return &Client{
		cfg:               cfg,
		transport:         tr,
		Verify:            &VerifyAPI{tr: tr},
		Masterdata:        &MasterdataAPI{tr: tr},
		Initiate:          &InitiateAPI{tr: tr},
		Confirm:           &ConfirmAPI{tr: tr},
		AccountValidation: &AccountValidationAPI{tr: tr},
		Tokens:            tokens,
	}, nil
}

// Config returns the configuration the client was constructed with.
func (c *Client) Config() Config { return c.cfg }
```

At this point the package will not compile because `VerifyAPI` and the
other API-group structs are not defined yet. Tasks 11–15 add them. To
keep the commit chain green, the next step adds **placeholder structs**
without methods so the package compiles. The methods themselves are
added in subsequent tasks.

- [ ] **Step 4: Add empty placeholder structs in `client.go`**

Append to `client.go`:

```go
// API-group placeholders. Methods are added in subsequent commits
// (Tasks 11–15).

// VerifyAPI exposes /v2/ping, /v2/account, /v2/verifytx, /v2/historystd.
type VerifyAPI struct{ tr *apiclient.Transport }

// MasterdataAPI exposes /v2/merchant, /v2/service, /v2/cashout,
// /v2/cashin, /v2/topup, /v2/product, /v2/voucher.
type MasterdataAPI struct{ tr *apiclient.Transport }

// InitiateAPI exposes /v2/bill, /v2/subscription, /v2/quotestd.
type InitiateAPI struct{ tr *apiclient.Transport }

// ConfirmAPI exposes /v2/collectstd.
type ConfirmAPI struct{ tr *apiclient.Transport }

// AccountValidationAPI exposes /v2/verify, /v2/validate.
type AccountValidationAPI struct{ tr *apiclient.Transport }
```

- [ ] **Step 5: Confirm tests pass and the package builds**

Run: `go build ./... && go test -race ./...`
Expected: builds and `client_test.go` PASS.

- [ ] **Step 6: Commit**

```bash
git add client.go client_test.go
git commit -m "feat: Client facade with API-group placeholders"
```

---

## Task 11: verify.go — VerifyAPI + Ping/Account/PaymentStatus/Commission

**Files:**
- Create: `verify.go`
- Create: `verify_test.go`

- [ ] **Step 1: Write failing tests (`verify_test.go`)**

```go
package smobilpay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newMockClient stands up an httptest server that always responds to the
// OAuth token mint and routes all other paths to handler.
func newMockClient(t *testing.T, handler http.HandlerFunc) (*Client, *http.Request) {
	t.Helper()
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"jwt-X","token_type":"Bearer","expires_in":3600}`)
			return
		}
		captured = r
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	cfg, err := NewConfig(WithBaseURL(srv.URL), WithCredentials("pub", "sec"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c, captured
}

func TestVerify_Ping(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v2/ping" {
			http.Error(w, "bad", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"time":"2024-01-15T10:30:00Z","version":"3.0.0","nonce":"n1","key":"k1"}`)
	})
	p, err := c.Verify.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if p.Version != "3.0.0" || p.Nonce != "n1" || p.Key != "k1" {
		t.Errorf("got %+v", p)
	}
	if y, m, d := p.Time.Date(); y != 2024 || m != time.January || d != 15 {
		t.Errorf("time = %v", p.Time)
	}
}

func TestVerify_Account(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"balance":1000.5,"currency":"XAF","key":"k","agentId":"a1","agentName":"A","agentAddress":"x","agentPhonenumber":"237699999999","companyName":"C","companyAddress":"y","companyPhonenumber":"237699999998","limitMax":10000,"limitRemaining":9000}`)
	})
	a, err := c.Verify.Account(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a.Currency != "XAF" || a.Balance != 1000.5 || a.AgentID != "a1" {
		t.Errorf("got %+v", a)
	}
}

func TestVerify_VerifyTransaction_requiresOneParam(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.Verify.VerifyTransaction(context.Background(), "", ""); err == nil {
		t.Error("expected error when both ptn and trid are empty")
	}
}

func TestVerify_VerifyTransaction_byPtn(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "ptn=P-1" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"ptn":"P-1","serviceid":"20053","merchant":"M","timestamp":"2024-01-15T10:30:00Z","receiptNumber":"r","veriCode":"v","trid":"t","priceLocalCur":500,"priceSystemCur":500,"localCur":"XAF","systemCur":"XAF","status":"SUCCESS","payItemId":"pi","errorCode":0}]`)
	})
	got, err := c.Verify.VerifyTransaction(context.Background(), "P-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PTN != "P-1" || got[0].Status != PaymentStatusSuccess {
		t.Errorf("got %+v", got)
	}
}

func TestVerify_HistoryByDateRange_validatesOrder(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	from := time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	if _, err := c.Verify.HistoryByDateRange(context.Background(), from, to); err == nil {
		t.Error("expected error when to < from")
	}
}

func TestVerify_HistoryByDateRange_sendsTimestamps(t *testing.T) {
	var captured string
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	from := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 5, 31, 0, 0, 0, 0, time.UTC)
	if _, err := c.Verify.HistoryByDateRange(context.Background(), from, to); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(captured, "timestamp_from=2024-05-01") ||
		!strings.Contains(captured, "timestamp_to=2024-05-31") {
		t.Errorf("query = %q", captured)
	}
}

// Ensure PaymentStatus.ServiceID stays a string (per partner spec) even
// though Service.ServiceID elsewhere is int64.
func TestPaymentStatus_ServiceIDIsString(t *testing.T) {
	raw := `{"ptn":"P","serviceid":"99","merchant":"M","timestamp":"2024-01-01T00:00:00Z","receiptNumber":"r","veriCode":"v","trid":"t","priceLocalCur":1,"priceSystemCur":1,"localCur":"X","systemCur":"X","status":"SUCCESS","payItemId":"p","errorCode":0}`
	var s PaymentStatus
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	if s.ServiceID != "99" {
		t.Errorf("ServiceID = %q", s.ServiceID)
	}
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./...`
Expected: build failure ("PaymentStatus undefined", "VerifyAPI.Ping undefined", ...).

- [ ] **Step 3: Implement `verify.go`**

```go
package smobilpay

import (
	"context"
	"errors"
	"time"

	"github.com/maviance/smobilpay-go/internal/apiclient"
)

// Ping is the response from GET /v2/ping — an authenticated round-trip
// probe that echoes server time, version, request nonce, and the public
// access key used to authenticate.
type Ping struct {
	Time    time.Time `json:"time"`
	Version string    `json:"version"`
	Nonce   string    `json:"nonce"`
	Key     string    `json:"key"`
}

// Account is the authenticated agent's profile, returned by GET /v2/account.
type Account struct {
	Balance            float64 `json:"balance"`
	Currency           string  `json:"currency"`
	Key                string  `json:"key"`
	AgentID            string  `json:"agentId"`
	AgentName          string  `json:"agentName"`
	AgentAddress       string  `json:"agentAddress"`
	AgentPhoneNumber   string  `json:"agentPhonenumber"`
	CompanyName        string  `json:"companyName"`
	CompanyAddress     string  `json:"companyAddress"`
	CompanyPhoneNumber string  `json:"companyPhonenumber"`
	LimitMax           float64 `json:"limitMax"`
	LimitRemaining     float64 `json:"limitRemaining"`
}

// Commission is the optional commission record on PaymentStatus.
type Commission struct {
	Earnings *float64 `json:"earnings"`
	Currency string   `json:"currency"`
}

// PaymentStatus is the current state of a previously-issued payment
// collection. ServiceID is a string here per the partner spec (it is an
// int64 elsewhere — e.g. Service.ServiceID).
type PaymentStatus struct {
	PTN            string            `json:"ptn"`
	ServiceID      string            `json:"serviceid"`
	Merchant       string            `json:"merchant"`
	Timestamp      time.Time         `json:"timestamp"`
	ReceiptNumber  string            `json:"receiptNumber"`
	VeriCode       string            `json:"veriCode"`
	ClearingDate   Date              `json:"clearingDate"`
	TRID           string            `json:"trid"`
	PriceLocalCur  *float64          `json:"priceLocalCur"`
	PriceSystemCur *float64          `json:"priceSystemCur"`
	LocalCur       string            `json:"localCur"`
	SystemCur      string            `json:"systemCur"`
	PIN            string            `json:"pin,omitempty"`
	Status         PaymentStatusType `json:"status"`
	PayItemID      string            `json:"payItemId"`
	PayItemDescr   string            `json:"payItemDescr,omitempty"`
	ErrorCode      int               `json:"errorCode"`
	Tag            string            `json:"tag,omitempty"`
	Commission     *Commission       `json:"commission,omitempty"`
}

// Ping returns the result of GET /v2/ping. Used as an auth probe.
func (v *VerifyAPI) Ping(ctx context.Context) (Ping, error) {
	var out Ping
	err := v.tr.Get(ctx, "/v2/ping", apiclient.NewQuery(), &out)
	return out, err
}

// Account returns the authenticated agent's profile (GET /v2/account).
func (v *VerifyAPI) Account(ctx context.Context) (Account, error) {
	var out Account
	err := v.tr.Get(ctx, "/v2/account", apiclient.NewQuery(), &out)
	return out, err
}

// VerifyTransaction looks up the live status of a previously-issued
// collection by PTN, by caller TRID, or both. At least one of the two
// must be non-empty.
func (v *VerifyAPI) VerifyTransaction(ctx context.Context, ptn, trid string) ([]PaymentStatus, error) {
	if ptn == "" && trid == "" {
		return nil, errors.New("smobilpay: VerifyTransaction requires ptn or trid")
	}
	q := apiclient.NewQuery().Add("ptn", ptn).Add("trid", trid)
	var out []PaymentStatus
	err := v.tr.Get(ctx, "/v2/verifytx", q, &out)
	return out, err
}

// HistoryByPtn returns the history record for a specific PTN (GET /v2/historystd).
func (v *VerifyAPI) HistoryByPtn(ctx context.Context, ptn string) ([]PaymentStatus, error) {
	if ptn == "" {
		return nil, errors.New("smobilpay: HistoryByPtn requires ptn")
	}
	q := apiclient.NewQuery().Add("ptn", ptn)
	var out []PaymentStatus
	err := v.tr.Get(ctx, "/v2/historystd", q, &out)
	return out, err
}

// HistoryByTrid returns history records for a caller TRID.
func (v *VerifyAPI) HistoryByTrid(ctx context.Context, trid string) ([]PaymentStatus, error) {
	if trid == "" {
		return nil, errors.New("smobilpay: HistoryByTrid requires trid")
	}
	q := apiclient.NewQuery().Add("trid", trid)
	var out []PaymentStatus
	err := v.tr.Get(ctx, "/v2/historystd", q, &out)
	return out, err
}

// HistoryByDateRange returns history records over an inclusive date
// range. The dates are sent as ISO-8601 with the Z offset.
func (v *VerifyAPI) HistoryByDateRange(ctx context.Context, from, to time.Time) ([]PaymentStatus, error) {
	if from.IsZero() || to.IsZero() {
		return nil, errors.New("smobilpay: HistoryByDateRange requires both from and to")
	}
	if to.Before(from) {
		return nil, errors.New("smobilpay: HistoryByDateRange: 'to' is before 'from'")
	}
	q := apiclient.NewQuery().
		Add("timestamp_from", from.UTC().Format("2006-01-02")).
		Add("timestamp_to", to.UTC().Format("2006-01-02"))
	var out []PaymentStatus
	err := v.tr.Get(ctx, "/v2/historystd", q, &out)
	return out, err
}
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: every verify_test PASS.

- [ ] **Step 5: Commit**

```bash
git add verify.go verify_test.go
git commit -m "feat: VerifyAPI (ping, account, verifytx, historystd) and its DTOs"
```

---

## Task 12: masterdata.go — MasterdataAPI + catalog DTOs

**Files:**
- Create: `masterdata.go`
- Create: `masterdata_test.go`

- [ ] **Step 1: Write failing tests (`masterdata_test.go`)**

```go
package smobilpay

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestMasterdata_Merchants(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/merchant" {
			http.Error(w, "bad", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"merchant":"ENEO","name":"ENEO","description":"d","country":"CMR","status":"Active","logo":"u","logoHash":"h"}]`)
	})
	got, err := c.Masterdata.Merchants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Merchant != "ENEO" || got[0].Status != MerchantStatusActive {
		t.Errorf("got %+v", got)
	}
}

func TestMasterdata_Services(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":10039,"merchant":"ENEO","title":"ENEO","description":"d","category":"c","country":"CMR","localCur":"XAF","type":"SEARCHABLE_BILL","status":"Active","isReqCustomerName":false,"isReqCustomerAddress":false,"isReqCustomerNumber":false,"isReqServiceNumber":true,"isVerifiable":false}]`)
	})
	got, err := c.Masterdata.Services(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ServiceID != 10039 || got[0].Type != ServiceTypeSearchableBill {
		t.Errorf("got %+v", got)
	}
}

func TestMasterdata_Cashouts_sendsServiceID(t *testing.T) {
	c, captured := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":20053,"merchant":"MOMO","payItemId":"PI","amountType":"FIXED","localCur":"XAF","name":"N"}]`)
	})
	_ = captured // not used directly; assertion via re-read
	got, err := c.Masterdata.Cashouts(context.Background(), 20053)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PayItemIDValue != "PI" {
		t.Errorf("got %+v", got)
	}
}

func TestMasterdata_Cashouts_omitsZeroServiceID(t *testing.T) {
	var capturedQuery string
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	if _, err := c.Masterdata.Cashouts(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if capturedQuery != "" {
		t.Errorf("expected no query for zero serviceID, got %q", capturedQuery)
	}
}

func TestPaymentItem_interfaceSatisfied(t *testing.T) {
	// Compile-time check: every concrete catalog type must satisfy
	// PaymentItem. The asserts blow up at compile time if not.
	var (
		_ PaymentItem = (*Cashout)(nil)
		_ PaymentItem = (*Cashin)(nil)
		_ PaymentItem = (*Topup)(nil)
		_ PaymentItem = (*Product)(nil)
		_ PaymentItem = (*Bill)(nil)
		_ PaymentItem = (*Subscription)(nil)
	)
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./...`
Expected: build failure ("Merchant undefined").

- [ ] **Step 3: Implement `masterdata.go`**

```go
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
```

> **Note:** `Bill` and `Subscription` are added in Task 13 (`initiate.go`) so the PaymentItem compile-time check in `masterdata_test.go` references them. Add stub types in `masterdata.go` to keep the test compilable for this commit:

Append to `masterdata.go`:

```go
// Bill and Subscription are declared here as zero-value placeholders so
// they exist for compile-time PaymentItem interface assertions; the
// real fields and methods land in initiate.go (Task 13).
type Bill struct{ paymentItemBase }
type Subscription struct{ paymentItemBase }
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: every masterdata_test PASS, including the PaymentItem compile-time interface assertions.

- [ ] **Step 5: Commit**

```bash
git add masterdata.go masterdata_test.go
git commit -m "feat: MasterdataAPI + Merchant/Service/Cashout/Cashin/Topup/Product DTOs"
```

---

## Task 13: initiate.go — InitiateAPI + Bill/Subscription/QuoteRequest/QuoteResponse

**Files:**
- Modify: `masterdata.go` (remove the placeholder Bill / Subscription stubs)
- Create: `initiate.go`
- Create: `initiate_test.go`

- [ ] **Step 1: Remove placeholder stubs from `masterdata.go`**

Delete the block:

```go
type Bill struct{ paymentItemBase }
type Subscription struct{ paymentItemBase }
```

- [ ] **Step 2: Write failing tests (`initiate_test.go`)**

```go
package smobilpay

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestInitiate_Bills_requiresParams(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.Initiate.Bills(context.Background(), "", 0, ""); err == nil {
		t.Error("expected error on empty merchant")
	}
	if _, err := c.Initiate.Bills(context.Background(), "M", 0, ""); err == nil {
		t.Error("expected error on empty serviceNumber")
	}
}

func TestInitiate_Bills(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		want := "merchant=ENEO&serviceid=10039&serviceNumber=203157530"
		if r.URL.RawQuery != want {
			t.Errorf("query = %q, want %q", r.URL.RawQuery, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":10039,"merchant":"ENEO","payItemId":"PI","amountType":"FIXED","localCur":"XAF","name":"N","amountLocalCur":1234.5,"billType":"REGULAR","payOrder":1,"serviceNumber":"203157530","billDate":"2024-01-15","billDueDate":"2024-02-15"}]`)
	})
	got, err := c.Initiate.Bills(context.Background(), "ENEO", 10039, "203157530")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].BillType != BillTypeRegular || got[0].PayOrder != 1 {
		t.Errorf("got %+v", got)
	}
	if y, m, _ := got[0].BillDate.Date(); y != 2024 || m != time.January {
		t.Errorf("BillDate = %v", got[0].BillDate)
	}
}

func TestInitiate_Subscriptions_requiresOneOfNumberOrCustomer(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.Initiate.Subscriptions(context.Background(), "M", 1, "", ""); err == nil {
		t.Error("expected error when both serviceNumber and customerNumber empty")
	}
}

func TestInitiate_Subscriptions_serviceNumberOnly(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "serviceNumber=DEC-1") ||
			strings.Contains(r.URL.RawQuery, "customerNumber=") {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"serviceid":5000,"merchant":"CMSABC","payItemId":"PI","amountType":"FIXED","localCur":"XAF","name":"N","serviceNumber":"DEC-1"}]`)
	})
	got, err := c.Initiate.Subscriptions(context.Background(), "CMSABC", 5000, "DEC-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ServiceNumber != "DEC-1" {
		t.Errorf("got %+v", got)
	}
}

func TestInitiate_Quote_validatesAmount(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.Initiate.Quote(context.Background(), QuoteRequest{Amount: 0, PayItemID: "X"}); err == nil {
		t.Error("expected error on amount<1")
	}
	if _, err := c.Initiate.Quote(context.Background(), QuoteRequest{Amount: 1, PayItemID: ""}); err == nil {
		t.Error("expected error on empty PayItemID")
	}
}

func TestInitiate_Quote(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v2/quotestd" {
			http.Error(w, "bad", 400)
			return
		}
		b, _ := io.ReadAll(r.Body)
		if string(b) != `{"amount":500,"payItemId":"PI"}` {
			t.Errorf("body = %s", b)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"quoteId":"q-1","expiresAt":"2024-01-15T10:35:00Z","payItemId":"PI","amountLocalCur":500,"priceLocalCur":510,"localCur":"XAF","systemCur":"XAF"}`)
	})
	q, err := c.Initiate.Quote(context.Background(), QuoteRequest{Amount: 500, PayItemID: "PI"})
	if err != nil {
		t.Fatal(err)
	}
	if q.QuoteID != "q-1" || q.PayItemID != "PI" {
		t.Errorf("got %+v", q)
	}
}
```

- [ ] **Step 3: Confirm tests fail**

Run: `go test ./...`
Expected: build failure ("QuoteRequest undefined", ...).

- [ ] **Step 4: Implement `initiate.go`**

```go
package smobilpay

import (
	"context"
	"errors"
	"time"

	"github.com/maviance/smobilpay-go/internal/apiclient"
)

// Bill is a bill payment item returned by /v2/bill.
type Bill struct {
	paymentItemBase
	BillType       BillType `json:"billType"`
	PenaltyAmount  *float64 `json:"penaltyAmount,omitempty"`
	PayOrder       int      `json:"payOrder"`
	ServiceNumber  string   `json:"serviceNumber"`
	BillNumber     string   `json:"billNumber,omitempty"`
	CustomerNumber string   `json:"customerNumber,omitempty"`
	BillMonth      string   `json:"billMonth,omitempty"`
	BillYear       string   `json:"billYear,omitempty"`
	BillDate       Date     `json:"billDate,omitempty"`
	BillDueDate    Date     `json:"billDueDate,omitempty"`
}

// Subscription is a subscription payment item returned by /v2/subscription.
type Subscription struct {
	paymentItemBase
	ServiceNumber     string `json:"serviceNumber"`
	CustomerReference string `json:"customerReference,omitempty"`
	CustomerName      string `json:"customerName,omitempty"`
	CustomerNumber    string `json:"customerNumber,omitempty"`
	StartDate         Date   `json:"startDate,omitempty"`
	DueDate           Date   `json:"dueDate,omitempty"`
	EndDate           Date   `json:"endDate,omitempty"`
}

// QuoteRequest is the body for POST /v2/quotestd. Amount must be >= 1.
type QuoteRequest struct {
	Amount    int    `json:"amount"`
	PayItemID string `json:"payItemId"`
}

// QuoteResponse is the response from POST /v2/quotestd. QuoteID is the
// value to pass to Confirm.Collect. Honor ExpiresAt — quotes are
// short-lived; on HTTP 498 re-quote and retry.
type QuoteResponse struct {
	QuoteID        string    `json:"quoteId"`
	ExpiresAt      time.Time `json:"expiresAt"`
	PayItemID      string    `json:"payItemId"`
	AmountLocalCur *float64  `json:"amountLocalCur,omitempty"`
	PriceLocalCur  *float64  `json:"priceLocalCur,omitempty"`
	PriceSystemCur *float64  `json:"priceSystemCur,omitempty"`
	LocalCur       string    `json:"localCur"`
	SystemCur      string    `json:"systemCur"`
	Promotion      string    `json:"promotion,omitempty"`
}

// Bills searches bills for a service number. For SEARCHABLE_BILL
// services this may return multiple open bills; NON_SEARCHABLE_BILL
// always returns one.
func (i *InitiateAPI) Bills(ctx context.Context, merchant string, serviceID int64, serviceNumber string) ([]Bill, error) {
	if merchant == "" {
		return nil, errors.New("smobilpay: Bills requires merchant")
	}
	if serviceNumber == "" {
		return nil, errors.New("smobilpay: Bills requires serviceNumber")
	}
	q := apiclient.NewQuery().
		Add("merchant", merchant).
		Add("serviceid", serviceID).
		Add("serviceNumber", serviceNumber)
	var out []Bill
	err := i.tr.Get(ctx, "/v2/bill", q, &out)
	return out, err
}

// Subscriptions searches subscriptions by service number, customer
// number, or both. At least one of the two must be non-empty.
func (i *InitiateAPI) Subscriptions(ctx context.Context, merchant string, serviceID int64, serviceNumber, customerNumber string) ([]Subscription, error) {
	if merchant == "" {
		return nil, errors.New("smobilpay: Subscriptions requires merchant")
	}
	if serviceNumber == "" && customerNumber == "" {
		return nil, errors.New("smobilpay: Subscriptions requires serviceNumber or customerNumber")
	}
	q := apiclient.NewQuery().
		Add("merchant", merchant).
		Add("serviceid", serviceID).
		Add("serviceNumber", serviceNumber).
		Add("customerNumber", customerNumber)
	var out []Subscription
	err := i.tr.Get(ctx, "/v2/subscription", q, &out)
	return out, err
}

// Quote requests a price quote for a payment collection. Quotes are
// short-lived; on HTTP 498 (APIError) re-quote before retrying.
func (i *InitiateAPI) Quote(ctx context.Context, req QuoteRequest) (QuoteResponse, error) {
	if req.Amount < 1 {
		return QuoteResponse{}, errors.New("smobilpay: Quote: amount must be >= 1")
	}
	if req.PayItemID == "" {
		return QuoteResponse{}, errors.New("smobilpay: Quote: payItemId is required")
	}
	var out QuoteResponse
	err := i.tr.Post(ctx, "/v2/quotestd", req, &out)
	return out, err
}
```

- [ ] **Step 5: Confirm tests pass**

Run: `go test -race ./...`
Expected: every initiate_test PASS.

- [ ] **Step 6: Commit**

```bash
git add initiate.go initiate_test.go masterdata.go
git commit -m "feat: InitiateAPI + Bill/Subscription/QuoteRequest/QuoteResponse"
```

---

## Task 14: confirm.go — ConfirmAPI + CollectionRequest/Response

**Files:**
- Create: `confirm.go`
- Create: `confirm_test.go`

- [ ] **Step 1: Write failing tests (`confirm_test.go`)**

```go
package smobilpay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCollectionRequest_validation(t *testing.T) {
	cases := []struct {
		name string
		req  CollectionRequest
		want string
	}{
		{"missing quoteId", CollectionRequest{CustomerPhoneNumber: "p", CustomerEmailAddress: "e"}, "quoteId"},
		{"missing phone", CollectionRequest{QuoteID: "q", CustomerEmailAddress: "e"}, "customerPhonenumber"},
		{"missing email", CollectionRequest{QuoteID: "q", CustomerPhoneNumber: "p"}, "customerEmailaddress"},
		{
			"tag too long",
			CollectionRequest{QuoteID: "q", CustomerPhoneNumber: "p", CustomerEmailAddress: "e",
				Tag: strings.Repeat("x", 51)},
			"tag",
		},
		{
			"callback too long",
			CollectionRequest{QuoteID: "q", CustomerPhoneNumber: "p", CustomerEmailAddress: "e",
				CallbackURL: "https://" + strings.Repeat("x", 248)},
			"callbackUrl",
		},
	}
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.Confirm.Collect(context.Background(), tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestCollectionRequest_marshalOmitsNil(t *testing.T) {
	req := CollectionRequest{
		QuoteID:              "q-1",
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "c@example.com",
	}
	b, _ := json.Marshal(req)
	got := string(b)
	if !strings.Contains(got, `"quoteId":"q-1"`) {
		t.Errorf("marshal: missing quoteId in %s", got)
	}
	if strings.Contains(got, "customerName") || strings.Contains(got, "trid") {
		t.Errorf("marshal: emitted nil field in %s", got)
	}
}

func TestConfirm_Collect(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v2/collectstd" {
			http.Error(w, "bad", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ptn":"P-1","timestamp":"2024-01-15T10:35:00Z","agentBalance":900.5,"receiptNumber":"r","veriCode":"v","priceLocalCur":510,"priceSystemCur":510,"localCur":"XAF","systemCur":"XAF","status":"PENDING","payItemId":"PI"}`)
	})
	resp, err := c.Confirm.Collect(context.Background(), CollectionRequest{
		QuoteID:              "q-1",
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "c@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PTN != "P-1" || resp.Status != PaymentStatusPending {
		t.Errorf("got %+v", resp)
	}
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./...`
Expected: build failure ("CollectionRequest undefined").

- [ ] **Step 3: Implement `confirm.go`**

```go
package smobilpay

import (
	"context"
	"fmt"
	"time"
)

// CollectionRequest is the body for POST /v2/collectstd. QuoteID,
// CustomerPhoneNumber, and CustomerEmailAddress are required; all other
// fields are required only when the chosen Service sets the
// corresponding IsReq* flag. Tag <= 50 chars, CallbackURL <= 255 chars.
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
	if r.CustomerEmailAddress == "" {
		return fmt.Errorf("smobilpay: CollectionRequest: customerEmailaddress is required")
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
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: every confirm_test PASS.

- [ ] **Step 5: Commit**

```bash
git add confirm.go confirm_test.go
git commit -m "feat: ConfirmAPI + CollectionRequest/Response with input validation"
```

---

## Task 15: accountvalidation.go — AccountValidationAPI + CustomerAccount

**Files:**
- Create: `accountvalidation.go`
- Create: `accountvalidation_test.go`

- [ ] **Step 1: Write failing tests (`accountvalidation_test.go`)**

```go
package smobilpay

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestVerifyServiceNumber_validates(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.AccountValidation.VerifyServiceNumber(context.Background(), "", 1, "n"); err == nil {
		t.Error("expected error on empty merchant")
	}
	if _, err := c.AccountValidation.VerifyServiceNumber(context.Background(), "M", 1, ""); err == nil {
		t.Error("expected error on empty serviceNumber")
	}
}

func TestVerifyServiceNumber_true(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		want := "merchant=ENEO&serviceid=1001&serviceNumber=203157530"
		if r.URL.RawQuery != want {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `true`)
	})
	ok, err := c.AccountValidation.VerifyServiceNumber(context.Background(), "ENEO", 1001, "203157530")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestValidateAccount(t *testing.T) {
	c, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"VERIFIED","name":"Jane","destination":"677389120"}`)
	})
	ca, err := c.AccountValidation.ValidateAccount(context.Background(), "677389120", 20053)
	if err != nil {
		t.Fatal(err)
	}
	if ca.Status != CustomerAccountStatusVerified || ca.Name != "Jane" {
		t.Errorf("got %+v", ca)
	}
}

func TestValidateAccount_validates(t *testing.T) {
	cfg, _ := NewConfig(WithBaseURL("https://x.invalid"), WithCredentials("p", "s"))
	c, _ := New(cfg)
	if _, err := c.AccountValidation.ValidateAccount(context.Background(), "", 1); err == nil {
		t.Error("expected error on empty destination")
	}
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./...`
Expected: build failure ("CustomerAccount undefined").

- [ ] **Step 3: Implement `accountvalidation.go`**

```go
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
func (a *AccountValidationAPI) VerifyServiceNumber(ctx context.Context, merchant string, serviceID int64, serviceNumber string) (bool, error) {
	if merchant == "" {
		return false, errors.New("smobilpay: VerifyServiceNumber requires merchant")
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
func (a *AccountValidationAPI) ValidateAccount(ctx context.Context, destination string, serviceID int64) (CustomerAccount, error) {
	if destination == "" {
		return CustomerAccount{}, errors.New("smobilpay: ValidateAccount requires destination")
	}
	q := apiclient.NewQuery().
		Add("destination", destination).
		Add("serviceId", serviceID)
	var out CustomerAccount
	err := a.tr.Get(ctx, "/v2/validate", q, &out)
	return out, err
}
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: every accountvalidation_test PASS.

- [ ] **Step 5: Commit**

```bash
git add accountvalidation.go accountvalidation_test.go
git commit -m "feat: AccountValidationAPI + CustomerAccount"
```

---

## Task 16: smoke-test.example.json — config schema template

**Files:**
- Create: `smoke-test.example.json`

- [ ] **Step 1: Write `smoke-test.example.json`**

```json
{
  "_comment": "Acceptance-environment test data. Spec terminology: cashout = collection (money OUT of customer wallet), cashin = disbursement (money INTO recipient wallet). Copy this file to smoke-test.json (which is gitignored) and fill in baseUrl, publicKey, secretKey. Set any per-flow block to null (or remove it) to skip that scenario. The Java client's smoke-test.json schema is drop-in compatible with this Go runner.",

  "baseUrl": "https://api.acceptance.example.invalid",
  "publicKey": "TODO_FILL_IN_PARTNER_PUBLIC_KEY",
  "secretKey": "TODO_FILL_IN_PARTNER_SECRET_KEY",
  "apiVersion": "3.0.0",

  "cashout":      { "serviceId": 20053, "amount": 500 },
  "bill":         { "merchant": "ENEO", "serviceId": 10039, "serviceNumber": "203157530" },
  "topup":        { "serviceId": 20051, "amount": 500 },
  "voucher":      { "serviceId": 90041, "amount": 500 },
  "product":      { "serviceId": 90006 },
  "subscription": { "merchant": "ENEOPREPAID300924", "serviceId": 300924, "serviceNumber": "20191953817", "customerNumber": null, "amount": 1000 },
  "cashin":       { "serviceId": 20052, "amount": 1000 },
  "verify":       { "merchant": "ENEO", "serviceId": 1001, "serviceNumber": "203157530" },
  "validate":     { "destination": "677389120", "serviceId": 20053 }
}
```

- [ ] **Step 2: Commit**

```bash
git add smoke-test.example.json
git commit -m "feat: smoke-test config template (Java-compatible schema)"
```

---

## Task 17: cmd/smoketest/config.go — SmokeConfig schema

**Files:**
- Create: `cmd/smoketest/config.go`
- Create: `cmd/smoketest/config_test.go`

- [ ] **Step 1: Write failing tests (`cmd/smoketest/config_test.go`)**

```go
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleConfig = `{
  "baseUrl": "https://api.example.invalid",
  "publicKey": "pub",
  "secretKey": "sec",
  "apiVersion": "3.0.0",
  "cashout": { "serviceId": 20053, "amount": 500 },
  "bill": { "merchant": "ENEO", "serviceId": 10039, "serviceNumber": "203157530" },
  "voucher": null
}`

func TestSmokeConfig_parses(t *testing.T) {
	var cfg SmokeConfig
	if err := json.Unmarshal([]byte(sampleConfig), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.BaseURL != "https://api.example.invalid" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.PublicKey != "pub" || cfg.SecretKey != "sec" {
		t.Errorf("creds = %+v", cfg)
	}
	if cfg.Cashout == nil || cfg.Cashout.ServiceID != 20053 || cfg.Cashout.Amount != 500 {
		t.Errorf("Cashout = %+v", cfg.Cashout)
	}
	if cfg.Bill == nil || cfg.Bill.Merchant != "ENEO" {
		t.Errorf("Bill = %+v", cfg.Bill)
	}
	if cfg.Voucher != nil {
		t.Errorf("Voucher should be nil for skip")
	}
}

func TestSmokeConfig_validate_requiresFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  SmokeConfig
		want string
	}{
		{"missing baseUrl", SmokeConfig{PublicKey: "p", SecretKey: "s"}, "baseUrl"},
		{"missing publicKey", SmokeConfig{BaseURL: "u", SecretKey: "s"}, "publicKey"},
		{"missing secretKey", SmokeConfig{BaseURL: "u", PublicKey: "p"}, "secretKey"},
		{"happy", SmokeConfig{BaseURL: "u", PublicKey: "p", SecretKey: "s"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.want == "" && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("err = %v, want substring %q", err, tc.want)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Confirm tests fail**

Run: `go test ./cmd/smoketest`
Expected: build failure ("SmokeConfig undefined").

- [ ] **Step 3: Implement `cmd/smoketest/config.go`**

```go
package main

import "errors"

// SmokeConfig is the JSON schema for the smoke-test config file —
// identical to the Java client's SmokeTestConfig so a single JSON file
// works for both runners.
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
	Validate     *ValidateCfg     `json:"validate,omitempty"`
}

// CashoutCfg drives the cashout (collection) scenario.
type CashoutCfg struct {
	ServiceID int64 `json:"serviceId"`
	Amount    int   `json:"amount"`
}

// BillCfg drives the bill payment scenario.
type BillCfg struct {
	Merchant      string `json:"merchant"`
	ServiceID     int64  `json:"serviceId"`
	ServiceNumber string `json:"serviceNumber"`
}

// TopupCfg drives the airtime top-up scenario.
type TopupCfg struct {
	ServiceID int64 `json:"serviceId"`
	Amount    int   `json:"amount"`
}

// VoucherCfg drives the voucher purchase scenario.
type VoucherCfg struct {
	ServiceID int64 `json:"serviceId"`
	Amount    int   `json:"amount"`
}

// ProductCfg drives the product purchase scenario.
type ProductCfg struct {
	ServiceID int64 `json:"serviceId"`
	Amount    int   `json:"amount,omitempty"`
}

// SubscriptionCfg drives the subscription top-up scenario.
type SubscriptionCfg struct {
	Merchant       string `json:"merchant"`
	ServiceID      int64  `json:"serviceId"`
	ServiceNumber  string `json:"serviceNumber,omitempty"`
	CustomerNumber string `json:"customerNumber,omitempty"`
	Amount         int    `json:"amount,omitempty"`
}

// CashinCfg drives the cashin (disbursement) scenario.
type CashinCfg struct {
	ServiceID int64 `json:"serviceId"`
	Amount    int   `json:"amount"`
}

// VerifyCfg drives the pre-payment serviceNumber verification scenario.
type VerifyCfg struct {
	Merchant      string `json:"merchant"`
	ServiceID     int64  `json:"serviceId"`
	ServiceNumber string `json:"serviceNumber"`
}

// ValidateCfg drives the account-validation scenario.
type ValidateCfg struct {
	Destination string `json:"destination"`
	ServiceID   int64  `json:"serviceId"`
}

// Validate enforces that the required top-level credentials are present.
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
```

- [ ] **Step 4: Confirm tests pass**

Run: `go test -race ./...`
Expected: every config_test PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/smoketest/config.go cmd/smoketest/config_test.go
git commit -m "feat: smoketest SmokeConfig schema (Java-compatible)"
```

---

## Task 18: cmd/smoketest/scenarios.go — scenario harness

**Files:**
- Create: `cmd/smoketest/scenarios.go`

This file is the longest in the project — one func per scenario. No
unit tests (it's exercised end-to-end against a real partner
environment; logic is straight-line orchestration).

- [ ] **Step 1: Implement `cmd/smoketest/scenarios.go`**

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	smob "github.com/maviance/smobilpay-go"
)

const sep = "----------------------------------------------------------------------"

// Harness owns counters and the smobilpay client for one smoketest run.
type Harness struct {
	client *smob.Client
	cfg    SmokeConfig

	passed  int
	failed  int
	skipped int
}

// skipError is used by scenarios to signal "this is an expected
// skip" — caught by run() and recorded as SKIP rather than FAIL.
type skipError struct{ msg string }

func (e skipError) Error() string { return e.msg }

func skip(format string, args ...any) error {
	return skipError{msg: fmt.Sprintf(format, args...)}
}

// run wraps a scenario function with the boilerplate banner +
// pass/skip/fail bookkeeping.
func (h *Harness) run(name string, fn func() error) {
	fmt.Println(sep)
	fmt.Println("RUN  " + name)
	err := fn()
	switch {
	case err == nil:
		h.passed++
		fmt.Println("PASS " + name)
	case errors.As(err, new(skipError)):
		var se skipError
		errors.As(err, &se)
		h.skipped++
		fmt.Println("SKIP " + name + " - " + se.msg)
	default:
		h.failed++
		fmt.Println("FAIL " + name + " - " + classify(err))
		printAPIErrorDetail(err)
	}
}

func classify(err error) string {
	var ae *smob.AuthError
	if errors.As(err, &ae) {
		return fmt.Sprintf("auth error (HTTP %d, oauth %s): %s",
			ae.HTTPStatus, ae.OAuthError, ae.Message)
	}
	var pe *smob.APIError
	if errors.As(err, &pe) {
		return fmt.Sprintf("API error (HTTP %d)", pe.HTTPStatus)
	}
	var te *smob.TransportError
	if errors.As(err, &te) {
		return fmt.Sprintf("transport error on %s: %v", te.Op, te.Cause)
	}
	return err.Error()
}

func printAPIErrorDetail(err error) {
	var pe *smob.APIError
	if !errors.As(err, &pe) {
		return
	}
	detail(fmt.Sprintf("respCode: %d", pe.RespCode))
	detail(fmt.Sprintf("devMsg:   %s", pe.DevMsg))
	if pe.UsrMsg != "" {
		detail(fmt.Sprintf("usrMsg:   %s", pe.UsrMsg))
	}
	if pe.Link != "" {
		detail(fmt.Sprintf("link:     %s", pe.Link))
	}
}

func detail(line string) { fmt.Println("     " + line) }

func resolveAmount(item smob.PaymentItem, cfgAmount int) (int, error) {
	if cfgAmount > 0 {
		return cfgAmount, nil
	}
	if a := item.AmountLocalCur(); a != nil && *a >= 1 {
		return int(*a), nil
	}
	return 0, fmt.Errorf("item %s has no fixed catalog amount; set \"amount\" in this block",
		item.PayItemID())
}

func quoteAndReport(ctx context.Context, client *smob.Client, item smob.PaymentItem, amount int) error {
	q, err := client.Initiate.Quote(ctx, smob.QuoteRequest{Amount: amount, PayItemID: item.PayItemID()})
	if err != nil {
		return err
	}
	detail("quoteId:        " + q.QuoteID)
	detail("expiresAt:      " + q.ExpiresAt.Format(time.RFC3339))
	var pl, ps float64
	if q.PriceLocalCur != nil {
		pl = *q.PriceLocalCur
	}
	if q.PriceSystemCur != nil {
		ps = *q.PriceSystemCur
	}
	detail(fmt.Sprintf("price (local):  %g %s", pl, q.LocalCur))
	detail(fmt.Sprintf("price (system): %g %s", ps, q.SystemCur))
	detail("promotion:      " + q.Promotion)
	detail("(intentionally NOT calling /v2/collectstd)")
	return nil
}

// --- Scenarios -------------------------------------------------------

func (h *Harness) scenarioPing(ctx context.Context) error {
	p, err := h.client.Verify.Ping(ctx)
	if err != nil {
		return err
	}
	if p.Version == "" {
		return errors.New("empty response")
	}
	detail("server time:    " + p.Time.Format(time.RFC3339))
	detail("server version: " + p.Version)
	detail("nonce echo:     " + p.Nonce)
	detail("public key:     " + p.Key)
	return nil
}

func (h *Harness) scenarioTokenRefresh(ctx context.Context) error {
	first, err := h.client.Tokens.AccessToken(ctx)
	if err != nil {
		return err
	}
	forced, err := h.client.Tokens.Refresh(ctx)
	if err != nil {
		return err
	}
	if forced == "" {
		return errors.New("refresh returned empty token")
	}
	if _, err := h.client.Verify.Ping(ctx); err != nil {
		return err
	}
	detail("first  bearer prefix: " + prefix(first, 12) + "...")
	detail("forced bearer prefix: " + prefix(forced, 12) + "...")
	detail(fmt.Sprintf("identical: %v", first == forced))
	return nil
}

func prefix(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func (h *Harness) scenarioAccount(ctx context.Context) error {
	a, err := h.client.Verify.Account(ctx)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("agent:           %s (id=%s)", a.AgentName, a.AgentID))
	detail("company:         " + a.CompanyName)
	detail(fmt.Sprintf("balance:         %g %s", a.Balance, a.Currency))
	detail(fmt.Sprintf("daily limit max: %g", a.LimitMax))
	detail(fmt.Sprintf("limit remaining: %g", a.LimitRemaining))
	return nil
}

func (h *Harness) scenarioMerchants(ctx context.Context) error {
	ms, err := h.client.Masterdata.Merchants(ctx)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("merchants: %d", len(ms)))
	sample := 5
	if len(ms) < sample {
		sample = len(ms)
	}
	for i := 0; i < sample; i++ {
		detail(fmt.Sprintf("  - %s : %s (%s, %s)",
			ms[i].Merchant, ms[i].Name, ms[i].Country, ms[i].Status))
	}
	if len(ms) > sample {
		detail(fmt.Sprintf("  ...and %d more", len(ms)-sample))
	}
	return nil
}

func (h *Harness) scenarioServices(ctx context.Context) error {
	ss, err := h.client.Masterdata.Services(ctx)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("services: %d", len(ss)))

	byType := map[smob.ServiceType]int{}
	for _, s := range ss {
		byType[s.Type]++
	}
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, string(t))
	}
	sort.Strings(types)
	detail("distribution by type:")
	for _, t := range types {
		detail(fmt.Sprintf("  - %s: %d", t, byType[smob.ServiceType(t)]))
	}
	listOfType(ss, smob.ServiceTypeVoucher, "VOUCHER services")
	listOfType(ss, smob.ServiceTypeSubscription, "SUBSCRIPTION services")
	listVerifiable(ss)
	return nil
}

func listOfType(services []smob.Service, t smob.ServiceType, label string) {
	var match []smob.Service
	for _, s := range services {
		if s.Type == t {
			match = append(match, s)
		}
	}
	if len(match) == 0 {
		return
	}
	detail(label + ":")
	for _, s := range match {
		detail(fmt.Sprintf("  - serviceId=%d merchant=%s title=%s", s.ServiceID, s.Merchant, s.Title))
	}
}

func listVerifiable(services []smob.Service) {
	var match []smob.Service
	for _, s := range services {
		if s.IsVerifiable {
			match = append(match, s)
		}
	}
	if len(match) == 0 {
		return
	}
	detail("verifiable services (isVerifiable=true) — candidates for the 'verify' block:")
	for _, s := range match {
		detail(fmt.Sprintf("  - serviceId=%d merchant=%s title=%s", s.ServiceID, s.Merchant, s.Title))
	}
}

func (h *Harness) scenarioCashout(ctx context.Context) error {
	c := h.cfg.Cashout
	if c == nil {
		return skip("no 'cashout' block in config")
	}
	items, err := h.client.Masterdata.Cashouts(ctx, c.ServiceID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no cashout items for serviceId=%d", c.ServiceID)
	}
	item := items[0]
	var amt float64
	if item.AmountLocalCur() != nil {
		amt = *item.AmountLocalCur()
	}
	detail(fmt.Sprintf("picked: %s (%s, %s, local=%g %s)", item.PayItemID(), item.Name(), item.AmountType(), amt, item.LocalCur()))
	return quoteAndReport(ctx, h.client, &item, c.Amount)
}

func (h *Harness) scenarioBill(ctx context.Context) error {
	c := h.cfg.Bill
	if c == nil {
		return skip("no 'bill' block in config")
	}
	bills, err := h.client.Initiate.Bills(ctx, c.Merchant, c.ServiceID, c.ServiceNumber)
	if err != nil {
		return err
	}
	if len(bills) == 0 {
		return fmt.Errorf("no bills for %s/%d/%s", c.Merchant, c.ServiceID, c.ServiceNumber)
	}
	bill := bills[0]
	var amt float64
	if bill.AmountLocalCur() != nil {
		amt = *bill.AmountLocalCur()
	}
	detail(fmt.Sprintf("picked: %s (%s, amount=%g %s, due=%s)",
		bill.PayItemID(), bill.BillType, amt, bill.LocalCur(), bill.BillDueDate.Format("2006-01-02")))
	return quoteAndReport(ctx, h.client, &bill, int(amt))
}

func (h *Harness) scenarioTopup(ctx context.Context) error {
	c := h.cfg.Topup
	if c == nil {
		return skip("no 'topup' block in config")
	}
	items, err := h.client.Masterdata.Topups(ctx, c.ServiceID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no topup items for serviceId=%d", c.ServiceID)
	}
	item := items[0]
	var amt float64
	if item.AmountLocalCur() != nil {
		amt = *item.AmountLocalCur()
	}
	detail(fmt.Sprintf("picked: %s (%s, %s, local=%g %s)", item.PayItemID(), item.Name(), item.AmountType(), amt, item.LocalCur()))
	return quoteAndReport(ctx, h.client, &item, c.Amount)
}

func (h *Harness) scenarioVoucher(ctx context.Context) error {
	c := h.cfg.Voucher
	if c == nil {
		return skip("no 'voucher' block in config")
	}
	items, err := h.client.Masterdata.Vouchers(ctx, c.ServiceID)
	if err != nil {
		var pe *smob.APIError
		if errors.As(err, &pe) && pe.RespCode == 41004 {
			return skip("/v2/voucher rejects serviceId=%d (respCode 41004)", c.ServiceID)
		}
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no vouchers for serviceId=%d", c.ServiceID)
	}
	item := items[0]
	amount, err := resolveAmount(&item, c.Amount)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("picked: %s (%s, %s)", item.PayItemID(), item.Name(), item.AmountType()))
	return quoteAndReport(ctx, h.client, &item, amount)
}

func (h *Harness) scenarioProduct(ctx context.Context) error {
	c := h.cfg.Product
	if c == nil {
		return skip("no 'product' block in config")
	}
	items, err := h.client.Masterdata.Products(ctx, c.ServiceID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no products for serviceId=%d", c.ServiceID)
	}
	item := items[0]
	amount, err := resolveAmount(&item, c.Amount)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("picked: %s (%s, %s)", item.PayItemID(), item.Name(), item.AmountType()))
	return quoteAndReport(ctx, h.client, &item, amount)
}

func (h *Harness) scenarioSubscription(ctx context.Context) error {
	c := h.cfg.Subscription
	if c == nil {
		return skip("no 'subscription' block in config")
	}
	if c.ServiceNumber == "" && c.CustomerNumber == "" {
		return skip("subscription block needs serviceNumber or customerNumber")
	}
	subs, err := h.client.Initiate.Subscriptions(ctx, c.Merchant, c.ServiceID, c.ServiceNumber, c.CustomerNumber)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return fmt.Errorf("no subscriptions for %s/%d (serviceNumber=%s, customerNumber=%s)",
			c.Merchant, c.ServiceID, c.ServiceNumber, c.CustomerNumber)
	}
	sub := subs[0]
	amount, err := resolveAmount(&sub, c.Amount)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("picked: %s (%s, customer=%s, due=%s)",
		sub.PayItemID(), sub.Name(), sub.CustomerName, sub.DueDate.Format("2006-01-02")))
	return quoteAndReport(ctx, h.client, &sub, amount)
}

func (h *Harness) scenarioCashin(ctx context.Context) error {
	c := h.cfg.Cashin
	if c == nil {
		return skip("no 'cashin' block in config")
	}
	items, err := h.client.Masterdata.Cashins(ctx, c.ServiceID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no cashin items for serviceId=%d", c.ServiceID)
	}
	item := items[0]
	var amt float64
	if item.AmountLocalCur() != nil {
		amt = *item.AmountLocalCur()
	}
	detail(fmt.Sprintf("picked: %s (%s, %s, local=%g %s)", item.PayItemID(), item.Name(), item.AmountType(), amt, item.LocalCur()))
	return quoteAndReport(ctx, h.client, &item, c.Amount)
}

func (h *Harness) scenarioVerifyServiceNumber(ctx context.Context) error {
	c := h.cfg.Verify
	if c == nil {
		return skip("no 'verify' block in config")
	}
	ok, err := h.client.AccountValidation.VerifyServiceNumber(ctx, c.Merchant, c.ServiceID, c.ServiceNumber)
	if err != nil {
		var pe *smob.APIError
		if errors.As(err, &pe) && pe.RespCode == 40408 {
			return skip("service %s/%d does not support pre-payment verification (respCode 40408)",
				c.Merchant, c.ServiceID)
		}
		return err
	}
	detail(fmt.Sprintf("%s for %s/%d -> %v", c.ServiceNumber, c.Merchant, c.ServiceID, ok))
	return nil
}

func (h *Harness) scenarioValidateAccount(ctx context.Context) error {
	c := h.cfg.Validate
	if c == nil {
		return skip("no 'validate' block in config")
	}
	ca, err := h.client.AccountValidation.ValidateAccount(ctx, c.Destination, c.ServiceID)
	if err != nil {
		var pe *smob.APIError
		if errors.As(err, &pe) && pe.HTTPStatus == 401 {
			return skip("GET /v2/validate is restricted and not enabled for this partner (HTTP 401)")
		}
		return err
	}
	detail("destination: " + ca.Destination)
	detail(fmt.Sprintf("status:      %s", ca.Status))
	detail("name:        " + ca.Name)
	return nil
}

func (h *Harness) scenarioHistoryLast7Days(ctx context.Context) error {
	today := time.Now().UTC()
	from := today.AddDate(0, 0, -7)
	rows, err := h.client.Verify.HistoryByDateRange(ctx, from, today)
	if err != nil {
		return err
	}
	detail("range:        " + from.Format("2006-01-02") + " -> " + today.Format("2006-01-02"))
	detail(fmt.Sprintf("transactions: %d", len(rows)))
	sample := 3
	if len(rows) < sample {
		sample = len(rows)
	}
	for i := 0; i < sample; i++ {
		r := rows[i]
		var price float64
		if r.PriceLocalCur != nil {
			price = *r.PriceLocalCur
		}
		detail(fmt.Sprintf("  - %s : %s, %g %s, trid=%s",
			r.PTN, r.Status, price, r.LocalCur, r.TRID))
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: builds cleanly. (No test target — exercised end-to-end.)

- [ ] **Step 3: Commit**

```bash
git add cmd/smoketest/scenarios.go
git commit -m "feat: smoketest scenarios (Java SmokeTest.java parity)"
```

---

## Task 19: cmd/smoketest/main.go — entry point + config resolution

**Files:**
- Create: `cmd/smoketest/main.go`

- [ ] **Step 1: Implement `cmd/smoketest/main.go`**

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	smob "github.com/maviance/smobilpay-go"
)

const banner = "======================================================================"

func main() {
	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err)
		os.Exit(2)
	}

	clientCfg, err := buildClientConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err)
		os.Exit(2)
	}
	client, err := smob.New(clientCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err)
		os.Exit(2)
	}

	fmt.Println()
	fmt.Println(banner)
	fmt.Printf("Smobilpay smoke test  —  baseUrl=%s, apiVersion=%s, publicKey=%s\n",
		clientCfg.BaseURL, clientCfg.APIVersion, redactKey(clientCfg.PublicKey))
	fmt.Println(banner)

	ctx := context.Background()
	h := &Harness{client: client, cfg: cfg}

	h.run("Ping (auth probe)", func() error { return h.scenarioPing(ctx) })
	h.run("OAuth 2.0 token refresh", func() error { return h.scenarioTokenRefresh(ctx) })
	h.run("Account profile", func() error { return h.scenarioAccount(ctx) })
	h.run("Merchant catalog", func() error { return h.scenarioMerchants(ctx) })
	h.run("Service catalog", func() error { return h.scenarioServices(ctx) })
	h.run("Collection — cash-out (discover + quote)", func() error { return h.scenarioCashout(ctx) })
	h.run("Collection — bill payment (discover + quote)", func() error { return h.scenarioBill(ctx) })
	h.run("Collection — airtime top-up (discover + quote)", func() error { return h.scenarioTopup(ctx) })
	h.run("Collection — voucher purchase (discover + quote)", func() error { return h.scenarioVoucher(ctx) })
	h.run("Collection — product purchase (discover + quote)", func() error { return h.scenarioProduct(ctx) })
	h.run("Collection — subscription top-up (discover + quote)", func() error { return h.scenarioSubscription(ctx) })
	h.run("Disbursement — cash-in (discover + quote)", func() error { return h.scenarioCashin(ctx) })
	h.run("Account validation — verify serviceNumber", func() error { return h.scenarioVerifyServiceNumber(ctx) })
	h.run("Account validation — validate destination", func() error { return h.scenarioValidateAccount(ctx) })
	h.run("History - last 7 days", func() error { return h.scenarioHistoryLast7Days(ctx) })

	fmt.Println(sep)
	fmt.Printf("Summary: %d passed, %d skipped, %d failed\n", h.passed, h.skipped, h.failed)
	fmt.Println(sep)

	if h.failed > 0 {
		os.Exit(1)
	}
}

func loadConfig(args []string) (SmokeConfig, error) {
	path := resolveConfigPath(args)
	if _, err := os.Stat(path); err != nil {
		return SmokeConfig{}, fmt.Errorf(
			"config file not found at %s. Pass a path as the first argument, "+
				"set SMOBILPAY_SMOKE_CONFIG, or create ./smoke-test.json "+
				"(see smoke-test.example.json)",
			path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SmokeConfig{}, fmt.Errorf("could not read %s: %w", path, err)
	}
	var cfg SmokeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return SmokeConfig{}, fmt.Errorf("could not parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return SmokeConfig{}, err
	}
	return cfg, nil
}

func resolveConfigPath(args []string) string {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return args[0]
	}
	if env := strings.TrimSpace(os.Getenv("SMOBILPAY_SMOKE_CONFIG")); env != "" {
		return env
	}
	abs, _ := filepath.Abs("smoke-test.json")
	return abs
}

func buildClientConfig(c SmokeConfig) (smob.Config, error) {
	opts := []smob.Option{
		smob.WithBaseURL(c.BaseURL),
		smob.WithCredentials(c.PublicKey, c.SecretKey),
	}
	if c.APIVersion != "" {
		opts = append(opts, smob.WithAPIVersion(c.APIVersion))
	}
	return smob.NewConfig(opts...)
}

func redactKey(k string) string {
	if len(k) <= 4 {
		return "****"
	}
	return k[:4] + "..." + k[len(k)-2:]
}
```

- [ ] **Step 2: Verify the smoke-test binary builds**

Run: `go build ./cmd/smoketest`
Expected: builds, no errors.

- [ ] **Step 3: Confirm the binary handles missing config gracefully**

```bash
# Should exit 2 with a clear error message
SMOBILPAY_SMOKE_CONFIG=/nonexistent ./smoketest; echo "exit=$?"
```

Expected: exit code 2; message mentions config file not found.

- [ ] **Step 4: Commit**

```bash
git add cmd/smoketest/main.go
git commit -m "feat: smoketest entry point with config resolution and exit codes"
```

---

## Task 20: examples/ — runnable per-flow snippets

**Files:**
- Create: `examples/cashout/main.go`
- Create: `examples/bill/main.go`
- Create: `examples/topup/main.go`
- Create: `examples/voucher/main.go`
- Create: `examples/product/main.go`
- Create: `examples/subscription/main.go`
- Create: `examples/cashin/main.go`
- Create: `examples/verify/main.go`
- Create: `examples/history/main.go`

Each file is a complete `package main` that compiles independently.
They illustrate the README flows so partners can copy-paste.

- [ ] **Step 1: Write `examples/cashout/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	smob "github.com/maviance/smobilpay-go"
)

func main() {
	cfg, err := smob.NewConfig(
		smob.WithBaseURL(os.Getenv("SMOBILPAY_BASE_URL")),
		smob.WithCredentials(os.Getenv("SMOBILPAY_PUBLIC_KEY"), os.Getenv("SMOBILPAY_SECRET_KEY")),
	)
	if err != nil {
		log.Fatal(err)
	}
	c, err := smob.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	cashouts, err := c.Masterdata.Cashouts(ctx, 20053)
	if err != nil {
		log.Fatal(err)
	}
	item := cashouts[0]

	quote, err := c.Initiate.Quote(ctx, smob.QuoteRequest{Amount: 500, PayItemID: item.PayItemID()})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := c.Confirm.Collect(ctx, smob.CollectionRequest{
		QuoteID:              quote.QuoteID,
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "customer@example.com",
		ServiceNumber:        "237699999999",
		TRID:                 "ORDER-EXAMPLE-001",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("PTN: %s, status: %s\n", resp.PTN, resp.Status)
}
```

- [ ] **Step 2: Write `examples/bill/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	smob "github.com/maviance/smobilpay-go"
)

func main() {
	cfg, _ := smob.NewConfig(
		smob.WithBaseURL(os.Getenv("SMOBILPAY_BASE_URL")),
		smob.WithCredentials(os.Getenv("SMOBILPAY_PUBLIC_KEY"), os.Getenv("SMOBILPAY_SECRET_KEY")),
	)
	c, _ := smob.New(cfg)
	ctx := context.Background()

	bills, err := c.Initiate.Bills(ctx, "ENEO", 10039, "203157530")
	if err != nil {
		log.Fatal(err)
	}
	bill := bills[0]
	amount := 0
	if bill.AmountLocalCur() != nil {
		amount = int(*bill.AmountLocalCur())
	}

	quote, err := c.Initiate.Quote(ctx, smob.QuoteRequest{Amount: amount, PayItemID: bill.PayItemID()})
	if err != nil {
		log.Fatal(err)
	}
	resp, err := c.Confirm.Collect(ctx, smob.CollectionRequest{
		QuoteID:              quote.QuoteID,
		CustomerPhoneNumber:  "237699999999",
		CustomerEmailAddress: "customer@example.com",
		ServiceNumber:        "203157530",
		CustomerName:         "Jane Doe",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("PTN: %s\n", resp.PTN)
}
```

- [ ] **Step 3: Write the remaining flow snippets**

For `topup`, `voucher`, `product`, `subscription`, `cashin`, `verify`, and `history`, write one file each following the same pattern as `cashout/main.go` and `bill/main.go`. Each is a self-contained `package main` that:

1. Constructs `cfg` and `client` from `SMOBILPAY_*` env vars.
2. Discovers a payment item via the appropriate masterdata or initiate call.
3. For payment flows: requests a quote, then calls `Confirm.Collect`.
4. For verify/history: just calls the relevant Verify or AccountValidation method and prints the result.

Use the equivalent Java README section as the reference shape.

- [ ] **Step 4: Confirm all examples build**

Run: `go build ./examples/...`
Expected: builds with no errors.

- [ ] **Step 5: Commit**

```bash
git add examples/
git commit -m "docs: per-flow runnable example snippets"
```

---

## Task 21: README.md — partner-facing documentation

**Files:**
- Modify: `README.md` (overwrite)

The README mirrors the Java client README section-for-section. It is
written for partner integrators landing on github.com/maviance/smobilpay-go.

- [ ] **Step 1: Overwrite `README.md`**

Replace the entire file with the structure below. Use the matching
section in `/root/s3p-clients/java/README.md` as the source for prose,
adapting code blocks to Go syntax shown in `examples/`.

Required top-level sections (in this exact order):

```markdown
# smobilpay-go

[1 paragraph: what it is, partner-facing]

## What this client does

[bulleted: collections, disbursements, account/service discovery,
 status verification, pre-payment validation]

## Requirements

- Go 1.22 or newer
- Network access to the base URL issued by Maviance support
- An OAuth 2.0 credential pair (publicKey / secretKey)

The runtime depends only on the Go standard library and golang.org/x/sync/singleflight.

## Installation

`go get github.com/maviance/smobilpay-go`

## Quick start

[Go snippet equivalent to the Java Quick start: NewConfig, New, Verify.Ping]

## Authentication

The Smobilpay API uses OAuth 2.0 `client_credentials` exclusively. Legacy
HMAC request signing is **not** supported.

[describe: lazy mint, cache to expiry-skew, manual refresh via client.Tokens.Refresh]

## Choosing the right flow

[Java-style flow table: discover → quote → confirm; one row per flow:
 collection, bill, topup, voucher, product, subscription, disbursement]

## Conventions

- All requests and responses are JSON.
- Monetary amounts on QuoteRequest.Amount are integers in local currency.
- Currencies are ISO 4217; countries ISO 3166-1 alpha-3.
- Phone numbers are E.164 without leading "+".
- Errors are surfaced as `*APIError` / `*AuthError` / `*TransportError`; use `errors.As`.
- The `x-api-version: 3.0.0` header is sent on every secured request.

## Collection — cash-out

[Go snippet]

## Collection — bill payment

## Collection — airtime top-up

## Collection — voucher purchase

## Collection — product purchase

## Collection — subscription top-up

## Disbursement — cash-in

## Pre-payment verification

## Catalog discovery

## Account and ping utilities

## Historical lookups

## Error handling

[Go snippet using errors.As(err, &apiErr); cover the 498 quote-expired hint]

## Configuration reference

| Option                | Default | Notes |
|-----------------------|---------|-------|
| WithBaseURL           | required | Issued during onboarding |
| WithCredentials       | required | publicKey / secretKey pair |
| WithAPIVersion        | "3.0.0"  | x-api-version header |
| WithRequestTimeout    | 30s      | Per-request timeout |
| WithTokenRefreshSkew  | 30s      | Mint a fresh token this far ahead of expiry |
| WithHTTPClient        | nil      | Inject a custom *http.Client |

## Onboarding

Base URL, partner credentials, callback URL registration, and the full
error catalog are issued by Maviance support — support@smobilpay.com.

## Development

```bash
make build    # go build ./...
make test     # go test -race ./...
make cover    # 80% coverage gate
make lint     # gofmt + go vet
```

### Smoke test against acceptance

```bash
cp smoke-test.example.json smoke-test.json
# edit smoke-test.json with baseUrl + credentials
make smoketest
# diff against Java client output:
make smoketest-compare JAVA_DIR=../java
```

## License

MIT © Maviance PLC. See [LICENSE](LICENSE).
```

- [ ] **Step 2: Render and proofread**

```bash
# Smoke-check the file builds nothing — just confirm it's valid markdown by counting H2 sections
grep -c '^## ' README.md
```

Expected: 17–20 H2 sections.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: partner-facing README at Java-parity depth"
```

---

## Task 22: Final integration verification

This task runs the full quality gate end-to-end. No code is written —
it asserts the build is shippable.

- [ ] **Step 1: Confirm build is clean**

Run: `make build`
Expected: exits 0, no output beyond standard Go build chatter.

- [ ] **Step 2: Confirm full test suite passes with race detector**

Run: `make test`
Expected: every package PASS with `-race`; no failures, no data races.

- [ ] **Step 3: Confirm coverage gate**

Run: `make cover`
Expected: prints `coverage XX.X% (gate 80.0%) OK`; exit code 0.

If coverage is below 80%, add table-driven test cases for the
uncovered branches in the file with the lowest coverage. Re-run.

- [ ] **Step 4: Confirm lint passes**

Run: `make lint`
Expected: no output; exit code 0.

- [ ] **Step 5: Confirm the smoketest binary builds and handles missing config**

```bash
go build -o /tmp/smoketest ./cmd/smoketest
SMOBILPAY_SMOKE_CONFIG=/nonexistent /tmp/smoketest; echo "exit=$?"
```

Expected: exit code 2; error message mentions config file not found.

- [ ] **Step 6: (Optional, requires acceptance creds) Run live smoke test**

If acceptance creds are available locally:

```bash
cp /root/s3p-clients/java/smoke-test.json ./smoke-test.json
make smoketest
```

Expected: `Summary: N passed, M skipped, 0 failed` with exit 0.

- [ ] **Step 7: (Optional, requires Java tooling) Run side-by-side comparison**

```bash
make smoketest-compare JAVA_DIR=/root/s3p-clients/java
```

Expected: `Go and Java smoke-test outputs match (modulo redacted volatile fields).` and exit 0.
Any non-empty diff means a parity break — investigate and fix the
Go client (not the normalizer) unless the Java side is the one
emitting volatile data we missed.

- [ ] **Step 8: Tag the release**

```bash
git tag -a v0.1.0 -m "Initial release: full Smobilpay partner API v3.2.0 client (OAuth2-only)"
```

(Do not push the tag without explicit instruction from the user.)

- [ ] **Step 9: Done — celebrate. Report status to user.**

Summarise:
- Number of source files
- Number of test files
- Final coverage percentage
- Whether the smoketest-compare diff was clean




