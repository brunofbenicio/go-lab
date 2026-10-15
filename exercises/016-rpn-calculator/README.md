# 016 - RPN calculator

## Problem

In Reverse Polish Notation the operator comes after its operands, so no
parentheses are needed: `3 4 + 2 *` means `(3 + 4) * 2`.

`Eval(expr string) (float64, error)` evaluates a whitespace-separated
expression with a stack. It supports:

- binary operators `+ - * / % ^`
- unary operators `neg`, `sqrt` and `abs`

Report problems with distinct errors (`ErrEmpty`, `ErrDivisionByZero`,
`ErrStackUnderflow`, `ErrLeftover`, `ErrToken`).

## Examples

```go
rpn.Eval("5 1 2 + 4 * + 3 -") // 14
rpn.Eval("9 sqrt 2 neg *")    // -6
rpn.Eval("1 +")               // error: not enough operands
```

## Concepts

Stack evaluation, operator tables of functions, wrapping errors with `%w`, and
rejecting `inf`/`nan` that `strconv.ParseFloat` would otherwise accept.

## Running the tests

```sh
go test -race ./exercises/016-rpn-calculator/
```
