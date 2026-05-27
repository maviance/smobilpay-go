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

// API-group placeholders. Methods are added in subsequent commits
// (Tasks 11-15).

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
