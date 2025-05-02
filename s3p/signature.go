package s3p

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/url"
	"sort"
	"strings"
)

func GenerateSignature(method string, rawURL string, params map[string]string, secret string) string {
	baseStr := buildBaseString(method, rawURL, params)
	h := hmac.New(sha1.New, []byte(secret))
	h.Write([]byte(baseStr))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func buildBaseString(method, rawURL string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var paramParts []string
	for _, k := range keys {
		paramParts = append(paramParts, k+"="+params[k])
	}
	paramStr := strings.Join(paramParts, "&")

	return strings.ToUpper(method) + "&" +
		percentEncode(strings.TrimSpace(rawURL)) + "&" +
		percentEncode(paramStr)
}

func percentEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
