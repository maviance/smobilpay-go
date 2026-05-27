# Go Client Rebuild — Design Spec

**Status:** Draft  
**Date:** 2026-05-27  
**Owner:** Michael Nowag  
**Module:** `github.com/maviance/smobilpay-go`

---

## 1. Purpose

Rebuild `github.com/maviance/smobilpay-go` from a single 34-line HMAC-SHA1
signature helper into a **full Smobilpay partner API v3.2.0 client**,
disciplined-port-style, with parity to the existing Java client
(`/root/s3p-clients/java`) and the partner OpenAPI spec
(`/root/php-smobilpay-s3p-api/apidocs/s3p_3.2.0_openapi_specs_partner.yml`).

The current repo contents (the `s3p/` package and its example) are
**deleted entirely**. There is no consumer to migrate.

## 2. Goals & non-goals

### Goals

- Cover every partner-spec endpoint a partner integrator needs to move
  money in and out, sell value-added services, and drive a payment UI
  from the static catalog. Surface mirrors the Java client 1:1.
- **OAuth 2.0 `client_credentials` only.** HMAC is removed end to end.
- Idiomatic Go: functional options, `context.Context` on every method,
  typed errors with `errors.As`, table-driven tests, `httptest` for
  transport stubs, `net/http` + `encoding/json` only in the public
  module. The only third-party dep on the runtime side is
  `golang.org/x/sync/singleflight`.
- 80% test coverage gate enforced in `make cover` / CI.
- Smoke-test harness in `cmd/smoketest` that consumes the same
  `smoke-test.json` schema as the Java client, so values from the Java
  config can be copied verbatim, and a `make smoketest-compare` target
  that diffs normalized outputs between the two runners.
- Partner-facing README at parity with the Java README — install,
  quickstart, every flow, error handling, configuration reference,
  onboarding, development.

### Non-goals

- Auto-generated client from the OpenAPI spec (the user explicitly
  distrusts the prior auto-generated code).
- Backwards compatibility with the deleted `s3p.GenerateSignature`
  HMAC helper.
- Async / streaming API surface. Synchronous, matching Java.
- Built-in retry/backoff. Quote expiry (HTTP 498) requires re-quote,
  not blind retry — we surface the error and let callers decide.
- A separate `types/` or `model/` subpackage. Public DTOs are
  co-located with the API that returns them, mirroring modern Go SDKs
  (anthropic-sdk-go, openai-go, stripe-go).

## 3. Decisions

The following decisions are locked in before implementation; revisit
them only if a hard blocker surfaces during build-out.

| Decision | Choice | Notes |
|---|---|---|
| Module path | `github.com/maviance/smobilpay-go` | Unchanged from current `go.mod`. |
| Go minimum version | `1.22` | Stable, gives access to modern stdlib idioms. |
| Package layout | Single root package + `internal/apiclient/` | No public sub-packages; types live next to their operations. |
| Auth mode | OAuth 2.0 `client_credentials` only | HMAC support removed. |
| Token cache | `golang.org/x/sync/singleflight` | Dedupes concurrent mint attempts. |
| HTTP client | `net/http` stdlib | Injectable for proxy / custom TLS. |
| JSON | `encoding/json` stdlib | Custom `UnmarshalJSON` for lenient `LocalDate`. |
| Error model | Typed errors (`*APIError`, `*AuthError`, `*TransportError`) via `errors.As` | No sentinel-kind switch. |
| Tests | stdlib `testing` + `httptest`; table-driven; race-enabled | No testify. |
| Coverage gate | 80% package-level via `make cover` | DTO-heavy files informational only. |
| Smoke test packaging | `cmd/smoketest/` + Makefile targets | Same JSON schema as Java. |
| Build automation | `Makefile` | Targets: `build`, `test`, `cover`, `lint`, `smoketest`, `smoketest-compare`, `tidy`. |
| License | MIT — kept | `LICENSE` file unchanged. |
| Documentation depth | Java README parity + godoc on every export | `doc.go` carries package overview. |

## 4. Public API shape

### 4.1 Construction (functional options)

