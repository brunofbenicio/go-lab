# Fake clock

## What it solves

Code that calls `time.Now()` directly can only be tested by sleeping, which
makes tests slow and flaky. Injecting a `Clock` lets the test decide what time
it is and move it forward instantly, so a one-hour TTL is verified in
microseconds and gives the same result on every machine.

## When to use

- Expiry, TTLs, token validity, rate limits, retention policies.
- Anything that compares "now" with a stored timestamp.
- Whenever a test would otherwise need `time.Sleep`.

## What this example shows

- A one-method `Clock` interface and a `SystemClock` for production.
- A `fakeClock` with `Advance(d)`, starting at a fixed instant.
- Boundary testing: one second before expiry, and exactly at expiry.
- A mutex in the fake, because the code under test may read it from several
  goroutines (run with `-race` to check).

## Tips

- Inject only what you need: a one-method interface, or a `func() time.Time`.
- Use a fixed start date, never `time.Now()`, inside the fake.
- For timers and tickers, abstract them too, or inject sleep/after functions.
  (Go 1.24 also has the experimental `testing/synctest`, enabled with
  `GOEXPERIMENT=synctest`.)

## Running

```sh
go test -race -v ./testing-examples/fake-clock/
```
