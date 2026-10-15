// Package bank is a tiny in-memory ledger whose operations fail in several
// distinct ways. It is the subject for testing errors with errors.Is and errors.As.
package bank

import (
	"errors"
	"fmt"
)

// Sentinel errors: callers match them with errors.Is.
var (
	ErrAccountNotFound   = errors.New("bank: account not found")
	ErrInsufficientFunds = errors.New("bank: insufficient funds")
	ErrLimitExceeded     = errors.New("bank: withdrawal limit exceeded")
	ErrEmptyID           = errors.New("bank: empty account id")
	ErrNegativeBalance   = errors.New("bank: negative opening balance")
)

// LimitError carries the details of a withdrawal limit violation. Callers
// extract it with errors.As; errors.Is(err, ErrLimitExceeded) also matches
// because of the Is method.
type LimitError struct {
	Account   string
	Limit     int64
	Requested int64
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("bank: account %q: requested %d, limit %d", e.Account, e.Requested, e.Limit)
}

// Is makes a *LimitError equivalent to the ErrLimitExceeded sentinel.
func (e *LimitError) Is(target error) bool { return target == ErrLimitExceeded }

// Bank holds balances in cents.
type Bank struct {
	balances map[string]int64
	limit    int64 // maximum amount per withdrawal
}

// New returns a Bank that rejects single withdrawals above limit.
func New(limit int64) *Bank {
	return &Bank{balances: make(map[string]int64), limit: limit}
}

// Open creates an account. Every problem with the arguments is reported at
// once, joined with errors.Join, and each one can be matched with errors.Is.
func (b *Bank) Open(id string, opening int64) error {
	var errs []error
	if id == "" {
		errs = append(errs, ErrEmptyID)
	}
	if opening < 0 {
		errs = append(errs, ErrNegativeBalance)
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	b.balances[id] = opening
	return nil
}

// Withdraw removes amount from the account.
func (b *Bank) Withdraw(id string, amount int64) error {
	bal, ok := b.balances[id]
	if !ok {
		return fmt.Errorf("withdraw from %q: %w", id, ErrAccountNotFound)
	}
	if amount > b.limit {
		return &LimitError{Account: id, Limit: b.limit, Requested: amount}
	}
	if amount > bal {
		return fmt.Errorf("withdraw %d from %q (balance %d): %w", amount, id, bal, ErrInsufficientFunds)
	}
	b.balances[id] = bal - amount
	return nil
}

// Transfer moves amount between accounts. Errors from Withdraw are wrapped
// with context, and the original cause stays reachable through the chain.
func (b *Bank) Transfer(from, to string, amount int64) error {
	if _, ok := b.balances[to]; !ok {
		return fmt.Errorf("transfer to %q: %w", to, ErrAccountNotFound)
	}
	if err := b.Withdraw(from, amount); err != nil {
		return fmt.Errorf("transfer %s -> %s: %w", from, to, err)
	}
	b.balances[to] += amount
	return nil
}

// Balance returns the balance of id and whether the account exists.
func (b *Bank) Balance(id string) (int64, bool) {
	bal, ok := b.balances[id]
	return bal, ok
}
