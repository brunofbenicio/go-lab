# Faking the network with `http.RoundTripper`

## What it solves

`http.Client` delegates the actual exchange to an `http.RoundTripper`. By
swapping it for a function that returns canned responses, you can test code
that makes HTTP calls (or that *is* an HTTP middleware) without a server, a
port or a network, and you can simulate failures that are hard to provoke.

## When to use

- Client code that builds `http.Client` calls and must be tested offline.
- Transport middleware: authentication, retries, logging, caching, metrics.
- Simulating timeouts, resets and sequences like "503, 503, 200".

For end-to-end checks through the real stack (TLS, cancellation), see
`httptest-server`.

## What this example shows

- `roundTripFunc`: an adapter that makes a function a `RoundTripper`.
- `scripted`: a fake network that plays a sequence of outcomes and records requests.
- `RetryTransport` tested for success after retries, giving up, network errors,
  non-retryable statuses, cancellation and body replay (`GetBody`).
- An injectable `Sleep`, so exponential backoff is verified without waiting.
- Checking the `RoundTripper` contract: the caller's request is not modified,
  and discarded response bodies are closed.

## Running

```sh
go test -race -v ./testing-examples/fake-roundtripper/
```
