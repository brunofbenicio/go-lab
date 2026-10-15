package money

import (
	"errors"
	"math"
	"testing"
)

func TestParseAndString(t *testing.T) {
	tests := []struct {
		in        string
		wantCents int64
		wantStr   string
	}{
		{"12.34", 1234, "12.34 BRL"},
		{"10", 1000, "10.00 BRL"},
		{"0.5", 50, "0.50 BRL"},
		{"0.05", 5, "0.05 BRL"},
		{"0", 0, "0.00 BRL"},
		{"-0.05", -5, "-0.05 BRL"},
		{"-1234567.89", -123456789, "-1234567.89 BRL"},
		{"92233720368547758.07", math.MaxInt64, "92233720368547758.07 BRL"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			m, err := Parse(tt.in, "BRL")
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if m.Cents() != tt.wantCents || m.String() != tt.wantStr || m.Currency() != "BRL" {
				t.Errorf("got %d / %q, want %d / %q", m.Cents(), m, tt.wantCents, tt.wantStr)
			}
		})
	}
	if got := New(math.MinInt64, "BRL").String(); got != "-92233720368547758.08 BRL" {
		t.Errorf("String of MinInt64 cents = %q", got)
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"", "-", ".", "1.", ".5", "1.234", "abc", "1,5", "1.2.3", "--1", "+1", "1e3", "92233720368547758.08"} {
		if _, err := Parse(in, "BRL"); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("Parse(%q) error = %v, want ErrInvalidAmount", in, err)
		}
	}
}

func TestArithmetic(t *testing.T) {
	brl := func(c int64) Money { return New(c, "BRL") }
	tests := []struct {
		name    string
		op      func() (Money, error)
		want    int64
		wantErr error
	}{
		{"add", func() (Money, error) { return brl(150).Add(brl(275)) }, 425, nil},
		{"add negative", func() (Money, error) { return brl(100).Add(brl(-250)) }, -150, nil},
		{"sub", func() (Money, error) { return brl(500).Sub(brl(125)) }, 375, nil},
		{"mul", func() (Money, error) { return brl(333).Mul(3) }, 999, nil},
		{"mul by zero", func() (Money, error) { return brl(333).Mul(0) }, 0, nil},
		{"mul negative", func() (Money, error) { return brl(100).Mul(-2) }, -200, nil},
		{"add currency mismatch", func() (Money, error) { return brl(1).Add(New(1, "USD")) }, 0, ErrCurrencyMismatch},
		{"sub currency mismatch", func() (Money, error) { return brl(1).Sub(New(1, "USD")) }, 0, ErrCurrencyMismatch},
		{"add overflow", func() (Money, error) { return brl(math.MaxInt64).Add(brl(1)) }, 0, ErrOverflow},
		{"add underflow", func() (Money, error) { return brl(math.MinInt64).Add(brl(-1)) }, 0, ErrOverflow},
		{"sub overflow", func() (Money, error) { return brl(math.MaxInt64).Sub(brl(-1)) }, 0, ErrOverflow},
		{"sub underflow", func() (Money, error) { return brl(math.MinInt64).Sub(brl(1)) }, 0, ErrOverflow},
		{"mul overflow", func() (Money, error) { return brl(math.MaxInt64).Mul(2) }, 0, ErrOverflow},
		{"mul min by -1", func() (Money, error) { return brl(math.MinInt64).Mul(-1) }, 0, ErrOverflow},
		{"mul -1 by min", func() (Money, error) { return brl(-1).Mul(math.MinInt64) }, 0, ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.op()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got.Cents() != tt.want {
				t.Errorf("cents = %d, want %d", got.Cents(), tt.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	a, b := New(100, "BRL"), New(200, "BRL")
	for _, tc := range []struct {
		x, y Money
		want int
	}{{a, b, -1}, {b, a, 1}, {a, a, 0}} {
		if got, err := tc.x.Compare(tc.y); err != nil || got != tc.want {
			t.Errorf("Compare = %d, %v; want %d", got, err, tc.want)
		}
	}
	if _, err := a.Compare(New(100, "USD")); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Compare across currencies: err = %v", err)
	}
}

func TestAllocate(t *testing.T) {
	tests := []struct {
		name   string
		cents  int64
		ratios []int
		want   []int64
	}{
		{"three equal parts", 100, []int{1, 1, 1}, []int64{34, 33, 33}},
		{"proportional", 100, []int{70, 30}, []int64{70, 30}},
		{"odd cent goes to the first", 5, []int{1, 1}, []int64{3, 2}},
		{"negative amount", -100, []int{1, 1, 1}, []int64{-34, -33, -33}},
		{"zero ratio gets nothing", 10, []int{0, 1, 1}, []int64{0, 5, 5}},
		{"fewer cents than parts", 1, []int{1, 1, 1}, []int64{1, 0, 0}},
		{"zero amount", 0, []int{2, 3}, []int64{0, 0}},
		{"huge amount does not overflow", math.MaxInt64, []int{1, 1}, []int64{math.MaxInt64/2 + 1, math.MaxInt64 / 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, err := New(tt.cents, "BRL").Allocate(tt.ratios...)
			if err != nil {
				t.Fatalf("Allocate: %v", err)
			}
			if len(parts) != len(tt.want) {
				t.Fatalf("got %d parts, want %d", len(parts), len(tt.want))
			}
			var sum int64
			for j, p := range parts {
				sum += p.Cents()
				if p.Cents() != tt.want[j] {
					t.Errorf("part %d = %d, want %d", j, p.Cents(), tt.want[j])
				}
			}
			if sum != tt.cents {
				t.Errorf("parts add up to %d, want exactly %d", sum, tt.cents)
			}
		})
	}
}

func TestAllocateNeverLosesACent(t *testing.T) {
	ratioSets := [][]int{{1, 1, 1}, {1, 2, 3}, {7, 0, 13, 5}, {1, 1, 1, 1, 1, 1, 1}}
	for cents := int64(-250); cents <= 250; cents++ {
		for _, ratios := range ratioSets {
			parts, err := New(cents, "BRL").Allocate(ratios...)
			if err != nil {
				t.Fatal(err)
			}
			var sum int64
			for _, p := range parts {
				sum += p.Cents()
			}
			if sum != cents {
				t.Fatalf("Allocate(%d, %v) adds up to %d", cents, ratios, sum)
			}
		}
	}
}

func TestAllocateInvalidRatios(t *testing.T) {
	for _, ratios := range [][]int{nil, {0, 0}, {1, -1}, {math.MaxInt, math.MaxInt, math.MaxInt}} {
		if _, err := New(100, "BRL").Allocate(ratios...); !errors.Is(err, ErrInvalidRatios) {
			t.Errorf("Allocate(%v) error = %v, want ErrInvalidRatios", ratios, err)
		}
	}
}
