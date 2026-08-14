// Package decimal implements exact decimal arithmetic on top of arbitrary
// precision integers.
package decimal

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Decimal is an exact decimal number stored as an unscaled integer
// coefficient multiplied by ten to the power of negative scale. The zero value
// represents zero.
type Decimal struct {
	coeff *big.Int
	scale int32
}

// Parse parses b as a decimal number literal and returns its exact value. The
// literal may carry an optional minus sign, integer and fraction digits, and
// an optional e or E exponent. Empty and malformed input is rejected.
func Parse(b []byte) (*Decimal, error) {
	s := string(b)
	if s == "" {
		return nil, fmt.Errorf("decimal: empty number")
	}
	i := 0
	neg := false
	if s[i] == '-' {
		neg = true
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return nil, fmt.Errorf("decimal: invalid number %q", s)
	}
	intPart := s[start:i]
	fracPart := ""
	if i < len(s) && s[i] == '.' {
		i++
		fs := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == fs {
			return nil, fmt.Errorf("decimal: invalid number %q", s)
		}
		fracPart = s[fs:i]
	}
	exp := int32(0)
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		expNeg := false
		if i < len(s) && (s[i] == '-' || s[i] == '+') {
			expNeg = s[i] == '-'
			i++
		}
		es := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == es {
			return nil, fmt.Errorf("decimal: invalid number %q", s)
		}
		n, err := strconv.Atoi(s[es:i])
		if err != nil {
			return nil, fmt.Errorf("decimal: invalid number %q", s)
		}
		if expNeg {
			n = -n
		}
		exp = int32(n)
	}
	if i != len(s) {
		return nil, fmt.Errorf("decimal: invalid number %q", s)
	}
	coeff, ok := new(big.Int).SetString(intPart+fracPart, 10)
	if !ok {
		return nil, fmt.Errorf("decimal: invalid number %q", s)
	}
	if neg {
		coeff.Neg(coeff)
	}
	d := &Decimal{coeff: coeff, scale: int32(len(fracPart)) - exp}
	if d.scale < 0 {
		d.coeff.Mul(d.coeff, tenTo(-d.scale))
		d.scale = 0
	}
	return d, nil
}

// FromBigInt returns a Decimal whose coefficient is a copy of coeff at the
// given scale.
func FromBigInt(coeff *big.Int, scale int32) Decimal {
	return Decimal{coeff: new(big.Int).Set(coeff), scale: scale}
}

// Scale returns the number of fractional digits.
func (d Decimal) Scale() int32 { return d.scale }

// Float64 returns the closest float64 approximation of the value.
func (d Decimal) Float64() float64 {
	num := new(big.Float).SetPrec(256).SetInt(d.coefficient())
	if d.scale > 0 {
		den := new(big.Float).SetPrec(256).SetInt(tenTo(d.scale))
		num.Quo(num, den)
	}
	f, _ := num.Float64()
	return f
}

// CmpInt compares d with v, returning -1, 0, or +1 when d is less than, equal
// to, or greater than v.
func (d Decimal) CmpInt(v int64) int {
	c := d.coefficient()
	other := new(big.Int).SetInt64(v)
	if d.scale > 0 {
		other.Mul(other, tenTo(d.scale))
	} else if d.scale < 0 {
		c = new(big.Int).Mul(c, tenTo(-d.scale))
	}
	return c.Cmp(other)
}

// Unscaled returns a copy of the unscaled coefficient.
func (d Decimal) Unscaled() *big.Int {
	return new(big.Int).Set(d.coefficient())
}

// ScaleTo rounds d to the given number of fractional digits using
// round-half-away-from-zero, matching the rounding PostgreSQL numeric applies.
func (d *Decimal) ScaleTo(scale int32) {
	d.coeff = scaledCoeff(d.coefficient(), d.scale, scale)
	d.scale = scale
}

// Format renders d as a fixed-point string with exactly scale fractional
// digits, rounding half away from zero when the value carries more digits.
// The result never uses scientific notation and never carries a minus sign for
// a zero value.
func (d Decimal) Format(scale int32) string {
	return formatFixed(scaledCoeff(d.coefficient(), d.scale, scale), scale)
}

// MarshalJSON renders d as a bare JSON number at its own scale.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return []byte(d.Format(d.scale)), nil
}

// UnmarshalJSON parses a JSON number literal into d.
func (d *Decimal) UnmarshalJSON(b []byte) error {
	v, err := Parse(b)
	if err != nil {
		return err
	}
	*d = *v
	return nil
}

// unscaledAt returns a fresh big.Int holding the coefficient scaled to scale
// fractional digits, rounding half away from zero when needed.
func (d Decimal) unscaledAt(scale int32) *big.Int {
	return scaledCoeff(d.coefficient(), d.scale, scale)
}

// coefficient returns the coefficient, substituting zero when d is the zero
// value.
func (d Decimal) coefficient() *big.Int {
	if d.coeff == nil {
		return new(big.Int)
	}
	return d.coeff
}

// tenTo returns 10 raised to the n-th power.
func tenTo(n int32) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// scaledCoeff returns a fresh big.Int holding a copy of c scaled from
// fromScale to toScale fractional digits, rounding half away from zero when
// digits are dropped.
func scaledCoeff(c *big.Int, fromScale, toScale int32) *big.Int {
	if fromScale == toScale {
		return new(big.Int).Set(c)
	}
	if toScale > fromScale {
		return new(big.Int).Mul(c, tenTo(toScale-fromScale))
	}
	divisor := tenTo(fromScale - toScale)
	quo, rem := new(big.Int).QuoRem(c, divisor, new(big.Int))
	if rem.Sign() != 0 {
		rem.Abs(rem)
		rem.Mul(rem, big.NewInt(2))
		if rem.Cmp(divisor) >= 0 {
			if c.Sign() >= 0 {
				quo.Add(quo, big.NewInt(1))
			} else {
				quo.Sub(quo, big.NewInt(1))
			}
		}
	}
	return quo
}

// formatFixed renders coeff as a fixed-point string with exactly scale
// fractional digits, without scientific notation or a minus sign for zero.
func formatFixed(coeff *big.Int, scale int32) string {
	neg := coeff.Sign() < 0
	digits := new(big.Int).Abs(coeff).String()
	if int32(len(digits)) <= scale {
		pad := strings.Repeat("0", int(scale)-len(digits))
		s := "0." + pad + digits
		if neg {
			return "-" + s
		}
		return s
	}
	cut := len(digits) - int(scale)
	intPart := digits[:cut]
	if scale == 0 {
		if neg {
			return "-" + intPart
		}
		return intPart
	}
	s := intPart + "." + digits[cut:]
	if neg {
		return "-" + s
	}
	return s
}
