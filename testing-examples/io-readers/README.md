# Testing `io.Reader` and `io.Writer` code

## What it solves

Code that reads from files, sockets or HTTP bodies must work however the
stream delivers its data: one byte at a time, half a buffer, data together with
`io.EOF`, or an error in the middle. Testing only with `strings.NewReader` hides
these bugs. The standard library has everything needed to simulate them.

## When to use

- Parsers and protocol decoders (length prefixes, headers, line readers).
- Wrappers and middleware around readers/writers.
- Any function that takes `io.Reader` or `io.Writer`: test it without files.

## What this example shows

- `strings.NewReader` / `bytes.NewReader` as input and `bytes.Buffer` as output.
- `testing/iotest` wrappers: `OneByteReader`, `HalfReader`, `DataErrReader`,
  `ErrReader` and `TimeoutReader`.
- Distinguishing a clean `io.EOF` from `io.ErrUnexpectedEOF` on truncated input.
- A hand-written failing `io.Writer` to cover write errors.
- `iotest.TestReader` to validate a custom `io.Reader` implementation.
- `io.Discard` when only the error matters.

## Running

```sh
go test -v ./testing-examples/io-readers/
```
