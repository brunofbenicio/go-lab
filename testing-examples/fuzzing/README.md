# Fuzzing with `testing.F`

## What it solves

Hand-written examples only cover the cases you thought of. A fuzz test states a
*property* that must hold for every input, and Go's coverage-guided fuzzer
generates and mutates inputs looking for one that breaks it. It is very good at
finding off-by-one errors, panics on odd input and resource exhaustion.

## When to use

- Parsers, decoders, encoders, validators: anything that consumes untrusted bytes.
- Round trips (`Decode(Encode(x)) == x`) and idempotent functions.
- Code that must never panic, whatever it receives.

## What this example shows

- `FuzzRoundTrip`: a round-trip property plus a bound on the output size.
- `FuzzDecode`: arbitrary, mostly invalid input must never panic, and accepted
  input must be stable when re-encoded.
- Seeds with `f.Add`, deliberately placed at the 255/256 boundary.
- A versioned regression corpus entry in `testdata/fuzz/FuzzRoundTrip/`.
- `MaxDecoded`: the kind of limit fuzzing pushes you to add (every 2 input
  bytes can expand to 255, so a large input could otherwise exhaust memory).

## How it runs

Under a plain `go test`, a fuzz target is an ordinary test that runs once per
seed (the `f.Add` values and the files in `testdata/fuzz/<FuzzName>/`). That
keeps fuzz targets in CI at almost no cost. The actual search is opt-in:

```sh
go test ./testing-examples/fuzzing/                                    # seeds only
go test -fuzz FuzzRoundTrip -fuzztime 20s ./testing-examples/fuzzing/  # real fuzzing
```

When the fuzzer finds a failure, it writes the input to
`testdata/fuzz/<FuzzName>/<hash>`. Fix the bug and commit that file: it becomes
a permanent regression test.
