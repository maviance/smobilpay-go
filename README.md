# smobilpay-go

Go client library for the **Smobilpay partner API** (v3.2.0).

This is the curated, partner-facing client. It covers every endpoint a
partner integrator needs to move money in and out, sell value-added
services, and drive a payment UI from the static catalog.

## What this client does

- **Payment collections.** Take payment from a customer's mobile wallet
  via a quote-then-confirm flow. Money flows *out* of the customer's
  wallet against a `Cashout` item. Works for cash-out (generic
  mobile-money collection), bill payment, top-up, voucher purchase,
  product purchase, and subscription top-up.
- **Disbursements.** Send funds out to a recipient's mobile wallet using
  the same quote-then-confirm flow against a `Cashin` item. Money flows
  *into* the recipient's wallet.
- **Account and service discovery.** Retrieve the static catalog of
  merchants, services, products, and payment items needed to drive a
  payment UI.
- **Status verification.** Look up the live status of a previously
  issued transaction by `ptn` or by your own custom `trid`, and search
  historical activity by date range.
- **Pre-payment account validation.** Check that a customer's service
  number is well-formed and accepted by the merchant before quoting.

## Requirements

- **Go 1.22 or newer** at runtime and at build time.
- Network access to the base URL issued by Maviance support.
- An OAuth 2.0 credential pair (`publicKey` / `secretKey`) issued during
  partner onboarding.

This client uses the standard library's `net/http`. The only third-party
runtime dependency is `golang.org/x/sync/singleflight` (used to coalesce
concurrent token refreshes). No HTTP client conflicts with the standard
Go ecosystem.

## Installation

```bash
go get github.com/maviance/smobilpay-go
```

Import as:

```go
import smob "github.com/maviance/smobilpay-go"
```

## Quick start

