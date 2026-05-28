// Package smobilpay is the Go client for the Smobilpay partner API (v3.2.0).
//
// The client covers every partner-facing endpoint a partner needs to move
// money in and out, sell value-added services, and drive a payment UI from
// the static catalog.
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
//	import "github.com/maviance/smobilpay-go"
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
//	if err != nil { /* call failed */ }
//	_ = ping
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