```go
import "github.com/maviance/smobilpay-go"

cfg, err := smobilpay.NewConfig(
    smobilpay.WithBaseURL("https://api.example.invalid"),
    smobilpay.WithCredentials(pub, sec),
    smobilpay.WithAPIVersion("3.0.0"),              // default
    smobilpay.WithRequestTimeout(30 * time.Second), // default
    smobilpay.WithTokenRefreshSkew(30 * time.Second), // default
    smobilpay.WithHTTPClient(customHTTPClient),     // optional
)
if err != nil { /* invalid configuration */ }

client, err := smobilpay.New(cfg)
```

### 4.2 API groups as fields

Method names match the Java client one-for-one (PascalCase per Go).
Every method takes `ctx context.Context` as the first argument.

```go
// Verify
ping, err          := client.Verify.Ping(ctx)
account, err       := client.Verify.Account(ctx)
statuses, err      := client.Verify.VerifyTransaction(ctx, ptn, trid string)
rows, err          := client.Verify.HistoryByPtn(ctx, ptn string)
rows, err          := client.Verify.HistoryByTrid(ctx, trid string)
rows, err          := client.Verify.HistoryByDateRange(ctx, from, to time.Time)

// Masterdata
merchants, err     := client.Masterdata.Merchants(ctx)
services, err      := client.Masterdata.Services(ctx)
products, err      := client.Masterdata.Products(ctx, serviceID int64)
vouchers, err      := client.Masterdata.Vouchers(ctx, serviceID int64)
topups, err        := client.Masterdata.Topups(ctx, serviceID int64)
cashins, err       := client.Masterdata.Cashins(ctx, serviceID int64)
cashouts, err      := client.Masterdata.Cashouts(ctx, serviceID int64)

// Initiate
bills, err         := client.Initiate.Bills(ctx, merchant string, serviceID int64, serviceNumber string)
subs, err          := client.Initiate.Subscriptions(ctx, merchant string, serviceID int64, serviceNumber, customerNumber string)
quote, err         := client.Initiate.Quote(ctx, smobilpay.QuoteRequest{...})

// Confirm
resp, err          := client.Confirm.Collect(ctx, smobilpay.CollectionRequest{...})

// Account Validation
ok, err            := client.AccountValidation.VerifyServiceNumber(ctx, merchant string, serviceID int64, serviceNumber string)
ca, err            := client.AccountValidation.ValidateAccount(ctx, destination string, serviceID int64)

// Token diagnostics
tok, err           := client.Tokens.Refresh(ctx)  // force fresh mint
cached, ok         := client.Tokens.Cached()      // current cached token
```

### 4.3 Errors

```go
type APIError struct {
    HTTPStatus int
    RespCode   int     // canonical machine identifier (e.g. 41004, 40408)
    DevMsg     string
    UsrMsg     string
    Link       string
    RawBody    string
}

type AuthError struct {
    HTTPStatus int
    OAuthError string  // e.g. "invalid_client"
    Message    string
    Cause      error
}

type TransportError struct {
    Op    string // e.g. "GET /v2/ping"
    Cause error
}
```

Each implements `error`; `AuthError` and `TransportError` implement
`Unwrap()`. Consumers introspect via `errors.As`:

```go
var apiErr *smobilpay.APIError
if errors.As(err, &apiErr) {
    if apiErr.HTTPStatus == 498 {
        // quote expired — re-quote
    }
    if apiErr.RespCode == 41004 {
        // catalog inconsistency for voucher endpoint
    }
}
```

## 5. Module layout

