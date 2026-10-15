// Package cart implements a small shopping cart. It exists to give the tests
// something with several related operations, which is where subtests shine.
package cart

import (
	"errors"
	"sort"
)

// Errors returned by Cart operations.
var (
	ErrInvalidQuantity = errors.New("cart: quantity must be positive")
	ErrUnknownCoupon   = errors.New("cart: unknown coupon")
)

// coupons maps a coupon code to its percentage discount.
var coupons = map[string]int64{"OFF10": 10, "OFF50": 50}

// Item is a line in the cart. Prices are in cents.
type Item struct {
	SKU      string
	Price    int64
	Quantity int
}

// Cart holds items and an optional coupon. The zero value is not usable; call New.
type Cart struct {
	items    map[string]Item
	discount int64 // percentage, 0-100
}

// New returns an empty cart.
func New() *Cart {
	return &Cart{items: make(map[string]Item)}
}

// Add puts qty units of sku in the cart, merging with an existing line.
func (c *Cart) Add(sku string, price int64, qty int) error {
	if qty <= 0 {
		return ErrInvalidQuantity
	}
	it := c.items[sku]
	it.SKU, it.Price = sku, price
	it.Quantity += qty
	c.items[sku] = it
	return nil
}

// Remove deletes the line for sku and reports whether it existed.
func (c *Cart) Remove(sku string) bool {
	_, ok := c.items[sku]
	delete(c.items, sku)
	return ok
}

// ApplyCoupon sets the discount for the cart; the last coupon wins.
func (c *Cart) ApplyCoupon(code string) error {
	pct, ok := coupons[code]
	if !ok {
		return ErrUnknownCoupon
	}
	c.discount = pct
	return nil
}

// Items returns the lines sorted by SKU.
func (c *Cart) Items() []Item {
	out := make([]Item, 0, len(c.items))
	for _, it := range c.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SKU < out[j].SKU })
	return out
}

// Total returns the amount due in cents, after the coupon discount.
func (c *Cart) Total() int64 {
	var sum int64
	for _, it := range c.items {
		sum += it.Price * int64(it.Quantity)
	}
	return sum - sum*c.discount/100
}
