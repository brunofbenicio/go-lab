// Package order models a customer order with line items, a coupon and a
// shipping address, and validates it. Objects like this have many fields,
// which makes constructing them in tests noisy: the subject for test builders.
package order

import (
	"errors"
	"fmt"
	"strings"
)

// Address is where an order ships to.
type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	Country string `json:"country"` // ISO 3166-1 alpha-2, e.g. "BR"
}

// Line is one product on the order. Prices are in cents.
type Line struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
	Price    int64  `json:"price"`
}

// Order is a purchase request.
type Order struct {
	ID       string  `json:"id"`
	Customer string  `json:"customer"` // e-mail address
	Lines    []Line  `json:"lines"`
	Coupon   string  `json:"coupon,omitempty"`
	Ship     Address `json:"ship"`
}

// Validation errors, joined into the error returned by Validate.
var (
	ErrNoID        = errors.New("order: missing id")
	ErrBadCustomer = errors.New("order: customer must be an e-mail address")
	ErrNoLines     = errors.New("order: no lines")
	ErrBadLine     = errors.New("order: invalid line")
	ErrBadCountry  = errors.New("order: country must be a 2-letter code")
)

// Validate reports every problem it finds, not only the first one.
func (o Order) Validate() error {
	var errs []error
	if o.ID == "" {
		errs = append(errs, ErrNoID)
	}
	if !strings.Contains(o.Customer, "@") {
		errs = append(errs, ErrBadCustomer)
	}
	if len(o.Lines) == 0 {
		errs = append(errs, ErrNoLines)
	}
	for i, l := range o.Lines {
		if l.SKU == "" || l.Quantity <= 0 || l.Price < 0 {
			errs = append(errs, fmt.Errorf("%w: line %d", ErrBadLine, i))
		}
	}
	if len(o.Ship.Country) != 2 {
		errs = append(errs, ErrBadCountry)
	}
	return errors.Join(errs...)
}

// Total returns the amount due in cents. The coupon "SAVE10" takes 10% off.
func (o Order) Total() int64 {
	var sum int64
	for _, l := range o.Lines {
		sum += int64(l.Quantity) * l.Price
	}
	if o.Coupon == "SAVE10" {
		sum -= sum / 10
	}
	return sum
}
