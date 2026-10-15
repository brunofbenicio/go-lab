# money

Monetary amounts as integer cents, so there is no floating-point rounding
error. Arithmetic is overflow-checked, mixing currencies is an error, and
`Allocate` splits an amount proportionally without losing or inventing a cent.

## Usage

```go
price, _ := money.Parse("19.90", "BRL")
total, _ := price.Mul(3)               // 59.70 BRL
parts, _ := total.Allocate(1, 1, 1)    // each part sums back to the total
sum, err := total.Add(money.New(10, "USD")) // ErrCurrencyMismatch
```

Amounts always have two decimal places; `String` prints `59.70 BRL`.

## Tests

```sh
go test -race ./pkg/money/
```
