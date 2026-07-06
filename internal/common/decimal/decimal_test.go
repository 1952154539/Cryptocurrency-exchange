package decimal

import (
	"testing"
)

func TestNewFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0", "0"},
		{"1", "1"},
		{"1.5", "1.5"},
		{"0.0001", "0.0001"},
		{"123.456789", "123.456789"},
		{"-1.5", "-1.5"},
		{"1000000.000000000001", "1000000.000000000001"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d, err := NewFromString(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, d.String())
			}
		})
	}
}

func TestAdd(t *testing.T) {
	a := mustDecimal("1.5")
	b := mustDecimal("2.3")
	c := a.Add(b)
	if c.String() != "3.8" {
		t.Errorf("1.5 + 2.3 = 3.8, got %s", c.String())
	}
}

func TestSub(t *testing.T) {
	a := mustDecimal("5.0")
	b := mustDecimal("2.5")
	c := a.Sub(b)
	if c.String() != "2.5" {
		t.Errorf("5.0 - 2.5 = 2.5, got %s", c.String())
	}
}

func TestMul(t *testing.T) {
	a := mustDecimal("2.0")
	b := mustDecimal("3.0")
	c := a.Mul(b)
	if c.String() != "6" {
		t.Errorf("2.0 * 3.0 = 6, got %s", c.String())
	}
}

func TestDiv(t *testing.T) {
	a := mustDecimal("6.0")
	b := mustDecimal("3.0")
	c := a.Div(b)
	if c.String() != "2" {
		t.Errorf("6.0 / 3.0 = 2, got %s", c.String())
	}
}

func TestCmp(t *testing.T) {
	a := mustDecimal("1.0")
	b := mustDecimal("2.0")
	if a.Cmp(b) != -1 {
		t.Error("1.0 should be < 2.0")
	}
	if b.Cmp(a) != 1 {
		t.Error("2.0 should be > 1.0")
	}
	if a.Cmp(mustDecimal("1.0")) != 0 {
		t.Error("1.0 should == 1.0")
	}
}

func TestIsZero(t *testing.T) {
	if !Zero.IsZero() {
		t.Error("Zero should be zero")
	}
	if mustDecimal("0").IsZero() == false {
		t.Error("0 should be zero")
	}
	if mustDecimal("1").IsZero() {
		t.Error("1 should not be zero")
	}
}

func TestIsNegative(t *testing.T) {
	if !mustDecimal("-1").IsNegative() {
		t.Error("-1 should be negative")
	}
	if mustDecimal("1").IsNegative() {
		t.Error("1 should not be negative")
	}
}

func TestArbitraryPrecision(t *testing.T) {
	a := mustDecimal("0.000000000000000001")
	b := mustDecimal("0.000000000000000002")
	c := a.Add(b)
	if c.String() != "0.000000000000000003" {
		t.Errorf("expected 0.000000000000000003, got %s", c.String())
	}
}

func mustDecimal(s string) Decimal {
	d, err := NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}
