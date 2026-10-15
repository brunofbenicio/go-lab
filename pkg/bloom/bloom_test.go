package bloom

import (
	"fmt"
	"testing"
)

func TestNoFalseNegatives(t *testing.T) {
	f := New(1000, 0.01)
	for i := 0; i < 1000; i++ {
		f.AddString(fmt.Sprintf("member-%d", i))
	}
	for i := 0; i < 1000; i++ {
		if key := fmt.Sprintf("member-%d", i); !f.MayContainString(key) {
			t.Fatalf("false negative for %q", key)
		}
	}
}

func TestFalsePositiveRateIsNearTarget(t *testing.T) {
	const members, probes = 1000, 20000
	f := New(members, 0.01)
	for i := 0; i < members; i++ {
		f.Add([]byte(fmt.Sprintf("member-%d", i)))
	}
	falsePositives := 0
	for i := 0; i < probes; i++ {
		if f.MayContain([]byte(fmt.Sprintf("stranger-%d", i))) {
			falsePositives++
		}
	}
	// Target is 1%; allow a generous margin so the check is robust.
	if rate := float64(falsePositives) / probes; rate > 0.03 {
		t.Errorf("false positive rate = %.2f%%, want <= 3%%", rate*100)
	}
}

func TestSizing(t *testing.T) {
	tests := []struct {
		name       string
		expected   int
		fp         float64
		wantBits   uint64
		wantHashes uint
	}{
		{"1000 items at 1%", 1000, 0.01, 9586, 7},
		{"100 items at 10%", 100, 0.1, 480, 3},
		{"zero expected is clamped", 0, 0.01, 10, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New(tt.expected, tt.fp)
			if f.Bits() != tt.wantBits || f.Hashes() != tt.wantHashes {
				t.Errorf("Bits/Hashes = %d/%d, want %d/%d", f.Bits(), f.Hashes(), tt.wantBits, tt.wantHashes)
			}
		})
	}
}

func TestEmptyFilterContainsNothing(t *testing.T) {
	f := New(100, 0.01)
	for _, s := range []string{"", "a", "hello"} {
		if f.MayContainString(s) {
			t.Errorf("empty filter reported %q as present", s)
		}
	}
	f.AddString("")
	if !f.MayContainString("") {
		t.Error("the empty string was added and must be found")
	}
}

func TestNewWithParamsClampsToOne(t *testing.T) {
	f := NewWithParams(0, 0)
	if f.Bits() != 1 || f.Hashes() != 1 {
		t.Fatalf("Bits/Hashes = %d/%d, want 1/1", f.Bits(), f.Hashes())
	}
	f.AddString("x")
	if !f.MayContainString("x") {
		t.Error("a 1-bit filter must still have no false negatives")
	}
}

func TestNewPanicsOnInvalidRate(t *testing.T) {
	for _, fp := range []float64{0, 1, -0.5, 2} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New(10, %v) should panic", fp)
				}
			}()
			New(10, fp)
		}()
	}
}
