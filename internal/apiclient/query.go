// Package apiclient holds the internal HTTP transport, OAuth 2.0 token
// manager, and query-string builder. Not part of the public API; only
// the smobilpay root package may depend on it.
package apiclient

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Query builds an ordered query string. nil values, empty strings, and
// zero int64 values are silently skipped — matching the Java client's
// QueryParams.add behaviour — so callers can pass optional parameters
// uniformly without nil-checking.
//
// Pass a pointer (*int64, *float64, *string) to force-include an
// otherwise-skippable zero value.
type Query struct {
	keys []string
	vals []string
}

// NewQuery returns an empty Query.
func NewQuery() *Query { return &Query{} }

// IsEmpty reports whether the query is empty.
func (q *Query) IsEmpty() bool { return len(q.keys) == 0 }

// Add appends key=value if value is non-empty after encoding rules.
// Returns q for chaining.
func (q *Query) Add(key string, value any) *Query {
	s, ok := encodeValue(value)
	if !ok {
		return q
	}
	q.keys = append(q.keys, key)
	q.vals = append(q.vals, s)
	return q
}

// Encode renders the query string (no leading "?"), URL-encoding values
// and preserving insertion order.
func (q *Query) Encode() string {
	if q.IsEmpty() {
		return ""
	}
	var b strings.Builder
	for i, k := range q.keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(k))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(q.vals[i]))
	}
	return b.String()
}

func encodeValue(value any) (string, bool) {
	switch v := value.(type) {
	case nil:
		return "", false
	case string:
		if v == "" {
			return "", false
		}
		return v, true
	case *string:
		if v == nil {
			return "", false
		}
		return *v, true
	case int:
		if v == 0 {
			return "", false
		}
		return strconv.Itoa(v), true
	case int64:
		if v == 0 {
			return "", false
		}
		return strconv.FormatInt(v, 10), true
	case *int64:
		if v == nil {
			return "", false
		}
		return strconv.FormatInt(*v, 10), true
	case float64:
		if v == 0 {
			return "", false
		}
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case *float64:
		if v == nil {
			return "", false
		}
		return strconv.FormatFloat(*v, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(v), true
	case time.Time:
		if v.IsZero() {
			return "", false
		}
		return v.UTC().Format(time.RFC3339), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}
