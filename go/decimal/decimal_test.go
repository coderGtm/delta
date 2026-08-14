package decimal

import (
	"encoding/json"
	"math"
	"math/big"
	"testing"
)

func mustParse(t *testing.T, s string) Decimal {
	t.Helper()
	d, err := Parse([]byte(s))
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return *d
}

func TestParse(t *testing.T) {
	cases := []struct {
		in    string
		coeff string
		scale int32
	}{
		{"40.7128", "407128", 4},
		{"-0.5", "-5", 1},
		{"1e3", "1000", 0},
		{"1.5e-2", "15", 3},
		{"0", "0", 0},
		{"-0", "0", 0},
		{"1.5E2", "150", 0},
	}
	for _, tc := range cases {
		d := mustParse(t, tc.in)
		want, _ := new(big.Int).SetString(tc.coeff, 10)
		if d.coeff.Cmp(want) != 0 {
			t.Errorf("Parse(%q) coeff = %s, want %s", tc.in, d.coeff, tc.coeff)
		}
		if d.scale != tc.scale {
			t.Errorf("Parse(%q) scale = %d, want %d", tc.in, d.scale, tc.scale)
		}
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	for _, in := range []string{"abc", "", "NaN", "1.", ".5", "-", "+1", "1e", "1.2.3", "Infinity"} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", in)
		}
	}
}

func TestParseExponentBounds(t *testing.T) {
	tenPow100 := new(big.Int).Exp(big.NewInt(10), big.NewInt(100), nil)
	valid := []struct {
		in    string
		coeff *big.Int
		scale int32
	}{
		{"1e100", tenPow100, 0},
		{"-1e100", new(big.Int).Neg(tenPow100), 0},
		{"1e-100", big.NewInt(1), 100},
		{"1e0", big.NewInt(1), 0},
	}
	for _, tc := range valid {
		d, err := Parse([]byte(tc.in))
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if d.coeff.Cmp(tc.coeff) != 0 || d.scale != tc.scale {
			t.Errorf("Parse(%q) = coeff %s scale %d, want coeff %s scale %d", tc.in, d.coeff, d.scale, tc.coeff, tc.scale)
		}
	}

	for _, in := range []string{"1e4294967297", "1e-3000000000", "1e99999999999999999999", "1e101", "1e-101"} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", in)
		}
	}
}

func TestScaleToRoundHalfAway(t *testing.T) {
	cases := []struct {
		in    string
		scale int32
		want  string
	}{
		{"12.345", 2, "12.35"},
		{"-12.345", 2, "-12.35"},
		{"12.344", 2, "12.34"},
		{"12.5", 0, "13"},
		{"-12.5", 0, "-13"},
		{"12.05", 1, "12.1"},
		{"123456789012345678901234567890.5", 0, "123456789012345678901234567891"},
	}
	for _, tc := range cases {
		d := mustParse(t, tc.in)
		d.ScaleTo(tc.scale)
		if got := d.Format(tc.scale); got != tc.want {
			t.Errorf("ScaleTo(%q -> %d) = %s, want %s", tc.in, tc.scale, got, tc.want)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		in    string
		scale int32
		want  string
	}{
		{"40.7128", 7, "40.7128000"},
		{"-74.006", 7, "-74.0060000"},
		{"0", 7, "0.0000000"},
		{"-0", 7, "0.0000000"},
		{"1", 2, "1.00"},
		{"0.5", 7, "0.5000000"},
		{"99.9", 0, "100"},
	}
	for _, tc := range cases {
		d := mustParse(t, tc.in)
		if got := d.Format(tc.scale); got != tc.want {
			t.Errorf("Format(%q, %d) = %s, want %s", tc.in, tc.scale, got, tc.want)
		}
	}
	if got := FromBigInt(big.NewInt(-1), 1).Format(0); got != "0" {
		t.Errorf("rounded negative zero = %q, want %q", got, "0")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	cases := []struct {
		in  string
		raw string
	}{
		{"40.7128", "40.7128"},
		{"-74.006", "-74.006"},
		{"1e3", "1000"},
		{"1.5e-2", "0.015"},
	}
	for _, tc := range cases {
		d := mustParse(t, tc.in)
		raw, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("Marshal(%q): %v", tc.in, err)
		}
		if string(raw) != tc.raw {
			t.Errorf("Marshal(%q) = %s, want %s", tc.in, raw, tc.raw)
		}
		var back Decimal
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("Unmarshal(%s): %v", raw, err)
		}
		if back.coeff.Cmp(d.coeff) != 0 || back.scale != d.scale {
			t.Errorf("round trip %q = %+v, want %+v", tc.in, back, d)
		}
	}

	d := mustParse(t, "40.7128")
	d.ScaleTo(7)
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal(scale 7): %v", err)
	}
	if string(raw) != "40.7128000" {
		t.Errorf("Marshal(40.7128@7) = %s, want %s", raw, "40.7128000")
	}
}

func TestCmpInt(t *testing.T) {
	cases := []struct {
		in   string
		v    int64
		want int
	}{
		{"90", 90, 0},
		{"-90", -90, 0},
		{"89.9", 90, -1},
		{"90.1", 90, 1},
		{"180", 180, 0},
		{"-180", -180, 0},
	}
	for _, tc := range cases {
		if got := mustParse(t, tc.in).CmpInt(tc.v); got != tc.want {
			t.Errorf("CmpInt(%q, %d) = %d, want %d", tc.in, tc.v, got, tc.want)
		}
	}
}

func TestFloat64(t *testing.T) {
	if got := mustParse(t, "40.7128").Float64(); math.Abs(got-40.7128) > 1e-9 {
		t.Errorf("Float64(40.7128) = %v, want ~40.7128", got)
	}
	if got := mustParse(t, "0").Float64(); got != 0 {
		t.Errorf("Float64(0) = %v, want 0", got)
	}
	if got := mustParse(t, "-12.5").Float64(); got != -12.5 {
		t.Errorf("Float64(-12.5) = %v, want -12.5", got)
	}
}