```
github.com/maviance/smobilpay-go/
├── go.mod                          # module github.com/maviance/smobilpay-go, go 1.22
├── go.sum
├── LICENSE                         # MIT (kept)
├── README.md                       # partner-facing, Java-parity depth
├── Makefile                        # build, test, cover, lint, smoketest, smoketest-compare, tidy
│
├── doc.go                          # package-level godoc overview (pkg.go.dev landing)
├── client.go                       # Client struct, New(), Tokens accessor
├── option.go                       # Config + functional options (WithBaseURL, ...)
├── error.go                        # APIError, AuthError, TransportError
│
│   # One file per resource: API group methods AND its DTOs co-located.
├── masterdata.go                   # MasterdataAPI + Merchant, Service, Cashout, Cashin, Topup, Product
├── initiate.go                     # InitiateAPI + Bill, Subscription, QuoteRequest, QuoteResponse
├── confirm.go                      # ConfirmAPI + CollectionRequest, CollectionResponse
├── verify.go                       # VerifyAPI + Ping, Account, PaymentStatus
├── accountvalidation.go            # AccountValidationAPI + CustomerAccount
├── types.go                        # cross-cutting: Date (lenient), enums, PaymentItem interface
├── *_test.go                       # mirrors source files, table-driven, race-enabled
│
├── internal/
│   └── apiclient/                  # transport + oauth2 + query, one package
│       ├── transport.go
│       ├── oauth2.go
│       ├── query.go
│       └── *_test.go
│
├── cmd/
│   └── smoketest/
│       ├── main.go                 # config resolution, scenario harness, exit codes 0/1/2
│       ├── config.go               # SmokeConfig schema (matches Java's smoke-test.json)
│       └── scenarios.go            # one func per scenario, mirrors Java's SmokeTest.java
│
├── smoke-test.example.json         # identical schema to Java's; the Java file is drop-in compatible
├── examples/
│   └── *.go                        # short per-flow snippets matching the README
└── docs/
    └── superpowers/specs/
        └── 2026-05-27-go-client-rebuild-design.md   # this document
```

### 5.1 Why this layout

| Choice | Rationale |
|---|---|
| Single root package | User-chosen. Easy import surface; no `import "github.com/.../model"`. |
| DTOs co-located with operations | Modern Go SDK convention (anthropic-sdk-go, openai-go, stripe-go). No `models.go` monolith. |
| Cross-cutting `types.go` | Date helper and enums used across multiple resource files live in one place. |
| Single `internal/apiclient/` | Transport, OAuth, query-builder are one concern. Two subpackages for ~6 files is over-organized. |
| `cmd/smoketest/` | Canonical Go pattern for executables; signals intent better than `examples/`. |
| `examples/` | Short, runnable snippets that match the README sections. |
| `accountvalidation.go` | No underscores in filenames (Go convention). |

## 6. Authentication design

### 6.1 OAuth 2.0 token manager (`internal/apiclient/oauth2.go`)

- POSTs `Basic base64(publicKey:secretKey)` + body `grant_type=client_credentials`
  to `{baseURL}/oauth/token` with headers
  `Content-Type: application/x-www-form-urlencoded`, `Accept: application/json`.
- Parses `{ "access_token": "...", "token_type": "Bearer", "expires_in": N }`.
- Caches the token in memory; reused until `now + skew >= expiresAt`.
- Concurrent mint attempts are deduped via
  `singleflight.Group.Do("token", ...)` — only one in-flight mint at a time.
