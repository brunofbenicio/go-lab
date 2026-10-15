# Hand-written test doubles: stub, fake and spy

## What it solves

Code that talks to a database, a mail server or another service is slow and
hard to test. If it depends on a small *interface*, the test can pass a
replacement written by hand, with no mocking framework and no network.

## When to use

- The dependency is I/O or non-deterministic (database, SMTP, HTTP, clock).
- You need to force a failure that is hard to trigger for real.
- You want to verify that something was (or was not) called.

## The three kinds of double in this example

| Double       | What it does                                  | Here           |
| ------------ | --------------------------------------------- | -------------- |
| Stub         | returns canned answers                        | `stubStore`    |
| Fake         | works for real, but simpler (in-memory)       | `memStore`     |
| Spy / mock   | records calls so the test can assert on them  | `spyMailer`    |

## Tips

- Define the interface in the *consumer* package, as small as possible.
- Add `var _ Iface = (*double)(nil)` so the compiler checks the double.
- Prefer checking state (fake) over checking calls (spy) when both work;
  tests that verify every call are coupled to the implementation.
- `t.Context()` (Go 1.24) gives each test a context cancelled when it ends.

## Running

```sh
go test -v ./testing-examples/test-doubles/
```
