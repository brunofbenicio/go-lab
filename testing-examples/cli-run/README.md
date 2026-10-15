# Testing a CLI with `run(args, stdin, stdout, stderr)`

## What it solves

Command-line programs read `os.Args`, `os.Stdin`, `os.Stdout`, `os.Stderr` and
call `os.Exit`: global state that is awkward to test (you would have to build
the binary and spawn a process). The fix is a small refactoring: `main` becomes
a one-liner that passes the real streams to `run`, and `run` returns the exit
code instead of exiting.

```go
func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
```

## When to use

- Any `package main` with real logic: flag parsing, I/O, exit codes.
- To assert on all three outputs separately (stdout, stderr, exit status).

## What this example shows

- `minigrep`, a small `grep` with `-i`, `-n` and `-c`.
- A private `flag.NewFlagSet` with `ContinueOnError`, writing to the injected
  `stderr`, so each run is independent and `-h` or a bad flag does not exit the process.
- A test helper `execute` that feeds `strings.NewReader` as stdin and captures
  `bytes.Buffer`s for stdout and stderr.
- Tables for success cases and for usage errors; real files created in `t.TempDir()`.
- Exit codes as part of the contract: 0 match, 1 no match, 2 error.

## Tips

- Pass `io.Writer`/`io.Reader`, not `*os.File`, so tests can inject buffers.
- Keep `main` so small that it needs no test.
- Do not use the global `flag.CommandLine` in testable code; it keeps state
  between calls and panics when a flag is defined twice.

## Running

```sh
go test -race -v ./testing-examples/cli-run/
printf 'one\ntwo\nthree\n' | go run ./testing-examples/cli-run -n t   # try it by hand
```
