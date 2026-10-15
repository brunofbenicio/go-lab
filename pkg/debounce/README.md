# debounce

Delays a function until calls stop arriving: each `Call` restarts the wait and
the function runs once, `wait` after the last call. Useful for autosave,
search-as-you-type and reloading after a burst of file events.

## Usage

```go
save := debounce.New(500*time.Millisecond, func() { persist(doc) })
save.Call() // on every keystroke
save.Flush() // run now, e.g. on shutdown
save.Cancel() // drop the pending run
```

The timer is injectable with `WithAfterFunc`, which makes tests deterministic.
A `Debouncer` is safe for concurrent use.

## Tests

```sh
go test -race ./pkg/debounce/
```
