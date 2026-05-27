package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	req, err := t.newRequest(ctx, "GET", path, q, nil)
	if err != nil {
		return err
	}
	return t.do(req, out)
}

// Post executes an authenticated POST with body serialized as JSON and
// decodes the JSON response body into out.
func (t *Transport) Post(ctx context.Context, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return &TransportError{Op: "POST " + path, Cause: err}
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := t.newRequest(ctx, "POST", path, nil, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return t.do(req, out)
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

func (t *Transport) do(req *http.Request, out any) error {
	op := req.Method + " " + req.URL.Path
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return &TransportError{Op: op, Cause: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &TransportError{Op: op, Cause: fmt.Errorf("read response body: %w", err)}
	}

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

func decodeAPIError(status int, body []byte) error {
	apiErr := &APIError{HTTPStatus: status, RawBody: string(body)}
	if len(body) > 0 {
		var env struct {
			RespCode int    `json:"respCode"`
			DevMsg   string `json:"devMsg"`
			UsrMsg   string `json:"usrMsg"`
			Link     string `json:"link"`
		}
		if err := json.Unmarshal(body, &env); err == nil && env.RespCode != 0 {
			apiErr.RespCode = env.RespCode
			apiErr.DevMsg = env.DevMsg
			apiErr.UsrMsg = env.UsrMsg
			apiErr.Link = env.Link
		}
	}
	return apiErr
}
