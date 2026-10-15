package cart

import (
	"errors"
	"testing"
)

// newFilledCart is a fixture: every subtest gets its own fresh cart, so the
// subtests never depend on each other or on execution order.
func newFilledCart(t *testing.T) *Cart {
	t.Helper()
	c := New()
	if err := c.Add("book", 2500, 2); err != nil { // 5000
		t.Fatalf("fixture: %v", err)
	}
	if err := c.Add("pen", 150, 4); err != nil { // 600
		t.Fatalf("fixture: %v", err)
	}
	return c
}

// TestCart groups the tests by method. Each t.Run creates a named subtest that
// can be selected on its own, for example:
//
//	go test -run 'TestCart/Add/merges' -v
func TestCart(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		t.Run("creates a new line", func(t *testing.T) {
			c := newFilledCart(t)
			if err := c.Add("mug", 1200, 1); err != nil {
				t.Fatalf("Add: %v", err)
			}
			if got := len(c.Items()); got != 3 {
				t.Errorf("len(Items()) = %d, want 3", got)
			}
		})

		t.Run("merges quantity of the same SKU", func(t *testing.T) {
			c := newFilledCart(t)
			if err := c.Add("pen", 150, 6); err != nil {
				t.Fatalf("Add: %v", err)
			}
			items := c.Items()
			if len(items) != 2 || items[1].Quantity != 10 {
				t.Errorf("Items() = %+v, want pen quantity 10", items)
			}
		})

		t.Run("rejects non-positive quantity", func(t *testing.T) {
			for _, qty := range []int{0, -3} {
				// Nested subtests can also be created inside a loop.
				t.Run("", func(t *testing.T) { // "" becomes #00, #01, ...
					if err := New().Add("pen", 150, qty); !errors.Is(err, ErrInvalidQuantity) {
						t.Errorf("Add(qty=%d) error = %v, want ErrInvalidQuantity", qty, err)
					}
				})
			}
		})
	})

	t.Run("Remove", func(t *testing.T) {
		t.Run("existing SKU", func(t *testing.T) {
			c := newFilledCart(t)
			if !c.Remove("book") {
				t.Fatal("Remove(book) = false, want true")
			}
			if got := c.Total(); got != 600 {
				t.Errorf("Total() = %d, want 600", got)
			}
		})

		t.Run("missing SKU", func(t *testing.T) {
			if newFilledCart(t).Remove("ghost") {
				t.Error("Remove(ghost) = true, want false")
			}
		})
	})

	t.Run("Total", func(t *testing.T) {
		t.Run("sums every line", func(t *testing.T) {
			if got := newFilledCart(t).Total(); got != 5600 {
				t.Errorf("Total() = %d, want 5600", got)
			}
		})

		t.Run("with coupon", func(t *testing.T) {
			c := newFilledCart(t)
			if err := c.ApplyCoupon("OFF50"); err != nil {
				t.Fatalf("ApplyCoupon: %v", err) // aborts only this subtest
			}
			if got := c.Total(); got != 2800 {
				t.Errorf("Total() = %d, want 2800", got)
			}
		})

		t.Run("unknown coupon leaves the total untouched", func(t *testing.T) {
			c := newFilledCart(t)
			if err := c.ApplyCoupon("NOPE"); !errors.Is(err, ErrUnknownCoupon) {
				t.Fatalf("ApplyCoupon error = %v, want ErrUnknownCoupon", err)
			}
			if got := c.Total(); got != 5600 {
				t.Errorf("Total() = %d, want 5600", got)
			}
		})
	})
}