The library exposes a single `Client` facade. Construct it once per
application with your partner credentials; it lazily mints and caches
the OAuth 2.0 bearer token for the lifetime of the process.

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
		smob.WithBaseURL("https://api.example.invalid"),          // issued during onboarding
		smob.WithCredentials(
			os.Getenv("SMOBILPAY_PUBLIC_KEY"),
			os.Getenv("SMOBILPAY_SECRET_KEY")),
	)
	if err != nil {
		log.Fatal(err)
	}

	client, err := smob.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	pong, err := client.Verify.Ping(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Server time:", pong.Time)
	fmt.Println("Server version:", pong.Version)
}
```

## Authentication

The Smobilpay API uses **OAuth 2.0 `client_credentials`** exclusively.
Legacy HMAC request signing is **not** supported.

The client handles token issuance for you:

1. On the first authenticated request, the client POSTs
   `Basic base64(publicKey:secretKey)` to `{baseUrl}/oauth/token`
   with `grant_type=client_credentials`.
2. The returned JWT is cached in memory and attached as
   `Authorization: Bearer <jwt>` on every subsequent request.
3. The token is reused until it is within `TokenRefreshSkew` of expiry
   (default: 30s); then a fresh one is minted automatically. Concurrent
   refreshes are coalesced via `singleflight`.

To force a refresh (e.g. after a 401), call
`client.Tokens.Refresh(ctx)`.

## Choosing the right flow

Every flow follows the same three-step shape — **discover → quote →
confirm** — and confirmation goes through a single endpoint
(`POST /v2/collectstd`) regardless of whether the payment item is a
cash-out (collection), a bill, a top-up, a voucher, a subscription, a
product, or a cash-in (disbursement).

What changes per flow is the masterdata call you use to discover the
right `PayItemID`:

| Use case                       | Masterdata call                                   | Item type      | Confirm call               | Notes                                                            |
|--------------------------------|---------------------------------------------------|----------------|----------------------------|------------------------------------------------------------------|
| Collection (cash-out)          | `Masterdata.Cashouts(ctx, serviceID)`             | `Cashout`      | `Confirm.Collect(ctx, r)`  | Generic mobile-money collection. Money flows *out* of customer's wallet. |
| Bill payment                   | `Initiate.Bills(ctx, merchant, sid, sn)`          | `Bill`         | `Confirm.Collect(ctx, r)`  | Bill is looked up by `serviceNumber`, not from static masterdata. |
| Airtime top-up                 | `Masterdata.Topups(ctx, serviceID)`               | `Topup`        | `Confirm.Collect(ctx, r)`  | Recipient phone goes on `CustomerPhoneNumber` or `ServiceNumber`. |
| Voucher purchase               | `Masterdata.Vouchers(ctx, serviceID)`             | `Product`      | `Confirm.Collect(ctx, r)`  | Code returned on `CollectionResponse.PIN`.                       |
| Product purchase               | `Masterdata.Products(ctx, serviceID)`             | `Product`      | `Confirm.Collect(ctx, r)`  | Same shape as voucher but no PIN on the response.                |
| Subscription top-up (pay-TV …) | `Initiate.Subscriptions(ctx, m, sid, sn, cn)`     | `Subscription` | `Confirm.Collect(ctx, r)`  | Looked up by `serviceNumber` *or* `customerNumber`.              |
| Disbursement (cash-in)         | `Masterdata.Cashins(ctx, serviceID)`              | `Cashin`       | `Confirm.Collect(ctx, r)`  | Payout to recipient. Same `Collect` endpoint, item is a cash-in. |

For every item type the `PayItemID()` value is what flows into the
quote request. The `IsReq*` flags on the `Service` masterdata entry
tell you which optional `CollectionRequest` fields (customer name,
service number, customer number, …) become required for that service.

## Conventions

- All requests and responses are **JSON**.
- **Monetary amounts** on `QuoteRequest.Amount` are integers in the
  local currency of the payment item (no decimals). Other amount fields
  on responses are floats per spec.
- **Currencies** are ISO 4217 codes (e.g. `XAF`, `EUR`).
- **Countries** are ISO 3166-1 alpha-3 codes (e.g. `CMR`).
- **Phone numbers** are E.164 without the leading `+` (e.g.
  `237699999999`).
- **Errors** raised by the API are returned as `*smob.APIError`. Use
  `errors.As(err, &apiErr)` and match on `apiErr.RespCode` for
  programmatic handling — that is the canonical machine identifier per
  the partner spec. Auth failures surface as `*smob.AuthError` and
  network/decoding failures as `*smob.TransportError`.
- The `x-api-version: 3.0.0` header is attached on every secured
  request. Override via `smob.WithAPIVersion(...)` if you need a
  different protocol shape.

## Collection — cash-out

A `Cashout` item collects funds *out* of the customer's mobile wallet
into the partner's balance. This is the generic mobile-money collection
flow.

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
		smob.WithBaseURL("https://api.example.invalid"),
		smob.WithCredentials(
			os.Getenv("SMOBILPAY_PUBLIC_KEY"),
			os.Getenv("SMOBILPAY_SECRET_KEY")),
	)
	if err != nil {
		log.Fatal(err)
	}
	client, err := smob.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// 1. Discover the cash-out items available for service 999999
	cashouts, err := client.Masterdata.Cashouts(ctx, 999999)
	if err != nil {
		log.Fatal(err)
	}
	item := cashouts[0]

	// 2. Request a quote (amounts are integers in local currency)
	quote, err := client.Initiate.Quote(ctx, smob.QuoteRequest{
		Amount:    500,
		PayItemID: item.PayItemID(),
	})
	if err != nil {
		log.Fatal(err)
	}

	// 3. Confirm the collection
	resp, err := client.Confirm.Collect(ctx, smob.CollectionRequest{
		QuoteID:              quote.QuoteID,
		CustomerPhoneNumber:  "237699999999",     // E.164, no leading +
		CustomerEmailAddress: "customer@example.com",
		ServiceNumber:        "2371122334455",    // when service.IsReqServiceNumber
		TRID:                 "ORDER-2026-05-02-0001",
		Tag:                  "retail-front-desk", // reporting tag (<= 50 chars)
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("PTN:", resp.PTN)
	fmt.Println("Status:", resp.Status) // PENDING on x-api-version 3.0.0

	// 4. Poll for final status by PTN
	statuses, err := client.Verify.VerifyTransaction(ctx, resp.PTN, "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Final status:", statuses[0].Status)
}
```

## Collection — bill payment

```go
bills, err := client.Initiate.Bills(ctx, "CDE", 4321, "METER-001")
if err != nil {
	log.Fatal(err)
}
bill := bills[0]

amount := 0
if bill.AmountLocalCur() != nil {
	amount = int(*bill.AmountLocalCur())
}

quote, err := client.Initiate.Quote(ctx, smob.QuoteRequest{
	Amount:    amount,
	PayItemID: bill.PayItemID(),
})
if err != nil {
	log.Fatal(err)
}

resp, err := client.Confirm.Collect(ctx, smob.CollectionRequest{
	QuoteID:              quote.QuoteID,
	CustomerPhoneNumber:  "237699999999",
	CustomerEmailAddress: "customer@example.com",
	ServiceNumber:        "METER-001",
	CustomerName:         "Jane Doe", // when service.IsReqCustomerName
})
```

## Collection — airtime top-up

