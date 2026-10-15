# `Example` functions: documentation that is tested

## What it solves

Code samples in documentation rot: the API changes and the sample stops
compiling or printing what it claims. `Example` functions live in `_test.go`
files, show up in godoc next to the identifier they document, and are run by
`go test`, which compares what they print with the `// Output:` comment.

## When to use

- Every exported function, type or method whose usage is not obvious.
- A package overview (`func Example()`), the first thing readers look at.
- Alongside ordinary tests: examples show the typical use, tests cover the edges.

## What this example shows

- The naming rules: `Example` (package), `ExampleParse` (function),
  `ExampleVersion_Compare` (method), `ExampleParse_invalid` (variation suffix).
- `// Output:` with one line and with several lines.
- `// Unordered output:` for results whose order is random (map iteration).
- An external test package (`semver_test`), so examples read exactly like user code.

## Tips

- An example without an `Output` comment is compiled but never run.
- Trailing whitespace is ignored when comparing output.
- `go vet` checks that example names refer to real identifiers.
- View the rendered docs with `go doc -all ./testing-examples/examples`.

## Running

```sh
go test -v -run Example ./testing-examples/examples/
```
