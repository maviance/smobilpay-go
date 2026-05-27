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
