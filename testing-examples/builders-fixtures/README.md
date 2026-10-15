# Test builders and fixtures

## What it solves

Domain objects have many fields, but each test cares about only one or two.
Spelling out a full `Order{...}` literal in every test buries the point under
noise and makes every test break when a field is added. A *test builder*
produces a valid default object and lets the test change only what matters.

## When to use

- Structs with many fields or nested structs (orders, users, configs).
- Validation tests: "a valid order, except that X is missing".
- Bulky inputs (JSON, XML) that belong in `testdata/` as fixture files.

## What this example shows

- `anOrder()`: a builder whose default is a complete, valid order.
- `with...` methods returning copies, so variations derived from a shared base
  do not interfere (and a test proves it, including the slice-aliasing trap).
- Table-driven validation where each row is one line: `anOrder().withID("").build()`.
- A fixture file in `testdata/valid_order.json` plus a `loadFixture` helper.
  Go ignores `testdata` directories when building packages.

## Tips

- Keep builders in `_test.go` files so they never ship in the binary.
- The builder's default must itself be valid; test that explicitly (first row).
- Prefer builders to global shared fixtures: a shared object that tests mutate
  creates order dependence.
- Functional options (`newOrder(t, withCoupon("X"))`) are an equivalent style.

## Running

```sh
go test -race -v ./testing-examples/builders-fixtures/
```
