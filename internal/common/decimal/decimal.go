// Package decimal provides fixed-precision decimal arithmetic for financial calculations.
// Uses int64 with 18 decimal places to avoid floating-point rounding errors.
package decimal

import (
	"database/sql/driver"
	"fmt"
	"math/big"
	"strings"
)

const Precision = 18

// Decimal represents a fixed-point decimal number with 18 decimal places.
// Zero value is 0.
type Decimal struct {
	value *big.Int // unscaled integer value
}

var (
	Zero      = NewFromInt64(0)
	One       = NewFromInt64(1)
	bigTen    = big.NewInt(10)
	tenPow18  = new(big.Int).Exp(bigTen, big.NewInt(Precision), nil)
)

// NewFromInt64 creates a Decimal from an int64 (e.g., NewFromInt64(1) = 1.0).
func NewFromInt64(n int64) Decimal {
	v := new(big.Int).Mul(big.NewInt(n), tenPow18)
	return Decimal{value: v}
}

// NewFromString parses a decimal string like "123.456".
func NewFromString(s string) (Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Zero, nil
	}

	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}

	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return Decimal{}, fmt.Errorf("invalid decimal: %s", s)
	}
	intPart := parts[0]

	v := new(big.Int)
	v, ok := v.SetString(intPart, 10)
	if !ok {
		return Decimal{}, fmt.Errorf("invalid decimal: %s", s)
	}
	v.Mul(v, tenPow18)

	if len(parts) > 1 {
		fracPart := parts[1]
		if len(fracPart) > Precision {
			// Round the extra digit instead of truncating
			extra := fracPart[Precision]
			fracPart = fracPart[:Precision]
			if extra >= '5' {
				// Round up: increment the last digit
				fracBytes := []byte(fracPart)
				for i := len(fracBytes) - 1; i >= 0; i-- {
					if fracBytes[i] < '9' {
						fracBytes[i]++
						fracPart = string(fracBytes)
						break
					}
					fracBytes[i] = '0'
				}
			}
		}
		fracVal := new(big.Int)
		fracVal, ok = fracVal.SetString(fracPart, 10)
		if !ok {
			return Decimal{}, fmt.Errorf("invalid decimal: %s", s)
		}
		scale := Precision - len(fracPart)
		if scale > 0 {
			multiplier := new(big.Int).Exp(bigTen, big.NewInt(int64(scale)), nil)
			fracVal.Mul(fracVal, multiplier)
		}
		v.Add(v, fracVal)
	}

	if neg {
		v.Neg(v)
	}

	return Decimal{value: v}, nil
}

// val safely returns the underlying big.Int, defaulting to zero.
func (d Decimal) val() *big.Int {
	if d.value == nil {
		return new(big.Int)
	}
	return d.value
}

// Add returns d + o.
func (d Decimal) Add(o Decimal) Decimal {
	v := new(big.Int).Add(d.val(), o.val())
	return Decimal{value: v}
}

// Sub returns d - o.
func (d Decimal) Sub(o Decimal) Decimal {
	v := new(big.Int).Sub(d.val(), o.val())
	return Decimal{value: v}
}

// Mul returns d * o (with precision truncation).
func (d Decimal) Mul(o Decimal) Decimal {
	v := new(big.Int).Mul(d.val(), o.val())
	v.Div(v, tenPow18)
	return Decimal{value: v}
}

// Div returns d / o. Returns Zero if o is zero.
func (d Decimal) Div(o Decimal) Decimal {
	if o.IsZero() {
		return Zero
	}
	v := new(big.Int).Mul(d.val(), tenPow18)
	v.Div(v, o.val())
	return Decimal{value: v}
}

// Mod returns d % o.
func (d Decimal) Mod(o Decimal) Decimal {
	if o.IsZero() {
		return Zero
	}
	v := new(big.Int).Mod(d.val(), o.val())
	return Decimal{value: v}
}

// Cmp compares d and o: -1 if d < o, 0 if d == o, +1 if d > o.
func (d Decimal) Cmp(o Decimal) int {
	return d.val().Cmp(o.val())
}

// IsZero returns true if d == 0.
func (d Decimal) IsZero() bool {
	return d.val().Sign() == 0
}

// IsNegative returns true if d < 0.
func (d Decimal) IsNegative() bool {
	return d.val().Sign() < 0
}

// Abs returns the absolute value.
func (d Decimal) Abs() Decimal {
	if d.IsNegative() {
		v := new(big.Int).Neg(d.val())
		return Decimal{value: v}
	}
	return d
}

// Min returns the smaller of d and o.
func (d Decimal) Min(o Decimal) Decimal {
	if d.Cmp(o) <= 0 {
		return d
	}
	return o
}

// Max returns the larger of d and o.
func (d Decimal) Max(o Decimal) Decimal {
	if d.Cmp(o) >= 0 {
		return d
	}
	return o
}

// String returns the decimal string representation.
func (d Decimal) String() string {
	v := new(big.Int).Abs(d.val())
	intPart := new(big.Int).Div(v, tenPow18)
	fracPart := new(big.Int).Mod(v, tenPow18)

	fracStr := fmt.Sprintf("%018d", fracPart)
	fracStr = strings.TrimRight(fracStr, "0")
	if fracStr == "" {
		if d.IsNegative() && intPart.Sign() != 0 {
			return "-" + intPart.String()
		}
		return intPart.String()
	}

	result := intPart.String() + "." + fracStr
	if d.IsNegative() {
		result = "-" + result
	}
	return result
}

// Float64 returns the float64 approximation. Use only for display.
func (d Decimal) Float64() float64 {
	if d.value == nil {
		return 0
	}
	f := new(big.Float).SetInt(d.value)
	f.Quo(f, new(big.Float).SetInt(tenPow18))
	v, _ := f.Float64()
	return v
}

// Scan implements sql.Scanner for database reading.
func (d *Decimal) Scan(src interface{}) error {
	if src == nil {
		d.value = new(big.Int)
		return nil
	}
	switch v := src.(type) {
	case []byte:
		parsed, err := NewFromString(string(v))
		if err != nil {
			return err
		}
		*d = parsed
	case string:
		parsed, err := NewFromString(v)
		if err != nil {
			return err
		}
		*d = parsed
	case int64:
		*d = NewFromInt64(v)
	case float64:
		dd, err := NewFromString(fmt.Sprintf("%.18f", v))
		if err != nil {
			return err
		}
		*d = dd
	default:
		return fmt.Errorf("unsupported Scan type: %T", src)
	}
	return nil
}

// Value implements driver.Valuer for database writing.
func (d Decimal) Value() (driver.Value, error) {
	if d.value == nil {
		return "0", nil
	}
	return d.String(), nil
}
