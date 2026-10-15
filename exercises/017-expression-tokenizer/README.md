# 017 - Expression tokenizer

## Problem

Write the first stage of an expression parser. `Tokenize(src string)
([]Token, error)` turns text such as `max(x1, 2.5e-3) * (y + 4)` into tokens:

| Kind       | Examples                          |
|------------|-----------------------------------|
| `Number`   | `12`, `3.14`, `.5`, `1e10`, `2.5E-3` |
| `Ident`    | `x1`, `_tmp`, `area`              |
| `Operator` | `+ - * / ^ %`                     |
| `LParen` / `RParen` / `Comma` | `(` `)` `,`    |

Every token records its byte offset. Whitespace is skipped. Unexpected
characters and malformed numbers (`1.`, `2e`) produce a `*SyntaxError` with the
offending position.

## Examples

```go
tokenizer.Tokenize("1 + 2*3")
// Number("1")@0 Operator("+")@2 Number("2")@4 Operator("*")@5 Number("3")@6
```

## Concepts

Hand-written scanners, decoding UTF-8 runes for identifiers, and error
positions that make diagnostics useful.

## Running the tests

```sh
go test -race ./exercises/017-expression-tokenizer/
```
