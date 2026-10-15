# Testing `log/slog` output

## What it solves

Logs are part of a service's observable behavior: operators alert on them, and
a leaked password in a log line is a security bug. Pointing the logger at a
buffer lets tests assert on levels, messages and attributes instead of
trusting that "it logs something".

## When to use

- Verifying that important events are logged at the right level.
- Guarding against sensitive data in logs.
- Testing a custom `slog.Handler` (wrappers, redactors, routers).

## What this example shows

- A logger writing JSON to a `bytes.Buffer`, parsed into `[]map[string]any`
  and compared with `reflect.DeepEqual` (key order does not matter).
- `ReplaceAttr` dropping the timestamp, the only non-deterministic field.
- Level filtering (a Debug record that must not appear at Info).
- A `RedactHandler` wrapper, checked with `strings.Contains` on the raw
  output ("the secret appears nowhere") and on the parsed groups.
- `testing/slogtest.TestHandler`: the standard conformance suite for custom handlers.
- `slog.NewTextHandler` for exact line comparisons.

## Tips

- Inject the `*slog.Logger` into the code under test; avoid `slog.Default()`.
- Never assert on the time attribute; remove it or use a handler that omits it.
- Use `t.Errorf` with the full output in the message: failing log tests are
  easier to debug when you can see everything that was written.

## Running

```sh
go test -race -v ./testing-examples/slog-output/
```
