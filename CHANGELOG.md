# Changelog

All notable changes to **smobilpay-go** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2026-06-16

Complete rewrite of the client. v1.x was a single HMAC-SHA1 signature helper that
left request assembly and transport to the caller. v2.0.0 is a full partner API
client that authenticates with **OAuth 2.0 `client_credentials`** and covers every
endpoint a partner integrator needs to move money in and out, sell value-added
services, and drive a payment UI from the static catalog.

### ⚠️ Breaking changes

- **Authentication migrated from HMAC-SHA1 request signing to OAuth 2.0
  `client_credentials`.** Legacy HMAC request signing is no longer supported, and
  there is no fallback. The old `s3pAuth_*` request parameters
  (`s3pAuth_nonce`, `s3pAuth_timestamp`, `s3pAuth_token`,
  `s3pAuth_signature_method`) and the per-request shared-secret signature are
  gone. The client now mints a bearer token and sends it as
  `Authorization: Bearer <jwt>` on every secured request.
- **Removed the `s3p` package and its `GenerateSignature` function.** Code that
  imported `github.com/maviance/smobilpay-go/s3p` and called
  `s3p.GenerateSignature(method, url, params, secret)` will no longer compile.
- **New public surface.** The library is now the root package
  `github.com/maviance/smobilpay-go` (import as `smob`). Instead of a standalone
  signature function it exposes a single `Client` facade, constructed via
  `smob.NewConfig(...)` + `smob.New(cfg)`, with API access grouped under
  `client.Verify`, `client.Masterdata`, `client.Initiate`, `client.Confirm`, and
  `client.AccountValidation`.
- **Credentials changed shape.** A single shared `secret` is replaced by an OAuth
  2.0 `publicKey` / `secretKey` credential pair issued during partner onboarding,
  supplied via `smob.WithCredentials(publicKey, secretKey)`.
- **Minimum Go version raised from 1.20 to 1.22.**
- **New runtime dependency:** `golang.org/x/sync` (used for `singleflight` to
  coalesce concurrent token refreshes). v1.x had no third-party dependencies.

### Added

- **OAuth 2.0 token management.** An internal `OAuth2Manager` mints tokens via
  `POST {baseURL}/oauth/token` with a form-urlencoded
  `grant_type=client_credentials` body and `Basic base64(publicKey:secretKey)`
  authorization. Tokens are cached in memory and reused until they are within
  `TokenRefreshSkew` of expiry (default 30s), then re-minted automatically.
  Concurrent mints are deduplicated with `singleflight`. Manual refresh is
  available via `client.Tokens.Refresh(ctx)`.
- **Reactive 401 refresh-and-retry.** The HTTP transport transparently forces a
  token refresh and retries a request once on a `401 Unauthorized` response
  (idempotent and non-idempotent requests alike; the request body is rebuilt for
  the retry). A persistent 401 after the single retry surfaces as an `*APIError`.
- **`Client` facade** with five API groups, safe for concurrent use and intended
  to be reused for the lifetime of the application:
  - **`Verify`** — `Ping`, `Account`, `VerifyTransaction(ptn, trid)`,
    `HistoryByPtn`, `HistoryByTrid`, `HistoryByDateRange`.
  - **`Masterdata`** — `Merchants`, `Services`, `Cashouts`, `Cashins`, `Topups`,
    `Products`, `Vouchers`.
  - **`Initiate`** — `Bills`, `Subscriptions`, `Quote`.
  - **`Confirm`** — `Collect` (single `/v2/collectstd` endpoint for every flow:
    cash-out, bill, top-up, voucher, product, subscription, and cash-in).
  - **`AccountValidation`** — `VerifyServiceNumber`, `ValidateAccount`.
- **Typed error model** so callers can branch with `errors.As`:
  - `*APIError` — non-2xx API responses; exposes `RespCode` (canonical machine
    identifier), `DevMsg`, `UsrMsg`, and `Link`.
  - `*AuthError` — OAuth token issuance/refresh failures; exposes `HTTPStatus`,
    `OAuthError` (RFC 6749 `error` code), `Message`, and `Cause`.
  - `*TransportError` — network/timeout/malformed-URI/unparsable-body failures;
    exposes `Op` (`"METHOD /path"`) and `Cause`.
- **Functional-options configuration** via `NewConfig`: `WithBaseURL`,
  `WithCredentials`, `WithAPIVersion` (default `3.0.0`, sent as `x-api-version`),
  `WithRequestTimeout` (default 30s), `WithTokenRefreshSkew` (default 30s), and
  `WithHTTPClient` (inject a custom `*http.Client` for proxies/TLS).
- **Typed DTOs and enums** for the full domain: `Merchant`, `Service`,
  `Cashout`, `Cashin`, `Topup`, `Product`, `Bill`, `Subscription`,
  `QuoteRequest`/`QuoteResponse`, `CollectionRequest`/`CollectionResponse`,
  `CustomerAccount`, `Ping`, `Account`, `PaymentStatus`, plus the `PaymentItem`
  interface and enums (`ServiceType`, `AmountType`, `BillType`,
  `PaymentStatusType`, `CustomerAccountStatus`, `MerchantStatus`,
  `ServiceStatus`).
- **Input validation at the call boundary** — required `serviceID`, required
  `PayItemID` and `Amount >= 1` on quotes, digits-only phone numbers, RFC 5322
  email validation, `tag`/`callbackUrl` length limits, and "exactly one of"
  constraints on history and subscription lookups — so malformed requests fail
  fast before hitting the network.
- **Lenient wire decoding** — a tolerant `Date` type (accepts `YYYY-MM-DD`,
  RFC 3339, and naive `YYYY-MM-DDTHH:MM:SS`) and lenient numeric/boolean
  decoders that absorb the server's string-encoded numbers.
- **Smoke-test harness** (`cmd/smoketest`) driven by a JSON config
  (`smoke-test.example.json`) that exercises ping, catalog discovery, and
  quote-only payment scenarios against acceptance, with opt-in real
  `/v2/collectstd` execution per flow.
- **Documentation** — a partner-facing `README.md` at Java-client parity depth,
  a package overview in `doc.go`, and runnable per-flow examples under
  `examples/` (`bill`, `cashin`, `cashout`, `history`, `product`,
  `subscription`, `topup`, `verify`, `voucher`).

### Changed

- Minimum/`go.mod` Go version bumped from `1.20` to `1.22`.
- The library is consumed as the root package `smobilpay-go` (import alias
  `smob`) rather than the `s3p` subpackage.

### Removed

- `s3p/signature.go` and `s3p/signature_test.go` — the HMAC-SHA1
  `GenerateSignature` helper and its tests.
- `examples/main.go` — the standalone HMAC signature example.

## [1.0.0] - 2025-05-02

Initial stable release. A minimal HMAC-SHA1 signature generator: the single
exported function `s3p.GenerateSignature(method, url, params, secret)` returned a
Base64-encoded signature for Smobilpay request authentication. Building the HTTP
request and attaching the signature and `s3pAuth_*` parameters was the caller's
responsibility. No external dependencies; Go 1.20.

[2.0.0]: https://github.com/maviance/smobilpay-go/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/maviance/smobilpay-go/releases/tag/v1.0.0