```go
topups, err := client.Masterdata.Topups(ctx, serviceID)
if err != nil {
	log.Fatal(err)
}
topup := topups[0]

// FIXED-amount top-ups quote at the catalog price; CUSTOM-amount top-ups
// take any integer in the local currency.
amount := 500
if topup.AmountLocalCur() != nil {
	amount = int(*topup.AmountLocalCur())
}

quote, err := client.Initiate.Quote(ctx, smob.QuoteRequest{
	Amount:    amount,
	PayItemID: topup.PayItemID(),
})
if err != nil {
	log.Fatal(err)
}

resp, err := client.Confirm.Collect(ctx, smob.CollectionRequest{
	QuoteID:              quote.QuoteID,
	CustomerPhoneNumber:  "237699999999",
	CustomerEmailAddress: "customer@example.com",
	ServiceNumber:        "237699999999", // recipient MSISDN
})
```

## Collection — voucher purchase

For services of type `VOUCHER` the digital code is delivered on
`CollectionResponse.PIN` once the collection succeeds.

```go
vouchers, err := client.Masterdata.Vouchers(ctx, serviceID)
if err != nil {
	log.Fatal(err)
}
voucher := vouchers[0]

quote, err := client.Initiate.Quote(ctx, smob.QuoteRequest{
	Amount:    int(*voucher.AmountLocalCur()),
	PayItemID: voucher.PayItemID(),
})
if err != nil {
	log.Fatal(err)
}

resp, err := client.Confirm.Collect(ctx, smob.CollectionRequest{
	QuoteID:              quote.QuoteID,
	CustomerPhoneNumber:  customerPhone,
	CustomerEmailAddress: customerEmail,
})
if err != nil {
	log.Fatal(err)
}

redemptionPIN := resp.PIN
```

## Collection — product purchase

Generic products work like vouchers but do not return a redemption PIN:

```go
products, err := client.Masterdata.Products(ctx, serviceID)
if err != nil {
	log.Fatal(err)
}
product := products[0]

quote, err := client.Initiate.Quote(ctx, smob.QuoteRequest{
	Amount:    int(*product.AmountLocalCur()),
	PayItemID: product.PayItemID(),
})
if err != nil {
	log.Fatal(err)
}

resp, err := client.Confirm.Collect(ctx, smob.CollectionRequest{
	QuoteID:              quote.QuoteID,
	CustomerPhoneNumber:  customerPhone,
	CustomerEmailAddress: customerEmail,
})
```

## Collection — subscription top-up

Subscriptions (e.g. pay-TV like Canal+) are looked up by *either*
`serviceNumber` *or* `customerNumber` — pass one and leave the other
empty (`""`), not both. The returned slice may contain several
`Subscription` items representing different renewal options for the
same customer; pick one and quote against its `PayItemID()`.

```go
subs, err := client.Initiate.Subscriptions(
	ctx,
	"CANALPLUS",
	4321,
	"DECODER-001234", // serviceNumber
	"")               // customerNumber (or vice versa)
if err != nil {
	log.Fatal(err)
}
sub := subs[0]

quote, err := client.Initiate.Quote(ctx, smob.QuoteRequest{
	Amount:    int(*sub.AmountLocalCur()),
	PayItemID: sub.PayItemID(),
})
if err != nil {
	log.Fatal(err)
}

resp, err := client.Confirm.Collect(ctx, smob.CollectionRequest{
	QuoteID:              quote.QuoteID,
	CustomerPhoneNumber:  "237699999999",
	CustomerEmailAddress: "customer@example.com",
	ServiceNumber:        "DECODER-001234",
	CustomerName:         sub.CustomerName(),
})
```

## Disbursement — cash-in

A `Cashin` item pays funds *into* a recipient's mobile wallet from the
partner's balance. It goes through the same `/v2/collectstd` endpoint
as collections — same `CollectionRequest`, same `CollectionResponse`.

```go
cashins, err := client.Masterdata.Cashins(ctx, serviceID)
if err != nil {
	log.Fatal(err)
}
cashin := cashins[0]

quote, err := client.Initiate.Quote(ctx, smob.QuoteRequest{
	Amount:    10_000,
	PayItemID: cashin.PayItemID(),
})
if err != nil {
	log.Fatal(err)
}

resp, err := client.Confirm.Collect(ctx, smob.CollectionRequest{
	QuoteID:              quote.QuoteID,
	CustomerPhoneNumber:  "237699999999",      // recipient phone
	CustomerEmailAddress: "recipient@example.com",
	ServiceNumber:        "237699999999",      // recipient MSISDN
	TRID:                 "PAYOUT-2026-05-02-0001",
})
```

## Pre-payment verification

For services that report `IsVerifiable: true`, you can verify a service
number before quoting:

```go
valid, err := client.AccountValidation.VerifyServiceNumber(
	ctx, "ENEO", 1234, "01234567")
```

## Catalog discovery

Most integrations cache the catalog and refresh it on a schedule:

```go
merchants, err := client.Masterdata.Merchants(ctx)
if err != nil {
	log.Fatal(err)
}
services, err := client.Masterdata.Services(ctx)
if err != nil {
	log.Fatal(err)
}
```