- `Refresh(ctx)` bypasses the cache to force a fresh mint.
- Clock is an injected `func() time.Time` for deterministic tests.
- If `token_type` is absent or null in the response, default to
  `"Bearer"` (matches Java's `OAuth2TokenManager.parseTokenResponse`).

### 6.2 Token type

```go
type Token struct {
    AccessToken string
    TokenType   string    // always "Bearer" in practice
    ExpiresAt   time.Time // absolute; issuedAt + expires_in seconds
}

func (t Token) Expired(now time.Time, skew time.Duration) bool
```

### 6.3 Failure modes

| Failure | Surface |
|---|---|
| Non-2xx from `/oauth/token` | `*AuthError{HTTPStatus, OAuthError, Message}` |
| Missing `access_token` field | `*AuthError{HTTPStatus: 200, Message: "missing access_token"}` |
| Missing `expires_in` field | `*AuthError{HTTPStatus: 200, Message: "missing expires_in"}` |
| Network / timeout | `*TransportError{Op: "POST /oauth/token", Cause}` |
| Body not JSON | `*AuthError{Message: "token response not valid JSON", Cause}` |

## 7. HTTP transport (`internal/apiclient/transport.go`)

- Wraps a `*http.Client` injected via `WithHTTPClient` (defaults to
  `&http.Client{Timeout: cfg.RequestTimeout}`).
- Resolves request URI as `baseURL + path + "?" + query.Encode()`.
  Trailing `/` on `baseURL` is normalized.
- On every authenticated call:
  1. Asks token manager for a bearer (auto-mints if expired).
  2. Attaches headers: `Authorization: Bearer <jwt>`, `x-api-version: 3.0.0` (or config value), `Accept: application/json`.
  3. For `POST` bodies: serializes via `encoding/json`, sets
     `Content-Type: application/json`.
  4. Sends with `ctx`.
- On non-2xx:
  - Tries to decode the body as the `ApiError` envelope
    (`{respCode, devMsg, usrMsg, link}`); on success returns
    `*APIError{HTTPStatus, ...envelope, RawBody}`.
  - On decode failure returns `*APIError{HTTPStatus, RawBody}` with
    `RespCode == 0`.
- On 2xx: decodes body into the requested type, returning
  `*TransportError{Op, Cause}` on JSON decode failure.

### 7.1 Query parameter builder (`internal/apiclient/query.go`)

- Builder with `Add(key string, value any)` semantics:
  - `nil`, empty string, and zero `int64(0)` for optional params **are
    skipped** (matches Java's `QueryParams.add` behavior).
  - `time.Time` formatted as ISO-8601 with `Z` suffix.
- Values URL-encoded; keys preserved in insertion order for stable
  request URIs (matters for diffing wire traffic during the
  comparison run).

## 8. Models

### 8.1 Style

- Public structs with `json:"<field>"` tags.
- Optional / nullable fields are pointer types (`*float64`,
  `*time.Time`) so we can distinguish "not present" from "zero" —
  Java does this with boxed `Float`/`Long`.
- Enums are named string types with exported constants:

```go
type ServiceType string

const (
    ServiceTypeCashout      ServiceType = "CASHOUT"
    ServiceTypeBill         ServiceType = "BILL"
    ServiceTypeTopup        ServiceType = "TOPUP"
    ServiceTypeVoucher      ServiceType = "VOUCHER"
    ServiceTypeProduct      ServiceType = "PRODUCT"
    ServiceTypeSubscription ServiceType = "SUBSCRIPTION"
    ServiceTypeCashin       ServiceType = "CASHIN"
)
```

### 8.2 Lenient `Date`

Server returns dates in multiple shapes (`YYYY-MM-DD`,
`YYYY-MM-DDTHH:MM:SSZ`, sometimes with a timezone offset). A custom
type lives in `types.go`:

```go
type Date struct{ time.Time }

func (d *Date) UnmarshalJSON(b []byte) error { ... } // tries multiple layouts
func (d Date) MarshalJSON() ([]byte, error)  { ... } // canonical YYYY-MM-DD
```

Mirrors the Java `LenientLocalDateDeserializer`.

### 8.3 `PaymentItem` interface

All catalog items (Cashout, Cashin, Topup, Product, Bill, Subscription)
share a minimum surface — a `payItemId`, an `amountType`, an
`amountLocalCur`, a `localCur`, a `name`. The smoke-test harness needs
to treat any of them uniformly when calling
`Initiate.Quote(ctx, QuoteRequest{Amount, PayItemID})`. We expose:

```go
type PaymentItem interface {
    PayItemID() string
    AmountType() AmountType
    AmountLocalCur() *float64
    LocalCur() string
    Name() string
}
```

Each concrete type (`Cashout`, `Bill`, ...) implements it via small
methods. Matches Java's `PaymentItem` interface.

### 8.4 DTO inventory

By file:

| File | Operation types | DTOs |
|---|---|---|
| `verify.go` | `Ping`, `Account`, `VerifyTransaction`, `HistoryByPtn`, `HistoryByTrid`, `HistoryByDateRange` | `Ping`, `Account`, `PaymentStatus`, `PaymentStatusType`, `Commission` |
| `masterdata.go` | `Merchants`, `Services`, `Products`, `Vouchers`, `Topups`, `Cashins`, `Cashouts` | `Merchant`, `MerchantStatus`, `Service`, `ServiceStatus`, `ServiceType`, `Cashout`, `Cashin`, `Topup`, `Product`, `AmountType`, `I18nText` |
| `initiate.go` | `Bills`, `Subscriptions`, `Quote` | `Bill`, `BillType`, `Subscription`, `QuoteRequest`, `QuoteResponse` |
| `confirm.go` | `Collect` | `CollectionRequest`, `CollectionResponse` |
| `accountvalidation.go` | `VerifyServiceNumber`, `ValidateAccount` | `CustomerAccount`, `CustomerAccount.Status` |
| `types.go` | — | `Date`, `PaymentItem` interface, cross-cutting enums |
| `error.go` | — | `APIError`, `AuthError`, `TransportError` |

Total: ~30 DTOs + 7 enums. Matches the Java client.

## 9. Smoke test harness (`cmd/smoketest/`)

### 9.1 Behavior

Read-only / quote-only. **Never calls `/v2/collectstd`** — no money
moves. Mirrors the Java `SmokeTest.java` scenario-for-scenario.

Scenarios (in order):

1. Ping (auth probe) — proves OAuth mint + bearer + `x-api-version` work end to end.
2. OAuth 2.0 token refresh — force a fresh mint, re-ping.
3. Account profile — agent identity, balance, daily limit.
4. Merchant catalog — first 5 + count.
5. Service catalog — distribution by type, hints for hard-to-source serviceIds (voucher, subscription, verifiable).
6. Collection — cash-out (discover + quote).
7. Collection — bill payment (discover + quote).
8. Collection — airtime top-up (discover + quote).
9. Collection — voucher purchase (discover + quote).
10. Collection — product purchase (discover + quote).
11. Collection — subscription top-up (discover + quote).
12. Disbursement — cash-in (discover + quote).
13. Account validation — verify serviceNumber.
14. Account validation — validate destination.
15. History — last 7 days.

Each scenario prints `RUN <name>`, indented `     <detail>` lines, and
a terminating `PASS <name>`, `SKIP <name> - <reason>`, or
`FAIL <name> - <error class>: <message>`. Final summary:
`Summary: P passed, S skipped, F failed`.

### 9.2 Configuration

`SmokeConfig` matches the Java `SmokeTestConfig` JSON schema:

```json
{
  "baseUrl": "...",
  "publicKey": "...",
  "secretKey": "...",
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

- The Java `smoke-test.json` (already populated with acceptance creds)
  works **drop-in** for the Go runner.
- Path resolution: first CLI arg → `SMOBILPAY_SMOKE_CONFIG` env →
  `./smoke-test.json`. Same precedence as Java.
- Any per-flow block set to `null` (or absent) skips that scenario.
- `apiVersion` defaults to `3.0.0`.
- Missing required fields (`baseUrl`, `publicKey`, `secretKey`) → exit 2.
- Any scenario failure → exit 1.
- All non-skipped scenarios passing → exit 0.

### 9.3 Known-skip cases

Mirrored from Java:

| Endpoint | Condition | Skip reason |
|---|---|---|
| `/v2/voucher` | `APIError.RespCode == 41004` | catalog inconsistency for the voucher service. |
| `/v2/verify` | `APIError.RespCode == 40408` | service does not support pre-payment verification. |
| `/v2/validate` | `APIError.HTTPStatus == 401` | restricted endpoint not enabled for this partner. |

### 9.4 Diff-friendly output

Volatile fields (timestamps, nonces, PTNs, quote IDs, TRIDs, bearer
prefixes, "X transactions" counts, "X items" counts) are emitted in
a normalized form during the comparison run. A `tools/normalize.sh`
script applied to both Go and Java outputs strips these to
`<REDACTED>` so the diff focuses on:

- scenario names
- PASS / SKIP / FAIL classification
- known stable identifiers (service IDs, merchant codes, payItemIds, respCodes, amount types, currency codes)
- structural shape of the per-scenario detail lines

If parity exists, the diff between the two normalized outputs is empty.

`tools/normalize.sh` is a thin `sed` pipeline. Specifically, it
substitutes the following patterns to `<REDACTED>` (case-sensitive,
line-anchored where shown):

| Field shown by harness | Pattern (regex) |
|---|---|
| `server time:    <ISO-8601>` | `^(\s+server time:\s+).*$` |
| `nonce echo:     <value>` | `^(\s+nonce echo:\s+).*$` |
| `quoteId:        <opaque>` | `^(\s+quoteId:\s+).*$` |
| `expiresAt:      <ISO-8601>` | `^(\s+expiresAt:\s+).*$` |
| `first  bearer prefix: <12 chars>...` | `^(\s+(first|forced)\s+bearer prefix:\s+).*$` |
| `transactions: <N>` | `^(\s+transactions:\s+)\d+$` |
| `services: <N>` and `merchants: <N>` | `^(\s+(services|merchants):\s+)\d+$` |
| Per-type distribution counts `- CASHIN: N` | `^(\s+-\s+[A-Z]+:\s+)\d+$` |
| `range:        <date> -> <date>` | `^(\s+range:\s+).*$` |
| "...and N more" tails | `^(\s+\.\.\.and\s+)\d+(\s+more)$` |
| `PTN-<digits>` anywhere in detail | `PTN-\d+` |

Stable identifiers (`payItemId` strings, merchant codes, service IDs,
currency codes, respCodes) are explicitly **not** redacted — they're
what makes the diff meaningful.

### 9.5 `make smoketest-compare`

```
make smoketest-compare JAVA_DIR=/root/s3p-clients/java
```

Steps:

1. Verify `JAVA_DIR/smoke-test.json` exists.
2. Run Go harness with the same config → `build/smoketest.go.txt`.
3. Run `(cd $JAVA_DIR && ./gradlew runSmokeTest --console=plain --args="$PWD/smoke-test.json")` → `build/smoketest.java.txt`.
4. Normalize both outputs through the redaction filter →
   `build/smoketest.go.norm.txt`, `build/smoketest.java.norm.txt`.
5. `diff -u build/smoketest.java.norm.txt build/smoketest.go.norm.txt`.
6. Exit 0 if diff is empty, 1 otherwise.

The Java path defaults to `../java` relative to the Go repo if
`JAVA_DIR` is not set.

## 10. Testing strategy

### 10.1 Unit + transport tests

- One `_test.go` file per source file.
- Table-driven (`tests := []struct{ name string; ... }{...}` + `t.Run`).
- `httptest.NewServer` for the HTTP transport — assert request method,
  path, query, headers, body; respond with canned JSON.
- OAuth manager tested with an injected clock and a stubbed server
  that emits `expires_in` of varying lengths to exercise the refresh
  window.
- Models tested by JSON round-trip: marshal → unmarshal → compare.
- Lenient `Date` tested against every wire format we've seen.

### 10.2 Coverage

- `go test -race -coverprofile=cover.out ./...` in `make cover`.
- 80% package-level threshold enforced for the root package and
  `internal/apiclient`. `cmd/smoketest` is excluded (it's an
  executable, not library code).
- The `total` line is parsed by a small shell snippet in `make cover`
  that fails the build if total < 80.0.

### 10.3 Race detection

Mandatory on every test invocation:

```bash
go test -race ./...
```

### 10.4 Linting

`make lint`:

- `gofmt -l .` — fail if any file is not formatted.
- `go vet ./...`.
- (Optional) `staticcheck ./...` if installed.

## 11. Documentation

### 11.1 README

Mirrors the Java README structure exactly:

1. What this client does
2. Requirements (Go 1.22+)
3. Installation (`go get github.com/maviance/smobilpay-go`)
4. Quick start
5. Authentication
6. Choosing the right flow (the table from the Java README, ported)
7. Conventions
8. Collection — cash-out
9. Collection — bill payment
10. Collection — airtime top-up
11. Collection — voucher purchase
12. Collection — product purchase
13. Collection — subscription top-up
14. Disbursement — cash-in
15. Pre-payment verification
16. Catalog discovery
17. Account and ping utilities
18. Historical lookups
19. Error handling
20. Configuration reference
21. Onboarding
22. Development
23. License

### 11.2 godoc

Every exported symbol carries a godoc comment in the Java client's
voice — partner-facing, precise, and conservative about implementation
details. `doc.go` carries the package-level overview that lands on
`pkg.go.dev`.

### 11.3 Examples directory

`examples/` holds runnable single-file programs that match the
README sections (one per flow). These are excluded from the module's
public API (they're `package main`) and are documented as "snippets
to copy and adapt."

## 12. Makefile

```makefile
GO          ?= go
GOFLAGS     ?=
COVERFILE   ?= cover.out
COVER_MIN   ?= 80.0
JAVA_DIR    ?= ../java
BUILD_DIR   ?= build

.PHONY: build test cover lint smoketest smoketest-compare tidy clean

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

cover:
	$(GO) test -race -coverprofile=$(COVERFILE) ./...
	@total=$$($(GO) tool cover -func=$(COVERFILE) | awk '/^total:/ { print $$3 }' | tr -d '%'); \
	 awk -v t=$$total -v m=$(COVER_MIN) 'BEGIN { if (t+0 < m+0) { printf "coverage %.1f%% below %.1f%% gate\n", t, m; exit 1 } else { printf "coverage %.1f%% (gate %.1f%%) OK\n", t, m } }'

lint:
	gofmt -l . | (! grep .)
	$(GO) vet ./...

smoketest:
	$(GO) run ./cmd/smoketest

smoketest-compare: $(BUILD_DIR)
	$(GO) run ./cmd/smoketest > $(BUILD_DIR)/smoketest.go.txt
	cd $(JAVA_DIR) && ./gradlew runSmokeTest --console=plain --args="$$PWD/smoke-test.json" > $(CURDIR)/$(BUILD_DIR)/smoketest.java.txt
	./tools/normalize.sh $(BUILD_DIR)/smoketest.go.txt   > $(BUILD_DIR)/smoketest.go.norm.txt
	./tools/normalize.sh $(BUILD_DIR)/smoketest.java.txt > $(BUILD_DIR)/smoketest.java.norm.txt
	diff -u $(BUILD_DIR)/smoketest.java.norm.txt $(BUILD_DIR)/smoketest.go.norm.txt

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BUILD_DIR) $(COVERFILE)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)
```

## 13. Dependencies

| Dep | Purpose | Scope |
|---|---|---|
| `golang.org/x/sync/singleflight` | Dedupe concurrent OAuth token mints | Runtime |
| (stdlib only otherwise) | HTTP, JSON, testing, httptest, errors, context, time, sync, encoding/base64, net/url | Runtime + test |

No testify, no testcontainers, no log libraries (callers bring their
own logger — we use no logging in the library itself).

## 14. Migration

There is no consumer-facing migration. In the first commit of the
rebuild:

- The current `s3p/` package (HMAC signature helper + its test) is
  deleted.
- The current `examples/main.go` is deleted; the new `examples/`
  directory is scaffolded fresh with per-flow snippets matching the
  new README sections.
- `go.mod` is rewritten (module path retained, Go version bumped to
  1.22).
- `README.md` is replaced wholesale.
- `LICENSE` is left intact.

Any downstream Go consumer of the old `s3p.GenerateSignature` HMAC
function will fail to compile after upgrade — that is intentional
and matches the "HMAC is dropped" decision.

## 15. Out-of-band assets used

- Java reference client: `/root/s3p-clients/java/`
- Java smoke-test config (acceptance creds): `/root/s3p-clients/java/smoke-test.json`
- Partner OpenAPI spec (source of truth): `/root/php-smobilpay-s3p-api/apidocs/s3p_3.2.0_openapi_specs_partner.yml`
- Partner LLMS brief: `/root/php-smobilpay-s3p-api/apidocs/s3p_3.2.0_llms.txt`
- Partner developer page: `/root/php-smobilpay-s3p-api/apidocs/s3p_3.2.0_developer_page.html`

These are reference material — none are vendored into the Go module.

## 16. Risks & open questions

| Risk | Mitigation |
|---|---|
| Acceptance environment changes break smoke-test creds | `smoke-test.json` is git-ignored; `smoke-test.example.json` carries the schema. Java's working creds can be copied. |
| Java client and Go client drift over time | Smoke-test diff parity is the regression gate. CI runs it nightly against acceptance. |
| Lenient date parser misses a wire format | Add the format to `Date.UnmarshalJSON` test table; ship it. |
| 80% coverage gate is too aggressive for DTO-heavy files | Measured per-package, not per-file. JSON round-trip tests cover DTO branches cheaply. |
| Singleflight introduces a non-stdlib dep | Acceptable — `golang.org/x/*` is the de-facto stdlib annex; alternative (hand-rolled mutex) is more error-prone for marginal gain. |

## 17. Definition of done

- `go build ./...` succeeds.
- `make test` passes with `-race`.
- `make cover` reports ≥ 80% and exits 0.
- `make lint` produces no output.
- `make smoketest` succeeds against acceptance (all non-skipped scenarios PASS).
- `make smoketest-compare` produces an empty diff (modulo redacted volatile fields).
- README renders cleanly on GitHub and pkg.go.dev.
- Every exported symbol carries a godoc comment.
- Old `s3p/` package and old example are removed from the repo.
