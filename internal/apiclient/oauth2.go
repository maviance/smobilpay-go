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
