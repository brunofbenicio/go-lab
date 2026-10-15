# Testing errors with `errors.Is` and `errors.As`

## What it solves

Comparing `err.Error()` to a string, or using `err == ErrX`, breaks as soon as
someone wraps the error with `fmt.Errorf("...: %w", err)`. `errors.Is` and
`errors.As` look through the whole chain, so tests check the *kind* of failure
and not its wording.

## When to use

- Any function that returns sentinel errors (`var ErrX = errors.New(...)`).
- Functions that return a custom error type carrying data (use `errors.As`).
- Validation that reports several problems at once (`errors.Join`).

## What this example shows

- Sentinels matched with `errors.Is` in a table-driven test.
- A `*LimitError` extracted with `errors.As` to assert on its fields.
- A custom `Is` method that makes a typed error match a sentinel.
- Wrapped errors (`%w`) keeping their cause, and `==` failing on them.
- `errors.Join` results checked with `errors.Is` for each part.

## Tips

- Declare `want` in the table as an `error` and use `errors.Is(err, tt.want)`:
  it also handles `nil` (`errors.Is(nil, nil)` is true).
- Pass `errors.As` a pointer to the target type: `var e *MyErr; errors.As(err, &e)`.

## Running

```sh
go test -v ./testing-examples/errors-is-as/
```
