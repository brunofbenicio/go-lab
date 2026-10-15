package order

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// orderBuilder creates Orders for tests. anOrder() returns a complete, VALID
// order, so each test states only what makes its scenario different:
//
//	anOrder().withoutLines().build()   // "an order, but without lines"
//
// Every with... method returns a modified copy, so a builder can be shared and
// specialized without tests interfering with each other.
type orderBuilder struct{ o Order }

func anOrder() orderBuilder {
	return orderBuilder{o: Order{
		ID:       "ord-1",
		Customer: "buyer@example.com",
		Lines:    []Line{{SKU: "book", Quantity: 2, Price: 2500}},
		Ship:     Address{Street: "1 Main St", City: "Springfield", Country: "BR"},
	}}
}

func (b orderBuilder) withID(id string) orderBuilder      { b.o.ID = id; return b }
func (b orderBuilder) withCustomer(c string) orderBuilder { b.o.Customer = c; return b }
func (b orderBuilder) withCoupon(c string) orderBuilder   { b.o.Coupon = c; return b }
func (b orderBuilder) withCountry(c string) orderBuilder  { b.o.Ship.Country = c; return b }
func (b orderBuilder) withoutLines() orderBuilder         { b.o.Lines = nil; return b }
func (b orderBuilder) withLine(sku string, qty int, price int64) orderBuilder {
	// Clone before appending: the copy must not share the slice's backing array.
	b.o.Lines = append(slices.Clone(b.o.Lines), Line{SKU: sku, Quantity: qty, Price: price})
	return b
}
func (b orderBuilder) build() Order { return b.o }

func TestValidate(t *testing.T) {
	tests := []struct {
		name  string
		order Order
		want  []error // every error that must be present; nil means valid
	}{
		{"default builder is valid", anOrder().build(), nil},
		{"missing id", anOrder().withID("").build(), []error{ErrNoID}},
		{"bad customer", anOrder().withCustomer("nobody").build(), []error{ErrBadCustomer}},
		{"no lines", anOrder().withoutLines().build(), []error{ErrNoLines}},
		{"zero quantity", anOrder().withLine("pen", 0, 100).build(), []error{ErrBadLine}},
		{"bad country", anOrder().withCountry("BRA").build(), []error{ErrBadCountry}},
		{"several problems at once",
			anOrder().withID("").withCustomer("x").withoutLines().build(),
			[]error{ErrNoID, ErrBadCustomer, ErrNoLines}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.order.Validate()
			if tt.want == nil && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			for _, want := range tt.want {
				if !errors.Is(err, want) {
					t.Errorf("Validate() = %v, want it to include %v", err, want)
				}
			}
		})
	}
}

func TestTotal(t *testing.T) {
	base := anOrder().withLine("pen", 4, 150) // 2*2500 + 4*150 = 5600

	if got := base.build().Total(); got != 5600 {
		t.Errorf("Total() = %d, want 5600", got)
	}
	if got := base.withCoupon("SAVE10").build().Total(); got != 5040 {
		t.Errorf("Total() with coupon = %d, want 5040", got)
	}
	if got := base.withCoupon("UNKNOWN").build().Total(); got != 5600 {
		t.Errorf("Total() with unknown coupon = %d, want 5600", got)
	}
}

// The builder returns copies: deriving variations from a shared base must not
// leak changes between them. This guards the builder itself.
func TestBuilderVariationsAreIndependent(t *testing.T) {
	base := anOrder()
	withPen := base.withLine("pen", 1, 100)
	withMug := base.withLine("mug", 1, 900)

	if n := len(base.build().Lines); n != 1 {
		t.Errorf("base has %d lines, want 1", n)
	}
	if got := withPen.build().Lines[1].SKU; got != "pen" {
		t.Errorf("second line of withPen = %q, want pen", got)
	}
	if got := withMug.build().Lines[1].SKU; got != "mug" {
		t.Errorf("second line of withMug = %q, want mug", got)
	}
}

// A fixture file keeps realistic, bulky data out of the test code.
func TestValidateFixtureFile(t *testing.T) {
	var o Order
	loadFixture(t, "valid_order.json", &o)

	if err := o.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	// 3*2000 + 1*3000 = 9000, minus the 10% of the SAVE10 coupon in the file.
	if got := o.Total(); got != 8100 {
		t.Errorf("Total() = %d, want 8100", got)
	}
}

// loadFixture decodes testdata/<name> into v, failing the test on any error.
func loadFixture(t *testing.T, name string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
}
