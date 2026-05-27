package apiclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	h.mu.Lock()
	respond := h.respond
	h.mu.Unlock()
	respond(w, r)
}

func okTokenHandler(token string, expiresIn int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != tokenPath {
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
	m := newTestManager(t, srv.URL, clock)
	if _, err := m.AccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	atomic.StoreInt64(&advance, 6) // skew=5 means token treated as expired
	h.mu.Lock()
	h.respond = okTokenHandler("jwt-2", 10)
	h.mu.Unlock()
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
	h.mu.Lock()
	h.respond = okTokenHandler("jwt-2", 3600)
	h.mu.Unlock()
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

func TestOAuth2Manager_Cached_emptyThenPopulated(t *testing.T) {
	h := &oauthHandler{respond: okTokenHandler("jwt-A", 3600)}
	srv := httptest.NewServer(h)
	defer srv.Close()

	m := newTestManager(t, srv.URL, time.Now)
	if _, ok := m.Cached(); ok {
		t.Fatal("Cached() should be empty before any AccessToken call")
	}
	if _, err := m.AccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	tok, ok := m.Cached()
	if !ok {
		t.Fatal("Cached() should be populated after a successful mint")
	}
	if tok.AccessToken != "jwt-A" {
		t.Errorf("Cached().AccessToken = %q, want jwt-A", tok.AccessToken)
	}
}

func TestOAuth2Manager_Refresh_failureLeavesCacheIntact(t *testing.T) {
	// First mint succeeds; then we flip the handler to 500 and call
	// Refresh, which must error but must NOT overwrite the cached token.
	h := &oauthHandler{respond: okTokenHandler("jwt-good", 3600)}
	srv := httptest.NewServer(h)
	defer srv.Close()

	m := newTestManager(t, srv.URL, time.Now)
	if _, err := m.AccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Sanity: cache populated.
	before, _ := m.Cached()
	if before.AccessToken != "jwt-good" {
		t.Fatalf("setup: cached = %q", before.AccessToken)
	}

	h.mu.Lock()
	h.respond = func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = io.WriteString(w, "boom")
	}
	h.mu.Unlock()

	if _, err := m.Refresh(context.Background()); err == nil {
		t.Fatal("expected error on 500 during Refresh")
	}

	after, ok := m.Cached()
	if !ok || after.AccessToken != "jwt-good" {
		t.Errorf("cache was disturbed after failed Refresh: ok=%v, token=%q", ok, after.AccessToken)
	}
}

func TestOAuth2Manager_missingExpiresIn(t *testing.T) {
	h := &oauthHandler{respond: func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"access_token":"jwt-X","token_type":"Bearer"}`)
	}}
	srv := httptest.NewServer(h)
	defer srv.Close()
	m := newTestManager(t, srv.URL, time.Now)
	_, err := m.AccessToken(context.Background())
	var ae *apiclient.AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %v", err)
	}
	if !strings.Contains(ae.Message, "expires_in") {
		t.Errorf("message = %q, want substring expires_in", ae.Message)
	}
}

func TestOAuth2Manager_authErrorOnNonJSONBody(t *testing.T) {
	h := &oauthHandler{respond: func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = io.WriteString(w, "Unauthorized")
	}}
	srv := httptest.NewServer(h)
	defer srv.Close()
	m := newTestManager(t, srv.URL, time.Now)
	_, err := m.AccessToken(context.Background())
	var ae *apiclient.AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %v", err)
	}
	if ae.HTTPStatus != 401 || ae.OAuthError != "" {
		t.Errorf("got %+v", ae)
	}
}

func TestOAuth2Manager_authErrorOnMalformedJSONBody(t *testing.T) {
	h := &oauthHandler{respond: func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":`)
	}}
	srv := httptest.NewServer(h)
	defer srv.Close()
	m := newTestManager(t, srv.URL, time.Now)
	_, err := m.AccessToken(context.Background())
	var ae *apiclient.AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %v", err)
	}
	if !strings.Contains(ae.Message, "not valid JSON") {
		t.Errorf("message = %q", ae.Message)
	}
}

func TestOAuth2Manager_transportErrorOnDial(t *testing.T) {
	// Point at an unreachable port to force a dial failure.
	m := apiclient.NewOAuth2Manager(
		"http://127.0.0.1:1", "pub", "sec",
		5*time.Second,
		&http.Client{Timeout: 200 * time.Millisecond},
		time.Now,
	)
	_, err := m.AccessToken(context.Background())
	var te *apiclient.TransportError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TransportError, got %v", err)
	}
	if !strings.HasPrefix(te.Op, "POST /oauth/token") {
		t.Errorf("Op = %q", te.Op)
	}
}

func TestNewOAuth2Manager_defaultsClockAndHTTPClient(t *testing.T) {
	// Pass nil for both clock and httpClient — the constructor should
	// substitute time.Now and http.DefaultClient without panicking.
	m := apiclient.NewOAuth2Manager("https://x.invalid", "p", "s", time.Second, nil, nil)
	if m == nil {
		t.Fatal("NewOAuth2Manager returned nil")
	}
	// Force a mint attempt — should hit the network and produce a
	// TransportError (because x.invalid won't resolve / connect quickly).
	// We don't care about success; only that the manager is wired up.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _ = m.AccessToken(ctx)
}
