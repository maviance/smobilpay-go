package smobilpay

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDate_UnmarshalJSON_dateOnly(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"2024-01-15"`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if y, m, day := d.Date(); y != 2024 || m != time.January || day != 15 {
		t.Errorf("Date() = %d-%02d-%02d, want 2024-01-15", y, m, day)
	}
}

func TestDate_UnmarshalJSON_dateTimeZ(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"2024-01-15T10:30:00Z"`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if y, m, day := d.Date(); y != 2024 || m != time.January || day != 15 {
		t.Errorf("Date() = %d-%02d-%02d, want 2024-01-15", y, m, day)
	}
}

func TestDate_UnmarshalJSON_dateTimeOffset(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"2024-01-15T10:30:00+01:00"`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if y, m, day := d.Date(); y != 2024 || m != time.January || day != 15 {
		t.Errorf("Date() = %d-%02d-%02d, want 2024-01-15", y, m, day)
	}
}

func TestDate_UnmarshalJSON_null(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`null`), &d); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if !d.IsZero() {
		t.Error("null should leave Date zero-valued")
	}
}

func TestDate_UnmarshalJSON_bad(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"not-a-date"`), &d); err == nil {
		t.Error("expected error on bad date")
	}
}

func TestDate_MarshalJSON_canonical(t *testing.T) {
	d := Date{time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC)}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `"2024-01-15"` {
		t.Errorf("marshal = %s, want \"2024-01-15\"", b)
	}
}

func TestDate_MarshalJSON_zero(t *testing.T) {
	var d Date
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal zero: %v", err)
	}
	if string(b) != `null` {
		t.Errorf("zero marshals to %s, want null", b)
	}
}

func TestServiceType_constants(t *testing.T) {
	cases := []struct {
		got, want ServiceType
	}{
		{ServiceTypeSearchableBill, "SEARCHABLE_BILL"},
		{ServiceTypeNonSearchableBill, "NON_SEARCHABLE_BILL"},
		{ServiceTypeProduct, "PRODUCT"},
		{ServiceTypeTopup, "TOPUP"},
		{ServiceTypeSubscription, "SUBSCRIPTION"},
		{ServiceTypeCashin, "CASHIN"},
		{ServiceTypeCashout, "CASHOUT"},
		{ServiceTypeVoucher, "VOUCHER"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("ServiceType = %q, want %q", c.got, c.want)
		}
	}
}

func TestMerchantStatus_jsonRoundtrip(t *testing.T) {
	// Server emits mixed-case "Active" / "Inactive" — preserve verbatim.
	for _, raw := range []string{`"Active"`, `"Inactive"`} {
		var s MerchantStatus
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			t.Fatalf("unmarshal %s: %v", raw, err)
		}
		b, _ := json.Marshal(s)
		if string(b) != raw {
			t.Errorf("roundtrip %s = %s", raw, b)
		}
	}
}