The `Service` value tells you which flow applies (cash-out, bill,
top-up, voucher, product, subscription, cash-in) via its `ServiceType`
field and which optional `CollectionRequest` fields the merchant
requires via the `IsReq*` boolean flags.

## Account and ping utilities

```go
// Liveness check + protocol/version handshake.
pong, err := client.Verify.Ping(ctx)

// Aggregator-level account info: balance, currency, status.
account, err := client.Verify.Account(ctx)
```

## Historical lookups

Search by **exactly one** of:

```go
client.Verify.HistoryByPtn(ctx, "PTN-202605020800001")
client.Verify.HistoryByTrid(ctx, "ORDER-2026-05-02-0001")
client.Verify.HistoryByDateRange(
	ctx,
	time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC))
```

Combinations are rejected by the server with an error envelope.

## Error handling

```go
import (
	"errors"
	"log"

	smob "github.com/maviance/smobilpay-go"
)

quote, err := client.Initiate.Quote(ctx, req)
if err != nil {
	var authErr *smob.AuthError
	var apiErr *smob.APIError
	var transportErr *smob.TransportError

	switch {
	case errors.As(err, &authErr):
		// OAuth 2.0 token issuance failed — bad credentials, etc.
		log.Printf("auth failed: status=%d oauthError=%s",
			authErr.HTTPStatus, authErr.OAuthError)

	case errors.As(err, &apiErr):
		// API returned a non-2xx with the standard Error envelope
		log.Printf("API error: respCode=%d devMsg=%s link=%s",
			apiErr.RespCode, apiErr.DevMsg, apiErr.Link)
		if apiErr.HTTPStatus == 498 {
			// Quote expired — re-quote and retry
		}

	case errors.As(err, &transportErr):
		// Network failure, timeout, malformed URI, unparsable body
		log.Printf("transport error on %s: %v",
			transportErr.Op, transportErr.Cause)

	default:
		log.Printf("unexpected error: %v", err)
	}
}
```

The full Smobilpay error catalog (the `RespCode` → meaning mapping) is
delivered to partners during onboarding.

## Configuration reference

| Option                     | Default       | Notes                                                |
|----------------------------|---------------|------------------------------------------------------|
| `WithBaseURL(url)`         | required      | Issued during onboarding. Trailing `/` is stripped.  |
| `WithCredentials(pk, sk)`  | required      | OAuth 2.0 `publicKey` / `secretKey` pair             |
| `WithAPIVersion(v)`        | `3.0.0`       | Value sent as `x-api-version` header                 |
| `WithRequestTimeout(d)`    | `30s`         | Per-request timeout for the default HTTP client      |
| `WithTokenRefreshSkew(d)`  | `30s`         | Mint a fresh token this far ahead of expiry          |
| `WithHTTPClient(c)`        | auto-created  | Inject a custom `*http.Client` (proxy, custom TLS …) |

To use a custom `*http.Client` (proxies, custom TLS, etc.):

```go
http := &http.Client{
	Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	Timeout:   30 * time.Second,
}
cfg, err := smob.NewConfig(
	smob.WithBaseURL("https://api.example.invalid"),
	smob.WithCredentials(pk, sk),
	smob.WithHTTPClient(http),
)
```

When you inject an `*http.Client`, its own `Timeout` field governs
per-request deadlines; the value passed to `WithRequestTimeout` is
**not** applied to an injected client. Pass an `*http.Client` whose
`Timeout` is `0` to disable per-request timeouts entirely.

## Onboarding

Base URL, partner credentials (`publicKey` / `secretKey`), callback URL
registration, and the full error catalog are issued by Maviance support
during partner onboarding. They are intentionally not published in the
spec or this README. Contact **support@smobilpay.com**.

## Development

Build:

```bash
make build
```

Run tests + race detector:

```bash
make test
```

Coverage gate (80% line coverage, excluding `cmd/smoketest`):

```bash
make cover
```

Lint (`go vet` + `staticcheck` when available):

```bash
make lint
```

### Smoke test against acceptance

```bash
cp smoke-test.example.json smoke-test.json
# edit smoke-test.json with baseUrl + credentials
make smoketest
# diff against Java client output (verifies cross-language parity):
make smoketest-compare JAVA_DIR=../java
```

### Real /v2/collectstd via opt-in

By default every collection scenario stops at the quote. To exercise a
real collect on a given flow, add the following keys to that block in
`smoke-test.json`:

```json
{
  "collect": true,
  "customerPhonenumber": "699999999",
  "customerEmailaddress": "you@example.com",
  "serviceNumber": "699999999"
}
```

The harness will then call `/v2/collectstd` after the quote and poll
`/v2/verifytx` once after a 2-second settle to surface the final
status. **WARNING:** opting in moves real money on the partner balance.

## License

MIT © Maviance PLC. See [LICENSE](./LICENSE).
