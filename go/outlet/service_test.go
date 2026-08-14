package outlet

import (
	"math/big"
	"strings"
	"testing"

	"github.com/coderGtm/delta/go/decimal"
	"github.com/coderGtm/delta/go/httpapi"
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

func intPtr(v int) *int { return &v }

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
	n, err := pgNumericFromDecimal(*dec(t, "40.7128"))
	if err != nil {
		t.Fatalf("pgNumericFromDecimal: %v", err)
	}
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
