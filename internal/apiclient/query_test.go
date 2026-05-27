package apiclient

import (
	"testing"
	"time"
)

func TestQuery_empty(t *testing.T) {
	q := NewQuery()
	if got := q.Encode(); got != "" {
		t.Errorf("empty Encode() = %q, want \"\"", got)
	}
	if !q.IsEmpty() {
		t.Error("IsEmpty() = false on empty builder")
	}
}

func TestQuery_skipsNilOrEmpty(t *testing.T) {
	q := NewQuery().
		Add("nilv", nil).
		Add("empty", "").
		Add("kept", "v")
	if got := q.Encode(); got != "kept=v" {
		t.Errorf("Encode() = %q, want %q", got, "kept=v")
	}
}

func TestQuery_skipsZeroInt64(t *testing.T) {
	q := NewQuery().Add("z", int64(0)).Add("n", int64(99))
	if got := q.Encode(); got != "n=99" {
		t.Errorf("Encode() = %q, want %q", got, "n=99")
	}
}

func TestQuery_keepsZeroIntWhenWrappedInPtr(t *testing.T) {
	zero := int64(0)
	q := NewQuery().Add("z", &zero)
	if got := q.Encode(); got != "z=0" {
		t.Errorf("Encode() = %q, want %q", got, "z=0")
	}
}

func TestQuery_encodesTimeISO8601(t *testing.T) {
	at := time.Date(2024, 5, 1, 12, 30, 0, 0, time.UTC)
	q := NewQuery().Add("ts", at)
	want := "ts=2024-05-01T12%3A30%3A00Z"
	if got := q.Encode(); got != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

func TestQuery_preservesInsertionOrder(t *testing.T) {
	q := NewQuery().
		Add("b", "2").
		Add("a", "1").
		Add("c", "3")
	if got := q.Encode(); got != "b=2&a=1&c=3" {
		t.Errorf("Encode() = %q, want %q", got, "b=2&a=1&c=3")
	}
}

func TestQuery_urlEncodesValues(t *testing.T) {
	q := NewQuery().Add("k", "a b&c")
	if got := q.Encode(); got != "k=a+b%26c" {
		t.Errorf("Encode() = %q", got)
	}
}

func TestQuery_pointerString_nilSkipped_nonNilKept(t *testing.T) {
	var nilPtr *string
	kept := ""
	q := NewQuery().Add("nil", nilPtr).Add("empty-ptr", &kept).Add("v", "x")
	got := q.Encode()
	// nil pointer skipped; *string("") forced via pointer.
	if got != "empty-ptr=&v=x" {
		t.Errorf("Encode() = %q", got)
	}
}

func TestQuery_int64PointerNilSkipped(t *testing.T) {
	var nilPtr *int64
	q := NewQuery().Add("nil", nilPtr).Add("kept", int64(5))
	if got := q.Encode(); got != "kept=5" {
		t.Errorf("Encode() = %q", got)
	}
}

func TestQuery_float64_skipZeroIncludeNonZero(t *testing.T) {
	q := NewQuery().Add("z", 0.0).Add("n", 3.14)
	if got := q.Encode(); got != "n=3.14" {
		t.Errorf("Encode() = %q", got)
	}
}

func TestQuery_float64PointerForceIncludesZero(t *testing.T) {
	zero := 0.0
	var nilPtr *float64
	q := NewQuery().Add("nil", nilPtr).Add("z", &zero)
	if got := q.Encode(); got != "z=0" {
		t.Errorf("Encode() = %q", got)
	}
}

func TestQuery_boolAlwaysIncluded(t *testing.T) {
	q := NewQuery().Add("t", true).Add("f", false)
	if got := q.Encode(); got != "t=true&f=false" {
		t.Errorf("Encode() = %q", got)
	}
}

func TestQuery_timeZeroSkipped(t *testing.T) {
	var zero time.Time
	q := NewQuery().Add("ts", zero).Add("k", "v")
	if got := q.Encode(); got != "k=v" {
		t.Errorf("Encode() = %q", got)
	}
}

func TestQuery_unsupportedTypePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on unsupported value type")
		}
	}()
	q := NewQuery()
	q.Add("dur", time.Second) // time.Duration is not in the type switch
}
