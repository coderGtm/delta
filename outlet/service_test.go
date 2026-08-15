package outlet

import (
	"encoding/json"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/coderGtm/delta/decimal"
	"github.com/coderGtm/delta/httpapi"
	"github.com/jackc/pgx/v5/pgtype"
)

func dec(t *testing.T, s string) *decimal.Decimal {
	t.Helper()
	d, err := decimal.Parse([]byte(s))
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return d
}

func intPtr(v int) *FlexInt { return (*FlexInt)(&v) }

func TestValidateOutlet(t *testing.T) {
	if err := validateOutlet("HQ", dec(t, "10.5"), dec(t, "20.25"), intPtr(50)); err != nil {
		t.Fatalf("valid outlet rejected: %v", err)
	}

	err := validateOutlet("", nil, nil, nil)
	ae := err
	want := "Outlet name is required, Latitude is required, Longitude is required, Radius in meters is required"
	if ae.Code != "VALIDATION_ERROR" || ae.Message != want {
		t.Errorf("got %q %q, want VALIDATION_ERROR %q", ae.Code, ae.Message, want)
	}

	err = validateOutlet("x", dec(t, "200"), dec(t, "-200"), intPtr(-5))
	ae = err
	want = "Latitude must be less than or equal to 90, Longitude must be greater than or equal to -180, Radius in meters must be greater than zero"
	if ae.Code != "VALIDATION_ERROR" || ae.Message != want {
		t.Errorf("got %q %q, want VALIDATION_ERROR %q", ae.Code, ae.Message, want)
	}

	long := strings.Repeat("a", 151)
	err = validateOutlet(long, dec(t, "10"), dec(t, "20"), intPtr(5))
	ae = err
	if ae.Code != "VALIDATION_ERROR" || ae.Message != "Outlet name must be at most 150 characters" {
		t.Errorf("got %q %q, want VALIDATION_ERROR %q", ae.Code, ae.Message, "Outlet name must be at most 150 characters")
	}

	if err := validateOutlet(strings.Repeat("خ", 150), dec(t, "10"), dec(t, "20"), intPtr(5)); err != nil {
		t.Errorf("150-rune (300-byte) name rejected: %v", err)
	}

	err = validateOutlet(strings.Repeat("خ", 151), dec(t, "10"), dec(t, "20"), intPtr(5))
	ae = err
	if ae.Code != "VALIDATION_ERROR" || ae.Message != "Outlet name must be at most 150 characters" {
		t.Errorf("got %q %q, want VALIDATION_ERROR %q", ae.Code, ae.Message, "Outlet name must be at most 150 characters")
	}

	if err := validateOutlet("HQ", dec(t, "10"), dec(t, "20"), intPtr(math.MaxInt32)); err != nil {
		t.Errorf("radius = MaxInt32 rejected: %v", err)
	}

	err = validateOutlet("HQ", dec(t, "10"), dec(t, "20"), intPtr(math.MaxInt32+1))
	ae = err
	if ae.Code != "VALIDATION_ERROR" || ae.Message != "Radius in meters must be less than or equal to 2147483647" {
		t.Errorf("got %q %q, want VALIDATION_ERROR %q", ae.Code, ae.Message, "Radius in meters must be less than or equal to 2147483647")
	}
}

func TestAssertOwnerRole(t *testing.T) {
	if err := assertOwnerRole("OWNER"); err != nil {
		t.Fatalf("OWNER rejected: %v", err)
	}
	err := assertOwnerRole("EMPLOYEE")
	ae, ok := err.(*httpapi.APIError)
	if !ok || ae.Code != "FORBIDDEN" || ae.Message != "Only outlet owners can perform this action" {
		t.Errorf("got %v, want FORBIDDEN with owner message", err)
	}
}

func TestPgNumericFromDecimal(t *testing.T) {
	n := pgNumericFromDecimal(*dec(t, "40.7128"))
	if !n.Valid || n.Exp != -7 {
		t.Errorf("numeric = %+v, want Valid with Exp -7", n)
	}
	if n.Int.Cmp(big.NewInt(407128000)) != 0 {
		t.Errorf("numeric Int = %s, want 407128000", n.Int)
	}
}

func TestDecimalFromPgNumeric(t *testing.T) {
	cases := []struct {
		n    pgtype.Numeric
		want string
	}{
		{pgtype.Numeric{Int: big.NewInt(407128000), Exp: -7, Valid: true}, "40.7128000"},
		{pgtype.Numeric{Int: big.NewInt(407128), Exp: -4, Valid: true}, "40.7128000"},
		{pgtype.Numeric{Int: big.NewInt(-740060000), Exp: -7, Valid: true}, "-74.0060000"},
		{pgtype.Numeric{Int: big.NewInt(0), Exp: 0, Valid: true}, "0.0000000"},
	}
	for _, tc := range cases {
		d, err := decimalFromPgNumeric(tc.n)
		if err != nil {
			t.Fatalf("decimalFromPgNumeric(%+v): %v", tc.n, err)
		}
		if got := d.Format(7); got != tc.want {
			t.Errorf("decimalFromPgNumeric(%+v) = %s, want %s", tc.n, got, tc.want)
		}
	}

	invalid := []pgtype.Numeric{
		{},
		{Int: big.NewInt(1), NaN: true, Valid: true},
		{Int: big.NewInt(1), InfinityModifier: pgtype.Infinity, Valid: true},
	}
	for _, n := range invalid {
		if _, err := decimalFromPgNumeric(n); err == nil {
			t.Errorf("decimalFromPgNumeric(%+v) succeeded, want error", n)
		}
	}
}

func TestFlexIntUnmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want FlexInt
	}{
		{`500`, 500},
		{`500.0`, 500},
		{`500.75`, 500},
		{`"500"`, 500},
		{`"500.0"`, 500},
		{`-5`, -5},
	}
	for _, tc := range cases {
		var f FlexInt
		if err := json.Unmarshal([]byte(tc.in), &f); err != nil {
			t.Fatalf("Unmarshal(%s): %v", tc.in, err)
		}
		if f != tc.want {
			t.Errorf("Unmarshal(%s) = %d, want %d", tc.in, f, tc.want)
		}
	}
	if err := json.Unmarshal([]byte(`"abc"`), new(FlexInt)); err == nil {
		t.Fatal("expected error for non-numeric string")
	}
}

func TestCreateOutletRequestLenientDecode(t *testing.T) {
	// The mobile client was built against a JSON decoder that coerces
	// fractional integers and quoted numbers; both must be accepted.
	payload := `{"name":"HQ","latitude":"40.7128","longitude":"-74.006","radiusMeters":500.0}`
	var req CreateOutletRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ve := validateOutlet(req.Name, req.Latitude, req.Longitude, req.RadiusMeters); ve != nil {
		t.Fatalf("valid outlet rejected: %v", ve)
	}
	if req.RadiusMeters == nil || *req.RadiusMeters != 500 {
		t.Errorf("radius = %v, want 500", req.RadiusMeters)
	}
}
