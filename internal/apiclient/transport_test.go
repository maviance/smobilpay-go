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

// tokenPath and okTokenHandler are already declared in oauth2_test.go
// (same package apiclient_test). Reuse them here.

type pingDTO struct {
	Time    string `json:"time"`
	Version string `json:"version"`
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

// Token-fetch failure surfaces as *AuthError, NOT *TransportError.
// The transport propagates the OAuth manager's error bare.
func TestTransport_GET_tokenFetchFailureSurfacesAsAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"error":"invalid_client"}`)
			return
		}
		// This handler should never be reached because token mint fails first.
		t.Errorf("unexpected call to %s", r.URL.Path)
	}))
	defer srv.Close()

	mgr := apiclient.NewOAuth2Manager(srv.URL, "pub", "sec", 30*time.Second, http.DefaultClient, time.Now)
	tr := apiclient.NewTransport(srv.URL, "3.0.0", http.DefaultClient, mgr)

	var out pingDTO
	err := tr.Get(context.Background(), "/v2/ping", apiclient.NewQuery(), &out)
	var ae *apiclient.AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %v (%T)", err, err)
	}
	if ae.HTTPStatus != 401 || ae.OAuthError != "invalid_client" {
		t.Errorf("got %+v", ae)
	}
}

func TestTransport_GET_contextCancellation(t *testing.T) {
	// Server hangs forever; the request's context will be cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			okTokenHandler("jwt-X", 3600)(w, r)
			return
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	tr := newTestTransport(t, srv.URL, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var out pingDTO
	err := tr.Get(ctx, "/v2/ping", apiclient.NewQuery(), &out)
	var te *apiclient.TransportError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TransportError on ctx cancel, got %v (%T)", err, err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected wrapped context.DeadlineExceeded, got %v", err)
	}
}

func TestTransport_GET_nilOutDiscardsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			okTokenHandler("jwt-X", 3600)(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"time":"2024-01-01T00:00:00Z","version":"3.0.0"}`)
	}))
	defer srv.Close()

	tr := newTestTransport(t, srv.URL, srv.URL)
	// Pass nil for out — the body should be silently discarded with no error.
	if err := tr.Get(context.Background(), "/v2/ping", apiclient.NewQuery(), nil); err != nil {
		t.Errorf("expected nil err with nil out, got %v", err)
	}
}

func TestTransport_TokensAccessor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	tr := newTestTransport(t, srv.URL, srv.URL)
	if tr.Tokens() == nil {
		t.Error("Tokens() returned nil")
	}
}

func TestNewTransport_defaultsHTTPClient(t *testing.T) {
	// Passing nil for httpClient should not panic; constructor wires
	// up http.DefaultClient internally.
	mgr := apiclient.NewOAuth2Manager("https://x.invalid", "p", "s", time.Second, nil, nil)
	tr := apiclient.NewTransport("https://x.invalid", "3.0.0", nil, mgr)
	if tr == nil {
		t.Fatal("NewTransport returned nil")
	}
	// Smoke: Tokens() must return the wired manager.
	if tr.Tokens() != mgr {
		t.Error("Tokens() did not return the manager passed to NewTransport")
	}
}
