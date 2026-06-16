package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	return t.do(ctx, func() (*http.Request, error) {
		return t.newRequest(ctx, "GET", path, q, nil)
	}, out)
}

// Post executes an authenticated POST with body serialized as JSON and
// decodes the JSON response body into out.
func (t *Transport) Post(ctx context.Context, path string, body any, out any) error {
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return &TransportError{Op: "POST " + path, Cause: err}
		}
		raw = b
	}
	return t.do(ctx, func() (*http.Request, error) {
		var bodyReader io.Reader
		if raw != nil {
			bodyReader = bytes.NewReader(raw)
		}
		req, err := t.newRequest(ctx, "POST", path, nil, bodyReader)
		if err != nil {
			return nil, err
		}
		if raw != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, nil
	}, out)
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

// do executes a request produced by build, retrying once on a 401 after
// forcing a token refresh. build is a thunk because the retry needs a fresh
// *http.Request: the first attempt consumes the body reader, and the retry
// must carry the refreshed bearer. A 401 is rejected at the auth layer before
// any business logic runs, so retrying is safe even for non-idempotent POSTs.
// The retry is bounded to one attempt; a persistent 401 (e.g. a restricted
// endpoint) falls through to decodeAPIError.
func (t *Transport) do(ctx context.Context, build func() (*http.Request, error), out any) error {
	req, err := build()
	if err != nil {
		return err
	}
	resp, body, err := t.send(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		if _, rerr := t.tokens.Refresh(ctx); rerr != nil {
			return rerr
		}
		req, err = build()
		if err != nil {
			return err
		}
		resp, body, err = t.send(req)
		if err != nil {
			return err
		}
	}

	op := req.Method + " " + req.URL.Path
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

// send performs a single HTTP attempt and returns the response together with
// its fully-read body. Transport-level failures become *TransportError.
func (t *Transport) send(req *http.Request) (*http.Response, []byte, error) {
	op := req.Method + " " + req.URL.Path
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, nil, &TransportError{Op: op, Cause: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, &TransportError{Op: op, Cause: fmt.Errorf("read response body: %w", err)}
	}
	return resp, body, nil
}

func decodeAPIError(status int, body []byte) error {
	apiErr := &APIError{HTTPStatus: status, RawBody: string(body)}
	if len(body) > 0 {
		var env struct {
			RespCode flexInt `json:"respCode"`
			DevMsg   string  `json:"devMsg"`
			UsrMsg   string  `json:"usrMsg"`
			Link     string  `json:"link"`
		}
		if err := json.Unmarshal(body, &env); err == nil && env.RespCode != 0 {
			apiErr.RespCode = int(env.RespCode)
			apiErr.DevMsg = env.DevMsg
			apiErr.UsrMsg = env.UsrMsg
			apiErr.Link = env.Link
		}
	}
	return apiErr
}

// flexInt decodes a JSON int or a quoted-string int. The Smobilpay
// error envelope serialises respCode as a string in some responses; this
// absorbs both forms so the per-respCode skip logic in callers keeps
// working.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*f = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*f = 0
			return nil
		}
		v, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("apiclient: flexInt: cannot parse %q: %w", s, err)
		}
		*f = flexInt(v)
		return nil
	}
	var v int
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*f = flexInt(v)
	return nil
}
