# 018 - Concurrent bank account

## Problem

Model bank accounts that many goroutines can use at the same time.

- `NewAccount(owner, initialCents)`, `Balance`, `Deposit`, `Withdraw` guarded by
  a `sync.Mutex`. Amounts are `int64` cents.
- `Transfer(from, to, cents)` moves money atomically between two accounts.
  Two opposite transfers (`A -> B` and `B -> A`) running at once must not
  deadlock, so lock the accounts in a global order.
- Reject non-positive amounts, overdrafts, transfers to the same account and
  balance overflow with distinct errors.

## Examples

```go
a, _ := bank.NewAccount("alice", 10_00)
b, _ := bank.NewAccount("bob", 0)
err := bank.Transfer(a, b, 2_50) // alice: 7_50, bob: 2_50
```

## Concepts

Mutexes protecting shared state, lock ordering to avoid deadlocks, `sync/atomic`
counters, and tests that run under `-race`.

## Running the tests

```sh
go test -race ./exercises/018-concurrent-bank-account/
```
